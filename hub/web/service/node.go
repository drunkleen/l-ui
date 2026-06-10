package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	goruntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/drunkleen/l-ui/hub/web/runtime"
	"github.com/drunkleen/l-ui/internal/bundle"
	"github.com/drunkleen/l-ui/internal/config"
	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/nodeauth"
	"github.com/drunkleen/l-ui/internal/util/common"
	"github.com/drunkleen/l-ui/internal/util/netsafe"
	"github.com/drunkleen/l-ui/internal/util/random"
	"github.com/drunkleen/l-ui/internal/util/retry"
)

type HeartbeatPatch struct {
	Status        string
	LastHeartbeat int64
	LatencyMs     int
	DiskCurrent   uint64
	DiskTotal     uint64
	NetUp         uint64
	NetDown       uint64
	XrayVersion   string
	PanelVersion  string
	CpuPct        float64
	MemPct        float64
	UptimeSecs    uint64
	LastError     string
}

type NodeService struct {
	bootstrapMu   sync.Mutex
	bootstrapJobs map[string]*NodeBootstrapJob
}

var nodeHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     60 * time.Second,
		DialContext:         netsafe.SSRFGuardedDialContext,
	},
}

var (
	nodeSkipClient *http.Client
	nodePinMu      sync.RWMutex
	nodePinClients = map[string]*http.Client{}
)

// nodeHTTPClientFor returns the HTTP client used to reach a node, honoring its
// per-node TLS verification mode. "verify" (or any http node) uses the shared
// client with default certificate validation. "skip" disables validation.
// "pin" disables the default chain check but verifies the leaf certificate's
// SHA-256 against the stored pin, keeping MITM protection for self-signed certs.
func nodeHTTPClientFor(n *model.Node) (*http.Client, error) {
	mode := n.TlsVerifyMode
	if mode == "" {
		mode = "verify"
	}
	if mode == "verify" || n.Scheme == "http" {
		return nodeHTTPClient, nil
	}
	if mode == "skip" {
		if nodeSkipClient == nil {
			nodeSkipClient = &http.Client{
				Transport: &http.Transport{
					MaxIdleConns:        64,
					MaxIdleConnsPerHost: 4,
					IdleConnTimeout:     60 * time.Second,
					DialContext:         netsafe.SSRFGuardedDialContext,
					TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
				},
			}
		}
		return nodeSkipClient, nil
	}
	// pin mode — one client per pinned hash
	pinKey := n.PinnedCertSha256
	if pinKey == "" {
		return nil, common.NewError("pin mode requires a pinned certificate SHA-256")
	}
	nodePinMu.RLock()
	if c, ok := nodePinClients[pinKey]; ok {
		nodePinMu.RUnlock()
		return c, nil
	}
	nodePinMu.RUnlock()

	want, err := decodeCertPin(pinKey)
	if err != nil {
		return nil, err
	}
	tlsCfg := &tls.Config{InsecureSkipVerify: true}
	tlsCfg.VerifyConnection = func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return common.NewError("node presented no certificate")
		}
		sum := sha256.Sum256(cs.PeerCertificates[0].Raw)
		if subtle.ConstantTimeCompare(sum[:], want) != 1 {
			return common.NewError("node certificate does not match pinned SHA-256")
		}
		return nil
	}
	c := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        64,
			MaxIdleConnsPerHost: 4,
			IdleConnTimeout:     60 * time.Second,
			DialContext:         netsafe.SSRFGuardedDialContext,
			TLSClientConfig:     tlsCfg,
		},
	}
	nodePinMu.Lock()
	nodePinClients[pinKey] = c
	nodePinMu.Unlock()
	return c, nil
}

// decodeCertPin accepts a SHA-256 certificate hash as base64 (the format used
// by Xray's pinnedPeerCertSha256) or hex with optional colons (the openssl
// -fingerprint style) and returns the 32 raw bytes.
func decodeCertPin(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, common.NewError("certificate pin is empty")
	}
	if b, err := hex.DecodeString(strings.ReplaceAll(s, ":", "")); err == nil && len(b) == sha256.Size {
		return b, nil
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(s); err == nil && len(b) == sha256.Size {
			return b, nil
		}
	}
	return nil, common.NewError("certificate pin must be a SHA-256 hash (base64 or hex)")
}

// FetchCertFingerprint connects to the node over HTTPS without verifying the
// certificate and returns the leaf certificate's SHA-256 as base64, so the UI
// can offer a "fetch and pin current certificate" action.
func (s *NodeService) FetchCertFingerprint(ctx context.Context, n *model.Node) (string, error) {
	addr, err := netsafe.NormalizeHost(n.Address)
	if err != nil {
		return "", err
	}
	scheme := n.Scheme
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	if scheme != "https" {
		return "", common.NewError("certificate pinning is only available for https nodes")
	}
	if n.Port <= 0 || n.Port > 65535 {
		return "", common.NewError("node port must be 1-65535")
	}
	probeURL := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(addr, strconv.Itoa(n.Port)),
		Path:   common.NormalizeBasePath(n.BasePath) + "api/v1/status",
	}
	req, err := http.NewRequestWithContext(
		netsafe.ContextWithAllowPrivate(ctx, n.AllowPrivateAddress),
		http.MethodGet, probeURL.String(), nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{
		Transport: &http.Transport{
			DialContext:     netsafe.SSRFGuardedDialContext,
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // lgtm[go/disabled-certificate-check]
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return "", common.NewError("node did not present a TLS certificate")
	}
	sum := sha256.Sum256(resp.TLS.PeerCertificates[0].Raw)
	return base64.StdEncoding.EncodeToString(sum[:]), nil
}

func (s *NodeService) GetAll() ([]*model.Node, error) {
	db := database.GetDB()
	var nodes []*model.Node
	err := db.Model(model.Node{}).Order("id asc").Find(&nodes).Error
	if err != nil || len(nodes) == 0 {
		return nodes, err
	}
	for _, n := range nodes {
		n.DriftReasons = nodeDriftReasons(n)
	}

	type inboundRow struct {
		Id     int
		NodeID int `gorm:"column:node_id"`
	}
	var inboundRows []inboundRow
	if err := db.Table("inbounds").
		Select("id, node_id").
		Where("node_id IS NOT NULL").
		Scan(&inboundRows).Error; err != nil {
		return nodes, nil
	}
	if len(inboundRows) == 0 {
		return nodes, nil
	}
	inboundsByNode := make(map[int][]int, len(nodes))
	nodeByInbound := make(map[int]int, len(inboundRows))
	for _, row := range inboundRows {
		inboundsByNode[row.NodeID] = append(inboundsByNode[row.NodeID], row.Id)
		nodeByInbound[row.Id] = row.NodeID
	}

	type clientCountRow struct {
		NodeID int `gorm:"column:node_id"`
		Count  int `gorm:"column:count"`
	}
	var clientCounts []clientCountRow
	if err := db.Model(&model.Inbound{}).
		Select("inbounds.node_id AS node_id, COUNT(DISTINCT client_inbounds.client_id) AS count").
		Joins("JOIN client_inbounds ON client_inbounds.inbound_id = inbounds.id").
		Where("inbounds.node_id IS NOT NULL").
		Group("inbounds.node_id").
		Scan(&clientCounts).Error; err == nil {
		for _, row := range clientCounts {
			for _, n := range nodes {
				if n.Id == row.NodeID {
					n.ClientCount = row.Count
					break
				}
			}
		}
	}

	now := time.Now().UnixMilli()
	type trafficRow struct {
		InboundID  int `gorm:"column:inbound_id"`
		Email      string
		Enable     bool
		Total      int64
		Up         int64
		Down       int64
		ExpiryTime int64 `gorm:"column:expiry_time"`
	}
	var trafficRows []trafficRow
	inboundIDs := make([]int, 0, len(nodeByInbound))
	for id := range nodeByInbound {
		inboundIDs = append(inboundIDs, id)
	}
	if err := db.Table("client_traffics").
		Select("inbound_id, email, enable, total, up, down, expiry_time").
		Where("inbound_id IN ?", inboundIDs).
		Scan(&trafficRows).Error; err == nil {
		onlineByNodeSet := s.onlineEmailsByNode()
		depletedByNode := make(map[int]int)
		onlineByNode := make(map[int]int)
		for _, row := range trafficRows {
			nodeID, ok := nodeByInbound[row.InboundID]
			if !ok {
				continue
			}
			expired := row.ExpiryTime > 0 && row.ExpiryTime <= now
			exhausted := row.Total > 0 && row.Up+row.Down >= row.Total
			if expired || exhausted || !row.Enable {
				depletedByNode[nodeID]++
			}
			// Scope online by the node the inbound lives on: a client online
			// on one node must not count as online on another.
			if set, ok := onlineByNodeSet[nodeID]; ok {
				if _, isOnline := set[row.Email]; isOnline {
					onlineByNode[nodeID]++
				}
			}
		}
		for _, n := range nodes {
			n.InboundCount = len(inboundsByNode[n.Id])
			n.DepletedCount = depletedByNode[n.Id]
			n.OnlineCount = onlineByNode[n.Id]
		}
	}

	return nodes, nil
}

func nodeDriftReasons(n *model.Node) []string {
	reasons := make([]string, 0, 5)
	if n == nil {
		return []string{"node-missing"}
	}
	if !n.Enable {
		reasons = append(reasons, "disabled")
	}
	if n.Status != "online" {
		reasons = append(reasons, "offline")
	}
	if pv := strings.TrimSpace(n.PanelVersion); pv != "" && pv != config.GetVersion() {
		reasons = append(reasons, "panel-version-mismatch")
	}
	if strings.TrimSpace(n.XrayVersion) == "" {
		reasons = append(reasons, "xray-version-unknown")
	}
	if strings.TrimSpace(n.LastError) != "" {
		reasons = append(reasons, "last-error")
	}
	if n.ConfigVersion == 0 {
		reasons = append(reasons, "config-not-pushed")
	}
	return reasons
}

func (s *NodeService) onlineEmailsByNode() map[int]map[string]struct{} {
	svc := InboundService{}
	byNode := svc.GetOnlineClientsByNode()
	out := make(map[int]map[string]struct{}, len(byNode))
	for nodeID, emails := range byNode {
		set := make(map[string]struct{}, len(emails))
		for _, email := range emails {
			set[email] = struct{}{}
		}
		out[nodeID] = set
	}
	return out
}

func (s *NodeService) GetById(id int) (*model.Node, error) {
	db := database.GetDB()
	n := &model.Node{}
	if err := db.Model(model.Node{}).Where("id = ?", id).First(n).Error; err != nil {
		return nil, err
	}
	return n, nil
}

func (s *NodeService) ListGroups() ([]string, error) {
	db := database.GetDB()
	var groups []string
	if err := db.Model(model.Node{}).Where("group_name != ''").
		Select("DISTINCT group_name").Order("group_name asc").
		Pluck("group_name", &groups).Error; err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *NodeService) SetGroup(id int, group string) error {
	db := database.GetDB()
	return db.Model(model.Node{}).Where("id = ?", id).Update("group_name", group).Error
}

func (s *NodeService) normalize(n *model.Node) error {
	n.Name = strings.TrimSpace(n.Name)
	n.ApiToken = strings.TrimSpace(n.ApiToken)
	if n.Name == "" {
		return common.NewError("node name is required")
	}
	addr, err := netsafe.NormalizeHost(n.Address)
	if err != nil {
		return common.NewError(err.Error())
	}
	n.Address = addr
	if n.Port <= 0 || n.Port > 65535 {
		return common.NewError("node port must be 1-65535")
	}
	if n.Scheme != "http" && n.Scheme != "https" {
		n.Scheme = "https"
	}
	if n.TlsVerifyMode != "skip" && n.TlsVerifyMode != "pin" {
		n.TlsVerifyMode = "verify"
	}
	n.PinnedCertSha256 = strings.TrimSpace(n.PinnedCertSha256)
	if n.TlsVerifyMode == "pin" {
		if _, err := decodeCertPin(n.PinnedCertSha256); err != nil {
			return common.NewError(err.Error())
		}
	}
	n.BasePath = common.NormalizeBasePath(n.BasePath)
	return nil
}

func (s *NodeService) Create(n *model.Node) error {
	if err := s.normalize(n); err != nil {
		return err
	}
	db := database.GetDB()
	return db.Create(n).Error
}

func (s *NodeService) Update(id int, in *model.Node) error {
	if err := s.normalize(in); err != nil {
		return err
	}
	db := database.GetDB()
	existing := &model.Node{}
	if err := db.Where("id = ?", id).First(existing).Error; err != nil {
		return err
	}
	updates := map[string]any{
		"name":                  in.Name,
		"scheme":                in.Scheme,
		"address":               in.Address,
		"port":                  in.Port,
		"base_path":             in.BasePath,
		"api_token":             in.ApiToken,
		"enable":                in.Enable,
		"allow_private_address": in.AllowPrivateAddress,
		"tls_verify_mode":       in.TlsVerifyMode,
		"pinned_cert_sha256":    in.PinnedCertSha256,
		"group_name":            in.Group,
	}
	if err := db.Model(model.Node{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	if mgr := runtime.GetManager(); mgr != nil && existing != nil {
		if remote, err := mgr.RemoteFor(existing); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			_ = remote.RestartXray(ctx)
			cancel()
		}
		mgr.InvalidateNode(id)
	}
	return nil
}

func (s *NodeService) Delete(id int, cleanupRemote bool) error {
	existing := &model.Node{}
	db := database.GetDB()
	if err := db.Where("id = ?", id).First(existing).Error; err != nil {
		return err
	}
	if cleanupRemote && runtime.GetManager() != nil {
		if remote, err := runtime.GetManager().RemoteFor(existing); err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			cleanupErr := remote.Cleanup(ctx)
			cancel()
			if cleanupErr != nil {
				return cleanupErr
			}
		}
	}
	if err := db.Where("id = ?", id).Delete(model.Node{}).Error; err != nil {
		return err
	}
	if err := db.Where("node_id = ?", id).Delete(&model.NodeClientTraffic{}).Error; err != nil {
		return err
	}
	if mgr := runtime.GetManager(); mgr != nil {
		mgr.InvalidateNode(id)
	}
	for _, k := range NodeMetricKeys {
		nodeMetrics.drop(nodeMetricKey(id, k))
	}
	return nil
}

func (s *NodeService) SetEnable(id int, enable bool) error {
	db := database.GetDB()
	return db.Model(model.Node{}).Where("id = ?", id).Update("enable", enable).Error
}

// GetWebCertFiles asks a node for its own web TLS certificate/key file paths,
// used by "Set Cert from Panel" so a node-assigned inbound gets paths that
// exist on the node rather than the central panel. See issue #4854.
func (s *NodeService) GetWebCertFiles(id int) (*runtime.WebCertFiles, error) {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nil, fmt.Errorf("node not found")
	}
	if !n.Enable {
		return nil, fmt.Errorf("node is disabled")
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nil, fmt.Errorf("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return remote.GetWebCertFiles(ctx)
}

// NodeUpdateResult reports the outcome of triggering a panel self-update on one
// node so the UI can show per-node success/failure for a bulk request.
type NodeUpdateResult struct {
	Id    int    `json:"id"`
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (s *NodeService) Reinstall(id int) error {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nodeNotFoundErr()
	}
	if !n.Enable {
		return nodeDisabledErr()
	}
	arch := goruntime.GOARCH
	if strings.TrimSpace(n.BundleSHA256) != "" {
		var nodeBundle model.NodeBundle
		if err := database.GetDB().Where("sha256 = ?", n.BundleSHA256).First(&nodeBundle).Error; err == nil && strings.TrimSpace(nodeBundle.Arch) != "" {
			arch = nodeBundle.Arch
		}
	}
	bnd, err := bundle.BuildNodeBundle(arch)
	if err != nil {
		return err
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := remote.ReinstallBundle(ctx, bnd.Path); err != nil {
		return nodeServiceErr(err.Error())
	}
	if err := database.GetDB().Model(model.Node{}).Where("id = ?", id).Update("bundle_sha256", bnd.SHA256).Error; err != nil {
		return err
	}
	return nil
}

func (s *NodeService) RotateCredentials(id int) error {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nodeNotFoundErr()
	}
	if !n.Enable {
		return nodeDisabledErr()
	}
	newToken := random.Seq(48)
	mgr := runtime.GetManager()
	if mgr == nil {
		return nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := remote.RotateToken(ctx, newToken); err != nil {
		return nodeServiceErr(err.Error())
	}
	if err := database.GetDB().Model(model.Node{}).Where("id = ?", id).Update("api_token", newToken).Error; err != nil {
		return err
	}
	if mgr != nil {
		mgr.InvalidateNode(id)
	}
	return nil
}

func (s *NodeService) Reconcile(id int) error {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nodeNotFoundErr()
	}
	probeAndPersist := func(timeout time.Duration) (*HeartbeatPatch, error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		patch, probeErr := s.Probe(ctx, n)
		if probeErr != nil {
			patch.Status = "offline"
		} else {
			patch.Status = "online"
		}
		if err := s.UpdateHeartbeat(id, patch); err != nil {
			return nil, nodeServiceErr(err.Error())
		}
		return &patch, probeErr
	}

	if _, probeErr := probeAndPersist(10 * time.Second); probeErr == nil {
		return nil
	}

	mgr := runtime.GetManager()
	if mgr == nil {
		return nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return err
	}

	ctxRestart, cancelRestart := context.WithTimeout(context.Background(), 20*time.Second)
	_ = remote.RestartXray(ctxRestart)
	cancelRestart()
	if _, probeErr := probeAndPersist(10 * time.Second); probeErr == nil {
		return nil
	}

	if err := s.Reinstall(id); err != nil {
		return nodeServiceErr(err.Error())
	}
	if _, probeErr := probeAndPersist(10 * time.Second); probeErr != nil {
		return nodeServiceErr(probeErr.Error())
	}
	return nil
}

func (s *NodeService) ListFirewallRules(id int) (*runtime.UfwStatus, error) {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nil, nodeNotFoundErr()
	}
	if !n.Enable {
		return nil, nodeDisabledErr()
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nil, nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	status, err := remote.ListUfwRules(ctx)
	if err != nil {
		return nil, nodeServiceErr(err.Error())
	}
	return status, nil
}

func (s *NodeService) AllowFirewallPort(id int, port int, protocol string) error {
	return s.applyFirewallPortRule(id, port, protocol, "allow")
}

func (s *NodeService) DenyFirewallPort(id int, port int, protocol string) error {
	return s.applyFirewallPortRule(id, port, protocol, "deny")
}

func (s *NodeService) applyFirewallPortRule(id int, port int, protocol string, action string) error {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nodeNotFoundErr()
	}
	if !n.Enable {
		return nodeDisabledErr()
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var opErr error
	switch action {
	case "allow":
		opErr = remote.AllowUfwPort(ctx, port, protocol)
	case "deny":
		opErr = remote.DenyUfwPort(ctx, port, protocol)
	default:
		return nodeServiceErr("invalid firewall action")
	}
	if opErr != nil {
		return nodeServiceErr(opErr.Error())
	}
	return nil
}

func (s *NodeService) EnableNodeFirewall(id int) error {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nodeNotFoundErr()
	}
	if !n.Enable {
		return nodeDisabledErr()
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return remote.EnableUfw(ctx)
}

func (s *NodeService) DisableNodeFirewall(id int) error {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nodeNotFoundErr()
	}
	if !n.Enable {
		return nodeDisabledErr()
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return remote.DisableUfw(ctx)
}

func (s *NodeService) DeleteFirewallRule(id int, ruleNumber string) error {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nodeNotFoundErr()
	}
	if !n.Enable {
		return nodeDisabledErr()
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := remote.DeleteUfwRule(ctx, ruleNumber); err != nil {
		return nodeServiceErr(err.Error())
	}
	return nil
}

func (s *NodeService) FetchLogs(id int, lines int) (string, error) {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return "", nodeNotFoundErr()
	}
	if !n.Enable {
		return "", nodeDisabledErr()
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return "", nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	logs, err := remote.FetchLogs(ctx, lines)
	if err != nil {
		return "", nodeServiceErr(err.Error())
	}
	return logs, nil
}

func (s *NodeService) RestartAgent(id int) error {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nodeNotFoundErr()
	}
	if !n.Enable {
		return nodeDisabledErr()
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := remote.RestartAgent(ctx); err != nil {
		return nodeServiceErr(err.Error())
	}
	return nil
}

func (s *NodeService) RestartXray(id int) error {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nodeNotFoundErr()
	}
	if !n.Enable {
		return nodeDisabledErr()
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := remote.RestartXray(ctx); err != nil {
		return nodeServiceErr(err.Error())
	}
	return nil
}

func (s *NodeService) PushNodeConfig(id int, hubNodeID, hubEndpoint string, xrayConfig, clientList json.RawMessage) (int, error) {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return 0, nodeNotFoundErr()
	}
	if !n.Enable {
		return 0, nodeDisabledErr()
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return 0, nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	configVersion, err := remote.PushNodeConfig(ctx, hubNodeID, hubEndpoint, xrayConfig, clientList)
	if err != nil {
		return 0, nodeServiceErr(err.Error())
	}
	if configVersion > 0 {
		db := database.GetDB()
		_ = db.Model(model.Node{}).Where("id = ?", id).Update("config_version", configVersion).Error
	}
	return configVersion, nil
}

func (s *NodeService) ApplyNodeConfig(id int, xrayConfig json.RawMessage) error {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nodeNotFoundErr()
	}
	if !n.Enable {
		return nodeDisabledErr()
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := remote.ApplyNodeConfig(ctx, xrayConfig); err != nil {
		return nodeServiceErr(err.Error())
	}
	return nil
}

func (s *NodeService) GetNodeConfigVersion(id int) (int, error) {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return 0, nodeNotFoundErr()
	}
	if !n.Enable {
		return 0, nodeDisabledErr()
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return 0, nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	version, err := remote.GetRemoteConfigVersion(ctx)
	if err != nil {
		return 0, err
	}
	if version > 0 {
		db := database.GetDB()
		_ = db.Model(model.Node{}).Where("id = ?", id).Update("config_version", version).Error
	}
	return version, nil
}

func (s *NodeService) UpdateXray(id int, version string) error {
	n, err := s.GetById(id)
	if err != nil || n == nil {
		return nodeNotFoundErr()
	}
	if !n.Enable {
		return nodeDisabledErr()
	}
	mgr := runtime.GetManager()
	if mgr == nil {
		return nodeServiceErr("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(n)
	if err != nil {
		return err
	}
	ctxStatus, cancelStatus := context.WithTimeout(context.Background(), 10*time.Second)
	status, statusErr := remote.GetStatus(ctxStatus)
	cancelStatus()
	if statusErr == nil && status != nil && strings.TrimSpace(status.Xray.Version) == strings.TrimSpace(version) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if err := remote.InstallXray(ctx, version); err != nil {
		return nodeServiceErr(err.Error())
	}
	statusCtx, statusCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer statusCancel()
	st, err := remote.GetStatus(statusCtx)
	if err != nil {
		return nodeServiceErr(err.Error())
	}
	if strings.TrimSpace(version) != "latest" && strings.TrimSpace(st.Xray.Version) != strings.TrimSpace(version) {
		return nodeVersionErr("requested xray version was not installed")
	}
	return nil
}

// UpdatePanels triggers the official self-updater on each given node. Only
// enabled, online nodes are eligible — an offline node can't be reached, so it
// is reported as skipped rather than silently dropped.
func (s *NodeService) UpdatePanels(ids []int) ([]NodeUpdateResult, error) {
	mgr := runtime.GetManager()
	if mgr == nil {
		return nil, fmt.Errorf("runtime manager unavailable")
	}
	results := make([]NodeUpdateResult, 0, len(ids))
	for _, id := range ids {
		n, err := s.GetById(id)
		if err != nil || n == nil {
			results = append(results, NodeUpdateResult{Id: id, OK: false, Error: "node not found"})
			continue
		}
		res := NodeUpdateResult{Id: id, Name: n.Name}
		switch {
		case !n.Enable:
			res.Error = "node is disabled"
		case n.Status != "online":
			res.Error = "node is offline"
		default:
			remote, remoteErr := mgr.RemoteFor(n)
			if remoteErr != nil {
				res.Error = remoteErr.Error()
				break
			}
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			updErr := remote.UpdatePanel(ctx)
			cancel()
			if updErr != nil {
				res.Error = updErr.Error()
			} else {
				res.OK = true
			}
		}
		results = append(results, res)
	}
	return results, nil
}

func (s *NodeService) UpdateHeartbeat(id int, p HeartbeatPatch) error {
	db := database.GetDB()
	updates := map[string]any{
		"status":         p.Status,
		"last_heartbeat": p.LastHeartbeat,
		"latency_ms":     p.LatencyMs,
		"disk_current":   p.DiskCurrent,
		"disk_total":     p.DiskTotal,
		"net_up":         p.NetUp,
		"net_down":       p.NetDown,
		"xray_version":   p.XrayVersion,
		"panel_version":  p.PanelVersion,
		"cpu_pct":        p.CpuPct,
		"mem_pct":        p.MemPct,
		"uptime_secs":    p.UptimeSecs,
		"last_error":     p.LastError,
	}
	if err := db.Model(model.Node{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}
	if p.Status == "online" {
		now := time.Unix(p.LastHeartbeat, 0)
		nodeMetrics.append(nodeMetricKey(id, "cpu"), now, p.CpuPct)
		nodeMetrics.append(nodeMetricKey(id, "mem"), now, p.MemPct)
		nodeMetrics.append(nodeMetricKey(id, "netUp"), now, float64(p.NetUp))
		nodeMetrics.append(nodeMetricKey(id, "netDown"), now, float64(p.NetDown))
		diskPct := 0.0
		if p.DiskTotal > 0 {
			diskPct = float64(p.DiskCurrent) * 100.0 / float64(p.DiskTotal)
		}
		nodeMetrics.append(nodeMetricKey(id, "diskUsage"), now, diskPct)
	}
	return nil
}

func nodeMetricKey(id int, metric string) string {
	return "node:" + strconv.Itoa(id) + ":" + metric
}

func (s *NodeService) AggregateNodeMetric(id int, metric string, bucketSeconds int, maxPoints int) []map[string]any {
	return nodeMetrics.aggregate(nodeMetricKey(id, metric), bucketSeconds, maxPoints)
}

func (s *NodeService) Probe(ctx context.Context, n *model.Node) (HeartbeatPatch, error) {
	patch := HeartbeatPatch{LastHeartbeat: time.Now().Unix()}

	addr, err := netsafe.NormalizeHost(n.Address)
	if err != nil {
		patch.LastError = err.Error()
		return patch, err
	}
	scheme := n.Scheme
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	if n.Port <= 0 || n.Port > 65535 {
		patch.LastError = "node port must be 1-65535"
		return patch, errors.New(patch.LastError)
	}
	probeURL := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(addr, strconv.Itoa(n.Port)),
		Path:   common.NormalizeBasePath(n.BasePath) + "api/v1/status",
	}

	req, err := http.NewRequestWithContext(
		netsafe.ContextWithAllowPrivate(ctx, n.AllowPrivateAddress),
		http.MethodGet, probeURL.String(), nil)
	if err != nil {
		patch.LastError = err.Error()
		return patch, err
	}
	if n.ApiToken != "" {
		timestamp := time.Now().Unix()
		nonce := random.Seq(24)
		req.Header.Set("Authorization", "Bearer "+n.ApiToken)
		req.Header.Set(nodeauth.HeaderTimestamp, strconv.FormatInt(timestamp, 10))
		req.Header.Set(nodeauth.HeaderNonce, nonce)
		req.Header.Set(nodeauth.HeaderSignature, nodeauth.Sign(n.ApiToken, http.MethodGet, probeURL.Path, nil, timestamp, nonce))
	}
	req.Header.Set("Accept", "application/json")

	client, err := nodeHTTPClientFor(n)
	if err != nil {
		patch.LastError = err.Error()
		return patch, err
	}

	var resp *http.Response
	var lastStart time.Time
	if err := retry.Do(ctx, retry.Config{
		MaxAttempts:    3,
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     2 * time.Second,
		JitterFactor:   0.2,
	}, func(ctx context.Context) error {
		if resp != nil {
			resp.Body.Close()
		}
		var doErr error
		lastStart = time.Now()
		resp, doErr = client.Do(req)
		if doErr != nil {
			return doErr
		}
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("HTTP %d from remote panel", resp.StatusCode)
		}
		return nil
	}); err != nil {
		patch.LastError = err.Error()
		return patch, err
	}
	defer resp.Body.Close()
	patch.LatencyMs = int(time.Since(lastStart) / time.Millisecond)

	var envelope struct {
		Success bool   `json:"success"`
		Msg     string `json:"msg"`
		Obj     *struct {
			CpuPct float64 `json:"cpu"`
			Mem    struct {
				Current uint64 `json:"current"`
				Total   uint64 `json:"total"`
			} `json:"mem"`
			Disk struct {
				Current uint64 `json:"current"`
				Total   uint64 `json:"total"`
			} `json:"disk"`
			NetIO struct {
				Up   uint64 `json:"up"`
				Down uint64 `json:"down"`
			} `json:"netIO"`
			Xray struct {
				Version string `json:"version"`
			} `json:"xray"`
			PanelVersion string `json:"panelVersion"`
			Uptime       uint64 `json:"uptime"`
		} `json:"obj"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		patch.LastError = "decode response: " + err.Error()
		return patch, err
	}
	if !envelope.Success || envelope.Obj == nil {
		patch.LastError = "remote returned success=false: " + envelope.Msg
		return patch, errors.New(patch.LastError)
	}
	o := envelope.Obj
	patch.CpuPct = o.CpuPct
	if o.Mem.Total > 0 {
		patch.MemPct = float64(o.Mem.Current) * 100.0 / float64(o.Mem.Total)
	}
	patch.DiskCurrent = o.Disk.Current
	patch.DiskTotal = o.Disk.Total
	patch.NetUp = o.NetIO.Up
	patch.NetDown = o.NetIO.Down
	patch.XrayVersion = o.Xray.Version
	patch.PanelVersion = o.PanelVersion
	patch.UptimeSecs = o.Uptime
	return patch, nil
}

type ProbeResultUI struct {
	Status       string  `json:"status"`
	LatencyMs    int     `json:"latencyMs"`
	XrayVersion  string  `json:"xrayVersion"`
	PanelVersion string  `json:"panelVersion"`
	CpuPct       float64 `json:"cpuPct"`
	MemPct       float64 `json:"memPct"`
	UptimeSecs   uint64  `json:"uptimeSecs"`
	Error        string  `json:"error"`
}

func (p HeartbeatPatch) ToUI(ok bool) ProbeResultUI {
	r := ProbeResultUI{
		LatencyMs:    p.LatencyMs,
		XrayVersion:  p.XrayVersion,
		PanelVersion: p.PanelVersion,
		CpuPct:       p.CpuPct,
		MemPct:       p.MemPct,
		UptimeSecs:   p.UptimeSecs,
		Error:        FriendlyProbeError(p.LastError),
	}
	if ok {
		r.Status = "online"
	} else {
		r.Status = "offline"
	}
	return r
}

func FriendlyProbeError(msg string) string {
	if strings.Contains(msg, "server gave HTTP response to HTTPS client") {
		return "the server speaks HTTP, not HTTPS; set the node scheme to http"
	}
	return msg
}

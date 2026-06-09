package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/drunkleen/l-ui/internal/nodeauth"
	"github.com/drunkleen/l-ui/internal/util/netsafe"
	"github.com/drunkleen/l-ui/internal/util/random"
	"github.com/drunkleen/l-ui/internal/util/retry"
)

const remoteHTTPTimeout = 10 * time.Second

var remoteHTTPClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        64,
		MaxIdleConnsPerHost: 4,
		IdleConnTimeout:     60 * time.Second,
		DialContext:         netsafe.SSRFGuardedDialContext,
	},
}

type envelope struct {
	Success bool            `json:"success"`
	Msg     string          `json:"msg"`
	Obj     json.RawMessage `json:"obj"`
}

type RemoteStatus struct {
	Xray struct {
		Version string `json:"version"`
	} `json:"xray"`
}

type Remote struct {
	node *model.Node

	mu            sync.RWMutex
	remoteIDByTag map[string]int
}

func NewRemote(n *model.Node) *Remote {
	return &Remote{
		node:          n,
		remoteIDByTag: make(map[string]int),
	}
}

func (r *Remote) Name() string { return "node:" + r.node.Name }

func (r *Remote) baseURL() (string, error) {
	addr, err := netsafe.NormalizeHost(r.node.Address)
	if err != nil {
		return "", err
	}
	scheme := r.node.Scheme
	if scheme != "http" && scheme != "https" {
		scheme = "https"
	}
	if r.node.Port <= 0 || r.node.Port > 65535 {
		return "", fmt.Errorf("invalid node port %d", r.node.Port)
	}
	bp := r.node.BasePath
	if bp == "" {
		bp = "/"
	}
	if !strings.HasSuffix(bp, "/") {
		bp += "/"
	}
	u := &url.URL{
		Scheme: scheme,
		Host:   net.JoinHostPort(addr, strconv.Itoa(r.node.Port)),
		Path:   bp,
	}
	return u.String(), nil
}

func (r *Remote) do(ctx context.Context, method, path string, body any) (*envelope, error) {
	if r.node.ApiToken == "" {
		return nil, errors.New("node has no API token configured")
	}

	base, err := r.baseURL()
	if err != nil {
		return nil, err
	}
	target := base + strings.TrimPrefix(path, "/")

	var (
		reqBody     io.Reader
		contentType string
	)
	switch b := body.(type) {
	case nil:
	case url.Values:
		reqBody = strings.NewReader(b.Encode())
		contentType = "application/x-www-form-urlencoded"
	default:
		buf, jerr := json.Marshal(b)
		if jerr != nil {
			return nil, fmt.Errorf("marshal body: %w", jerr)
		}
		reqBody = bytes.NewReader(buf)
		contentType = "application/json"
	}

	cctx, cancel := context.WithTimeout(netsafe.ContextWithAllowPrivate(ctx, r.node.AllowPrivateAddress), remoteHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, method, target, reqBody)
	if err != nil {
		return nil, err
	}
	timestamp := time.Now().Unix()
	nonce := random.Seq(24)
	bodyDigest := nodeauth.BodyDigest(bodyBytes(body))
	req.Header.Set("Authorization", "Bearer "+r.node.ApiToken)
	req.Header.Set(nodeauth.HeaderTimestamp, strconv.FormatInt(timestamp, 10))
	req.Header.Set(nodeauth.HeaderNonce, nonce)
	req.Header.Set(nodeauth.HeaderSignature, nodeauth.Sign(r.node.ApiToken, method, req.URL.Path, bodyBytes(body), timestamp, nonce))
	req.Header.Set(nodeauth.HeaderBodyDigest, bodyDigest)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := remoteHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s %s: HTTP %d", method, path, resp.StatusCode)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode envelope: %w", err)
	}
	if !env.Success {
		return &env, fmt.Errorf("remote: %s", env.Msg)
	}
	return &env, nil
}

func (r *Remote) doWithRetry(ctx context.Context, method, path string, body any) (*envelope, error) {
	var env *envelope
	err := retry.Do(ctx, retry.DefaultConfig, func(ctx context.Context) error {
		var innerErr error
		env, innerErr = r.do(ctx, method, path, body)
		return innerErr
	})
	if err != nil {
		return nil, err
	}
	return env, nil
}

func bodyBytes(body any) []byte {
	switch b := body.(type) {
	case nil:
		return nil
	case url.Values:
		return []byte(b.Encode())
	default:
		buf, _ := json.Marshal(b)
		return buf
	}
}

// decodeObj unmarshals env.Obj into T. Returns zero T and an error if
// env is nil, Obj is nil/empty, or unmarshal fails.
func decodeObj[T any](env *envelope) (T, error) {
	var zero T
	if env == nil || len(env.Obj) == 0 {
		return zero, nil
	}
	var v T
	if err := json.Unmarshal(env.Obj, &v); err != nil {
		return zero, fmt.Errorf("decode response: %w", err)
	}
	return v, nil
}

func (r *Remote) resolveRemoteID(ctx context.Context, tag string) (int, error) {
	if id, ok := r.cacheGetTag(tag); ok {
		return id, nil
	}
	if err := r.refreshRemoteIDs(ctx); err != nil {
		return 0, err
	}
	if id, ok := r.cacheGetTag(tag); ok {
		return id, nil
	}
	return 0, fmt.Errorf("remote inbound with tag %q not found on node %s", tag, r.node.Name)
}

// cacheGetTag looks up a remote inbound id by tag, tolerating an n<id>- prefix
// that lives on only one of the two panels: the node may carry the bare tag
// while the central panel stores the prefixed form, or vice versa.
func (r *Remote) cacheGetTag(tag string) (int, bool) {
	if id, ok := r.cacheGet(tag); ok {
		return id, true
	}
	prefix := fmt.Sprintf("n%d-", r.node.Id)
	if stripped, found := strings.CutPrefix(tag, prefix); found {
		return r.cacheGet(stripped)
	}
	return r.cacheGet(prefix + tag)
}

func (r *Remote) cacheGet(tag string) (int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.remoteIDByTag[tag]
	return id, ok
}

func (r *Remote) cacheSet(tag string, id int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.remoteIDByTag[tag] = id
}

func (r *Remote) cacheDel(tag string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.remoteIDByTag, tag)
}

func (r *Remote) refreshRemoteIDs(ctx context.Context) error {
	env, err := r.doWithRetry(ctx, http.MethodGet, "api/v1/inbounds/list", nil)
	if err != nil {
		return err
	}
	type inboundRef struct {
		Id  int    `json:"id"`
		Tag string `json:"tag"`
	}
	list, err := decodeObj[[]inboundRef](env)
	if err != nil {
		return fmt.Errorf("decode inbound list: %w", err)
	}
	next := make(map[string]int, len(list))
	for _, ib := range list {
		if ib.Tag == "" {
			continue
		}
		next[ib.Tag] = ib.Id
	}
	r.mu.Lock()
	r.remoteIDByTag = next
	r.mu.Unlock()
	return nil
}

func (r *Remote) AddInbound(ctx context.Context, ib *model.Inbound) error {
	payload := wireInbound(ib)
	env, err := r.do(ctx, http.MethodPost, "api/v1/inbounds/add", payload)
	if err != nil {
		return err
	}
	var created struct {
		Id  int    `json:"id"`
		Tag string `json:"tag"`
	}
	if len(env.Obj) > 0 {
		if err := json.Unmarshal(env.Obj, &created); err == nil && created.Id > 0 && created.Tag != "" {
			r.cacheSet(created.Tag, created.Id)
		}
	}
	return nil
}

func (r *Remote) DelInbound(ctx context.Context, ib *model.Inbound) error {
	id, err := r.resolveRemoteID(ctx, ib.Tag)
	if err != nil {
		logger.Warning("remote DelInbound: tag", ib.Tag, "not found on", r.node.Name)
		return nil
	}
	if _, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/inbounds/del/"+strconv.Itoa(id), nil); err != nil {
		return err
	}
	r.cacheDel(ib.Tag)
	return nil
}

func (r *Remote) UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error {
	id, err := r.resolveRemoteID(ctx, oldIb.Tag)
	if err != nil {
		return r.AddInbound(ctx, newIb)
	}
	payload := wireInbound(newIb)
	if _, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/inbounds/update/"+strconv.Itoa(id), payload); err != nil {
		return err
	}
	if oldIb.Tag != newIb.Tag {
		r.cacheDel(oldIb.Tag)
	}
	r.cacheSet(newIb.Tag, id)
	return nil
}

func (r *Remote) AddUser(ctx context.Context, ib *model.Inbound, _ map[string]any) error {
	return r.UpdateInbound(ctx, ib, ib)
}

func (r *Remote) RemoveUser(ctx context.Context, ib *model.Inbound, _ string) error {
	return r.UpdateInbound(ctx, ib, ib)
}

func (r *Remote) AddClient(ctx context.Context, ib *model.Inbound, client model.Client) error {
	id, err := r.resolveRemoteID(ctx, ib.Tag)
	if err != nil {
		return fmt.Errorf("remote AddClient: resolve tag %q: %w", ib.Tag, err)
	}
	payload := map[string]any{
		"client":     client,
		"inboundIds": []int{id},
	}
	if _, err := r.do(ctx, http.MethodPost, "api/v1/clients/add", payload); err != nil {
		return err
	}
	return nil
}

// DeleteUser is idempotent: master's per-inbound Delete loop may call it
// multiple times for the same node, and "not found" on the follow-ups is
// the expected success path.
func (r *Remote) DeleteUser(ctx context.Context, _ *model.Inbound, email string) error {
	if email == "" {
		return nil
	}
	_, err := r.doWithRetry(ctx, http.MethodPost,
		"api/v1/clients/del/"+url.PathEscape(email), nil)
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "not found") {
		return nil
	}
	return err
}

func (r *Remote) UpdateUser(ctx context.Context, _ *model.Inbound, oldEmail string, payload model.Client) error {
	if oldEmail == "" {
		oldEmail = payload.Email
	}
	if _, err := r.doWithRetry(ctx, http.MethodPost,
		"api/v1/clients/update/"+url.PathEscape(oldEmail), payload); err != nil {
		return err
	}
	return nil
}

func (r *Remote) RestartXray(ctx context.Context) error {
	_, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/xray/restart", nil)
	return err
}

// RestartAgent asks the remote node to restart its agent process. The node
// returns a 200 on success and then the connection drops; the hub treats a
// connection error as an expected outcome after the restart is initiated.
func (r *Remote) RestartAgent(ctx context.Context) error {
	_, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/restart", nil)
	return err
}

// FetchLogs retrieves log lines from the remote node.  lines <= 0 returns a
// default number of lines (50).
func (r *Remote) FetchLogs(ctx context.Context, lines int) (string, error) {
	path := "api/v1/logs"
	if lines > 0 {
		path += "?lines=" + strconv.Itoa(lines)
	}
	env, err := r.doWithRetry(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", err
	}
	logLines, err := decodeObj[string](env)
	if err != nil {
		return "", fmt.Errorf("decode logs: %w", err)
	}
	return logLines, nil
}

// UpdatePanel asks the node to run its own official self-updater (update.sh)
// and restart onto the latest release. The node returns as soon as the job is
// launched; the new version surfaces on the next heartbeat.
func (r *Remote) UpdatePanel(ctx context.Context) error {
	_, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/server/updatePanel", nil)
	return err
}

func (r *Remote) Cleanup(ctx context.Context) error {
	_, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/server/cleanup", nil)
	return err
}

func (r *Remote) GetStatus(ctx context.Context) (*RemoteStatus, error) {
	env, err := r.doWithRetry(ctx, http.MethodGet, "api/v1/status", nil)
	if err != nil {
		return nil, err
	}
	st, err := decodeObj[RemoteStatus](env)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (r *Remote) RotateToken(ctx context.Context, token string) error {
	_, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/server/rotateToken", map[string]string{"token": token})
	return err
}

func (r *Remote) ListUfwRules(ctx context.Context) (*UfwStatus, error) {
	env, err := r.doWithRetry(ctx, http.MethodGet, "api/v1/firewall/status", nil)
	if err != nil {
		return nil, err
	}
	agent, err := decodeObj[agentFirewallStatus](env)
	if err != nil {
		return nil, err
	}
	status := &UfwStatus{Active: agent.Active, Installed: agent.Installed, Rules: make([]UfwRule, len(agent.Rules))}
	for i, ar := range agent.Rules {
		status.Rules[i] = UfwRule{
			Number:   i + 1,
			Port:     ar.Port,
			Protocol: ar.Protocol,
			Action:   ar.Action,
			Comment:  ar.Comment,
		}
	}
	return status, nil
}

func (r *Remote) AllowUfwPort(ctx context.Context, port int, protocol string) error {
	_, err := r.do(ctx, http.MethodPost, "api/v1/firewall/rules",
		map[string]any{"port": port, "protocol": protocol, "action": "allow"})
	return err
}

func (r *Remote) DenyUfwPort(ctx context.Context, port int, protocol string) error {
	_, err := r.do(ctx, http.MethodPost, "api/v1/firewall/rules",
		map[string]any{"port": port, "protocol": protocol, "action": "deny"})
	return err
}

func (r *Remote) DeleteUfwRule(ctx context.Context, ruleNumber string) error {
	_, err := r.doWithRetry(ctx, http.MethodDelete, "api/v1/firewall/rules",
		map[string]string{"rule_number": ruleNumber})
	return err
}

func (r *Remote) EnableUfw(ctx context.Context) error {
	_, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/firewall/enable", nil)
	return err
}

func (r *Remote) DisableUfw(ctx context.Context) error {
	_, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/firewall/disable", nil)
	return err
}

type configVersionResult struct {
	ConfigVersion int `json:"config_version"`
}

func (r *Remote) PushNodeConfig(ctx context.Context, hubNodeID, hubEndpoint string, xrayConfig, clientList json.RawMessage) (int, error) {
	env, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/config/push",
		map[string]any{
			"hub_node_id":  hubNodeID,
			"hub_endpoint": hubEndpoint,
			"xray_config":  xrayConfig,
			"client_list":  clientList,
		})
	if err != nil {
		return 0, err
	}
	result, _ := decodeObj[configVersionResult](env)
	return result.ConfigVersion, nil
}

func (r *Remote) GetRemoteConfigVersion(ctx context.Context) (int, error) {
	env, err := r.doWithRetry(ctx, http.MethodGet, "api/v1/config", nil)
	if err != nil {
		return 0, err
	}
	result, _ := decodeObj[configVersionResult](env)
	return result.ConfigVersion, nil
}

func (r *Remote) ReinstallBundle(ctx context.Context, bundlePath string) error {
	file, err := os.Open(bundlePath)
	if err != nil {
		return err
	}
	defer file.Close()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("bundle", filepath.Base(bundlePath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, file); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	_, err = r.doRaw(ctx, http.MethodPost, "api/v1/server/reinstall", body.Bytes(), &body, writer.FormDataContentType())
	return err
}

func (r *Remote) ApplyNodeConfig(ctx context.Context, xrayConfig json.RawMessage) error {
	_, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/config/apply",
		map[string]any{"xray_config": xrayConfig})
	return err
}

func (r *Remote) InstallXray(ctx context.Context, version string) error {
	_, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/xray/install",
		map[string]string{"version": version})
	return err
}

func (r *Remote) GetXrayStatus(ctx context.Context) (running bool, version string, err error) {
	env, err := r.doWithRetry(ctx, http.MethodGet, "api/v1/xray/status", nil)
	if err != nil {
		return false, "", err
	}
	type xrayStatus struct {
		Running bool   `json:"running"`
		Version string `json:"version"`
	}
	result, err := decodeObj[xrayStatus](env)
	if err != nil {
		return false, "", err
	}
	return result.Running, result.Version, nil
}

func (r *Remote) doRaw(ctx context.Context, method, path string, bodyBytes []byte, body io.Reader, contentType string) (*envelope, error) {
	base, err := r.baseURL()
	if err != nil {
		return nil, err
	}
	target := base + strings.TrimPrefix(path, "/")
	cctx, cancel := context.WithTimeout(netsafe.ContextWithAllowPrivate(ctx, r.node.AllowPrivateAddress), remoteHTTPTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, method, target, body)
	if err != nil {
		return nil, err
	}
	timestamp := time.Now().Unix()
	nonce := random.Seq(24)
	bodyDigest := nodeauth.BodyDigest(bodyBytes)
	req.Header.Set("Authorization", "Bearer "+r.node.ApiToken)
	req.Header.Set(nodeauth.HeaderTimestamp, strconv.FormatInt(timestamp, 10))
	req.Header.Set(nodeauth.HeaderNonce, nonce)
	req.Header.Set(nodeauth.HeaderSignature, nodeauth.Sign(r.node.ApiToken, method, req.URL.Path, bodyBytes, timestamp, nonce))
	req.Header.Set(nodeauth.HeaderBodyDigest, bodyDigest)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", contentType)
	resp, err := remoteHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("remote returned http %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var env envelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	return &env, nil
}

// WebCertFiles holds a node's own web TLS certificate and key file paths.
type WebCertFiles struct {
	WebCertFile string `json:"webCertFile"`
	WebKeyFile  string `json:"webKeyFile"`
}

// GetWebCertFiles fetches the node's own web TLS certificate/key file paths so
// the central panel can offer them as the "Set Cert from Panel" default for a
// node-assigned inbound — those paths exist on the node, the central panel's
// don't. See issue #4854.
func (r *Remote) GetWebCertFiles(ctx context.Context) (*WebCertFiles, error) {
	env, err := r.doWithRetry(ctx, http.MethodGet, "api/v1/server/getWebCertFiles", nil)
	if err != nil {
		return nil, err
	}
	files, err := decodeObj[WebCertFiles](env)
	if err != nil {
		return nil, err
	}
	return &files, nil
}

func (r *Remote) ResetClientTraffic(ctx context.Context, _ *model.Inbound, email string) error {
	_, err := r.doWithRetry(ctx, http.MethodPost,
		"api/v1/clients/resetTraffic/"+url.PathEscape(email), nil)
	return err
}

func (r *Remote) ResetAllTraffics(ctx context.Context) error {
	_, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/inbounds/resetAllTraffics", nil)
	return err
}

type TrafficSnapshot struct {
	Inbounds      []*model.Inbound
	OnlineEmails  []string
	LastOnlineMap map[string]int64
}

func (r *Remote) FetchTrafficSnapshot(ctx context.Context) (*TrafficSnapshot, error) {
	snap := &TrafficSnapshot{LastOnlineMap: map[string]int64{}}

	envList, err := r.doWithRetry(ctx, http.MethodGet, "api/v1/inbounds/list", nil)
	if err != nil {
		return nil, err
	}
	inbounds, err := decodeObj[[]*model.Inbound](envList)
	if err != nil {
		return nil, fmt.Errorf("decode inbound list: %w", err)
	}
	snap.Inbounds = inbounds

	envOnlines, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/clients/onlines", nil)
	if err != nil {
		logger.Warning("remote", r.node.Name, "onlines fetch failed:", err)
	} else {
		onlines, _ := decodeObj[[]string](envOnlines)
		snap.OnlineEmails = onlines
	}

	envLastOnline, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/clients/lastOnline", nil)
	if err != nil {
		logger.Warning("remote", r.node.Name, "lastOnline fetch failed:", err)
	} else {
		lastOnline, _ := decodeObj[map[string]int64](envLastOnline)
		snap.LastOnlineMap = lastOnline
	}

	return snap, nil
}

// CertStatus holds the certificate metadata returned by a node.
type CertStatus struct {
	Subject     string `json:"subject"`
	Issuer      string `json:"issuer"`
	Serial      string `json:"serial"`
	NotBefore   int64  `json:"notBefore"`
	NotAfter    int64  `json:"notAfter"`
	Fingerprint string `json:"fingerprint"`
}

// PushCert sends a TLS certificate and key PEM to the node and asks it to
// install them as its TLS serving certificate.
func (r *Remote) PushCert(ctx context.Context, certPEM, keyPEM string) error {
	_, err := r.doWithRetry(ctx, http.MethodPost, "api/v1/certs",
		map[string]string{"certPEM": certPEM, "keyPEM": keyPEM})
	return err
}

// GetCertStatus fetches the node's currently installed TLS certificate
// metadata. If no cert is installed the returned status will be empty.
func (r *Remote) GetCertStatus(ctx context.Context) (*CertStatus, error) {
	env, err := r.doWithRetry(ctx, http.MethodGet, "api/v1/certs/status", nil)
	if err != nil {
		return nil, err
	}
	st, err := decodeObj[CertStatus](env)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func wireInbound(ib *model.Inbound) url.Values {
	v := url.Values{}
	v.Set("total", strconv.FormatInt(ib.Total, 10))
	v.Set("remark", ib.Remark)
	v.Set("enable", strconv.FormatBool(ib.Enable))
	v.Set("expiryTime", strconv.FormatInt(ib.ExpiryTime, 10))
	v.Set("listen", ib.Listen)
	v.Set("port", strconv.Itoa(ib.Port))
	v.Set("protocol", string(ib.Protocol))
	v.Set("settings", ib.Settings)
	v.Set("streamSettings", sanitizeStreamSettingsForRemote(ib.StreamSettings))
	v.Set("tag", ib.Tag)
	v.Set("sniffing", ib.Sniffing)
	if ib.TrafficReset != "" {
		v.Set("trafficReset", ib.TrafficReset)
	}
	return v
}

// sanitizeStreamSettingsForRemote strips file-based TLS certificate paths
// from the StreamSettings before sending to a remote node, but ONLY when
// inline certificate content (certificate / key) is also present in the same
// entry.  In that case the file paths are redundant and stripping them avoids
// confusion when the central panel's local paths don't exist on the remote.
//
// When a certificate entry contains ONLY file paths (no inline content) the
// paths are left untouched: the user explicitly entered paths that exist on
// the remote node's filesystem, and removing them would leave Xray with TLS
// configured but no certificate, causing Xray to crash on the remote node.
func sanitizeStreamSettingsForRemote(streamSettings string) string {
	if streamSettings == "" {
		return streamSettings
	}

	var stream map[string]any
	if err := json.Unmarshal([]byte(streamSettings), &stream); err != nil {
		return streamSettings
	}

	tlsSettings, ok := stream["tlsSettings"].(map[string]any)
	if !ok {
		return streamSettings
	}

	certificates, ok := tlsSettings["certificates"].([]any)
	if !ok {
		return streamSettings
	}

	changed := false
	for _, cert := range certificates {
		c, ok := cert.(map[string]any)
		if !ok {
			continue
		}
		// Only strip file paths when inline content is present so that the
		// remote Xray still has a valid certificate to use.
		hasCertFile := c["certificateFile"] != nil && c["certificateFile"] != ""
		hasKeyFile := c["keyFile"] != nil && c["keyFile"] != ""
		hasCertInline := isNonEmptySlice(c["certificate"])
		hasKeyInline := isNonEmptySlice(c["key"])
		if hasCertFile && hasCertInline {
			delete(c, "certificateFile")
			changed = true
		}
		if hasKeyFile && hasKeyInline {
			delete(c, "keyFile")
			changed = true
		}
	}

	if !changed {
		return streamSettings
	}
	out, err := json.Marshal(stream)
	if err != nil {
		return streamSettings
	}
	return string(out)
}

// isNonEmptySlice reports whether v is a non-nil, non-empty JSON array value.
func isNonEmptySlice(v any) bool {
	s, ok := v.([]any)
	return ok && len(s) > 0
}

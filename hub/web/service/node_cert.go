package service

import (
	"context"
	"fmt"
	"time"

	"github.com/drunkleen/l-ui/hub/web/runtime"
	"github.com/drunkleen/l-ui/internal/certgen"
	"github.com/drunkleen/l-ui/internal/database"
	"github.com/drunkleen/l-ui/internal/database/model"
	"github.com/drunkleen/l-ui/internal/logger"
)

type NodeCertService struct {
	settingSvc SettingService
	nodeSvc    NodeService
}

func NewNodeCertService() *NodeCertService {
	return &NodeCertService{}
}

func (s *NodeCertService) ensureCA() (*certgen.CertPair, error) {
	caCertStr, err := s.settingSvc.GetCaCert()
	if err != nil {
		return nil, fmt.Errorf("get CA cert: %w", err)
	}
	caKeyStr, err := s.settingSvc.GetCaKey()
	if err != nil {
		return nil, fmt.Errorf("get CA key: %w", err)
	}

	if caCertStr != "" && caKeyStr != "" {
		return &certgen.CertPair{
			CertPEM: []byte(caCertStr),
			KeyPEM:  []byte(caKeyStr),
		}, nil
	}

	logger.Infof("No CA cert found, generating new CA")
	ca, err := certgen.GenerateCA()
	if err != nil {
		return nil, fmt.Errorf("generate CA: %w", err)
	}

	if err := s.settingSvc.SetCaCert(string(ca.CertPEM)); err != nil {
		return nil, fmt.Errorf("store CA cert: %w", err)
	}
	if err := s.settingSvc.SetCaKey(string(ca.KeyPEM)); err != nil {
		return nil, fmt.Errorf("store CA key: %w", err)
	}

	logger.Infof("CA certificate generated and stored")
	return ca, nil
}

func (s *NodeCertService) GenerateNodeCert(node *model.Node) (*certgen.CertPair, error) {
	ca, err := s.ensureCA()
	if err != nil {
		return nil, err
	}

	hosts := certgen.HostsForNode(node.Address, node.Name)
	pair, err := certgen.IssueCert(ca, hosts)
	if err != nil {
		return nil, fmt.Errorf("issue cert for node %s: %w", node.Name, err)
	}

	return pair, nil
}

func (s *NodeCertService) PushCert(ctx context.Context, node *model.Node, pair *certgen.CertPair) error {
	mgr := runtime.GetManager()
	if mgr == nil {
		return fmt.Errorf("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(node)
	if err != nil {
		return fmt.Errorf("remote for node %s: %w", node.Name, err)
	}

	if err := remote.PushCert(ctx, string(pair.CertPEM), string(pair.KeyPEM)); err != nil {
		return fmt.Errorf("push cert: %w", err)
	}

	certSubject, certIssuer, serial, notBefore, notAfter, err := certgen.CertInfo(pair.CertPEM)
	if err != nil {
		logger.Warningf("cert info parse failed for node %s: %v", node.Name, err)
	} else {
		logger.Infof("Cert pushed to node %s: subject=%s issuer=%s serial=%s valid=%s-%s",
			node.Name, certSubject, certIssuer, serial,
			notBefore.Format(time.RFC3339), notAfter.Format(time.RFC3339))
	}

	db := database.GetDB()
	updates := map[string]any{
		"cert_serial": serial,
		"cert_expiry": notAfter.Unix(),
	}
	if err := db.Model(model.Node{}).Where("id = ?", node.Id).Updates(updates).Error; err != nil {
		logger.Warningf("update node cert fields: %v", err)
	}

	return nil
}

func (s *NodeCertService) GetCertStatus(ctx context.Context, node *model.Node) (*runtime.CertStatus, error) {
	mgr := runtime.GetManager()
	if mgr == nil {
		return nil, fmt.Errorf("runtime manager unavailable")
	}
	remote, err := mgr.RemoteFor(node)
	if err != nil {
		return nil, fmt.Errorf("remote for node %s: %w", node.Name, err)
	}
	return remote.GetCertStatus(ctx)
}

func (s *NodeCertService) RenewNodeCert(ctx context.Context, node *model.Node) error {
	pair, err := s.GenerateNodeCert(node)
	if err != nil {
		return fmt.Errorf("generate cert: %w", err)
	}
	return s.PushCert(ctx, node, pair)
}

func (s *NodeCertService) NeedsRenewal(node *model.Node) bool {
	if node.CertSerial == "" || node.CertExpiry == 0 {
		return true
	}
	return time.Now().Add(30 * 24 * time.Hour).After(time.Unix(node.CertExpiry, 0))
}

func (s *NodeCertService) GetRenewableNodes() ([]*model.Node, error) {
	db := database.GetDB()
	var nodes []*model.Node
	if err := db.Model(model.Node{}).
		Where("enable = ?", true).
		Where("api_token != ?", "").
		Where("cert_serial = ? OR cert_expiry < ?", "", time.Now().Add(30*24*time.Hour).Unix()).
		Find(&nodes).Error; err != nil {
		return nil, err
	}
	return nodes, nil
}

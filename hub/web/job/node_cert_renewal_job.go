package job

import (
	"context"
	"sync"
	"time"

	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/internal/logger"
)

const (
	nodeCertRenewalTimeout = 30 * time.Second
)

type NodeCertRenewalJob struct {
	nodeService service.NodeService
	certSvc     *service.NodeCertService
	running     sync.Mutex
}

func NewNodeCertRenewalJob() *NodeCertRenewalJob {
	return &NodeCertRenewalJob{
		certSvc: service.NewNodeCertService(),
	}
}

func (j *NodeCertRenewalJob) Run() {
	if !j.running.TryLock() {
		return
	}
	defer j.running.Unlock()

	nodes, err := j.certSvc.GetRenewableNodes()
	if err != nil {
		logger.Warning("cert renewal: get nodes failed:", err)
		return
	}

	if len(nodes) == 0 {
		return
	}

	logger.Infof("cert renewal: checking %d nodes for renewal", len(nodes))

	for _, node := range nodes {
		if !node.Enable {
			continue
		}
		if !j.certSvc.NeedsRenewal(node) {
			continue
		}
		logger.Infof("cert renewal: renewing cert for node %s (expiry: %d)", node.Name, node.CertExpiry)
		ctx, cancel := context.WithTimeout(context.Background(), nodeCertRenewalTimeout)
		if err := j.certSvc.RenewNodeCert(ctx, node); err != nil {
			logger.Warningf("cert renewal: renew node %s failed: %v", node.Name, err)
		}
		cancel()
	}
}

package job

import (
	"fmt"
	"sync"
	"time"

	"github.com/drunkleen/l-ui/hub/web/service"
	"github.com/drunkleen/l-ui/internal/logger"
)

const (
	nodeAlertCooldown = 1 * time.Hour
)

// nodeAlertState tracks per-node alerting state to avoid duplicate alerts.
type nodeAlertState struct {
	mu                 sync.Mutex
	consecutiveOffline map[int]int
	lastAlertTime      map[string]time.Time
}

var alertState = &nodeAlertState{
	consecutiveOffline: make(map[int]int),
	lastAlertTime:      make(map[string]time.Time),
}

func alarmKey(nodeID int, kind string) string {
	return fmt.Sprintf("%d:%s", nodeID, kind)
}

func (s *nodeAlertState) incOffline(nodeID int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.consecutiveOffline[nodeID]++
	return s.consecutiveOffline[nodeID]
}

func (s *nodeAlertState) resetOffline(nodeID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.consecutiveOffline, nodeID)
}

func (s *nodeAlertState) canAlert(nodeID int, kind string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := alarmKey(nodeID, kind)
	last, ok := s.lastAlertTime[key]
	if !ok || time.Since(last) >= nodeAlertCooldown {
		s.lastAlertTime[key] = time.Now()
		return true
	}
	return false
}

type NodeAlertJob struct {
	nodeService    service.NodeService
	settingService service.SettingService
	tgbotService   service.Tgbot
}

func NewNodeAlertJob() *NodeAlertJob {
	return &NodeAlertJob{}
}

func (j *NodeAlertJob) Run() {
	nodes, err := j.nodeService.GetAll()
	if err != nil {
		logger.Warning("node alert: load nodes failed:", err)
		return
	}
	if len(nodes) == 0 {
		return
	}

	cpuThresh, _ := j.settingService.GetNodeCpuThreshold()
	memThresh, _ := j.settingService.GetNodeMemThreshold()
	diskThresh, _ := j.settingService.GetNodeDiskThreshold()
	downThresh, _ := j.settingService.GetNodeDownThreshold()
	if downThresh <= 0 {
		downThresh = 3
	}

	for _, n := range nodes {
		if !n.Enable {
			alertState.resetOffline(n.Id)
			continue
		}

		if n.Status != "online" {
			count := alertState.incOffline(n.Id)
			if count >= downThresh && alertState.canAlert(n.Id, "down") {
				msg := fmt.Sprintf(
					"Node Down Alert\nNode: %s\nAddress: %s:%d\nStatus: offline for %d consecutive checks\nLast error: %s",
					n.Name, n.Address, n.Port, count, n.LastError)
				j.tgbotService.SendMsgToTgbotAdmins(msg)
				logger.Warning("node alert: sent node-down alert for", n.Name)
			}
		} else {
			wasOffline := alertState.consecutiveOffline[n.Id] >= downThresh
			alertState.resetOffline(n.Id)

			if wasOffline && alertState.canAlert(n.Id, "recovered") {
				msg := fmt.Sprintf(
					"Node Recovered\nNode: %s\nAddress: %s:%d\nStatus: back online",
					n.Name, n.Address, n.Port)
				j.tgbotService.SendMsgToTgbotAdmins(msg)
				logger.Warning("node alert: sent recovery alert for", n.Name)
			}

			// Resource threshold checks
			if cpuThresh > 0 && n.CpuPct > float64(cpuThresh) && alertState.canAlert(n.Id, "cpu") {
				msg := fmt.Sprintf(
					"Node CPU Alert\nNode: %s\nCPU: %.1f%% (threshold: %d%%)",
					n.Name, n.CpuPct, cpuThresh)
				j.tgbotService.SendMsgToTgbotAdmins(msg)
				logger.Warningf("node alert: CPU threshold exceeded for %s: %.1f%%", n.Name, n.CpuPct)
			}
			if memThresh > 0 && n.MemPct > float64(memThresh) && alertState.canAlert(n.Id, "mem") {
				msg := fmt.Sprintf(
					"Node Memory Alert\nNode: %s\nMemory: %.1f%% (threshold: %d%%)",
					n.Name, n.MemPct, memThresh)
				j.tgbotService.SendMsgToTgbotAdmins(msg)
				logger.Warningf("node alert: MEM threshold exceeded for %s: %.1f%%", n.Name, n.MemPct)
			}
			if diskThresh > 0 {
				diskPct := 0.0
				if n.DiskTotal > 0 {
					diskPct = float64(n.DiskCurrent) * 100.0 / float64(n.DiskTotal)
				}
				if diskPct > float64(diskThresh) && alertState.canAlert(n.Id, "disk") {
					msg := fmt.Sprintf(
						"Node Disk Alert\nNode: %s\nDisk: %.1f%% (threshold: %d%%)",
						n.Name, diskPct, diskThresh)
					j.tgbotService.SendMsgToTgbotAdmins(msg)
					logger.Warningf("node alert: disk threshold exceeded for %s: %.1f%%", n.Name, diskPct)
				}
			}
		}
	}
}

package service

import (
	"os"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

type SystemMetrics struct {
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryTotal   uint64    `json:"memory_total"`
	MemoryUsed    uint64    `json:"memory_used"`
	MemoryPercent float64   `json:"memory_percent"`
	DiskTotal     uint64    `json:"disk_total"`
	DiskUsed      uint64    `json:"disk_used"`
	DiskPercent   float64   `json:"disk_percent"`
	Uptime        uint64    `json:"uptime"`
	Load1         float64   `json:"load_1"`
	Load5         float64   `json:"load_5"`
	Load15        float64   `json:"load_15"`
	NetSent       uint64    `json:"net_sent"`
	NetRecv       uint64    `json:"net_recv"`
	Timestamp     time.Time `json:"timestamp"`
}

type SystemInfo struct {
	Hostname        string `json:"hostname"`
	OS              string `json:"os"`
	Platform        string `json:"platform"`
	PlatformVersion string `json:"platform_version"`
	KernelVersion   string `json:"kernel_version"`
	KernelArch      string `json:"kernel_arch"`
	CPUModel        string `json:"cpu_model"`
	CPUCores        int    `json:"cpu_cores"`
	GoVersion       string `json:"go_version"`
}

type SystemService struct{}

func NewSystemService() *SystemService {
	return &SystemService{}
}

func (s *SystemService) GetMetrics() (*SystemMetrics, error) {
	cpuPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		cpuPercent = []float64{0}
	}

	memInfo, _ := mem.VirtualMemory()

	diskUsage, _ := disk.Usage("/")

	hostInfo, _ := host.Info()

	loadAvg, _ := load.Avg()

	netIO, _ := net.IOCounters(false)

	var netSent, netRecv uint64
	if len(netIO) > 0 {
		netSent = netIO[0].BytesSent
		netRecv = netIO[0].BytesRecv
	}

	var cpuPct float64
	if len(cpuPercent) > 0 {
		cpuPct = cpuPercent[0]
	}

	var memTotal, memUsed, memPct float64
	if memInfo != nil {
		memTotal = float64(memInfo.Total)
		memUsed = float64(memInfo.Used)
		memPct = memInfo.UsedPercent
	}

	var diskTotal, diskUsed, diskPct float64
	if diskUsage != nil {
		diskTotal = float64(diskUsage.Total)
		diskUsed = float64(diskUsage.Used)
		diskPct = diskUsage.UsedPercent
	}

	var uptimeVal uint64
	if hostInfo != nil {
		uptimeVal = hostInfo.Uptime
	}

	var load1, load5, load15 float64
	if loadAvg != nil {
		load1 = loadAvg.Load1
		load5 = loadAvg.Load5
		load15 = loadAvg.Load15
	}

	return &SystemMetrics{
		CPUPercent:    cpuPct,
		MemoryTotal:   uint64(memTotal),
		MemoryUsed:    uint64(memUsed),
		MemoryPercent: memPct,
		DiskTotal:     uint64(diskTotal),
		DiskUsed:      uint64(diskUsed),
		DiskPercent:   diskPct,
		Uptime:        uptimeVal,
		Load1:         load1,
		Load5:         load5,
		Load15:        load15,
		NetSent:       netSent,
		NetRecv:       netRecv,
		Timestamp:     time.Now(),
	}, nil
}

func (s *SystemService) GetInfo() (*SystemInfo, error) {
	hostInfo, _ := host.Info()
	cpuInfo, _ := cpu.Info()

	info := &SystemInfo{
		Hostname:        "",
		OS:              runtime.GOOS,
		Platform:        "",
		PlatformVersion: "",
		KernelVersion:   "",
		KernelArch:      runtime.GOARCH,
		CPUModel:        "",
		CPUCores:        runtime.NumCPU(),
		GoVersion:       runtime.Version(),
	}

	if hostInfo != nil {
		info.Hostname = hostInfo.Hostname
		info.OS = hostInfo.OS
		info.Platform = hostInfo.Platform
		info.PlatformVersion = hostInfo.PlatformVersion
		info.KernelVersion = hostInfo.KernelVersion
		info.KernelArch = hostInfo.KernelArch
	}

	if len(cpuInfo) > 0 {
		info.CPUModel = cpuInfo[0].ModelName
	}

	hostname, _ := os.Hostname()
	if info.Hostname == "" {
		info.Hostname = hostname
	}

	return info, nil
}

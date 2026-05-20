package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	aethercontainer "aether/internal/entities/container"

	"github.com/pocketbase/pocketbase/core"
	psutilnet "github.com/shirou/gopsutil/v4/net"
	psutilprocess "github.com/shirou/gopsutil/v4/process"
)

const (
	i3dResourceEnvironmentLocal   = "local"
	i3dResourceEnvironmentRelease = "release"

	i3dResourceGroupBusiness   = "business"
	i3dResourceGroupMiddleware = "middleware"
	i3dResourceGroupFrontend   = "frontend"
	i3dResourceGroupMonitor    = "monitor"

	i3dResourceKindDocker  = "docker"
	i3dResourceKindProcess = "process"

	i3dResourceStatusUp      = "up"
	i3dResourceStatusDown    = "down"
	i3dResourceStatusUnknown = "unknown"

	i3dResourceDockerStatsTimeout = 8 * time.Second
	i3dResourceProcessCPUInterval = 250 * time.Millisecond
)

type i3dResourceTarget struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Group           string   `json:"group"`
	Kind            string   `json:"kind"`
	ContainerName   string   `json:"container_name,omitempty"`
	PIDFile         string   `json:"pid_file,omitempty"`
	Ports           []uint32 `json:"ports,omitempty"`
	WorkingDir      string   `json:"working_dir,omitempty"`
	CommandIncludes []string `json:"command_includes,omitempty"`
	HealthURL       string   `json:"health_url,omitempty"`
}

type i3dResourceContainerStats struct {
	Name                    string
	ContainerID             string
	Status                  string
	CPUPercent              float64
	MemoryBytes             uint64
	MemoryUsageBytes        uint64
	MemoryRSSBytes          uint64
	MemoryCacheBytes        uint64
	MemoryAnonBytes         uint64
	MemoryInactiveFileBytes uint64
	DiskReadBytesPS         uint64
	DiskWriteBytesPS        uint64
	NetworkRxBytesPS        uint64
	NetworkTxBytesPS        uint64
	GPUMemoryBytes          uint64
	PIDCount                int
	UptimeSeconds           uint64
	UpdatedAt               string
	Diagnostic              string
}

type i3dResourceContainerHistoryPoint struct {
	Timestamp  string
	Containers []i3dResourceContainerStats
}

type i3dResourceDockerCumulativeSample struct {
	cpuTotal    uint64
	systemTotal uint64
	readBytes   uint64
	writeBytes  uint64
	rxBytes     uint64
	txBytes     uint64
	readAt      time.Time
}

var i3dResourceDockerStatsCache = struct {
	sync.Mutex
	samples map[string]i3dResourceDockerCumulativeSample
}{samples: map[string]i3dResourceDockerCumulativeSample{}}

type i3dResourceProcessStats struct {
	TargetID         string
	Status           string
	CPUPercent       float64
	MemoryBytes      uint64
	MemoryRSSBytes   uint64
	DiskReadBytesPS  uint64
	DiskWriteBytesPS uint64
	NetworkRxBytesPS uint64
	NetworkTxBytesPS uint64
	GPUMemoryBytes   uint64
	UnitCount        int
	ThreadCount      int
	UptimeSeconds    uint64
	UpdatedAt        string
	Diagnostic       string
	PIDs             []int32
}

type i3dResourceGPUProcessStats struct {
	PID             int32
	ProcessName     string
	GPUUUID         string
	ContainerID     string
	UsedMemoryBytes uint64
}

type i3dResourceSummaryDTO struct {
	CPUPercent              float64 `json:"cpu_percent"`
	CPUCoresUsed            float64 `json:"cpu_cores_used"`
	MemoryBytes             uint64  `json:"memory_bytes"`
	MemoryUsageBytes        uint64  `json:"memory_usage_bytes"`
	MemoryRSSBytes          uint64  `json:"memory_rss_bytes"`
	MemoryCacheBytes        uint64  `json:"memory_cache_bytes"`
	MemoryAnonBytes         uint64  `json:"memory_anon_bytes"`
	MemoryInactiveFileBytes uint64  `json:"memory_inactive_file_bytes"`
	DiskReadBytesPS         uint64  `json:"disk_read_bps"`
	DiskWriteBytesPS        uint64  `json:"disk_write_bps"`
	NetworkRxBytesPS        uint64  `json:"network_rx_bps"`
	NetworkTxBytesPS        uint64  `json:"network_tx_bps"`
	GPUUtilPercent          float64 `json:"gpu_util_percent"`
	GPUMemoryBytes          uint64  `json:"gpu_memory_bytes"`
	PIDCount                int     `json:"pids"`
	ThreadCount             int     `json:"threads"`
	AbnormalCount           int     `json:"abnormal_count"`
}

type i3dResourceTargetDTO struct {
	ID                      string   `json:"id"`
	Name                    string   `json:"name"`
	Group                   string   `json:"group"`
	Kind                    string   `json:"kind"`
	Status                  string   `json:"status"`
	HealthStatus            string   `json:"health_status"`
	CPUPercent              float64  `json:"cpu_percent"`
	CPUCoresUsed            float64  `json:"cpu_cores_used"`
	MemoryBytes             uint64   `json:"memory_bytes"`
	MemoryUsageBytes        uint64   `json:"memory_usage_bytes"`
	MemoryRSSBytes          uint64   `json:"memory_rss_bytes"`
	MemoryCacheBytes        uint64   `json:"memory_cache_bytes"`
	MemoryAnonBytes         uint64   `json:"memory_anon_bytes"`
	MemoryInactiveFileBytes uint64   `json:"memory_inactive_file_bytes"`
	MemoryPercent           float64  `json:"memory_percent"`
	DiskReadBytesPS         uint64   `json:"disk_read_bps"`
	DiskWriteBytesPS        uint64   `json:"disk_write_bps"`
	NetworkRxBytesPS        uint64   `json:"network_rx_bps"`
	NetworkTxBytesPS        uint64   `json:"network_tx_bps"`
	GPUMemoryBytes          uint64   `json:"gpu_memory_bytes"`
	UnitCount               int      `json:"unit_count"`
	PIDCount                int      `json:"pids"`
	ThreadCount             int      `json:"threads"`
	UptimeSeconds           uint64   `json:"uptime_seconds"`
	RestartCount            int      `json:"restart_count"`
	ContainerName           string   `json:"container_name,omitempty"`
	PIDFile                 string   `json:"pid_file,omitempty"`
	Ports                   []uint32 `json:"ports,omitempty"`
	WorkingDir              string   `json:"working_dir,omitempty"`
	CommandIncludes         []string `json:"command_includes,omitempty"`
	UpdatedAt               string   `json:"updated_at"`
	Diagnostic              string   `json:"diagnostic,omitempty"`
}

type i3dResourceGroupDTO struct {
	ID                      string  `json:"id"`
	Name                    string  `json:"name"`
	CPUPercent              float64 `json:"cpu_percent"`
	MemoryBytes             uint64  `json:"memory_bytes"`
	MemoryUsageBytes        uint64  `json:"memory_usage_bytes"`
	MemoryRSSBytes          uint64  `json:"memory_rss_bytes"`
	MemoryCacheBytes        uint64  `json:"memory_cache_bytes"`
	MemoryAnonBytes         uint64  `json:"memory_anon_bytes"`
	MemoryInactiveFileBytes uint64  `json:"memory_inactive_file_bytes"`
	PIDCount                int     `json:"pids"`
	ThreadCount             int     `json:"threads"`
}

type i3dResourceOverviewDTO struct {
	Environment           string                 `json:"environment"`
	SampleIntervalSeconds int                    `json:"sample_interval_seconds"`
	UpdatedAt             string                 `json:"updated_at"`
	Summary               i3dResourceSummaryDTO  `json:"summary"`
	Groups                []i3dResourceGroupDTO  `json:"groups"`
	Items                 []i3dResourceTargetDTO `json:"items"`
}

type i3dResourceOverviewCacheEntry struct {
	response   i3dResourceOverviewDTO
	refreshed  time.Time
	refreshing bool
}

type i3dResourceOverviewCache struct {
	sync.Mutex
	maxAge  time.Duration
	entries map[string]*i3dResourceOverviewCacheEntry
}

func newI3DResourceOverviewCache(maxAge time.Duration) *i3dResourceOverviewCache {
	if maxAge <= 0 {
		maxAge = 5 * time.Second
	}
	return &i3dResourceOverviewCache{
		maxAge:  maxAge,
		entries: map[string]*i3dResourceOverviewCacheEntry{},
	}
}

func (c *i3dResourceOverviewCache) get(
	environment string,
	now time.Time,
	collect func() (i3dResourceOverviewDTO, error),
	runAsync func(func()),
) (i3dResourceOverviewDTO, error) {
	environment = normalizeI3DResourceEnvironment(environment)
	c.Lock()
	entry := c.entries[environment]
	if entry != nil {
		response := entry.response
		stale := now.Sub(entry.refreshed) >= c.maxAge
		if !stale {
			c.Unlock()
			return response, nil
		}
		if !entry.refreshing {
			entry.refreshing = true
			c.Unlock()
			runAsync(func() {
				response, err := collect()
				c.Lock()
				defer c.Unlock()
				entry := c.entries[environment]
				if entry == nil {
					entry = &i3dResourceOverviewCacheEntry{}
					c.entries[environment] = entry
				}
				if err == nil {
					entry.response = response
					entry.refreshed = time.Now()
				}
				entry.refreshing = false
			})
			return response, nil
		}
		c.Unlock()
		return response, nil
	}
	c.Unlock()

	response, err := collect()
	if err != nil {
		return i3dResourceOverviewDTO{}, err
	}
	c.Lock()
	c.entries[environment] = &i3dResourceOverviewCacheEntry{response: response, refreshed: now}
	c.Unlock()
	return response, nil
}

type i3dResourceTimeseriesPointDTO struct {
	Timestamp        string                                         `json:"timestamp"`
	CPUPercent       float64                                        `json:"cpu_percent"`
	CPUCoresUsed     float64                                        `json:"cpu_cores_used"`
	MemoryBytes      uint64                                         `json:"memory_bytes"`
	DiskReadBytesPS  uint64                                         `json:"disk_read_bps"`
	DiskWriteBytesPS uint64                                         `json:"disk_write_bps"`
	NetworkRxBytesPS uint64                                         `json:"network_rx_bps"`
	NetworkTxBytesPS uint64                                         `json:"network_tx_bps"`
	GPUMemoryBytes   uint64                                         `json:"gpu_memory_bytes"`
	Groups           map[string]i3dResourceGroupTimeseriesPointDTO  `json:"groups"`
	Targets          map[string]i3dResourceTargetTimeseriesPointDTO `json:"targets"`
}

type i3dResourceTimeseriesDTO struct {
	Environment string                          `json:"environment"`
	Items       []i3dResourceTimeseriesPointDTO `json:"items"`
}

type i3dResourceGroupTimeseriesPointDTO struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemoryBytes uint64  `json:"memory_bytes"`
}

type i3dResourceTargetTimeseriesPointDTO struct {
	Name           string  `json:"name"`
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryBytes    uint64  `json:"memory_bytes"`
	GPUMemoryBytes uint64  `json:"gpu_memory_bytes"`
}

type i3dResourceTimeseriesHistory struct {
	sync.Mutex
	limit   int
	entries map[string][]i3dResourceTimeseriesPointDTO
}

func newI3DResourceTimeseriesHistory(limit int) *i3dResourceTimeseriesHistory {
	if limit <= 0 {
		limit = 240
	}
	return &i3dResourceTimeseriesHistory{
		limit:   limit,
		entries: map[string][]i3dResourceTimeseriesPointDTO{},
	}
}

func (h *i3dResourceTimeseriesHistory) record(overview i3dResourceOverviewDTO) {
	if h == nil || overview.UpdatedAt == "" {
		return
	}
	environment := normalizeI3DResourceEnvironment(overview.Environment)
	point := i3dResourceTimeseriesPointDTO{
		Timestamp:        overview.UpdatedAt,
		CPUPercent:       overview.Summary.CPUPercent,
		CPUCoresUsed:     overview.Summary.CPUCoresUsed,
		MemoryBytes:      overview.Summary.MemoryBytes,
		DiskReadBytesPS:  overview.Summary.DiskReadBytesPS,
		DiskWriteBytesPS: overview.Summary.DiskWriteBytesPS,
		NetworkRxBytesPS: overview.Summary.NetworkRxBytesPS,
		NetworkTxBytesPS: overview.Summary.NetworkTxBytesPS,
		GPUMemoryBytes:   overview.Summary.GPUMemoryBytes,
		Groups:           make(map[string]i3dResourceGroupTimeseriesPointDTO, len(overview.Groups)),
		Targets:          make(map[string]i3dResourceTargetTimeseriesPointDTO, len(overview.Items)),
	}
	for _, group := range overview.Groups {
		point.Groups[group.ID] = i3dResourceGroupTimeseriesPointDTO{
			CPUPercent:  group.CPUPercent,
			MemoryBytes: group.MemoryBytes,
		}
	}
	for _, item := range overview.Items {
		point.Targets[item.ID] = i3dResourceTargetTimeseriesPointDTO{
			Name:           item.Name,
			CPUPercent:     item.CPUPercent,
			MemoryBytes:    item.MemoryBytes,
			GPUMemoryBytes: item.GPUMemoryBytes,
		}
	}

	h.Lock()
	defer h.Unlock()
	items := append(h.entries[environment], point)
	if len(items) > h.limit {
		items = items[len(items)-h.limit:]
	}
	h.entries[environment] = items
}

func (h *i3dResourceTimeseriesHistory) snapshot(environment string) i3dResourceTimeseriesDTO {
	environment = normalizeI3DResourceEnvironment(environment)
	if h == nil {
		return i3dResourceTimeseriesDTO{Environment: environment, Items: []i3dResourceTimeseriesPointDTO{}}
	}
	h.Lock()
	defer h.Unlock()
	items := append([]i3dResourceTimeseriesPointDTO(nil), h.entries[environment]...)
	return i3dResourceTimeseriesDTO{Environment: environment, Items: items}
}

func defaultI3DResourceTargets(environment string) []i3dResourceTarget {
	switch normalizeI3DResourceEnvironment(environment) {
	case i3dResourceEnvironmentRelease:
		prefix := i3dResourceContainerPrefix(i3dResourceEnvironmentRelease)
		return []i3dResourceTarget{
			{ID: "release.aether", Name: "Aether 监控", Group: i3dResourceGroupMonitor, Kind: i3dResourceKindProcess, Ports: []uint32{i3dResourceAetherPort(19101)}},
			{ID: "release.search.web", Name: "检索服务", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "search-service")},
			{ID: "release.inference.web", Name: "推理服务", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "inference-service")},
			{ID: "release.file.web", Name: "文件服务 API", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "file-service")},
			{ID: "release.file.consumer", Name: "文件服务事件消费者", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "file-service-consumer")},
			{ID: "release.file.worker", Name: "文件服务 Worker", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "file-service-worker")},
			{ID: "release.cad.web", Name: "CAD 作业服务 API", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "cad-job-service")},
			{ID: "release.cad.batch_worker", Name: "CAD 批量入库 Worker", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "cad-job-service-worker")},
			{ID: "release.cad.query_worker", Name: "CAD 查询上传 Worker", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "cad-job-service-query-worker")},
			{ID: "release.cad.compare_worker", Name: "CAD 对比 Worker", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "cad-job-service-compare-worker")},
			{ID: "release.middleware.postgres", Name: "PostgreSQL + pgvector", Group: i3dResourceGroupMiddleware, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "postgres")},
			{ID: "release.middleware.redis", Name: "Redis", Group: i3dResourceGroupMiddleware, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "redis")},
			{ID: "release.middleware.rabbitmq", Name: "RabbitMQ", Group: i3dResourceGroupMiddleware, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "rabbitmq")},
			{ID: "release.middleware.minio", Name: "MinIO", Group: i3dResourceGroupMiddleware, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "minio")},
			{ID: "release.middleware.xxl_mysql", Name: "XXL-Job MySQL", Group: i3dResourceGroupMiddleware, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "xxl-job-mysql")},
			{ID: "release.middleware.xxl_admin", Name: "XXL-Job Admin", Group: i3dResourceGroupMiddleware, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "xxl-job-admin")},
		}
	default:
		prefix := i3dResourceContainerPrefix(i3dResourceEnvironmentLocal)
		return []i3dResourceTarget{
			{ID: "local.search.web", Name: "检索服务", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindProcess, PIDFile: "../i3d-search-service/run/web.pid", Ports: []uint32{39200}},
			{ID: "local.inference.web", Name: "推理服务", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindProcess, PIDFile: "../i3d-inference-service/run/web.pid", Ports: []uint32{39210}},
			{ID: "local.file.web", Name: "文件服务 API", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindProcess, PIDFile: "../i3d-file-service/run/web.pid", Ports: []uint32{39230}},
			{ID: "local.file.worker", Name: "文件服务 Worker", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindProcess, PIDFile: "../i3d-file-service/run/worker.pid", WorkingDir: "../i3d-file-service", CommandIncludes: []string{"i3d-file-service/venv/bin/python", "-m celery", "file_center.config.celery"}},
			{ID: "local.file.consumer", Name: "文件服务事件消费者", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindProcess, PIDFile: "../i3d-file-service/run/consumer.pid", WorkingDir: "../i3d-file-service", CommandIncludes: []string{"i3d-file-service/venv/bin/python", "manage.py run_mq_consumer"}},
			{ID: "local.cad.web", Name: "CAD 作业服务 API", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindProcess, PIDFile: "../i3d-cad-job-service/run/web.pid", Ports: []uint32{39220}},
			{ID: "local.cad.worker", Name: "CAD 作业服务 Worker", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindProcess, PIDFile: "../i3d-cad-job-service/run/worker.pid", WorkingDir: "../i3d-cad-job-service", CommandIncludes: []string{"i3d-cad-job-service/venv/bin/python", "-m celery", "cad_job.tasks.celery_app"}},
			{ID: "local.aether", Name: "本地 Aether", Group: i3dResourceGroupMonitor, Kind: i3dResourceKindProcess, PIDFile: "run/hub/aether.pid", Ports: []uint32{19100}},
			{ID: "local.middleware.postgres", Name: "PostgreSQL + pgvector", Group: i3dResourceGroupMiddleware, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "postgres")},
			{ID: "local.middleware.redis", Name: "Redis", Group: i3dResourceGroupMiddleware, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "redis")},
			{ID: "local.middleware.rabbitmq", Name: "RabbitMQ", Group: i3dResourceGroupMiddleware, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "rabbitmq")},
			{ID: "local.middleware.minio", Name: "MinIO", Group: i3dResourceGroupMiddleware, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "minio")},
			{ID: "local.middleware.xxl_mysql", Name: "XXL-Job MySQL", Group: i3dResourceGroupMiddleware, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "xxl-job-mysql")},
			{ID: "local.middleware.xxl_admin", Name: "XXL-Job Admin", Group: i3dResourceGroupMiddleware, Kind: i3dResourceKindDocker, ContainerName: i3dResourceContainerName(prefix, "xxl-job-admin")},
		}
	}
}

func i3dResourceAetherPort(defaultPort uint32) uint32 {
	if value, ok := GetEnv("AETHER_HOST_PORT"); ok {
		port, err := strconv.ParseUint(strings.TrimSpace(value), 10, 32)
		if err == nil && port > 0 && port <= 65535 {
			return uint32(port)
		}
	}
	return defaultPort
}

func i3dResourceContainerPrefix(environment string) string {
	if environment == i3dResourceEnvironmentRelease {
		if prefix, ok := GetEnv("CONTAINER_PREFIX"); ok && strings.TrimSpace(prefix) != "" {
			return strings.TrimSpace(prefix)
		}
		return "i3d-release"
	}
	if prefix, ok := GetEnv("I3D_MIDDLEWARE_CONTAINER_PREFIX"); ok && strings.TrimSpace(prefix) != "" {
		return strings.TrimSpace(prefix)
	}
	return "i3d"
}

func i3dResourceContainerName(prefix string, suffix string) string {
	prefix = strings.TrimSuffix(strings.TrimSpace(prefix), "-")
	suffix = strings.TrimPrefix(strings.TrimSpace(suffix), "-")
	if prefix == "" {
		return suffix
	}
	if suffix == "" {
		return prefix
	}
	return prefix + "-" + suffix
}

func normalizeI3DResourceEnvironment(environment string) string {
	environment = strings.TrimSpace(strings.ToLower(environment))
	if environment == i3dResourceEnvironmentRelease {
		return i3dResourceEnvironmentRelease
	}
	return i3dResourceEnvironmentLocal
}

func buildI3DResourceOverview(environment string, targets []i3dResourceTarget, containers []i3dResourceContainerStats, processes []i3dResourceProcessStats, now func() time.Time) i3dResourceOverviewDTO {
	if now == nil {
		now = time.Now
	}
	containerByName := make(map[string]i3dResourceContainerStats, len(containers))
	for _, container := range containers {
		name := strings.TrimPrefix(strings.TrimSpace(container.Name), "/")
		if name != "" {
			container.Name = name
			containerByName[name] = container
		}
	}
	processByTargetID := make(map[string]i3dResourceProcessStats, len(processes))
	for _, process := range processes {
		if process.TargetID != "" {
			processByTargetID[process.TargetID] = process
		}
	}

	items := make([]i3dResourceTargetDTO, 0, len(targets))
	groupsByID := make(map[string]*i3dResourceGroupDTO)
	summary := i3dResourceSummaryDTO{}
	for _, target := range targets {
		item := i3dResourceTargetDTO{
			ID:              target.ID,
			Name:            target.Name,
			Group:           target.Group,
			Kind:            target.Kind,
			Status:          i3dResourceStatusDown,
			HealthStatus:    "none",
			ContainerName:   target.ContainerName,
			PIDFile:         target.PIDFile,
			Ports:           target.Ports,
			WorkingDir:      target.WorkingDir,
			CommandIncludes: target.CommandIncludes,
			UpdatedAt:       now().Format(time.RFC3339),
		}
		if target.Kind == i3dResourceKindDocker {
			if container, ok := containerByName[target.ContainerName]; ok {
				item.Status = mapI3DResourceContainerStatus(container.Status)
				item.CPUPercent = container.CPUPercent
				item.CPUCoresUsed = container.CPUPercent / 100
				item.MemoryBytes = container.MemoryBytes
				item.MemoryUsageBytes = container.MemoryUsageBytes
				item.MemoryRSSBytes = container.MemoryRSSBytes
				item.MemoryCacheBytes = container.MemoryCacheBytes
				item.MemoryAnonBytes = container.MemoryAnonBytes
				item.MemoryInactiveFileBytes = container.MemoryInactiveFileBytes
				item.DiskReadBytesPS = container.DiskReadBytesPS
				item.DiskWriteBytesPS = container.DiskWriteBytesPS
				item.NetworkRxBytesPS = container.NetworkRxBytesPS
				item.NetworkTxBytesPS = container.NetworkTxBytesPS
				item.GPUMemoryBytes = container.GPUMemoryBytes
				item.UptimeSeconds = container.UptimeSeconds
				item.Diagnostic = container.Diagnostic
				item.UnitCount = 1
				item.PIDCount = container.PIDCount
				if container.UpdatedAt != "" {
					item.UpdatedAt = container.UpdatedAt
				}
			} else {
				item.Diagnostic = "未找到声明的容器"
			}
		} else if processStats, ok := processByTargetID[target.ID]; ok {
			item.Status = processStats.Status
			item.CPUPercent = processStats.CPUPercent
			item.CPUCoresUsed = processStats.CPUPercent / 100
			item.MemoryBytes = processStats.MemoryBytes
			item.MemoryRSSBytes = processStats.MemoryRSSBytes
			item.DiskReadBytesPS = processStats.DiskReadBytesPS
			item.DiskWriteBytesPS = processStats.DiskWriteBytesPS
			item.NetworkRxBytesPS = processStats.NetworkRxBytesPS
			item.NetworkTxBytesPS = processStats.NetworkTxBytesPS
			item.GPUMemoryBytes = processStats.GPUMemoryBytes
			item.UnitCount = processStats.UnitCount
			item.PIDCount = len(processStats.PIDs)
			item.ThreadCount = processStats.ThreadCount
			item.UptimeSeconds = processStats.UptimeSeconds
			item.Diagnostic = processStats.Diagnostic
			if processStats.UpdatedAt != "" {
				item.UpdatedAt = processStats.UpdatedAt
			}
		} else {
			item.Diagnostic = "未找到声明的 PID 文件或进程"
		}

		if item.Status != i3dResourceStatusUp || strings.TrimSpace(item.Diagnostic) != "" {
			summary.AbnormalCount++
		}
		summary.CPUPercent += item.CPUPercent
		summary.CPUCoresUsed += item.CPUCoresUsed
		summary.MemoryBytes += item.MemoryBytes
		summary.MemoryUsageBytes += item.MemoryUsageBytes
		summary.MemoryRSSBytes += item.MemoryRSSBytes
		summary.MemoryCacheBytes += item.MemoryCacheBytes
		summary.MemoryAnonBytes += item.MemoryAnonBytes
		summary.MemoryInactiveFileBytes += item.MemoryInactiveFileBytes
		summary.DiskReadBytesPS += item.DiskReadBytesPS
		summary.DiskWriteBytesPS += item.DiskWriteBytesPS
		summary.NetworkRxBytesPS += item.NetworkRxBytesPS
		summary.NetworkTxBytesPS += item.NetworkTxBytesPS
		summary.GPUMemoryBytes += item.GPUMemoryBytes
		summary.PIDCount += item.PIDCount
		summary.ThreadCount += item.ThreadCount

		group := groupsByID[target.Group]
		if group == nil {
			group = &i3dResourceGroupDTO{ID: target.Group, Name: i3dResourceGroupName(target.Group)}
			groupsByID[target.Group] = group
		}
		group.CPUPercent += item.CPUPercent
		group.MemoryBytes += item.MemoryBytes
		group.MemoryUsageBytes += item.MemoryUsageBytes
		group.MemoryRSSBytes += item.MemoryRSSBytes
		group.MemoryCacheBytes += item.MemoryCacheBytes
		group.MemoryAnonBytes += item.MemoryAnonBytes
		group.MemoryInactiveFileBytes += item.MemoryInactiveFileBytes
		group.PIDCount += item.PIDCount
		group.ThreadCount += item.ThreadCount
		items = append(items, item)
	}

	groups := make([]i3dResourceGroupDTO, 0, len(groupsByID))
	for _, group := range groupsByID {
		groups = append(groups, *group)
	}
	sort.Slice(groups, func(i, j int) bool {
		return i3dResourceGroupOrder(groups[i].ID) < i3dResourceGroupOrder(groups[j].ID)
	})

	return i3dResourceOverviewDTO{
		Environment:           normalizeI3DResourceEnvironment(environment),
		SampleIntervalSeconds: 5,
		UpdatedAt:             now().Format(time.RFC3339),
		Summary:               summary,
		Groups:                groups,
		Items:                 items,
	}
}

func buildI3DResourceTimeseries(environment string, targets []i3dResourceTarget, history []i3dResourceContainerHistoryPoint) i3dResourceTimeseriesDTO {
	configuredContainerNames := make(map[string]bool)
	for _, target := range targets {
		if target.Kind != i3dResourceKindDocker {
			continue
		}
		name := strings.TrimPrefix(strings.TrimSpace(target.ContainerName), "/")
		if name != "" {
			configuredContainerNames[name] = true
		}
	}
	items := make([]i3dResourceTimeseriesPointDTO, 0, len(history))
	for _, point := range history {
		item := i3dResourceTimeseriesPointDTO{Timestamp: point.Timestamp}
		for _, container := range point.Containers {
			name := strings.TrimPrefix(strings.TrimSpace(container.Name), "/")
			if !configuredContainerNames[name] {
				continue
			}
			item.CPUPercent += container.CPUPercent
			item.CPUCoresUsed += container.CPUPercent / 100
			item.MemoryBytes += container.MemoryBytes
			item.DiskReadBytesPS += container.DiskReadBytesPS
			item.DiskWriteBytesPS += container.DiskWriteBytesPS
			item.NetworkRxBytesPS += container.NetworkRxBytesPS
			item.NetworkTxBytesPS += container.NetworkTxBytesPS
		}
		items = append(items, item)
	}
	return i3dResourceTimeseriesDTO{
		Environment: normalizeI3DResourceEnvironment(environment),
		Items:       items,
	}
}

func mergeI3DResourceContainerTraffic(containers []i3dResourceContainerStats, history []i3dResourceContainerStats) []i3dResourceContainerStats {
	trafficByName := make(map[string]i3dResourceContainerStats, len(history))
	for _, item := range history {
		name := strings.TrimPrefix(strings.TrimSpace(item.Name), "/")
		if name != "" {
			trafficByName[name] = item
		}
	}
	merged := make([]i3dResourceContainerStats, len(containers))
	copy(merged, containers)
	for i := range merged {
		name := strings.TrimPrefix(strings.TrimSpace(merged[i].Name), "/")
		if traffic, ok := trafficByName[name]; ok {
			merged[i].DiskReadBytesPS = traffic.DiskReadBytesPS
			merged[i].DiskWriteBytesPS = traffic.DiskWriteBytesPS
			merged[i].NetworkRxBytesPS = traffic.NetworkRxBytesPS
			merged[i].NetworkTxBytesPS = traffic.NetworkTxBytesPS
		}
	}
	return merged
}

func applyI3DResourceGPUStats(containers []i3dResourceContainerStats, processes []i3dResourceProcessStats, apps []i3dResourceGPUProcessStats) {
	if len(apps) == 0 {
		return
	}

	processIndexByPID := make(map[int32]int)
	for index, process := range processes {
		for _, pid := range process.PIDs {
			if pid > 0 {
				processIndexByPID[pid] = index
			}
		}
	}

	containerIndexByID := make(map[string]int)
	for index, container := range containers {
		id := strings.TrimSpace(container.ContainerID)
		if id != "" {
			containerIndexByID[id] = index
		}
	}

	for _, app := range apps {
		if app.PID > 0 {
			if index, ok := processIndexByPID[app.PID]; ok {
				processes[index].GPUMemoryBytes += app.UsedMemoryBytes
				continue
			}
		}
		if app.ContainerID == "" {
			continue
		}
		if index, ok := matchI3DResourceContainerIndex(containerIndexByID, app.ContainerID); ok {
			containers[index].GPUMemoryBytes += app.UsedMemoryBytes
		}
	}
}

func matchI3DResourceContainerIndex(containerIndexByID map[string]int, candidate string) (int, bool) {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return 0, false
	}
	for containerID, index := range containerIndexByID {
		if containerID == "" {
			continue
		}
		if strings.HasPrefix(candidate, containerID) || strings.HasPrefix(containerID, candidate) {
			return index, true
		}
	}
	return 0, false
}

func collectI3DResourceGPUProcessStats() []i3dResourceGPUProcessStats {
	output, err := exec.Command(
		"nvidia-smi",
		"--query-compute-apps=gpu_uuid,pid,process_name,used_gpu_memory",
		"--format=csv,noheader,nounits",
	).Output()
	if err != nil || len(output) == 0 {
		return nil
	}
	apps := parseI3DResourceNvidiaSMIComputeApps(string(output))
	for index := range apps {
		apps[index].ContainerID = detectI3DResourceGPUProcessContainerID(apps[index].PID)
	}
	return apps
}

func parseI3DResourceNvidiaSMIComputeApps(raw string) []i3dResourceGPUProcessStats {
	lines := strings.Split(raw, "\n")
	apps := make([]i3dResourceGPUProcessStats, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 4 {
			continue
		}
		gpuUUID := strings.TrimSpace(parts[0])
		pidValue, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 32)
		if err != nil || pidValue <= 0 {
			continue
		}
		processName := strings.TrimSpace(parts[2])
		memoryMiBText := strings.TrimSpace(strings.Join(parts[3:], ""))
		memoryMiBText = strings.TrimSuffix(memoryMiBText, "MiB")
		memoryMiBText = strings.TrimSpace(strings.ReplaceAll(memoryMiBText, ",", ""))
		memoryMiB, err := strconv.ParseUint(memoryMiBText, 10, 64)
		if err != nil {
			continue
		}
		apps = append(apps, i3dResourceGPUProcessStats{
			PID:             int32(pidValue),
			ProcessName:     processName,
			GPUUUID:         gpuUUID,
			UsedMemoryBytes: memoryMiB * 1024 * 1024,
		})
	}
	return apps
}

func detectI3DResourceGPUProcessContainerID(pid int32) string {
	if pid <= 0 {
		return ""
	}
	content, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return ""
	}
	return parseI3DResourceContainerIDFromCgroup(string(content))
}

func parseI3DResourceContainerIDFromCgroup(raw string) string {
	for _, field := range strings.FieldsFunc(raw, func(r rune) bool {
		return r == '/' || r == ':' || r == '\n'
	}) {
		field = strings.TrimSpace(field)
		if len(field) < 12 {
			continue
		}
		if strings.HasPrefix(field, "docker-") {
			field = strings.TrimPrefix(field, "docker-")
			field = strings.TrimSuffix(field, ".scope")
		}
		if isI3DResourceHexContainerID(field) {
			return field
		}
	}
	return ""
}

func isI3DResourceHexContainerID(value string) bool {
	if len(value) < 12 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return false
	}
	return true
}

func collectI3DResourceProcessStats(targets []i3dResourceTarget) []i3dResourceProcessStats {
	workspaceRoot := resolveI3DWorkspaceRoot()
	processTargets := make([]i3dResourceTarget, 0)
	for _, target := range targets {
		if target.Kind != i3dResourceKindProcess {
			continue
		}
		processTargets = append(processTargets, target)
	}

	statsByIndex := make([]i3dResourceProcessStats, len(processTargets))
	var wg sync.WaitGroup
	for i, target := range processTargets {
		wg.Add(1)
		go func(index int, target i3dResourceTarget) {
			defer wg.Done()
			statsByIndex[index] = collectI3DResourceProcessTarget(target, workspaceRoot)
		}(i, target)
	}
	wg.Wait()

	stats := make([]i3dResourceProcessStats, 0, len(statsByIndex))
	for _, stat := range statsByIndex {
		if stat.TargetID != "" {
			stats = append(stats, stat)
		}
	}
	return stats
}

func collectI3DResourceProcessTarget(target i3dResourceTarget, workspaceRoot string) i3dResourceProcessStats {
	stat := i3dResourceProcessStats{
		TargetID:   target.ID,
		Status:     i3dResourceStatusDown,
		UpdatedAt:  time.Now().Format(time.RFC3339),
		Diagnostic: "未找到声明的 PID 文件或进程",
	}
	pids, diagnostic, unknown := resolveI3DResourceTargetPIDs(target, workspaceRoot)
	stat.PIDs = append([]int32(nil), pids...)
	if len(pids) == 0 {
		if unknown {
			stat.Status = i3dResourceStatusUnknown
		}
		stat.Diagnostic = diagnostic
		return stat
	}

	processes := make([]*psutilprocess.Process, 0, len(pids))
	seen := map[int32]bool{}
	for _, pid := range pids {
		root, err := psutilprocess.NewProcess(pid)
		if err != nil || root == nil {
			continue
		}
		collectI3DResourceProcessTree(root, seen, &processes)
	}
	stat.PIDs = i3dResourceSortedPIDs(seen)

	cpuStart := collectI3DResourceProcessCPUSeconds(processes)
	cpuStartAt := time.Now()
	time.Sleep(i3dResourceProcessCPUInterval)
	cpuElapsed := time.Since(cpuStartAt)
	stat.CPUPercent = calculateI3DResourceProcessCPUPercent(
		cpuStart,
		collectI3DResourceProcessCPUSeconds(processes),
		cpuElapsed,
	)

	for _, proc := range processes {
		if proc == nil {
			continue
		}
		if memInfo, err := proc.MemoryInfo(); err == nil && memInfo != nil {
			stat.MemoryBytes += memInfo.RSS
			stat.MemoryRSSBytes += memInfo.RSS
		}
		if threads, err := proc.NumThreads(); err == nil && threads > 0 {
			stat.ThreadCount += int(threads)
		}
		if createTime, err := proc.CreateTime(); err == nil && createTime > 0 {
			uptime := uint64(time.Since(time.UnixMilli(createTime)).Seconds())
			if stat.UptimeSeconds == 0 || uptime < stat.UptimeSeconds {
				stat.UptimeSeconds = uptime
			}
		}
		stat.UnitCount++
	}
	if stat.UnitCount > 0 {
		stat.Status = i3dResourceStatusUp
		stat.Diagnostic = ""
	}
	return stat
}

func collectI3DResourceProcessTree(root *psutilprocess.Process, seen map[int32]bool, processes *[]*psutilprocess.Process) {
	if root == nil || seen[root.Pid] {
		return
	}
	seen[root.Pid] = true
	*processes = append(*processes, root)
	children, err := root.Children()
	if err != nil {
		return
	}
	for _, child := range children {
		collectI3DResourceProcessTree(child, seen, processes)
	}
}

func i3dResourceSortedPIDs(seen map[int32]bool) []int32 {
	pids := make([]int32, 0, len(seen))
	for pid := range seen {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
	return pids
}

func collectI3DResourceProcessCPUSeconds(processes []*psutilprocess.Process) map[int32]float64 {
	samples := make(map[int32]float64, len(processes))
	for _, proc := range processes {
		if proc == nil || proc.Pid <= 0 {
			continue
		}
		times, err := proc.Times()
		if err != nil || times == nil {
			continue
		}
		samples[proc.Pid] = times.User + times.System
	}
	return samples
}

func calculateI3DResourceProcessCPUPercent(start map[int32]float64, end map[int32]float64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return 0
	}
	totalDelta := 0.0
	for pid, startCPU := range start {
		endCPU, ok := end[pid]
		if !ok || endCPU <= startCPU {
			continue
		}
		totalDelta += endCPU - startCPU
	}
	if totalDelta <= 0 {
		return 0
	}
	return totalDelta / elapsed.Seconds() * 100
}

func resolveI3DResourceTargetPIDs(target i3dResourceTarget, workspaceRoot string) ([]int32, string, bool) {
	if strings.TrimSpace(target.PIDFile) != "" {
		pidFile := resolveI3DResourcePIDFile(workspaceRoot, target.PIDFile)
		content, err := os.ReadFile(pidFile)
		if err == nil {
			pidValue, err := strconv.ParseInt(strings.TrimSpace(string(content)), 10, 32)
			if err != nil || pidValue <= 0 {
				return nil, "PID 文件内容无效", true
			}
			if _, err := psutilprocess.NewProcess(int32(pidValue)); err == nil {
				return []int32{int32(pidValue)}, "", false
			}
		}
	}
	if len(target.Ports) > 0 {
		pids := findI3DResourceListeningPortPIDs(target.Ports)
		if len(pids) > 0 {
			return pids, "", false
		}
		return nil, "未找到声明端口的监听进程", false
	}
	if len(target.CommandIncludes) > 0 {
		workingDir := resolveI3DResourcePath(workspaceRoot, target.WorkingDir)
		pids := findI3DResourceCommandPIDs(target.CommandIncludes, workingDir)
		if len(pids) > 0 {
			return pids, "", false
		}
		return nil, "未找到声明命令匹配的进程", false
	}
	return nil, "未找到声明的 PID 文件或进程", false
}

func findI3DResourceListeningPortPIDs(ports []uint32) []int32 {
	portSet := make(map[uint32]bool, len(ports))
	for _, port := range ports {
		if port > 0 {
			portSet[port] = true
		}
	}
	connections, err := psutilnet.Connections("tcp")
	if err != nil {
		return nil
	}
	seen := map[int32]bool{}
	pids := make([]int32, 0)
	for _, connection := range connections {
		if !portSet[connection.Laddr.Port] || strings.ToUpper(connection.Status) != "LISTEN" || connection.Pid <= 0 {
			continue
		}
		if seen[connection.Pid] {
			continue
		}
		seen[connection.Pid] = true
		pids = append(pids, connection.Pid)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
	return pids
}

func findI3DResourceCommandPIDs(includes []string, workingDir string) []int32 {
	patterns := make([]string, 0, len(includes))
	for _, include := range includes {
		include = strings.TrimSpace(include)
		if include != "" {
			patterns = append(patterns, include)
		}
	}
	if len(patterns) == 0 {
		return nil
	}
	workingDir = filepath.Clean(strings.TrimSpace(workingDir))
	requireWorkingDir := workingDir != "" && workingDir != "."

	processes, err := psutilprocess.Processes()
	if err != nil {
		return nil
	}
	pids := make([]int32, 0)
	seen := map[int32]bool{}
	for _, process := range processes {
		if process == nil || process.Pid <= 0 || seen[process.Pid] {
			continue
		}
		command, err := process.Cmdline()
		if err != nil || strings.TrimSpace(command) == "" {
			continue
		}
		if requireWorkingDir {
			cwd, err := process.Cwd()
			if err != nil || filepath.Clean(cwd) != workingDir {
				continue
			}
		}
		matched := true
		for _, pattern := range patterns {
			if !strings.Contains(command, pattern) {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		seen[process.Pid] = true
		pids = append(pids, process.Pid)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
	return pids
}

func resolveI3DResourcePIDFile(workspaceRoot string, pidFile string) string {
	return resolveI3DResourcePath(workspaceRoot, pidFile)
}

func resolveI3DResourcePath(workspaceRoot string, path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	if strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return filepath.Join(workspaceRoot, strings.TrimPrefix(path, ".."+string(filepath.Separator)))
	}
	return filepath.Join(workspaceRoot, "beszel", path)
}

func resolveI3DWorkspaceRoot() string {
	if root, ok := GetEnv("I3D_WORKSPACE_ROOT"); ok && strings.TrimSpace(root) != "" {
		return strings.TrimSpace(root)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		if fileExists(filepath.Join(dir, "i3d-search-service")) && fileExists(filepath.Join(dir, "beszel")) {
			return dir
		}
		if filepath.Base(dir) == "beszel" {
			parent := filepath.Dir(dir)
			if fileExists(filepath.Join(parent, "i3d-search-service")) {
				return parent
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return cwd
		}
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func mapI3DResourceContainerStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch {
	case status == "running" || status == "healthy" || strings.Contains(status, "up"):
		return i3dResourceStatusUp
	case status == "":
		return i3dResourceStatusUnknown
	default:
		return i3dResourceStatusDown
	}
}

func i3dResourceGroupName(group string) string {
	switch group {
	case i3dResourceGroupBusiness:
		return "业务服务"
	case i3dResourceGroupMiddleware:
		return "中间件"
	case i3dResourceGroupFrontend:
		return "前端"
	case i3dResourceGroupMonitor:
		return "监控"
	default:
		return group
	}
}

func i3dResourceGroupOrder(group string) int {
	switch group {
	case i3dResourceGroupBusiness:
		return 10
	case i3dResourceGroupMiddleware:
		return 20
	case i3dResourceGroupFrontend:
		return 30
	case i3dResourceGroupMonitor:
		return 40
	default:
		return 100
	}
}

func (h *Hub) getI3DResourceOverview(e *core.RequestEvent) error {
	environment := resolveI3DResourceEnvironment(e)
	if h.i3dResources == nil {
		h.i3dResources = newI3DResourceOverviewCache(5 * time.Second)
	}
	response, err := h.i3dResources.get(environment, time.Now(), func() (i3dResourceOverviewDTO, error) {
		return h.collectI3DResourceOverview(environment)
	}, func(fn func()) {
		go fn()
	})
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, response)
}

func (h *Hub) collectI3DResourceOverview(environment string) (i3dResourceOverviewDTO, error) {
	targets := defaultI3DResourceTargets(environment)
	containers, err := h.latestI3DResourceContainerStats(targets)
	if err != nil {
		return i3dResourceOverviewDTO{}, err
	}
	processes := collectI3DResourceProcessStats(targets)
	applyI3DResourceGPUStats(containers, processes, collectI3DResourceGPUProcessStats())
	overview := buildI3DResourceOverview(environment, targets, containers, processes, nil)
	if h.i3dHistory == nil {
		h.i3dHistory = newI3DResourceTimeseriesHistory(240)
	}
	h.i3dHistory.record(overview)
	return overview, nil
}

func (h *Hub) warmI3DResourceOverviewCache() {
	environment := i3dResourceEnvironmentLocal
	if env, ok := GetEnv("AETHER_I3D_RESOURCE_ENV"); ok {
		environment = normalizeI3DResourceEnvironment(env)
	}
	if h.i3dResources == nil {
		h.i3dResources = newI3DResourceOverviewCache(5 * time.Second)
	}
	_, _ = h.i3dResources.get(environment, time.Now(), func() (i3dResourceOverviewDTO, error) {
		return h.collectI3DResourceOverview(environment)
	}, func(fn func()) {
		go fn()
	})
}

func resolveI3DResourceEnvironment(e *core.RequestEvent) string {
	environmentParam := strings.TrimSpace(e.Request.URL.Query().Get("environment"))
	environment := normalizeI3DResourceEnvironment(environmentParam)
	if environmentParam == "" {
		if env, ok := GetEnv("AETHER_I3D_RESOURCE_ENV"); ok {
			environment = normalizeI3DResourceEnvironment(env)
		}
	}
	return environment
}

func (h *Hub) latestI3DResourceContainerStats(targets []i3dResourceTarget) ([]i3dResourceContainerStats, error) {
	if containers, err := collectI3DResourceDockerStats(targets); err == nil && len(containers) > 0 {
		return containers, nil
	}

	type containerRow struct {
		Name    string  `db:"name"`
		Status  string  `db:"status"`
		CPU     float64 `db:"cpu"`
		Memory  float64 `db:"memory"`
		Net     float64 `db:"net"`
		Uptime  uint64  `db:"uptime"`
		Updated int64   `db:"updated"`
	}
	rows := []containerRow{}
	err := h.DB().
		Select("name", "status", "cpu", "memory", "net", "uptime", "updated").
		From("containers").
		OrderBy("updated DESC").
		All(&rows)
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	containers := make([]i3dResourceContainerStats, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimPrefix(strings.TrimSpace(row.Name), "/")
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		updatedAt := ""
		if row.Updated > 0 {
			updatedAt = time.UnixMilli(row.Updated).UTC().Format(time.RFC3339)
		}
		netBytes := uint64(0)
		if row.Net > 0 {
			netBytes = uint64(row.Net * 1024 * 1024)
		}
		containers = append(containers, i3dResourceContainerStats{
			Name:             name,
			Status:           row.Status,
			CPUPercent:       row.CPU,
			MemoryBytes:      uint64(row.Memory * 1024 * 1024),
			NetworkRxBytesPS: netBytes,
			UptimeSeconds:    row.Uptime,
			UpdatedAt:        updatedAt,
		})
	}
	if history, err := h.latestI3DResourceContainerHistory(1); err == nil && len(history) > 0 {
		containers = mergeI3DResourceContainerTraffic(containers, history[len(history)-1].Containers)
	}
	return containers, nil
}

func collectI3DResourceDockerStats(targets []i3dResourceTarget) ([]i3dResourceContainerStats, error) {
	targetNames := map[string]bool{}
	for _, target := range targets {
		if target.Kind != i3dResourceKindDocker {
			continue
		}
		name := strings.TrimPrefix(strings.TrimSpace(target.ContainerName), "/")
		if name != "" {
			targetNames[name] = true
		}
	}
	if len(targetNames) == 0 {
		return nil, nil
	}

	client, err := newI3DResourceDockerClient()
	if err != nil {
		return nil, err
	}
	containers, err := listI3DResourceDockerContainers(client)
	if err != nil {
		return nil, err
	}
	resultCh := make(chan i3dResourceContainerStats, len(targetNames))
	var wg sync.WaitGroup
	for _, item := range containers {
		name := item.primaryName()
		if !targetNames[name] {
			continue
		}
		wg.Add(1)
		go func(item i3dResourceDockerContainer, name string) {
			defer wg.Done()
			stat := i3dResourceContainerStats{
				Name:          name,
				ContainerID:   item.ID,
				Status:        item.State,
				UptimeSeconds: item.uptimeSeconds(),
				UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
			}
			if isI3DResourceDockerRunningState(item.State) {
				apiStats, err := readI3DResourceDockerContainerStats(client, item.ID)
				if err == nil {
					applyI3DResourceDockerAPIStats(&stat, item.ID, apiStats)
				} else {
					stat.Diagnostic = i3dResourceDiagnostic("Docker stats 采集失败", err)
				}
			}
			resultCh <- stat
		}(item, name)
	}
	wg.Wait()
	close(resultCh)

	result := make([]i3dResourceContainerStats, 0, len(targetNames))
	for stat := range resultCh {
		result = append(result, stat)
	}
	return result, nil
}

type i3dResourceDockerContainer struct {
	ID        string   `json:"Id"`
	Names     []string `json:"Names"`
	State     string   `json:"State"`
	Status    string   `json:"Status"`
	Created   int64    `json:"Created"`
	StartedAt string   `json:"StartedAt"`
}

func (c i3dResourceDockerContainer) primaryName() string {
	if len(c.Names) == 0 {
		return ""
	}
	return strings.TrimPrefix(strings.TrimSpace(c.Names[0]), "/")
}

func (c i3dResourceDockerContainer) uptimeSeconds() uint64 {
	if c.StartedAt != "" {
		if startedAt, err := time.Parse(time.RFC3339Nano, c.StartedAt); err == nil && !startedAt.IsZero() && startedAt.Before(time.Now()) {
			return uint64(time.Since(startedAt).Seconds())
		}
	}
	if c.Created > 0 {
		createdAt := time.Unix(c.Created, 0)
		if createdAt.Before(time.Now()) {
			return uint64(time.Since(createdAt).Seconds())
		}
	}
	return 0
}

func newI3DResourceDockerClient() (*http.Client, error) {
	dockerHost := "unix:///var/run/docker.sock"
	if host, ok := GetEnv("DOCKER_HOST"); ok && strings.TrimSpace(host) != "" {
		dockerHost = strings.TrimSpace(host)
	}
	if !strings.HasPrefix(dockerHost, "unix://") {
		return nil, fmt.Errorf("unsupported DOCKER_HOST for i3d resource monitor: %s", dockerHost)
	}
	socketPath := strings.TrimPrefix(dockerHost, "unix://")
	if _, err := os.Stat(socketPath); err != nil {
		return nil, err
	}
	return &http.Client{
		Timeout: i3dResourceDockerStatsTimeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network string, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		},
	}, nil
}

func listI3DResourceDockerContainers(client *http.Client) ([]i3dResourceDockerContainer, error) {
	resp, err := client.Get("http://docker/containers/json?all=1")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker containers api returned %s", resp.Status)
	}
	containers := []i3dResourceDockerContainer{}
	if err := json.NewDecoder(resp.Body).Decode(&containers); err != nil {
		return nil, err
	}
	return containers, nil
}

func readI3DResourceDockerContainerStats(client *http.Client, containerID string) (*aethercontainer.ApiStats, error) {
	stats, err := readI3DResourceDockerContainerStatsURL(client, containerID, "stream=0")
	if err == nil {
		return stats, nil
	}
	fallbackStats, fallbackErr := readI3DResourceDockerContainerStatsURL(client, containerID, "stream=false&one-shot=true")
	if fallbackErr == nil {
		return fallbackStats, nil
	}
	return nil, fmt.Errorf("stream=0: %v; one-shot: %w", err, fallbackErr)
}

func readI3DResourceDockerContainerStatsURL(client *http.Client, containerID string, query string) (*aethercontainer.ApiStats, error) {
	resp, err := client.Get("http://docker/containers/" + containerID + "/stats?" + query)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("docker stats api returned %s", resp.Status)
	}
	stats := &aethercontainer.ApiStats{}
	if err := json.NewDecoder(resp.Body).Decode(stats); err != nil {
		return nil, err
	}
	return stats, nil
}

func applyI3DResourceDockerAPIStats(stat *i3dResourceContainerStats, containerID string, apiStats *aethercontainer.ApiStats) {
	stat.CPUPercent = 0
	applyI3DResourceDockerMemoryStats(stat, apiStats)
	stat.PIDCount = int(apiStats.PidsStats.Current)
	totalCPU := apiStats.CPUStats.CPUUsage.TotalUsage
	totalSystem := apiStats.CPUStats.SystemUsage
	totalRead, totalWrite := calculateI3DResourceDockerDiskTotals(apiStats)
	totalRx, totalTx := calculateI3DResourceDockerNetworkTotals(apiStats)

	now := time.Now()
	i3dResourceDockerStatsCache.Lock()
	if apiStats.PreCPUStats.CPUUsage.TotalUsage > 0 && apiStats.PreCPUStats.SystemUsage > 0 {
		stat.CPUPercent = calculateI3DResourceDockerCPUPercent(apiStats)
	}
	if previous, ok := i3dResourceDockerStatsCache.samples[containerID]; ok {
		if stat.CPUPercent == 0 && previous.cpuTotal > 0 && previous.systemTotal > 0 && totalCPU > previous.cpuTotal && totalSystem > previous.systemTotal {
			stat.CPUPercent = calculateI3DResourceDockerCPUPercentFromDeltas(
				totalCPU-previous.cpuTotal,
				totalSystem-previous.systemTotal,
				apiStats.CPUStats.OnlineCPUs,
			)
		}
		elapsedMs := uint64(now.Sub(previous.readAt).Milliseconds())
		if elapsedMs > 0 {
			stat.DiskReadBytesPS = calculateI3DResourceRate(totalRead, previous.readBytes, elapsedMs)
			stat.DiskWriteBytesPS = calculateI3DResourceRate(totalWrite, previous.writeBytes, elapsedMs)
			stat.NetworkRxBytesPS = calculateI3DResourceRate(totalRx, previous.rxBytes, elapsedMs)
			stat.NetworkTxBytesPS = calculateI3DResourceRate(totalTx, previous.txBytes, elapsedMs)
		}
	}
	i3dResourceDockerStatsCache.samples[containerID] = i3dResourceDockerCumulativeSample{
		cpuTotal:    totalCPU,
		systemTotal: totalSystem,
		readBytes:   totalRead,
		writeBytes:  totalWrite,
		rxBytes:     totalRx,
		txBytes:     totalTx,
		readAt:      now,
	}
	i3dResourceDockerStatsCache.Unlock()
}

func i3dResourceDiagnostic(prefix string, err error) string {
	message := strings.TrimSpace(prefix)
	if err != nil {
		if message != "" {
			message += ": "
		}
		message += strings.TrimSpace(err.Error())
	}
	if len(message) > 240 {
		message = message[:240] + "..."
	}
	return message
}

func calculateI3DResourceDockerCPUPercent(apiStats *aethercontainer.ApiStats) float64 {
	if apiStats.PreCPUStats.CPUUsage.TotalUsage == 0 || apiStats.PreCPUStats.SystemUsage == 0 {
		return 0
	}
	if apiStats.CPUStats.CPUUsage.TotalUsage <= apiStats.PreCPUStats.CPUUsage.TotalUsage ||
		apiStats.CPUStats.SystemUsage <= apiStats.PreCPUStats.SystemUsage {
		return 0
	}
	return calculateI3DResourceDockerCPUPercentFromDeltas(
		apiStats.CPUStats.CPUUsage.TotalUsage-apiStats.PreCPUStats.CPUUsage.TotalUsage,
		apiStats.CPUStats.SystemUsage-apiStats.PreCPUStats.SystemUsage,
		apiStats.CPUStats.OnlineCPUs,
	)
}

func calculateI3DResourceDockerCPUPercentFromDeltas(cpuDelta uint64, systemDelta uint64, onlineCPUs int) float64 {
	if cpuDelta <= 0 || systemDelta <= 0 {
		return 0
	}
	if onlineCPUs <= 0 {
		onlineCPUs = 1
	}
	return float64(cpuDelta) / float64(systemDelta) * float64(onlineCPUs) * 100
}

func calculateI3DResourceDockerMemoryBytes(apiStats *aethercontainer.ApiStats) uint64 {
	cache := firstNonZeroUint64(
		apiStats.MemoryStats.Stats.TotalInactiveFile,
		apiStats.MemoryStats.Stats.InactiveFile,
	)
	if cache == 0 {
		cache = firstNonZeroUint64(
			apiStats.MemoryStats.Stats.TotalCache,
			apiStats.MemoryStats.Stats.Cache,
			apiStats.MemoryStats.Stats.File,
		)
	}
	if apiStats.MemoryStats.Usage > cache {
		return apiStats.MemoryStats.Usage - cache
	}
	return apiStats.MemoryStats.Usage
}

func applyI3DResourceDockerMemoryStats(stat *i3dResourceContainerStats, apiStats *aethercontainer.ApiStats) {
	memoryStats := apiStats.MemoryStats.Stats
	stat.MemoryUsageBytes = apiStats.MemoryStats.Usage
	stat.MemoryInactiveFileBytes = firstNonZeroUint64(memoryStats.TotalInactiveFile, memoryStats.InactiveFile)
	stat.MemoryCacheBytes = firstNonZeroUint64(memoryStats.TotalCache, memoryStats.Cache, memoryStats.File)
	stat.MemoryRSSBytes = firstNonZeroUint64(memoryStats.TotalRSS, memoryStats.RSS, memoryStats.Anon)
	stat.MemoryAnonBytes = calculateI3DResourceDockerAnonBytes(memoryStats)
	stat.MemoryBytes = calculateI3DResourceDockerMemoryBytes(apiStats)
}

func calculateI3DResourceDockerAnonBytes(memoryStats aethercontainer.MemoryStatsStats) uint64 {
	if memoryStats.Anon > 0 {
		return memoryStats.Anon
	}
	if value := sumNonZeroUint64(memoryStats.TotalActiveAnon, memoryStats.TotalInactiveAnon); value > 0 {
		return value
	}
	if value := sumNonZeroUint64(memoryStats.ActiveAnon, memoryStats.InactiveAnon); value > 0 {
		return value
	}
	return firstNonZeroUint64(memoryStats.TotalRSS, memoryStats.RSS)
}

func firstNonZeroUint64(values ...uint64) uint64 {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func sumNonZeroUint64(values ...uint64) uint64 {
	var total uint64
	for _, value := range values {
		total += value
	}
	return total
}

func calculateI3DResourceDockerDiskTotals(apiStats *aethercontainer.ApiStats) (uint64, uint64) {
	var totalRead, totalWrite uint64
	for _, entry := range apiStats.BlkioStats.IoServiceBytesRecursive {
		switch strings.ToLower(strings.TrimSpace(entry.Op)) {
		case "read":
			totalRead += entry.Value
		case "write":
			totalWrite += entry.Value
		}
	}
	return totalRead, totalWrite
}

func calculateI3DResourceDockerNetworkTotals(apiStats *aethercontainer.ApiStats) (uint64, uint64) {
	var totalRx, totalTx uint64
	for _, item := range apiStats.Networks {
		totalRx += item.RxBytes
		totalTx += item.TxBytes
	}
	return totalRx, totalTx
}

func calculateI3DResourceRate(current uint64, previous uint64, elapsedMs uint64) uint64 {
	if elapsedMs == 0 || current <= previous {
		return 0
	}
	return (current - previous) * 1000 / elapsedMs
}

func isI3DResourceDockerRunningState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "running", "healthy", "paused":
		return true
	default:
		return false
	}
}

func (h *Hub) latestI3DResourceContainerHistory(limit int) ([]i3dResourceContainerHistoryPoint, error) {
	if limit <= 0 || limit > 1000 {
		limit = 240
	}
	type containerStatsRow struct {
		Created string `db:"created"`
		Stats   []byte `db:"stats"`
	}
	rows := []containerStatsRow{}
	err := h.DB().
		NewQuery("SELECT created, stats FROM container_stats WHERE type = '1m' ORDER BY created DESC LIMIT {:limit}").
		Bind(map[string]any{"limit": limit}).
		All(&rows)
	if err != nil {
		return nil, err
	}
	history := make([]i3dResourceContainerHistoryPoint, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		containers, err := parseI3DResourceContainerStatsJSON(rows[i].Stats)
		if err != nil {
			continue
		}
		history = append(history, i3dResourceContainerHistoryPoint{
			Timestamp:  rows[i].Created,
			Containers: containers,
		})
	}
	return history, nil
}

func parseI3DResourceContainerStatsJSON(raw []byte) ([]i3dResourceContainerStats, error) {
	type compactContainerStats struct {
		Name        string  `json:"n"`
		CPU         float64 `json:"c"`
		MemoryMB    float64 `json:"m"`
		DiskRead    float64 `json:"dr"`
		DiskWrite   float64 `json:"dw"`
		NetworkSent float64 `json:"ns"`
		NetworkRecv float64 `json:"nr"`
		Uptime      uint64  `json:"u"`
	}
	compact := []compactContainerStats{}
	if err := json.Unmarshal(raw, &compact); err != nil {
		return nil, err
	}
	containers := make([]i3dResourceContainerStats, 0, len(compact))
	for _, item := range compact {
		containers = append(containers, i3dResourceContainerStats{
			Name:             strings.TrimPrefix(strings.TrimSpace(item.Name), "/"),
			CPUPercent:       item.CPU,
			MemoryBytes:      uint64(item.MemoryMB * 1024 * 1024),
			DiskReadBytesPS:  uint64(item.DiskRead * 1024 * 1024),
			DiskWriteBytesPS: uint64(item.DiskWrite * 1024 * 1024),
			NetworkRxBytesPS: uint64(item.NetworkRecv * 1024 * 1024),
			NetworkTxBytesPS: uint64(item.NetworkSent * 1024 * 1024),
			UptimeSeconds:    item.Uptime,
		})
	}
	return containers, nil
}

func (h *Hub) listI3DResourceTargets(e *core.RequestEvent) error {
	return h.getI3DResourceOverview(e)
}

func (h *Hub) getI3DResourceTimeseries(e *core.RequestEvent) error {
	environment := resolveI3DResourceEnvironment(e)
	if h.i3dHistory == nil {
		h.i3dHistory = newI3DResourceTimeseriesHistory(240)
	}
	response := h.i3dHistory.snapshot(environment)
	return e.JSON(http.StatusOK, response)
}

func (h *Hub) getI3DResourceMiddleware(e *core.RequestEvent) error {
	return e.JSON(http.StatusOK, map[string]any{"items": []any{}})
}

func (h *Hub) getI3DResourceDiagnostics(e *core.RequestEvent) error {
	type diagnostic struct {
		Level   string `json:"level"`
		Message string `json:"message"`
	}
	return e.JSON(http.StatusOK, map[string]any{
		"items": []diagnostic{
			{Level: "info", Message: "资源统计只来自显式 i3d 采集目标配置。"},
		},
	})
}

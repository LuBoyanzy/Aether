package hub

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	aethercontainer "aether/internal/entities/container"
)

func TestDefaultI3DResourceTargetsReleaseAreExact(t *testing.T) {
	targets := defaultI3DResourceTargets("release")

	expected := []string{
		"release.aether",
		"release.search.web",
		"release.inference.web",
		"release.file.web",
		"release.file.consumer",
		"release.file.worker",
		"release.cad.web",
		"release.cad.batch_worker",
		"release.cad.query_worker",
		"release.cad.compare_worker",
		"release.middleware.postgres",
		"release.middleware.redis",
		"release.middleware.rabbitmq",
		"release.middleware.minio",
		"release.middleware.xxl_mysql",
		"release.middleware.xxl_admin",
	}

	if len(targets) != len(expected) {
		t.Fatalf("expected %d release targets, got %d: %#v", len(expected), len(targets), targets)
	}
	for i, id := range expected {
		if targets[i].ID != id {
			t.Fatalf("target %d should be %s, got %s", i, id, targets[i].ID)
		}
	}
}

func TestDefaultI3DResourceTargetsLocalAreExact(t *testing.T) {
	targets := defaultI3DResourceTargets("local")

	expected := []string{
		"local.search.web",
		"local.inference.web",
		"local.file.web",
		"local.file.worker",
		"local.file.consumer",
		"local.cad.web",
		"local.cad.worker",
		"local.aether",
		"local.middleware.postgres",
		"local.middleware.redis",
		"local.middleware.rabbitmq",
		"local.middleware.minio",
		"local.middleware.xxl_mysql",
		"local.middleware.xxl_admin",
	}

	if len(targets) != len(expected) {
		t.Fatalf("expected %d local targets, got %d: %#v", len(expected), len(targets), targets)
	}
	for i, id := range expected {
		if targets[i].ID != id {
			t.Fatalf("target %d should be %s, got %s", i, id, targets[i].ID)
		}
	}

	byID := map[string]i3dResourceTarget{}
	for _, target := range targets {
		byID[target.ID] = target
	}
	if byID["local.search.web"].Ports[0] != 39200 {
		t.Fatalf("local search should be identified by port 39200, got %#v", byID["local.search.web"].Ports)
	}
	if byID["local.aether"].Ports[0] != 19100 {
		t.Fatalf("local aether should be identified by port 19100, got %#v", byID["local.aether"].Ports)
	}
	if _, ok := byID["local.frontend.webviews"]; ok {
		t.Fatalf("local frontend webviews should not be part of i3d resource monitoring")
	}
	if _, ok := byID["local.frontend.container"]; ok {
		t.Fatalf("local electron container should not be part of i3d resource monitoring")
	}
	if len(byID["local.file.worker"].CommandIncludes) == 0 {
		t.Fatalf("local file worker should be identified by an explicit command matcher")
	}
	if len(byID["local.file.consumer"].CommandIncludes) == 0 {
		t.Fatalf("local file consumer should be identified by an explicit command matcher")
	}
	if len(byID["local.cad.worker"].CommandIncludes) == 0 {
		t.Fatalf("local cad worker should be identified by an explicit command matcher")
	}
}

func TestDefaultI3DResourceTargetsReleaseUsesExplicitContainerPrefix(t *testing.T) {
	t.Setenv("CONTAINER_PREFIX", "customer-i3d")

	targets := defaultI3DResourceTargets("release")
	byID := map[string]i3dResourceTarget{}
	for _, target := range targets {
		byID[target.ID] = target
	}

	if byID["release.search.web"].ContainerName != "customer-i3d-search-service" {
		t.Fatalf("release search container should follow CONTAINER_PREFIX, got %s", byID["release.search.web"].ContainerName)
	}
	if byID["release.middleware.postgres"].ContainerName != "customer-i3d-postgres" {
		t.Fatalf("release postgres container should follow CONTAINER_PREFIX, got %s", byID["release.middleware.postgres"].ContainerName)
	}
	if byID["release.aether"].Kind != i3dResourceKindProcess {
		t.Fatalf("release aether should be tracked as a host process, got %s", byID["release.aether"].Kind)
	}
	if byID["release.aether"].ContainerName != "" {
		t.Fatalf("release aether should not expect a docker container, got %s", byID["release.aether"].ContainerName)
	}
	if byID["release.aether"].Ports[0] != 19101 {
		t.Fatalf("release aether should default to port 19101, got %#v", byID["release.aether"].Ports)
	}
}

func TestDefaultI3DResourceTargetsReleaseUsesExplicitAetherPort(t *testing.T) {
	t.Setenv("AETHER_HOST_PORT", "19199")

	targets := defaultI3DResourceTargets("release")
	byID := map[string]i3dResourceTarget{}
	for _, target := range targets {
		byID[target.ID] = target
	}

	if byID["release.aether"].Ports[0] != 19199 {
		t.Fatalf("release aether should follow AETHER_HOST_PORT, got %#v", byID["release.aether"].Ports)
	}
}

func TestDefaultI3DResourceTargetsLocalUsesExplicitMiddlewareContainerPrefix(t *testing.T) {
	t.Setenv("I3D_MIDDLEWARE_CONTAINER_PREFIX", "dev-stack")

	targets := defaultI3DResourceTargets("local")
	byID := map[string]i3dResourceTarget{}
	for _, target := range targets {
		byID[target.ID] = target
	}

	if byID["local.middleware.postgres"].ContainerName != "dev-stack-postgres" {
		t.Fatalf("local postgres container should follow I3D_MIDDLEWARE_CONTAINER_PREFIX, got %s", byID["local.middleware.postgres"].ContainerName)
	}
	if byID["local.middleware.xxl_admin"].ContainerName != "dev-stack-xxl-job-admin" {
		t.Fatalf("local xxl admin container should follow I3D_MIDDLEWARE_CONTAINER_PREFIX, got %s", byID["local.middleware.xxl_admin"].ContainerName)
	}
}

func TestBuildI3DResourceOverviewOnlyCountsConfiguredTargets(t *testing.T) {
	targets := []i3dResourceTarget{
		{
			ID:            "release.search.web",
			Name:          "检索服务",
			Group:         i3dResourceGroupBusiness,
			Kind:          i3dResourceKindDocker,
			ContainerName: "i3d-release-search-service",
		},
		{
			ID:            "release.middleware.postgres",
			Name:          "PostgreSQL + pgvector",
			Group:         i3dResourceGroupMiddleware,
			Kind:          i3dResourceKindDocker,
			ContainerName: "i3d-release-postgres",
		},
	}
	containers := []i3dResourceContainerStats{
		{
			Name:             "i3d-release-search-service",
			Status:           "running",
			CPUPercent:       120.5,
			MemoryBytes:      1024,
			MemoryUsageBytes: 1400,
			MemoryRSSBytes:   900,
			MemoryCacheBytes: 300,
			MemoryAnonBytes:  850,
			PIDCount:         4,
			NetworkRxBytesPS: 100,
			NetworkTxBytesPS: 20,
			UptimeSeconds:    10,
		},
		{
			Name:             "i3d-release-postgres",
			Status:           "running",
			CPUPercent:       30,
			MemoryBytes:      2048,
			MemoryUsageBytes: 2600,
			MemoryRSSBytes:   1200,
			MemoryCacheBytes: 500,
			MemoryAnonBytes:  1100,
			PIDCount:         6,
			NetworkRxBytesPS: 200,
			NetworkTxBytesPS: 40,
			UptimeSeconds:    20,
		},
		{
			Name:             "vscode-dev-container",
			Status:           "running",
			CPUPercent:       900,
			MemoryBytes:      999999,
			NetworkRxBytesPS: 999,
			NetworkTxBytesPS: 999,
			UptimeSeconds:    30,
		},
	}

	response := buildI3DResourceOverview("release", targets, containers, nil, nil)

	if len(response.Items) != 2 {
		t.Fatalf("expected exactly 2 configured targets, got %d", len(response.Items))
	}
	if response.Summary.CPUPercent != 150.5 {
		t.Fatalf("summary CPU should only include configured targets, got %.2f", response.Summary.CPUPercent)
	}
	if response.Summary.MemoryBytes != 3072 {
		t.Fatalf("summary memory should only include configured targets, got %d", response.Summary.MemoryBytes)
	}
	if response.Summary.MemoryUsageBytes != 4000 || response.Summary.MemoryRSSBytes != 2100 || response.Summary.MemoryCacheBytes != 800 || response.Summary.MemoryAnonBytes != 1950 {
		t.Fatalf("summary memory breakdown should only include configured targets, got %#v", response.Summary)
	}
	if response.Summary.PIDCount != 10 {
		t.Fatalf("summary pids should only include configured targets, got %d", response.Summary.PIDCount)
	}
	if response.Summary.NetworkRxBytesPS != 300 || response.Summary.NetworkTxBytesPS != 60 {
		t.Fatalf("summary network should only include configured targets, got rx=%d tx=%d", response.Summary.NetworkRxBytesPS, response.Summary.NetworkTxBytesPS)
	}
	for _, item := range response.Items {
		if item.Name == "vscode-dev-container" || item.ContainerName == "vscode-dev-container" {
			t.Fatalf("unconfigured container leaked into i3d resources: %#v", item)
		}
	}
}

func TestBuildI3DResourceOverviewSumsEveryDisplayedTargetExactly(t *testing.T) {
	targets := []i3dResourceTarget{
		{
			ID:            "release.search.web",
			Name:          "检索服务",
			Group:         i3dResourceGroupBusiness,
			Kind:          i3dResourceKindDocker,
			ContainerName: "i3d-release-search-service",
		},
		{
			ID:            "release.middleware.redis",
			Name:          "Redis",
			Group:         i3dResourceGroupMiddleware,
			Kind:          i3dResourceKindDocker,
			ContainerName: "i3d-release-redis",
		},
		{
			ID:      "local.aether",
			Name:    "本地 Aether",
			Group:   i3dResourceGroupMonitor,
			Kind:    i3dResourceKindProcess,
			PIDFile: "run/hub/aether.pid",
		},
	}
	containers := []i3dResourceContainerStats{
		{
			Name:             "/i3d-release-search-service",
			Status:           "running",
			CPUPercent:       25.5,
			MemoryBytes:      1000,
			MemoryRSSBytes:   700,
			MemoryCacheBytes: 150,
			PIDCount:         3,
			DiskReadBytesPS:  10,
			DiskWriteBytesPS: 20,
			NetworkRxBytesPS: 30,
			NetworkTxBytesPS: 40,
		},
		{
			Name:             "i3d-release-redis",
			Status:           "running",
			CPUPercent:       5,
			MemoryBytes:      2000,
			MemoryRSSBytes:   1600,
			MemoryCacheBytes: 200,
			PIDCount:         4,
			DiskReadBytesPS:  50,
			DiskWriteBytesPS: 60,
			NetworkRxBytesPS: 70,
			NetworkTxBytesPS: 80,
		},
		{
			Name:             "unconfigured-container",
			Status:           "running",
			CPUPercent:       999,
			MemoryBytes:      999999,
			DiskReadBytesPS:  999,
			DiskWriteBytesPS: 999,
			NetworkRxBytesPS: 999,
			NetworkTxBytesPS: 999,
		},
	}
	processes := []i3dResourceProcessStats{
		{
			TargetID:         "local.aether",
			Status:           i3dResourceStatusUp,
			CPUPercent:       10,
			MemoryBytes:      3000,
			MemoryRSSBytes:   3000,
			DiskReadBytesPS:  90,
			DiskWriteBytesPS: 100,
			NetworkRxBytesPS: 110,
			NetworkTxBytesPS: 120,
			UnitCount:        1,
			ThreadCount:      8,
			PIDs:             []int32{101, 102},
		},
		{
			TargetID:         "local.codex",
			Status:           i3dResourceStatusUp,
			CPUPercent:       888,
			MemoryBytes:      888888,
			DiskReadBytesPS:  888,
			DiskWriteBytesPS: 888,
			NetworkRxBytesPS: 888,
			NetworkTxBytesPS: 888,
			UnitCount:        1,
		},
	}

	response := buildI3DResourceOverview("local", targets, containers, processes, nil)

	if response.Summary.CPUPercent != 40.5 {
		t.Fatalf("summary CPU should equal configured target sum, got %.2f", response.Summary.CPUPercent)
	}
	if response.Summary.CPUCoresUsed != 0.405 {
		t.Fatalf("summary CPU cores should equal cpu_percent/100 per target, got %.3f", response.Summary.CPUCoresUsed)
	}
	if response.Summary.MemoryBytes != 6000 {
		t.Fatalf("summary memory should equal configured target sum, got %d", response.Summary.MemoryBytes)
	}
	if response.Summary.MemoryRSSBytes != 5300 || response.Summary.MemoryCacheBytes != 350 {
		t.Fatalf("summary memory breakdown should equal configured target sum, got %#v", response.Summary)
	}
	if response.Summary.PIDCount != 9 || response.Summary.ThreadCount != 8 {
		t.Fatalf("summary process counts should equal configured target sum, got pids=%d threads=%d", response.Summary.PIDCount, response.Summary.ThreadCount)
	}
	if response.Summary.DiskReadBytesPS != 150 || response.Summary.DiskWriteBytesPS != 180 {
		t.Fatalf("summary disk should equal configured target sum, got read=%d write=%d", response.Summary.DiskReadBytesPS, response.Summary.DiskWriteBytesPS)
	}
	if response.Summary.NetworkRxBytesPS != 210 || response.Summary.NetworkTxBytesPS != 240 {
		t.Fatalf("summary network should equal configured target sum, got rx=%d tx=%d", response.Summary.NetworkRxBytesPS, response.Summary.NetworkTxBytesPS)
	}

	groups := map[string]i3dResourceGroupDTO{}
	for _, group := range response.Groups {
		groups[group.ID] = group
	}
	if groups[i3dResourceGroupBusiness].CPUPercent != 25.5 || groups[i3dResourceGroupBusiness].MemoryBytes != 1000 {
		t.Fatalf("business group should only include business target, got %#v", groups[i3dResourceGroupBusiness])
	}
	if groups[i3dResourceGroupBusiness].MemoryRSSBytes != 700 || groups[i3dResourceGroupBusiness].PIDCount != 3 {
		t.Fatalf("business group should include business memory details, got %#v", groups[i3dResourceGroupBusiness])
	}
	if groups[i3dResourceGroupMiddleware].CPUPercent != 5 || groups[i3dResourceGroupMiddleware].MemoryBytes != 2000 {
		t.Fatalf("middleware group should only include middleware target, got %#v", groups[i3dResourceGroupMiddleware])
	}
	if groups[i3dResourceGroupMonitor].CPUPercent != 10 || groups[i3dResourceGroupMonitor].MemoryBytes != 3000 {
		t.Fatalf("monitor group should only include monitor target, got %#v", groups[i3dResourceGroupMonitor])
	}
}

func TestBuildI3DResourceOverviewKeepsMissingConfiguredTargetDown(t *testing.T) {
	targets := []i3dResourceTarget{
		{
			ID:            "release.inference.web",
			Name:          "推理服务",
			Group:         i3dResourceGroupBusiness,
			Kind:          i3dResourceKindDocker,
			ContainerName: "i3d-release-inference-service",
		},
	}

	response := buildI3DResourceOverview("release", targets, nil, nil, nil)

	if len(response.Items) != 1 {
		t.Fatalf("expected missing configured target to stay visible, got %d items", len(response.Items))
	}
	item := response.Items[0]
	if item.ID != "release.inference.web" {
		t.Fatalf("expected release.inference.web, got %s", item.ID)
	}
	if item.Status != i3dResourceStatusDown {
		t.Fatalf("missing configured target should be down, got %s", item.Status)
	}
	if response.Summary.AbnormalCount != 1 {
		t.Fatalf("missing configured target should count as abnormal, got %d", response.Summary.AbnormalCount)
	}
	if response.Summary.CPUPercent != 0 || response.Summary.MemoryBytes != 0 {
		t.Fatalf("missing target should not add resources: %#v", response.Summary)
	}
}

func TestBuildI3DResourceOverviewCountsMetricDiagnosticAsAbnormal(t *testing.T) {
	targets := []i3dResourceTarget{
		{
			ID:            "release.search.web",
			Name:          "检索服务",
			Group:         i3dResourceGroupBusiness,
			Kind:          i3dResourceKindDocker,
			ContainerName: "i3d-release-search-service",
		},
	}
	containers := []i3dResourceContainerStats{
		{
			Name:       "i3d-release-search-service",
			Status:     "running",
			Diagnostic: "Docker stats 采集失败",
		},
	}

	response := buildI3DResourceOverview("release", targets, containers, nil, nil)

	if response.Items[0].Status != i3dResourceStatusUp {
		t.Fatalf("running container should still report service status up, got %s", response.Items[0].Status)
	}
	if response.Items[0].Diagnostic == "" {
		t.Fatalf("metric diagnostic should be exposed on the resource item")
	}
	if response.Summary.AbnormalCount != 1 {
		t.Fatalf("metric diagnostic should count as abnormal, got %d", response.Summary.AbnormalCount)
	}
}

func TestBuildI3DResourceOverviewCountsOnlyConfiguredProcessTargets(t *testing.T) {
	targets := []i3dResourceTarget{
		{
			ID:      "local.search.web",
			Name:    "检索服务",
			Group:   i3dResourceGroupBusiness,
			Kind:    i3dResourceKindProcess,
			PIDFile: "../i3d-search-service/run/web.pid",
		},
	}
	processes := []i3dResourceProcessStats{
		{
			TargetID:         "local.search.web",
			Status:           i3dResourceStatusUp,
			CPUPercent:       80,
			MemoryBytes:      4096,
			MemoryRSSBytes:   4096,
			DiskReadBytesPS:  10,
			DiskWriteBytesPS: 20,
			UnitCount:        2,
			ThreadCount:      12,
			PIDs:             []int32{10, 11},
			UptimeSeconds:    30,
		},
		{
			TargetID:         "unrelated.vscode",
			Status:           i3dResourceStatusUp,
			CPUPercent:       900,
			MemoryBytes:      999999,
			DiskReadBytesPS:  999,
			DiskWriteBytesPS: 999,
			UnitCount:        1,
			UptimeSeconds:    30,
		},
	}

	response := buildI3DResourceOverview("local", targets, nil, processes, nil)

	if len(response.Items) != 1 {
		t.Fatalf("expected exactly 1 configured process target, got %d", len(response.Items))
	}
	if response.Summary.CPUPercent != 80 {
		t.Fatalf("summary CPU should only include configured process target, got %.2f", response.Summary.CPUPercent)
	}
	if response.Summary.MemoryBytes != 4096 {
		t.Fatalf("summary memory should only include configured process target, got %d", response.Summary.MemoryBytes)
	}
	if response.Items[0].UnitCount != 2 {
		t.Fatalf("expected process tree unit count 2, got %d", response.Items[0].UnitCount)
	}
	if response.Items[0].PIDCount != 2 || response.Items[0].ThreadCount != 12 || response.Items[0].MemoryRSSBytes != 4096 {
		t.Fatalf("expected process detail fields to be exposed, got %#v", response.Items[0])
	}
}

func TestBuildI3DResourceTimeseriesOnlyCountsConfiguredTargets(t *testing.T) {
	targets := []i3dResourceTarget{
		{
			ID:            "release.search.web",
			Name:          "检索服务",
			Group:         i3dResourceGroupBusiness,
			Kind:          i3dResourceKindDocker,
			ContainerName: "i3d-release-search-service",
		},
		{
			ID:            "release.middleware.redis",
			Name:          "Redis",
			Group:         i3dResourceGroupMiddleware,
			Kind:          i3dResourceKindDocker,
			ContainerName: "i3d-release-redis",
		},
	}
	history := []i3dResourceContainerHistoryPoint{
		{
			Timestamp: "2026-05-17T10:00:00Z",
			Containers: []i3dResourceContainerStats{
				{
					Name:             "i3d-release-search-service",
					CPUPercent:       10,
					MemoryBytes:      100,
					MemoryUsageBytes: 180,
					MemoryRSSBytes:   70,
					MemoryCacheBytes: 80,
					NetworkRxBytesPS: 1,
					NetworkTxBytesPS: 2,
				},
				{
					Name:             "i3d-release-redis",
					CPUPercent:       20,
					MemoryBytes:      200,
					MemoryUsageBytes: 260,
					MemoryRSSBytes:   160,
					MemoryCacheBytes: 60,
					DiskReadBytesPS:  5,
					DiskWriteBytesPS: 6,
					NetworkRxBytesPS: 3,
					NetworkTxBytesPS: 4,
				},
				{Name: "browser", CPUPercent: 900, MemoryBytes: 999999, DiskReadBytesPS: 999, DiskWriteBytesPS: 999, NetworkRxBytesPS: 999, NetworkTxBytesPS: 999},
			},
		},
	}

	response := buildI3DResourceTimeseries("release", targets, history)

	if len(response.Items) != 1 {
		t.Fatalf("expected one timeseries point, got %d", len(response.Items))
	}
	point := response.Items[0]
	if point.CPUPercent != 30 {
		t.Fatalf("timeseries CPU should only include configured targets, got %.2f", point.CPUPercent)
	}
	if point.MemoryBytes != 300 {
		t.Fatalf("timeseries memory should only include configured targets, got %d", point.MemoryBytes)
	}
	if point.MemoryUsageBytes != 440 || point.MemoryRSSBytes != 230 || point.MemoryCacheBytes != 140 {
		t.Fatalf("timeseries memory breakdown should only include configured targets, got %#v", point)
	}
	if point.DiskReadBytesPS != 5 || point.DiskWriteBytesPS != 6 {
		t.Fatalf("timeseries disk should only include configured targets, got read=%d write=%d", point.DiskReadBytesPS, point.DiskWriteBytesPS)
	}
	if point.NetworkRxBytesPS != 4 || point.NetworkTxBytesPS != 6 {
		t.Fatalf("timeseries network should only include configured targets, got rx=%d tx=%d", point.NetworkRxBytesPS, point.NetworkTxBytesPS)
	}
}

func TestParseI3DResourceContainerStatsJSONConvertsUnitsAndDirections(t *testing.T) {
	raw := []byte(`[{"n":"/i3d-release-search-service","c":12.5,"m":64,"nr":1.5,"ns":2.25,"dr":3.5,"dw":4.75,"u":30}]`)

	containers, err := parseI3DResourceContainerStatsJSON(raw)
	if err != nil {
		t.Fatalf("parse container stats json failed: %v", err)
	}
	if len(containers) != 1 {
		t.Fatalf("expected 1 container stat, got %d", len(containers))
	}
	stat := containers[0]
	if stat.Name != "i3d-release-search-service" {
		t.Fatalf("container name should be normalized, got %s", stat.Name)
	}
	if stat.CPUPercent != 12.5 {
		t.Fatalf("cpu should be parsed exactly, got %.2f", stat.CPUPercent)
	}
	if stat.MemoryBytes != 64*1024*1024 {
		t.Fatalf("memory MB should convert to bytes, got %d", stat.MemoryBytes)
	}
	if stat.NetworkRxBytesPS != uint64(1.5*1024*1024) {
		t.Fatalf("network recv should convert from nr MB/s to bytes/s, got %d", stat.NetworkRxBytesPS)
	}
	if stat.NetworkTxBytesPS != uint64(2.25*1024*1024) {
		t.Fatalf("network sent should convert from ns MB/s to bytes/s, got %d", stat.NetworkTxBytesPS)
	}
	if stat.DiskReadBytesPS != uint64(3.5*1024*1024) {
		t.Fatalf("disk read should convert from dr MB/s to bytes/s, got %d", stat.DiskReadBytesPS)
	}
	if stat.DiskWriteBytesPS != uint64(4.75*1024*1024) {
		t.Fatalf("disk write should convert from dw MB/s to bytes/s, got %d", stat.DiskWriteBytesPS)
	}
	if stat.UptimeSeconds != 30 {
		t.Fatalf("uptime should be parsed, got %d", stat.UptimeSeconds)
	}
}

func TestMergeI3DResourceContainerTrafficUsesDirectionalHistory(t *testing.T) {
	containers := []i3dResourceContainerStats{
		{
			Name:             "i3d-release-search-service",
			NetworkRxBytesPS: 999,
			NetworkTxBytesPS: 0,
		},
		{
			Name:             "i3d-release-redis",
			NetworkRxBytesPS: 888,
			NetworkTxBytesPS: 0,
		},
	}
	history := []i3dResourceContainerStats{
		{
			Name:             "/i3d-release-search-service",
			DiskReadBytesPS:  321,
			DiskWriteBytesPS: 654,
			NetworkRxBytesPS: 123,
			NetworkTxBytesPS: 456,
		},
	}

	merged := mergeI3DResourceContainerTraffic(containers, history)

	if merged[0].NetworkRxBytesPS != 123 || merged[0].NetworkTxBytesPS != 456 {
		t.Fatalf("directional history should override combined net, got rx=%d tx=%d", merged[0].NetworkRxBytesPS, merged[0].NetworkTxBytesPS)
	}
	if merged[0].DiskReadBytesPS != 321 || merged[0].DiskWriteBytesPS != 654 {
		t.Fatalf("disk history should override empty snapshot disk, got read=%d write=%d", merged[0].DiskReadBytesPS, merged[0].DiskWriteBytesPS)
	}
	if merged[1].NetworkRxBytesPS != 888 || merged[1].NetworkTxBytesPS != 0 {
		t.Fatalf("container without history should keep existing net values, got rx=%d tx=%d", merged[1].NetworkRxBytesPS, merged[1].NetworkTxBytesPS)
	}
}

func TestI3DResourceDockerStatsTimeoutAllowsConcurrentSnapshots(t *testing.T) {
	if i3dResourceDockerStatsTimeout < 8*time.Second {
		t.Fatalf("docker stats timeout should allow concurrent stream=false snapshots, got %s", i3dResourceDockerStatsTimeout)
	}
}

func TestApplyI3DResourceDockerAPIStatsUsesPreviousSampleWhenPreCPUIsMissing(t *testing.T) {
	i3dResourceDockerStatsCache.Lock()
	i3dResourceDockerStatsCache.samples = map[string]i3dResourceDockerCumulativeSample{}
	i3dResourceDockerStatsCache.Unlock()

	containerID := "abc123"
	first := &aethercontainer.ApiStats{}
	first.CPUStats.CPUUsage.TotalUsage = 100
	first.CPUStats.SystemUsage = 1000
	first.CPUStats.OnlineCPUs = 2
	first.MemoryStats.Usage = 2048
	first.MemoryStats.Stats.InactiveFile = 512
	first.MemoryStats.Stats.Cache = 768
	first.MemoryStats.Stats.RSS = 1024
	first.MemoryStats.Stats.ActiveAnon = 900
	first.MemoryStats.Stats.InactiveAnon = 100
	first.PidsStats.Current = 7

	stat := i3dResourceContainerStats{}
	applyI3DResourceDockerAPIStats(&stat, containerID, first)
	if stat.CPUPercent != 0 {
		t.Fatalf("first sample without precpu should not invent CPU percent, got %.2f", stat.CPUPercent)
	}
	if stat.MemoryBytes != 1536 {
		t.Fatalf("memory should use docker stats CLI cache-adjusted usage, got %d", stat.MemoryBytes)
	}
	if stat.MemoryUsageBytes != 2048 || stat.MemoryRSSBytes != 1024 || stat.MemoryCacheBytes != 768 || stat.MemoryAnonBytes != 1000 {
		t.Fatalf("memory breakdown should be exposed, got usage=%d rss=%d cache=%d anon=%d", stat.MemoryUsageBytes, stat.MemoryRSSBytes, stat.MemoryCacheBytes, stat.MemoryAnonBytes)
	}
	if stat.PIDCount != 7 {
		t.Fatalf("pids_stats current should be exposed, got %d", stat.PIDCount)
	}

	second := &aethercontainer.ApiStats{}
	second.CPUStats.CPUUsage.TotalUsage = 300
	second.CPUStats.SystemUsage = 3000
	second.CPUStats.OnlineCPUs = 2

	stat = i3dResourceContainerStats{}
	applyI3DResourceDockerAPIStats(&stat, containerID, second)
	if stat.CPUPercent != 20 {
		t.Fatalf("second one-shot sample should use cached CPU deltas, got %.2f", stat.CPUPercent)
	}
}

func TestCalculateI3DResourceProcessCPUPercentUsesShortWindowDelta(t *testing.T) {
	start := map[int32]float64{10: 1.0, 11: 2.0}
	end := map[int32]float64{10: 1.25, 11: 2.25}

	got := calculateI3DResourceProcessCPUPercent(start, end, 500*time.Millisecond)

	if got != 100 {
		t.Fatalf("expected two processes to consume one CPU core over the window, got %.2f", got)
	}
}

func TestResolveI3DResourcePIDFileUsesWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "i3d-search-service", "run"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "beszel", "run", "hub"), 0755); err != nil {
		t.Fatal(err)
	}

	searchPath := resolveI3DResourcePIDFile(root, "../i3d-search-service/run/web.pid")
	if searchPath != filepath.Join(root, "i3d-search-service", "run", "web.pid") {
		t.Fatalf("service pid file should resolve from workspace root, got %s", searchPath)
	}

	aetherPath := resolveI3DResourcePIDFile(root, "run/hub/aether.pid")
	if aetherPath != filepath.Join(root, "beszel", "run", "hub", "aether.pid") {
		t.Fatalf("aether pid file should resolve under beszel, got %s", aetherPath)
	}
}

func TestFindI3DResourceListeningPortPIDsFindsDeclaredPortOnly(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	port := uint32(listener.Addr().(*net.TCPAddr).Port)
	deadline := time.Now().Add(2 * time.Second)
	for {
		pids := findI3DResourceListeningPortPIDs([]uint32{port})
		if len(pids) > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected listener on declared port %d to be found", port)
		}
		time.Sleep(25 * time.Millisecond)
	}

	pids := findI3DResourceListeningPortPIDs([]uint32{port + 1})
	if len(pids) != 0 {
		t.Fatalf("unexpected pids for undeclared port %d: %#v", port+1, pids)
	}
}

func TestResolveI3DResourceTargetPIDsFindsDeclaredCommandOnly(t *testing.T) {
	token := "i3d-resource-helper-" + time.Now().Format("150405.000000000")
	helperDir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=TestI3DResourceCommandMatcherHelper", "--", token)
	cmd.Dir = helperDir
	cmd.Env = append(os.Environ(), "I3D_RESOURCE_HELPER_PROCESS=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	target := i3dResourceTarget{
		ID:              "local.helper",
		Kind:            i3dResourceKindProcess,
		WorkingDir:      helperDir,
		CommandIncludes: []string{token},
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		pids, diagnostic, unknown := resolveI3DResourceTargetPIDs(target, t.TempDir())
		if len(pids) > 0 {
			if diagnostic != "" || unknown {
				t.Fatalf("matched command should not return diagnostic=%q unknown=%v", diagnostic, unknown)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected command matcher to find helper process containing %q", token)
		}
		time.Sleep(25 * time.Millisecond)
	}

	target.CommandIncludes = []string{"definitely-not-" + token}
	pids, diagnostic, unknown := resolveI3DResourceTargetPIDs(target, t.TempDir())
	if len(pids) != 0 || diagnostic == "" || unknown {
		t.Fatalf("nonmatching command should be down with diagnostic, got pids=%#v diagnostic=%q unknown=%v", pids, diagnostic, unknown)
	}

	target.CommandIncludes = []string{token}
	target.WorkingDir = filepath.Join(t.TempDir(), "other-service")
	pids, diagnostic, unknown = resolveI3DResourceTargetPIDs(target, t.TempDir())
	if len(pids) != 0 || diagnostic == "" || unknown {
		t.Fatalf("wrong working directory should be down with diagnostic, got pids=%#v diagnostic=%q unknown=%v", pids, diagnostic, unknown)
	}
}

func TestI3DResourceCommandMatcherHelper(t *testing.T) {
	if os.Getenv("I3D_RESOURCE_HELPER_PROCESS") != "1" {
		return
	}
	time.Sleep(10 * time.Second)
	os.Exit(0)
}

func TestCalculateI3DResourceDockerAPIStats(t *testing.T) {
	stats := &aethercontainer.ApiStats{
		CPUStats: aethercontainer.CPUStats{
			CPUUsage:    aethercontainer.CPUUsage{TotalUsage: 3000},
			SystemUsage: 10000,
			OnlineCPUs:  8,
		},
		PreCPUStats: aethercontainer.CPUStats{
			CPUUsage:    aethercontainer.CPUUsage{TotalUsage: 1000},
			SystemUsage: 9000,
		},
		MemoryStats: aethercontainer.MemoryStats{
			Usage: 2048,
			Stats: aethercontainer.MemoryStatsStats{
				InactiveFile:      512,
				Cache:             768,
				RSS:               1024,
				ActiveAnon:        900,
				InactiveAnon:      100,
				TotalInactiveFile: 256,
			},
		},
		PidsStats: aethercontainer.PidsStats{Current: 6},
		BlkioStats: aethercontainer.BlkioStats{
			IoServiceBytesRecursive: []aethercontainer.BlkioEntry{
				{Op: "Read", Value: 100},
				{Op: "Write", Value: 200},
				{Op: "Sync", Value: 999},
			},
		},
		Networks: map[string]aethercontainer.NetworkStats{
			"eth0": {RxBytes: 300, TxBytes: 400},
			"eth1": {RxBytes: 500, TxBytes: 600},
		},
	}

	if got := calculateI3DResourceDockerCPUPercent(stats); got != 1600 {
		t.Fatalf("cpu percent should include online cpu multiplier, got %.2f", got)
	}
	if got := calculateI3DResourceDockerMemoryBytes(stats); got != 1792 {
		t.Fatalf("memory should subtract inactive file cache, got %d", got)
	}
	stat := i3dResourceContainerStats{}
	applyI3DResourceDockerMemoryStats(&stat, stats)
	if stat.MemoryUsageBytes != 2048 || stat.MemoryRSSBytes != 1024 || stat.MemoryCacheBytes != 768 || stat.MemoryAnonBytes != 1000 || stat.MemoryInactiveFileBytes != 256 {
		t.Fatalf("memory breakdown mismatch: %#v", stat)
	}
	read, write := calculateI3DResourceDockerDiskTotals(stats)
	if read != 100 || write != 200 {
		t.Fatalf("disk totals should only include read/write ops, got read=%d write=%d", read, write)
	}
	rx, tx := calculateI3DResourceDockerNetworkTotals(stats)
	if rx != 800 || tx != 1000 {
		t.Fatalf("network totals should sum all interfaces, got rx=%d tx=%d", rx, tx)
	}
	if got := calculateI3DResourceRate(2500, 1000, 500); got != 3000 {
		t.Fatalf("rate should calculate bytes per second, got %d", got)
	}
}

func TestApplyI3DResourceDockerAPIStatsUsesCachedDeltaWhenPreCPUIsMissing(t *testing.T) {
	i3dResourceDockerStatsCache.Lock()
	i3dResourceDockerStatsCache.samples = map[string]i3dResourceDockerCumulativeSample{}
	i3dResourceDockerStatsCache.Unlock()

	first := &aethercontainer.ApiStats{
		CPUStats: aethercontainer.CPUStats{
			CPUUsage:    aethercontainer.CPUUsage{TotalUsage: 1_000_000},
			SystemUsage: 10_000_000,
			OnlineCPUs:  4,
		},
	}
	stat := i3dResourceContainerStats{}
	applyI3DResourceDockerAPIStats(&stat, "container-a", first)
	if stat.CPUPercent != 0 {
		t.Fatalf("first one-shot sample with empty precpu should not report lifetime CPU, got %.2f", stat.CPUPercent)
	}

	second := &aethercontainer.ApiStats{
		CPUStats: aethercontainer.CPUStats{
			CPUUsage:    aethercontainer.CPUUsage{TotalUsage: 1_010_000},
			SystemUsage: 11_000_000,
			OnlineCPUs:  4,
		},
	}
	stat = i3dResourceContainerStats{}
	applyI3DResourceDockerAPIStats(&stat, "container-a", second)
	if stat.CPUPercent != 4 {
		t.Fatalf("second one-shot sample should use local cumulative CPU delta, got %.2f", stat.CPUPercent)
	}
}

func TestApplyI3DResourceDockerAPIStatsUsesDockerPreCPUWhenPresent(t *testing.T) {
	i3dResourceDockerStatsCache.Lock()
	i3dResourceDockerStatsCache.samples = map[string]i3dResourceDockerCumulativeSample{}
	i3dResourceDockerStatsCache.Unlock()

	first := &aethercontainer.ApiStats{
		CPUStats: aethercontainer.CPUStats{
			CPUUsage:    aethercontainer.CPUUsage{TotalUsage: 3_000_000},
			SystemUsage: 10_000_000,
			OnlineCPUs:  8,
		},
		PreCPUStats: aethercontainer.CPUStats{
			CPUUsage:    aethercontainer.CPUUsage{TotalUsage: 1_000_000},
			SystemUsage: 9_000_000,
		},
	}
	stat := i3dResourceContainerStats{}
	applyI3DResourceDockerAPIStats(&stat, "container-with-precpu", first)
	if stat.CPUPercent != 1600 {
		t.Fatalf("docker sample should prefer docker precpu delta when present, got %.2f", stat.CPUPercent)
	}
}

func TestParseI3DResourceNvidiaSMIComputeApps(t *testing.T) {
	raw := `GPU-a, 123, /usr/bin/python3, 512
GPU-a, 124, python, 1,024 MiB
GPU-a, bad-pid, python, 99
GPU-a, 125, python, [Not Supported]
`

	apps := parseI3DResourceNvidiaSMIComputeApps(raw)

	if len(apps) != 2 {
		t.Fatalf("expected 2 valid gpu process rows, got %#v", apps)
	}
	if apps[0].PID != 123 || apps[0].UsedMemoryBytes != 512*1024*1024 {
		t.Fatalf("first row should parse MiB into bytes, got %#v", apps[0])
	}
	if apps[1].PID != 124 || apps[1].UsedMemoryBytes != 1024*1024*1024 {
		t.Fatalf("second row should parse comma formatted MiB into bytes, got %#v", apps[1])
	}
}

func TestApplyI3DResourceGPUStatsOnlyCountsDeclaredProcessesAndContainers(t *testing.T) {
	containers := []i3dResourceContainerStats{
		{Name: "i3d-release-inference-service", ContainerID: "abcdef1234567890", Status: "running"},
		{Name: "unrelated-container", ContainerID: "ffff", Status: "running"},
	}
	processes := []i3dResourceProcessStats{
		{TargetID: "local.inference.web", Status: i3dResourceStatusUp, PIDs: []int32{31203}},
		{TargetID: "local.cad.worker", Status: i3dResourceStatusUp, PIDs: []int32{40000, 40001}},
	}
	apps := []i3dResourceGPUProcessStats{
		{PID: 31203, UsedMemoryBytes: 6 * 1024 * 1024 * 1024},
		{PID: 40001, UsedMemoryBytes: 2 * 1024 * 1024 * 1024},
		{PID: 50000, ContainerID: "abcdef1234567890", UsedMemoryBytes: 3 * 1024 * 1024 * 1024},
		{PID: 60000, UsedMemoryBytes: 99 * 1024 * 1024 * 1024},
	}

	applyI3DResourceGPUStats(containers, processes, apps)
	response := buildI3DResourceOverview("local", []i3dResourceTarget{
		{ID: "local.inference.web", Name: "推理服务", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindProcess},
		{ID: "local.cad.worker", Name: "CAD 作业服务 Worker", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindProcess},
		{ID: "release.inference.web", Name: "推理服务容器", Group: i3dResourceGroupBusiness, Kind: i3dResourceKindDocker, ContainerName: "i3d-release-inference-service"},
	}, containers, processes, nil)

	byID := map[string]i3dResourceTargetDTO{}
	for _, item := range response.Items {
		byID[item.ID] = item
	}
	if byID["local.inference.web"].GPUMemoryBytes != 6*1024*1024*1024 {
		t.Fatalf("local inference gpu memory should match declared PID, got %d", byID["local.inference.web"].GPUMemoryBytes)
	}
	if byID["local.cad.worker"].GPUMemoryBytes != 2*1024*1024*1024 {
		t.Fatalf("local cad worker gpu memory should match child PID, got %d", byID["local.cad.worker"].GPUMemoryBytes)
	}
	if byID["release.inference.web"].GPUMemoryBytes != 3*1024*1024*1024 {
		t.Fatalf("docker inference gpu memory should match container ID, got %d", byID["release.inference.web"].GPUMemoryBytes)
	}
	if response.Summary.GPUMemoryBytes != 11*1024*1024*1024 {
		t.Fatalf("summary gpu memory should only include declared targets, got %d", response.Summary.GPUMemoryBytes)
	}
}

func TestParseI3DResourceContainerIDFromCgroup(t *testing.T) {
	raw := `0::/system.slice/docker-abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890.scope
1:name=systemd:/docker/1234567890ab
`

	if got := parseI3DResourceContainerIDFromCgroup(raw); got != "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890" {
		t.Fatalf("should parse docker systemd scope container id, got %q", got)
	}
	if got := parseI3DResourceContainerIDFromCgroup("0::/user.slice/session-1.scope"); got != "" {
		t.Fatalf("non-container cgroup should not produce a container id, got %q", got)
	}
}

func TestI3DResourceOverviewCacheReturnsFreshSnapshotWithoutCollecting(t *testing.T) {
	cache := newI3DResourceOverviewCache(5 * time.Second)
	now := time.Now()
	calls := 0

	first, err := cache.get("local", now, func() (i3dResourceOverviewDTO, error) {
		calls++
		return i3dResourceOverviewDTO{Environment: "local", UpdatedAt: "first"}, nil
	}, func(fn func()) { fn() })
	if err != nil {
		t.Fatal(err)
	}
	second, err := cache.get("local", now.Add(time.Second), func() (i3dResourceOverviewDTO, error) {
		calls++
		return i3dResourceOverviewDTO{Environment: "local", UpdatedAt: "second"}, nil
	}, func(fn func()) { fn() })
	if err != nil {
		t.Fatal(err)
	}

	if calls != 1 {
		t.Fatalf("fresh cache should not collect again, calls=%d", calls)
	}
	if first.UpdatedAt != "first" || second.UpdatedAt != "first" {
		t.Fatalf("fresh cache should return first snapshot, got first=%q second=%q", first.UpdatedAt, second.UpdatedAt)
	}
}

func TestI3DResourceOverviewCacheReturnsStaleSnapshotAndRefreshesInBackground(t *testing.T) {
	cache := newI3DResourceOverviewCache(5 * time.Second)
	now := time.Now()
	if _, err := cache.get("local", now, func() (i3dResourceOverviewDTO, error) {
		return i3dResourceOverviewDTO{Environment: "local", UpdatedAt: "first"}, nil
	}, func(fn func()) { fn() }); err != nil {
		t.Fatal(err)
	}

	asyncJobs := []func(){}
	response, err := cache.get("local", now.Add(10*time.Second), func() (i3dResourceOverviewDTO, error) {
		return i3dResourceOverviewDTO{Environment: "local", UpdatedAt: "refreshed"}, nil
	}, func(fn func()) {
		asyncJobs = append(asyncJobs, fn)
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.UpdatedAt != "first" {
		t.Fatalf("stale cache should be returned immediately, got %q", response.UpdatedAt)
	}
	if len(asyncJobs) != 1 {
		t.Fatalf("stale cache should schedule one background refresh, got %d", len(asyncJobs))
	}

	asyncJobs[0]()
	refreshed, err := cache.get("local", time.Now(), func() (i3dResourceOverviewDTO, error) {
		t.Fatal("fresh refreshed cache should not collect")
		return i3dResourceOverviewDTO{}, nil
	}, func(fn func()) { fn() })
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.UpdatedAt != "refreshed" {
		t.Fatalf("background refresh should update cache, got %q", refreshed.UpdatedAt)
	}
}

func TestI3DResourceTimeseriesHistoryRecordsOverviewSnapshots(t *testing.T) {
	history := newI3DResourceTimeseriesHistory(3)
	first := i3dResourceOverviewDTO{
		Environment: "local",
		UpdatedAt:   "2026-05-18T09:00:00+08:00",
		Summary: i3dResourceSummaryDTO{
			CPUPercent:       12.5,
			CPUCoresUsed:     0.125,
			MemoryBytes:      1024,
			MemoryUsageBytes: 1400,
			MemoryRSSBytes:   700,
			MemoryCacheBytes: 500,
			DiskReadBytesPS:  10,
			DiskWriteBytesPS: 20,
			NetworkRxBytesPS: 30,
			NetworkTxBytesPS: 40,
		},
		Groups: []i3dResourceGroupDTO{
			{ID: i3dResourceGroupBusiness, Name: "业务服务", CPUPercent: 7.5, MemoryBytes: 512, MemoryUsageBytes: 800, MemoryRSSBytes: 300, MemoryCacheBytes: 400},
			{ID: i3dResourceGroupMiddleware, Name: "中间件", CPUPercent: 5, MemoryBytes: 512, MemoryUsageBytes: 600, MemoryRSSBytes: 400, MemoryCacheBytes: 100},
		},
		Items: []i3dResourceTargetDTO{
			{ID: "local.inference.web", Name: "推理服务", CPUPercent: 7.5, MemoryBytes: 512, MemoryUsageBytes: 800, MemoryRSSBytes: 300, MemoryCacheBytes: 400, GPUMemoryBytes: 256},
			{ID: "local.middleware.redis", Name: "Redis", CPUPercent: 5, MemoryBytes: 512, MemoryUsageBytes: 600, MemoryRSSBytes: 400, MemoryCacheBytes: 100},
		},
	}
	second := first
	second.UpdatedAt = "2026-05-18T09:00:05+08:00"
	second.Summary.CPUPercent = 20
	second.Groups[0].CPUPercent = 15

	history.record(first)
	history.record(second)
	response := history.snapshot("local")

	if len(response.Items) != 2 {
		t.Fatalf("timeseries should include recorded overview snapshots, got %d", len(response.Items))
	}
	if response.Items[0].Timestamp != first.UpdatedAt || response.Items[1].Timestamp != second.UpdatedAt {
		t.Fatalf("timestamps should follow overview UpdatedAt values, got %#v", response.Items)
	}
	if response.Items[0].CPUPercent != 12.5 || response.Items[1].CPUPercent != 20 {
		t.Fatalf("summary CPU should be recorded from overview snapshots, got %#v", response.Items)
	}
	if response.Items[1].Groups[i3dResourceGroupBusiness].CPUPercent != 15 {
		t.Fatalf("group CPU should be recorded for chart breakdown, got %#v", response.Items[1].Groups)
	}
	if response.Items[0].MemoryRSSBytes != 700 || response.Items[0].MemoryCacheBytes != 500 {
		t.Fatalf("summary memory details should be recorded from overview snapshots, got %#v", response.Items[0])
	}
	if response.Items[0].Groups[i3dResourceGroupBusiness].MemoryRSSBytes != 300 {
		t.Fatalf("group memory details should be recorded for chart breakdown, got %#v", response.Items[0].Groups)
	}
	if response.Items[0].Targets["local.inference.web"].GPUMemoryBytes != 256 {
		t.Fatalf("target GPU memory should be recorded for service breakdown, got %#v", response.Items[0].Targets)
	}
	if response.Items[0].Targets["local.inference.web"].MemoryRSSBytes != 300 {
		t.Fatalf("target memory details should be recorded for service breakdown, got %#v", response.Items[0].Targets)
	}
}

func TestI3DResourceTimeseriesHistoryKeepsPerEnvironmentRingBuffer(t *testing.T) {
	history := newI3DResourceTimeseriesHistory(2)
	history.record(i3dResourceOverviewDTO{Environment: "local", UpdatedAt: "local-1", Summary: i3dResourceSummaryDTO{CPUPercent: 1}})
	history.record(i3dResourceOverviewDTO{Environment: "release", UpdatedAt: "release-1", Summary: i3dResourceSummaryDTO{CPUPercent: 99}})
	history.record(i3dResourceOverviewDTO{Environment: "local", UpdatedAt: "local-2", Summary: i3dResourceSummaryDTO{CPUPercent: 2}})
	history.record(i3dResourceOverviewDTO{Environment: "local", UpdatedAt: "local-3", Summary: i3dResourceSummaryDTO{CPUPercent: 3}})

	local := history.snapshot("local")
	release := history.snapshot("release")

	if len(local.Items) != 2 {
		t.Fatalf("local history should be capped at 2 points, got %d", len(local.Items))
	}
	if local.Items[0].Timestamp != "local-2" || local.Items[1].Timestamp != "local-3" {
		t.Fatalf("local history should keep newest points in order, got %#v", local.Items)
	}
	if len(release.Items) != 1 || release.Items[0].Timestamp != "release-1" {
		t.Fatalf("release history should be isolated from local history, got %#v", release.Items)
	}
}

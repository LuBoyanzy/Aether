package hub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	psutilprocess "github.com/shirou/gopsutil/v4/process"
)

func TestI3DResourceLiveLocalCollectors(t *testing.T) {
	if os.Getenv("I3D_RESOURCE_LIVE_TEST") != "1" {
		t.Skip("set I3D_RESOURCE_LIVE_TEST=1 to verify the current local i3d runtime")
	}

	targets := defaultI3DResourceTargets(i3dResourceEnvironmentLocal)
	startedAt := time.Now()
	containers, err := collectI3DResourceDockerStats(targets)
	if err != nil {
		t.Fatalf("collect docker stats: %v", err)
	}
	dockerElapsed := time.Since(startedAt)
	processStartedAt := time.Now()
	processes := collectI3DResourceProcessStats(targets)
	processElapsed := time.Since(processStartedAt)
	overview := buildI3DResourceOverview(i3dResourceEnvironmentLocal, targets, containers, processes, nil)
	t.Logf("collect elapsed docker=%s process=%s total=%s", dockerElapsed, processElapsed, time.Since(startedAt))

	assertI3DResourceLiveHasExactTargets(t, targets, overview.Items)
	assertI3DResourceLiveAggregatesDisplayedItems(t, overview)
	assertI3DResourceLiveLocalProcesses(t, overview)
	assertI3DResourceLiveDockerContainers(t, targets, containers)
	assertI3DResourceLiveCommandMatchers(t, targets)
	logI3DResourceLiveOverview(t, overview)
}

func assertI3DResourceLiveHasExactTargets(t *testing.T, targets []i3dResourceTarget, items []i3dResourceTargetDTO) {
	t.Helper()
	if len(items) != len(targets) {
		t.Fatalf("overview should return exactly declared targets, got items=%d targets=%d", len(items), len(targets))
	}
	expected := make(map[string]bool, len(targets))
	for _, target := range targets {
		expected[target.ID] = true
	}
	for _, item := range items {
		if !expected[item.ID] {
			t.Fatalf("unexpected target leaked into overview: %#v", item)
		}
	}
}

func assertI3DResourceLiveAggregatesDisplayedItems(t *testing.T, overview i3dResourceOverviewDTO) {
	t.Helper()
	var cpu float64
	var memory uint64
	var diskRead uint64
	var diskWrite uint64
	var networkRx uint64
	var networkTx uint64
	var abnormal int
	for _, item := range overview.Items {
		if item.Status != i3dResourceStatusUp {
			abnormal++
			continue
		}
		cpu += item.CPUPercent
		memory += item.MemoryBytes
		diskRead += item.DiskReadBytesPS
		diskWrite += item.DiskWriteBytesPS
		networkRx += item.NetworkRxBytesPS
		networkTx += item.NetworkTxBytesPS
	}
	if overview.Summary.CPUPercent != cpu ||
		overview.Summary.MemoryBytes != memory ||
		overview.Summary.DiskReadBytesPS != diskRead ||
		overview.Summary.DiskWriteBytesPS != diskWrite ||
		overview.Summary.NetworkRxBytesPS != networkRx ||
		overview.Summary.NetworkTxBytesPS != networkTx ||
		overview.Summary.AbnormalCount != abnormal {
		t.Fatalf("summary should equal displayed up targets, summary=%#v cpu=%.2f memory=%d disk=%d/%d net=%d/%d abnormal=%d",
			overview.Summary, cpu, memory, diskRead, diskWrite, networkRx, networkTx, abnormal)
	}
}

func assertI3DResourceLiveLocalProcesses(t *testing.T, overview i3dResourceOverviewDTO) {
	t.Helper()
	requiredUp := map[string]bool{
		"local.search.web":        true,
		"local.inference.web":     true,
		"local.file.web":          true,
		"local.file.worker":       true,
		"local.file.consumer":     true,
		"local.cad.web":           true,
		"local.cad.worker":        true,
		"local.frontend.webviews": true,
		"local.aether":            true,
	}
	for _, item := range overview.Items {
		if !requiredUp[item.ID] {
			continue
		}
		if item.Status != i3dResourceStatusUp {
			t.Fatalf("%s should be up in current local runtime: status=%s diagnostic=%s", item.ID, item.Status, item.Diagnostic)
		}
		if item.UnitCount <= 0 || item.MemoryBytes == 0 {
			t.Fatalf("%s should report process units and memory, item=%#v", item.ID, item)
		}
	}
}

func assertI3DResourceLiveDockerContainers(t *testing.T, targets []i3dResourceTarget, containers []i3dResourceContainerStats) {
	t.Helper()
	declared := map[string]bool{}
	for _, target := range targets {
		if target.Kind == i3dResourceKindDocker && target.ContainerName != "" {
			declared[target.ContainerName] = true
		}
	}
	if len(containers) == 0 {
		t.Fatalf("expected local middleware docker containers to be collected")
	}
	for _, container := range containers {
		if !declared[container.Name] {
			t.Fatalf("unexpected docker container leaked into i3d resources: %#v", container)
		}
		if mapI3DResourceContainerStatus(container.Status) == i3dResourceStatusUp && container.MemoryBytes == 0 {
			t.Fatalf("running docker container should report memory: %#v", container)
		}
	}
}

func assertI3DResourceLiveCommandMatchers(t *testing.T, targets []i3dResourceTarget) {
	t.Helper()
	workspaceRoot := resolveI3DWorkspaceRoot()
	for _, target := range targets {
		if len(target.CommandIncludes) == 0 {
			continue
		}
		workingDir := resolveI3DResourcePath(workspaceRoot, target.WorkingDir)
		pids := findI3DResourceCommandPIDs(target.CommandIncludes, workingDir)
		if len(pids) == 0 {
			t.Fatalf("%s command matcher did not find a running process", target.ID)
		}
		for _, pid := range pids {
			proc, err := psutilprocess.NewProcess(pid)
			if err != nil {
				t.Fatalf("%s matched pid=%d but process cannot be opened: %v", target.ID, pid, err)
			}
			command, _ := proc.Cmdline()
			cwd, _ := proc.Cwd()
			if filepath.Clean(cwd) != workingDir {
				t.Fatalf("%s matched pid=%d outside working dir: cwd=%s want=%s command=%s", target.ID, pid, cwd, workingDir, command)
			}
			for _, include := range target.CommandIncludes {
				if !strings.Contains(command, include) {
					t.Fatalf("%s matched pid=%d without include %q: %s", target.ID, pid, include, command)
				}
			}
		}
	}
}

func logI3DResourceLiveOverview(t *testing.T, overview i3dResourceOverviewDTO) {
	t.Helper()
	t.Logf("summary cpu=%.2f%% cores=%.2f memory=%d disk=%d/%d net=%d/%d abnormal=%d",
		overview.Summary.CPUPercent,
		overview.Summary.CPUCoresUsed,
		overview.Summary.MemoryBytes,
		overview.Summary.DiskReadBytesPS,
		overview.Summary.DiskWriteBytesPS,
		overview.Summary.NetworkRxBytesPS,
		overview.Summary.NetworkTxBytesPS,
		overview.Summary.AbnormalCount,
	)
	for _, item := range overview.Items {
		t.Logf("target id=%s status=%s cpu=%.2f%% memory=%d units=%d source=%s%s%s",
			item.ID,
			item.Status,
			item.CPUPercent,
			item.MemoryBytes,
			item.UnitCount,
			item.ContainerName,
			item.PIDFile,
			strings.Join(item.CommandIncludes, "|"),
		)
	}
}

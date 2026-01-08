// docker_sdk_overview.go 实现 Docker 引擎概览信息读取。
// 该数据用于概览面板展示。
package agent

import (
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"

	dockermodel "aether/internal/entities/docker"
)

func (dm *dockerSDKManager) GetOverview() (*dockermodel.Overview, error) {
	if err := dm.ensureAvailable(); err != nil {
		return nil, err
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	info, err := dm.client.Info(ctx)
	if err != nil {
		return nil, err
	}
	version, err := dm.client.ServerVersion(ctx)
	if err != nil {
		return nil, err
	}
	if version.APIVersion == "" {
		return nil, errors.New("docker api version is empty")
	}

	memTotal := info.MemTotal
	if memTotal < 0 {
		return nil, fmt.Errorf("docker mem total is negative: %d", memTotal)
	}

	composeVersion := ""
	composeVersionValue, err := getComposeVersion(ctx)
	if err != nil {
		if errors.Is(err, errComposeCommandNotFound) {
			slog.Warn("Docker compose command not found; compose version unavailable", "err", err)
		} else {
			slog.Error("Failed to get Docker compose version", "err", err, "stack", string(debug.Stack()))
			return nil, err
		}
	} else {
		composeVersion = composeVersionValue
	}

	return &dockermodel.Overview{
		ServerVersion:     version.Version,
		APIVersion:        version.APIVersion,
		OperatingSystem:   info.OperatingSystem,
		KernelVersion:     info.KernelVersion,
		Architecture:      info.Architecture,
		Containers:        info.Containers,
		ContainersRunning: info.ContainersRunning,
		ContainersPaused:  info.ContainersPaused,
		ContainersStopped: info.ContainersStopped,
		Images:            info.Images,
		StorageDriver:     info.Driver,
		LoggingDriver:     info.LoggingDriver,
		CgroupDriver:      info.CgroupDriver,
		DockerRootDir:     info.DockerRootDir,
		CPUs:              info.NCPU,
		MemTotal:          uint64(memTotal),
		ComposeVersion:    composeVersion,
	}, nil
}

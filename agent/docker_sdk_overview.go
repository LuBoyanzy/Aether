// docker_sdk_overview.go 实现 Docker 引擎概览信息读取。
// 该数据用于概览面板展示。
package agent

import (
	"errors"
	"fmt"

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
	}, nil
}

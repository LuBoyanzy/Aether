// docker_sdk_container.go 实现容器相关的 Docker SDK 操作。
// 包括容器列表、详情、日志与启停操作。
package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	dockermodel "aether/internal/entities/docker"

	"github.com/docker/docker/api/types/container"
)

const (
	composeProjectLabel     = "com.docker.compose.project"
	composeWorkingDirLabel  = "com.docker.compose.project.working_dir"
	composeConfigFilesLabel = "com.docker.compose.project.config_files"
	composeServiceLabel     = "com.docker.compose.service"
)

func (dm *dockerSDKManager) ListContainers(all bool) ([]dockermodel.Container, error) {
	if err := dm.ensureAvailable(); err != nil {
		return nil, err
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	list, err := dm.client.ContainerList(ctx, container.ListOptions{All: all})
	if err != nil {
		return nil, err
	}

	containers := make([]dockermodel.Container, 0, len(list))
	for _, item := range list {
		name := ""
		if len(item.Names) > 0 {
			name = strings.TrimPrefix(item.Names[0], "/")
		}
		ports := make([]dockermodel.Port, 0, len(item.Ports))
		for _, port := range item.Ports {
			ports = append(ports, dockermodel.Port{
				IP:          port.IP,
				PrivatePort: uint16(port.PrivatePort),
				PublicPort:  uint16(port.PublicPort),
				Type:        port.Type,
			})
		}

		networks := make([]string, 0, len(item.NetworkSettings.Networks))
		for networkName := range item.NetworkSettings.Networks {
			networks = append(networks, networkName)
		}

		createdBy := ""
		if item.Labels != nil {
			createdBy = item.Labels[composeProjectLabel]
		}

		containers = append(containers, dockermodel.Container{
			ID:        item.ID,
			Name:      name,
			Image:     item.Image,
			ImageID:   item.ImageID,
			State:     item.State,
			Status:    item.Status,
			Created:   item.Created,
			Ports:     ports,
			Labels:    item.Labels,
			Networks:  networks,
			Command:   item.Command,
			CreatedBy: createdBy,
		})
	}
	return containers, nil
}

func (dm *dockerSDKManager) GetContainerInfo(containerID string) ([]byte, error) {
	if err := dm.ensureAvailable(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(containerID) == "" {
		return nil, errors.New("container id is required")
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	info, err := dm.client.ContainerInspect(ctx, containerID)
	if err != nil {
		return nil, err
	}
	if info.Config != nil {
		info.Config.Env = nil
	}
	return json.Marshal(info)
}

func (dm *dockerSDKManager) GetContainerLogs(containerID string) (string, error) {
	if err := dm.ensureAvailable(); err != nil {
		return "", err
	}
	if strings.TrimSpace(containerID) == "" {
		return "", errors.New("container id is required")
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	reader, err := dm.client.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", dockerLogsTail),
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var builder strings.Builder
	if err := decodeDockerLogStream(reader, &builder); err != nil {
		return "", err
	}

	logs := builder.String()
	if strings.Contains(logs, "\x1b") {
		logs = ansiEscapePattern.ReplaceAllString(logs, "")
	}
	return logs, nil
}

func (dm *dockerSDKManager) OperateContainer(containerID, operation, signal string) error {
	if err := dm.ensureAvailable(); err != nil {
		return err
	}
	if strings.TrimSpace(containerID) == "" {
		return errors.New("container id is required")
	}
	op := strings.ToLower(strings.TrimSpace(operation))
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	switch op {
	case "start":
		return dm.client.ContainerStart(ctx, containerID, container.StartOptions{})
	case "stop":
		return dm.client.ContainerStop(ctx, containerID, container.StopOptions{})
	case "restart":
		return dm.client.ContainerRestart(ctx, containerID, container.StopOptions{})
	case "kill":
		return dm.client.ContainerKill(ctx, containerID, signal)
	case "pause":
		return dm.client.ContainerPause(ctx, containerID)
	case "unpause":
		return dm.client.ContainerUnpause(ctx, containerID)
	default:
		return fmt.Errorf("unsupported operation: %s", operation)
	}
}

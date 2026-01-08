// docker_sdk_volume.go 实现存储卷相关的 Docker SDK 操作。
// 包括卷列表、创建与删除。
package agent

import (
	"errors"
	"strings"

	dockermodel "aether/internal/entities/docker"

	"github.com/docker/docker/api/types/volume"
)

func (dm *dockerSDKManager) ListVolumes() ([]dockermodel.Volume, error) {
	if err := dm.ensureAvailable(); err != nil {
		return nil, err
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	list, err := dm.client.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return nil, err
	}

	volumes := make([]dockermodel.Volume, 0, len(list.Volumes))
	for _, item := range list.Volumes {
		volumes = append(volumes, dockermodel.Volume{
			Name:       item.Name,
			Driver:     item.Driver,
			Mountpoint: item.Mountpoint,
			CreatedAt:  item.CreatedAt,
			Scope:      item.Scope,
			Labels:     item.Labels,
			Options:    item.Options,
		})
	}
	return volumes, nil
}

func (dm *dockerSDKManager) CreateVolume(req volume.CreateOptions) (string, error) {
	if err := dm.ensureAvailable(); err != nil {
		return "", err
	}
	if strings.TrimSpace(req.Name) == "" {
		return "", errors.New("volume name is required")
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	created, err := dm.client.VolumeCreate(ctx, req)
	if err != nil {
		return "", err
	}
	return created.Name, nil
}

func (dm *dockerSDKManager) RemoveVolume(name string, force bool) error {
	if err := dm.ensureAvailable(); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("volume name is required")
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	return dm.client.VolumeRemove(ctx, name, force)
}

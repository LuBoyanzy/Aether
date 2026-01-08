// docker_sdk_image.go 实现镜像相关的 Docker SDK 操作。
// 包括镜像列表、拉取、推送与删除。
package agent

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"aether/internal/common"
	dockermodel "aether/internal/entities/docker"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/registry"
)

func (dm *dockerSDKManager) ListImages(all bool) ([]dockermodel.Image, error) {
	if err := dm.ensureAvailable(); err != nil {
		return nil, err
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	list, err := dm.client.ImageList(ctx, image.ListOptions{All: all})
	if err != nil {
		return nil, err
	}

	images := make([]dockermodel.Image, 0, len(list))
	for _, item := range list {
		images = append(images, dockermodel.Image{
			ID:          item.ID,
			RepoTags:    item.RepoTags,
			RepoDigests: item.RepoDigests,
			Created:     item.Created,
			Size:        item.Size,
			SharedSize:  item.SharedSize,
			VirtualSize: item.VirtualSize,
			Containers:  item.Containers,
			Labels:      item.Labels,
		})
	}
	return images, nil
}

func (dm *dockerSDKManager) PullImage(imageName string, auth *registry.AuthConfig) (string, error) {
	if err := dm.ensureAvailable(); err != nil {
		return "", err
	}
	if strings.TrimSpace(imageName) == "" {
		return "", errors.New("image is required")
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	opts := image.PullOptions{}
	if auth != nil {
		encoded, err := registry.EncodeAuthConfig(*auth)
		if err != nil {
			return "", err
		}
		opts.RegistryAuth = encoded
	}
	reader, err := dm.client.ImagePull(ctx, imageName, opts)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	return readLimitedStream(reader, maxTotalLogSize)
}

func (dm *dockerSDKManager) PushImage(imageName string, auth *registry.AuthConfig) (string, error) {
	if err := dm.ensureAvailable(); err != nil {
		return "", err
	}
	if strings.TrimSpace(imageName) == "" {
		return "", errors.New("image is required")
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	opts := image.PushOptions{}
	if auth != nil {
		encoded, err := registry.EncodeAuthConfig(*auth)
		if err != nil {
			return "", err
		}
		opts.RegistryAuth = encoded
	}

	reader, err := dm.client.ImagePush(ctx, imageName, opts)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	return readLimitedStream(reader, maxTotalLogSize)
}

func (dm *dockerSDKManager) RemoveImage(imageID string, force bool) error {
	if err := dm.ensureAvailable(); err != nil {
		return err
	}
	if strings.TrimSpace(imageID) == "" {
		return errors.New("image id is required")
	}
	ctx, cancel := dm.newTimeoutContext()
	defer cancel()

	_, err := dm.client.ImageRemove(ctx, imageID, image.RemoveOptions{Force: force})
	return err
}

// readLimitedStream 读取 Docker 返回的日志流，并限制最大读取长度。
func readLimitedStream(reader io.Reader, limit int64) (string, error) {
	if limit <= 0 {
		return "", fmt.Errorf("invalid limit: %d", limit)
	}
	limited := io.LimitReader(reader, limit)
	content, err := io.ReadAll(limited)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// buildAuthConfig 将通用鉴权信息转换为 Docker SDK 需要的 AuthConfig。
func buildAuthConfig(auth *common.DockerRegistryAuth) *registry.AuthConfig {
	if auth == nil {
		return nil
	}
	return &registry.AuthConfig{
		ServerAddress: auth.Server,
		Username:      auth.Username,
		Password:      auth.Password,
	}
}

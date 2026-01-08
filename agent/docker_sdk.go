// docker_sdk.go 提供 Docker SDK 客户端初始化与通用辅助方法，用于 Agent 的 Docker 读写通道。
// 该模块负责读取配置并生成统一的 SDK 客户端。
package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/docker/docker/client"
)

// dockerSDKManager 管理 Docker SDK 客户端。
type dockerSDKManager struct {
	client  *client.Client
	host    string
	timeout time.Duration
}

// getDockerSDK 返回可用的 Docker SDK 管理器或初始化错误。
func (a *Agent) getDockerSDK() (*dockerSDKManager, error) {
	if a == nil {
		return nil, errors.New("agent is nil")
	}
	if a.dockerSDKErr != nil {
		return nil, a.dockerSDKErr
	}
	if a.dockerSDKManager == nil || a.dockerSDKManager.client == nil {
		return nil, errors.New("docker sdk not available")
	}
	return a.dockerSDKManager, nil
}

// newDockerSDKManager 初始化 Docker SDK 客户端。
func newDockerSDKManager() (*dockerSDKManager, error) {
	dockerHost, exists := GetEnv("DOCKER_HOST")
	if exists && dockerHost == "" {
		// 显式关闭 Docker SDK 功能
		return nil, nil
	}
	if !exists || dockerHost == "" {
		dockerHost = getDockerHost()
	}
	if dockerHost == "" {
		return nil, errors.New("DOCKER_HOST not found")
	}

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithHost(dockerHost), client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("init docker sdk client failed: %w", err)
	}

	timeout := time.Millisecond * time.Duration(dockerTimeoutMs)
	if t, set := GetEnv("DOCKER_TIMEOUT"); set {
		parsed, parseErr := time.ParseDuration(t)
		if parseErr != nil {
			_ = cli.Close()
			return nil, fmt.Errorf("invalid DOCKER_TIMEOUT: %w", parseErr)
		}
		timeout = parsed
	}

	manager := &dockerSDKManager{
		client:  cli,
		host:    dockerHost,
		timeout: timeout,
	}
	return manager, nil
}

// newTimeoutContext 创建带超时的上下文。
func (dm *dockerSDKManager) newTimeoutContext() (context.Context, context.CancelFunc) {
	if dm == nil {
		return context.WithTimeout(context.Background(), 0)
	}
	if dm.timeout <= 0 {
		return context.WithTimeout(context.Background(), time.Millisecond*time.Duration(dockerTimeoutMs))
	}
	return context.WithTimeout(context.Background(), dm.timeout)
}

// ensureAvailable 校验 Docker SDK 是否可用。
func (dm *dockerSDKManager) ensureAvailable() error {
	if dm == nil || dm.client == nil {
		return errors.New("docker sdk not available")
	}
	return nil
}

// close 关闭 Docker SDK 客户端。
func (dm *dockerSDKManager) close() {
	if dm == nil || dm.client == nil {
		return
	}
	if err := dm.client.Close(); err != nil {
		slog.Error("Docker SDK close failed", "err", err)
	}
}

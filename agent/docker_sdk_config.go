// docker_sdk_config.go 实现 Docker 配置文件读取与更新。
// 该模块负责 daemon.json 的读写与可选重启。
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	dockermodel "aether/internal/entities/docker"
)

const dockerDaemonDefaultLinuxPath = "/etc/docker/daemon.json"
const dockerDaemonDefaultWindowsPath = `C:\ProgramData\docker\config\daemon.json`

func resolveDockerDaemonPath(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return override, nil
	}
	if envPath, ok := GetEnv("DOCKER_DAEMON_JSON"); ok && strings.TrimSpace(envPath) != "" {
		return envPath, nil
	}
	switch runtime.GOOS {
	case "windows":
		return dockerDaemonDefaultWindowsPath, nil
	default:
		return dockerDaemonDefaultLinuxPath, nil
	}
}

func (dm *dockerSDKManager) ReadDaemonConfig(pathOverride string) (*dockermodel.DaemonConfig, error) {
	path, err := resolveDockerDaemonPath(pathOverride)
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &dockermodel.DaemonConfig{Path: path, Content: "", Exists: false}, nil
		}
		return nil, err
	}
	return &dockermodel.DaemonConfig{Path: path, Content: string(content), Exists: true}, nil
}

func (dm *dockerSDKManager) UpdateDaemonConfig(content string, pathOverride string, restart bool) error {
	if strings.TrimSpace(content) == "" {
		return errors.New("daemon.json content is required")
	}
	if !json.Valid([]byte(content)) {
		return errors.New("daemon.json is not valid JSON")
	}
	path, err := resolveDockerDaemonPath(pathOverride)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0640); err != nil {
		return err
	}
	if restart {
		return restartDockerService()
	}
	return nil
}

func restartDockerService() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if path, err := exec.LookPath("systemctl"); err == nil {
		return runCommand(ctx, path, "restart", "docker")
	}
	if path, err := exec.LookPath("service"); err == nil {
		return runCommand(ctx, path, "docker", "restart")
	}
	return errors.New("docker service restart is not supported on this system")
}

func runCommand(ctx context.Context, cmd string, args ...string) error {
	output, err := exec.CommandContext(ctx, cmd, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %s %s: %w: %s", cmd, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return nil
}

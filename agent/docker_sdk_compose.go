// docker_sdk_compose.go 实现编排项目的管理逻辑。
// 通过 Docker Compose 命令完成编排的创建、更新、操作与删除。
package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"aether/internal/common"
	dockermodel "aether/internal/entities/docker"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"gopkg.in/yaml.v3"
)

const (
	composeFileName    = "docker-compose.yml"
	composeEnvFile     = ".env"
	composeOutputLimit = 64 * 1024
)

var composeNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

func addComposeService(project *dockermodel.ComposeProject, service string) {
	service = strings.TrimSpace(service)
	if service == "" {
		return
	}
	for _, existing := range project.Services {
		if existing == service {
			return
		}
	}
	project.Services = append(project.Services, service)
}

func splitComposeConfigFiles(value string) []string {
	parts := strings.Split(value, ",")
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		files = append(files, item)
	}
	return files
}

func (a *Agent) composeBaseDir() (string, error) {
	baseDir := a.dataDir
	if strings.TrimSpace(baseDir) == "" {
		resolved, err := getDataDir()
		if err != nil {
			return "", err
		}
		baseDir = resolved
	}
	return filepath.Join(baseDir, "docker", "compose"), nil
}

func validateComposeName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("compose project name is required")
	}
	if !composeNamePattern.MatchString(name) {
		return fmt.Errorf("invalid compose project name: %s", name)
	}
	return nil
}

func validateComposeContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return errors.New("compose content is required")
	}
	var payload struct {
		Services map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(content), &payload); err != nil {
		return fmt.Errorf("compose yaml invalid: %w", err)
	}
	if len(payload.Services) == 0 {
		return errors.New("compose services is required")
	}
	return nil
}

func resolveComposeCommand(ctx context.Context) (string, []string, error) {
	if cmd, ok := GetEnv("DOCKER_COMPOSE_CMD"); ok && strings.TrimSpace(cmd) != "" {
		return cmd, []string{}, nil
	}
	if dockerPath, err := exec.LookPath("docker"); err == nil {
		if err := exec.CommandContext(ctx, dockerPath, "compose", "version").Run(); err == nil {
			return dockerPath, []string{"compose"}, nil
		}
	}
	if composePath, err := exec.LookPath("docker-compose"); err == nil {
		return composePath, []string{}, nil
	}
	return "", nil, errors.New("docker compose command not found")
}

func runCompose(ctx context.Context, workdir string, args ...string) (string, error) {
	cmdPath, baseArgs, err := resolveComposeCommand(ctx)
	if err != nil {
		return "", err
	}
	cmdArgs := append(baseArgs, args...)
	cmd := exec.CommandContext(ctx, cmdPath, cmdArgs...)
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if len(output) > composeOutputLimit {
		output = output[:composeOutputLimit]
	}
	if err != nil {
		return string(output), fmt.Errorf("compose command failed: %w", err)
	}
	return string(output), nil
}

func (a *Agent) ListComposeProjects() ([]dockermodel.ComposeProject, error) {
	sdk, err := a.getDockerSDK()
	if err != nil {
		return nil, err
	}
	baseDir, err := a.composeBaseDir()
	if err != nil {
		return nil, err
	}

	ctx, cancel := sdk.newTimeoutContext()
	defer cancel()
	args := filters.NewArgs(filters.Arg("label", composeProjectLabel))
	containers, err := sdk.client.ContainerList(ctx, container.ListOptions{All: true, Filters: args})
	if err != nil {
		return nil, err
	}

	projectMap := make(map[string]*dockermodel.ComposeProject)
	for _, item := range containers {
		projectName := strings.TrimSpace(item.Labels[composeProjectLabel])
		if projectName == "" {
			continue
		}
		entry, ok := projectMap[projectName]
		if !ok {
			entry = &dockermodel.ComposeProject{
				Name: projectName,
			}
			projectMap[projectName] = entry
		}
		if entry.Workdir == "" {
			if workdir := strings.TrimSpace(item.Labels[composeWorkingDirLabel]); workdir != "" {
				entry.Workdir = workdir
			}
		}
		if len(entry.ConfigFiles) == 0 {
			if configFiles := strings.TrimSpace(item.Labels[composeConfigFilesLabel]); configFiles != "" {
				entry.ConfigFiles = splitComposeConfigFiles(configFiles)
			}
		}
		addComposeService(entry, item.Labels[composeServiceLabel])
		entry.ContainerCount++
		if strings.ToLower(item.State) == "running" {
			entry.RunningCount++
		}
		entry.Containers = append(entry.Containers, dockermodel.ComposeContainer{
			ID:      item.ID,
			Name:    strings.TrimPrefix(item.Names[0], "/"),
			Image:   item.Image,
			State:   item.State,
			Status:  item.Status,
			Created: item.Created,
			Ports:   mapPorts(item.Ports),
		})
	}

	entries, err := os.ReadDir(baseDir)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		project, ok := projectMap[name]
		if !ok {
			project = &dockermodel.ComposeProject{Name: name}
			projectMap[name] = project
		}
		workdir := filepath.Join(baseDir, name)
		if project.Workdir == "" {
			project.Workdir = workdir
		}
		composePath := filepath.Join(workdir, composeFileName)
		if len(project.ConfigFiles) == 0 {
			project.ConfigFiles = []string{composePath}
		}
		info, statErr := entry.Info()
		if statErr == nil {
			project.CreatedAt = info.ModTime().Unix()
			project.UpdatedAt = info.ModTime().Unix()
		}
		services, parseErr := parseComposeServices(composePath)
		if parseErr != nil {
			return nil, parseErr
		}
		for _, service := range services {
			addComposeService(project, service)
		}
	}

	projects := make([]dockermodel.ComposeProject, 0, len(projectMap))
	for _, project := range projectMap {
		project.Status = composeStatus(project.ContainerCount, project.RunningCount)
		projects = append(projects, *project)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})
	return projects, nil
}

func parseComposeServices(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var payload struct {
		Services map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse compose file failed: %w", err)
	}
	services := make([]string, 0, len(payload.Services))
	for name := range payload.Services {
		services = append(services, name)
	}
	sort.Strings(services)
	return services, nil
}

func composeStatus(total, running int) string {
	if total == 0 {
		return "stopped"
	}
	if running == total {
		return "running"
	}
	if running > 0 {
		return "partial"
	}
	return "stopped"
}

func (a *Agent) CreateComposeProject(req common.DockerComposeProjectCreateRequest) (string, error) {
	if err := validateComposeName(req.Name); err != nil {
		return "", err
	}
	if err := validateComposeContent(req.Content); err != nil {
		return "", err
	}
	baseDir, err := a.composeBaseDir()
	if err != nil {
		return "", err
	}
	workdir := filepath.Join(baseDir, req.Name)
	if _, err := os.Stat(workdir); err == nil {
		return "", fmt.Errorf("compose project already exists: %s", req.Name)
	}
	if err := os.MkdirAll(workdir, 0755); err != nil {
		return "", err
	}
	composePath := filepath.Join(workdir, composeFileName)
	if err := os.WriteFile(composePath, []byte(req.Content), 0640); err != nil {
		return "", err
	}
	if strings.TrimSpace(req.Env) != "" {
		envPath := filepath.Join(workdir, composeEnvFile)
		if err := os.WriteFile(envPath, []byte(req.Env), 0640); err != nil {
			return "", err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	return runCompose(ctx, workdir, "-f", composePath, "up", "-d")
}

func (a *Agent) UpdateComposeProject(req common.DockerComposeProjectUpdateRequest) (string, error) {
	if err := validateComposeName(req.Name); err != nil {
		return "", err
	}
	if err := validateComposeContent(req.Content); err != nil {
		return "", err
	}
	baseDir, err := a.composeBaseDir()
	if err != nil {
		return "", err
	}
	workdir := filepath.Join(baseDir, req.Name)
	composePath := filepath.Join(workdir, composeFileName)
	if _, err := os.Stat(composePath); err != nil {
		return "", err
	}
	if err := os.WriteFile(composePath, []byte(req.Content), 0640); err != nil {
		return "", err
	}
	if strings.TrimSpace(req.Env) != "" {
		envPath := filepath.Join(workdir, composeEnvFile)
		if err := os.WriteFile(envPath, []byte(req.Env), 0640); err != nil {
			return "", err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	return runCompose(ctx, workdir, "-f", composePath, "up", "-d")
}

func (a *Agent) OperateComposeProject(req common.DockerComposeProjectOperateRequest) (string, error) {
	if err := validateComposeName(req.Name); err != nil {
		return "", err
	}
	operation := strings.ToLower(strings.TrimSpace(req.Operation))
	allowed := map[string]bool{"up": true, "down": true, "start": true, "stop": true, "restart": true, "pull": true}
	if !allowed[operation] {
		return "", fmt.Errorf("unsupported compose operation: %s", req.Operation)
	}
	baseDir, err := a.composeBaseDir()
	if err != nil {
		return "", err
	}
	workdir := filepath.Join(baseDir, req.Name)
	composePath := filepath.Join(workdir, composeFileName)
	if _, err := os.Stat(composePath); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	args := []string{"-f", composePath, operation}
	if operation == "up" {
		args = append(args, "-d")
	}
	if operation == "down" {
		args = append(args, "--remove-orphans")
	}
	return runCompose(ctx, workdir, args...)
}

func (a *Agent) DeleteComposeProject(req common.DockerComposeProjectDeleteRequest) (string, error) {
	if err := validateComposeName(req.Name); err != nil {
		return "", err
	}
	baseDir, err := a.composeBaseDir()
	if err != nil {
		return "", err
	}
	workdir := filepath.Join(baseDir, req.Name)
	composePath := filepath.Join(workdir, composeFileName)
	if _, err := os.Stat(composePath); err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	output, err := runCompose(ctx, workdir, "-f", composePath, "down", "--remove-orphans")
	if err != nil {
		return output, err
	}
	if req.RemoveFile {
		if err := os.RemoveAll(workdir); err != nil {
			return output, err
		}
	}
	return output, nil
}

func mapPorts(ports []container.Port) []dockermodel.Port {
	result := make([]dockermodel.Port, 0, len(ports))
	for _, port := range ports {
		result = append(result, dockermodel.Port{
			IP:          port.IP,
			PrivatePort: uint16(port.PrivatePort),
			PublicPort:  uint16(port.PublicPort),
			Type:        port.Type,
		})
	}
	return result
}

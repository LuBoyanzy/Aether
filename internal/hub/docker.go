// Package hub 提供 Docker 模块相关 API 路由与处理器。
// 该文件负责 Hub 与 Agent 间的 Docker 请求桥接与审计写入。
package hub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"aether/internal/common"
	"aether/internal/hub/systems"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
	"gopkg.in/yaml.v3"
)

func requireWritable(e *core.RequestEvent) error {
	if e.Auth == nil || e.Auth.GetString("role") == "readonly" {
		return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}
	return nil
}

func parseBoolParam(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "1" || value == "true" || value == "yes"
}

var (
	errSystemForbidden = errors.New("forbidden")
	errSystemNotFound  = errors.New("system not found")
)

func (h *Hub) resolveSystem(systemID string) (*systems.System, error) {
	if strings.TrimSpace(systemID) == "" {
		return nil, errors.New("system is required")
	}
	system, err := h.sm.GetSystem(systemID)
	if err != nil {
		return nil, err
	}
	return system, nil
}

func (h *Hub) resolveSystemRecordForUser(e *core.RequestEvent, systemID string) (*core.Record, error) {
	systemID = strings.TrimSpace(systemID)
	if systemID == "" {
		return nil, errors.New("system is required")
	}
	record, err := h.FindRecordById("systems", systemID)
	if err != nil {
		return nil, errSystemNotFound
	}
	shareAllSystems, _ := GetEnv("SHARE_ALL_SYSTEMS")
	if shareAllSystems == "true" {
		return record, nil
	}
	if e.Auth == nil {
		return nil, errSystemForbidden
	}
	for _, userID := range record.GetStringSlice("users") {
		if userID == e.Auth.Id {
			return record, nil
		}
	}
	return nil, errSystemForbidden
}

func (h *Hub) logServiceConfigError(message string, err error, fields ...any) {
	if err == nil {
		return
	}
	payload := []any{
		"logger", "hub",
		"err", err,
		"stack", string(debug.Stack()),
	}
	payload = append(payload, fields...)
	h.Logger().Error(message, payload...)
}

func validateServiceConfigURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", errors.New("url is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("url scheme must be http or https")
	}
	if parsed.Host == "" {
		return "", errors.New("url host is required")
	}
	return trimmed, nil
}

func respondSystemAccessError(e *core.RequestEvent, err error) error {
	switch {
	case errors.Is(err, errSystemForbidden):
		return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, errSystemNotFound):
		return e.JSON(http.StatusNotFound, map[string]string{"error": "system not found"})
	default:
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
}

func (h *Hub) getDockerOverview(e *core.RequestEvent) error {
	systemID := e.Request.URL.Query().Get("system")
	system, err := h.resolveSystem(systemID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	overview, err := system.FetchDockerOverviewFromAgent()
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, overview)
}

func (h *Hub) listDockerContainers(e *core.RequestEvent) error {
	systemID := e.Request.URL.Query().Get("system")
	all := parseBoolParam(e.Request.URL.Query().Get("all"))
	system, err := h.resolveSystem(systemID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	containers, err := system.FetchDockerContainersFromAgent(all)
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, containers)
}

func (h *Hub) listDockerImages(e *core.RequestEvent) error {
	systemID := e.Request.URL.Query().Get("system")
	all := parseBoolParam(e.Request.URL.Query().Get("all"))
	system, err := h.resolveSystem(systemID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	images, err := system.FetchDockerImagesFromAgent(all)
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, images)
}

type dockerImageOpPayload struct {
	System     string `json:"system"`
	Image      string `json:"image"`
	RegistryID string `json:"registryId"`
	Force      bool   `json:"force"`
}

func (h *Hub) pullDockerImage(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerImageOpPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	auth, err := h.getRegistryAuth(payload.RegistryID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	logs, err := system.PullDockerImageFromAgent(common.DockerImagePullRequest{Image: payload.Image, Registry: auth})
	status := dockerAuditStatusSuccess
	message := "pull image"
	if err != nil {
		status = dockerAuditStatusFailed
		message = err.Error()
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     payload.System,
		UserID:       e.Auth.Id,
		Action:       "image.pull",
		ResourceType: "image",
		ResourceID:   payload.Image,
		Status:       status,
		Detail:       message,
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok", "logs": logs})
}

func (h *Hub) pushDockerImage(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerImageOpPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	auth, err := h.getRegistryAuth(payload.RegistryID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	logs, err := system.PushDockerImageFromAgent(common.DockerImagePushRequest{Image: payload.Image, Registry: auth})
	status := dockerAuditStatusSuccess
	message := "push image"
	if err != nil {
		status = dockerAuditStatusFailed
		message = err.Error()
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     payload.System,
		UserID:       e.Auth.Id,
		Action:       "image.push",
		ResourceType: "image",
		ResourceID:   payload.Image,
		Status:       status,
		Detail:       message,
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok", "logs": logs})
}

func (h *Hub) removeDockerImage(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerImageOpPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	err = system.RemoveDockerImageFromAgent(common.DockerImageRemoveRequest{ImageID: payload.Image, Force: payload.Force})
	status := dockerAuditStatusSuccess
	message := "remove image"
	if err != nil {
		status = dockerAuditStatusFailed
		message = err.Error()
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     payload.System,
		UserID:       e.Auth.Id,
		Action:       "image.remove",
		ResourceType: "image",
		ResourceID:   payload.Image,
		Status:       status,
		Detail:       message,
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

type dockerNetworkPayload struct {
	System     string            `json:"system"`
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	EnableIPv6 bool              `json:"enableIPv6"`
	Internal   bool              `json:"internal"`
	Attachable bool              `json:"attachable"`
	Labels     map[string]string `json:"labels"`
	Options    map[string]string `json:"options"`
	NetworkID  string            `json:"networkId"`
}

func (h *Hub) listDockerNetworks(e *core.RequestEvent) error {
	systemID := e.Request.URL.Query().Get("system")
	system, err := h.resolveSystem(systemID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	items, err := system.FetchDockerNetworksFromAgent()
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, items)
}

func (h *Hub) createDockerNetwork(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerNetworkPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	err = system.CreateDockerNetworkFromAgent(common.DockerNetworkCreateRequest{
		Name:       payload.Name,
		Driver:     payload.Driver,
		EnableIPv6: payload.EnableIPv6,
		Internal:   payload.Internal,
		Attachable: payload.Attachable,
		Labels:     payload.Labels,
		Options:    payload.Options,
	})
	status := dockerAuditStatusSuccess
	message := "create network"
	if err != nil {
		status = dockerAuditStatusFailed
		message = err.Error()
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     payload.System,
		UserID:       e.Auth.Id,
		Action:       "network.create",
		ResourceType: "network",
		ResourceID:   payload.Name,
		Status:       status,
		Detail:       message,
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Hub) removeDockerNetwork(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerNetworkPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	err = system.RemoveDockerNetworkFromAgent(common.DockerNetworkRemoveRequest{NetworkID: payload.NetworkID})
	status := dockerAuditStatusSuccess
	message := "remove network"
	if err != nil {
		status = dockerAuditStatusFailed
		message = err.Error()
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     payload.System,
		UserID:       e.Auth.Id,
		Action:       "network.remove",
		ResourceType: "network",
		ResourceID:   payload.NetworkID,
		Status:       status,
		Detail:       message,
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

type dockerVolumePayload struct {
	System  string            `json:"system"`
	Name    string            `json:"name"`
	Driver  string            `json:"driver"`
	Labels  map[string]string `json:"labels"`
	Options map[string]string `json:"options"`
	Force   bool              `json:"force"`
}

func (h *Hub) listDockerVolumes(e *core.RequestEvent) error {
	systemID := e.Request.URL.Query().Get("system")
	system, err := h.resolveSystem(systemID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	items, err := system.FetchDockerVolumesFromAgent()
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, items)
}

func (h *Hub) createDockerVolume(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerVolumePayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	err = system.CreateDockerVolumeFromAgent(common.DockerVolumeCreateRequest{
		Name:    payload.Name,
		Driver:  payload.Driver,
		Labels:  payload.Labels,
		Options: payload.Options,
	})
	status := dockerAuditStatusSuccess
	message := "create volume"
	if err != nil {
		status = dockerAuditStatusFailed
		message = err.Error()
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     payload.System,
		UserID:       e.Auth.Id,
		Action:       "volume.create",
		ResourceType: "volume",
		ResourceID:   payload.Name,
		Status:       status,
		Detail:       message,
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Hub) removeDockerVolume(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerVolumePayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	err = system.RemoveDockerVolumeFromAgent(common.DockerVolumeRemoveRequest{Name: payload.Name, Force: payload.Force})
	status := dockerAuditStatusSuccess
	message := "remove volume"
	if err != nil {
		status = dockerAuditStatusFailed
		message = err.Error()
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     payload.System,
		UserID:       e.Auth.Id,
		Action:       "volume.remove",
		ResourceType: "volume",
		ResourceID:   payload.Name,
		Status:       status,
		Detail:       message,
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

type dockerComposePayload struct {
	System     string `json:"system"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	Env        string `json:"env"`
	Operation  string `json:"operation"`
	RemoveFile bool   `json:"removeFile"`
}

func (h *Hub) listDockerComposeProjects(e *core.RequestEvent) error {
	systemID := e.Request.URL.Query().Get("system")
	system, err := h.resolveSystem(systemID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	items, err := system.FetchDockerComposeProjectsFromAgent()
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, items)
}

func (h *Hub) createDockerComposeProject(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerComposePayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	output, err := system.CreateDockerComposeProjectFromAgent(common.DockerComposeProjectCreateRequest{
		Name:    payload.Name,
		Content: payload.Content,
		Env:     payload.Env,
	})
	status := dockerAuditStatusSuccess
	message := "create compose"
	if err != nil {
		status = dockerAuditStatusFailed
		message = err.Error()
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     payload.System,
		UserID:       e.Auth.Id,
		Action:       "compose.create",
		ResourceType: "compose",
		ResourceID:   payload.Name,
		Status:       status,
		Detail:       message,
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok", "logs": output})
}

func (h *Hub) updateDockerComposeProject(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerComposePayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	output, err := system.UpdateDockerComposeProjectFromAgent(common.DockerComposeProjectUpdateRequest{
		Name:    payload.Name,
		Content: payload.Content,
		Env:     payload.Env,
	})
	status := dockerAuditStatusSuccess
	message := "update compose"
	if err != nil {
		status = dockerAuditStatusFailed
		message = err.Error()
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     payload.System,
		UserID:       e.Auth.Id,
		Action:       "compose.update",
		ResourceType: "compose",
		ResourceID:   payload.Name,
		Status:       status,
		Detail:       message,
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok", "logs": output})
}

func (h *Hub) operateDockerComposeProject(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerComposePayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	output, err := system.OperateDockerComposeProjectFromAgent(common.DockerComposeProjectOperateRequest{
		Name:      payload.Name,
		Operation: payload.Operation,
	})
	status := dockerAuditStatusSuccess
	message := "operate compose"
	if err != nil {
		status = dockerAuditStatusFailed
		message = err.Error()
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     payload.System,
		UserID:       e.Auth.Id,
		Action:       "compose.operate",
		ResourceType: "compose",
		ResourceID:   payload.Name,
		Status:       status,
		Detail:       message,
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok", "logs": output})
}

func (h *Hub) deleteDockerComposeProject(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerComposePayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	output, err := system.DeleteDockerComposeProjectFromAgent(common.DockerComposeProjectDeleteRequest{
		Name:       payload.Name,
		RemoveFile: payload.RemoveFile,
	})
	status := dockerAuditStatusSuccess
	message := "delete compose"
	if err != nil {
		status = dockerAuditStatusFailed
		message = err.Error()
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     payload.System,
		UserID:       e.Auth.Id,
		Action:       "compose.delete",
		ResourceType: "compose",
		ResourceID:   payload.Name,
		Status:       status,
		Detail:       message,
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok", "logs": output})
}

func (h *Hub) getDockerConfig(e *core.RequestEvent) error {
	systemID := e.Request.URL.Query().Get("system")
	system, err := h.resolveSystem(systemID)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	config, err := system.FetchDockerConfigFromAgent()
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, config)
}

type dockerConfigPayload struct {
	System  string `json:"system"`
	Content string `json:"content"`
	Path    string `json:"path"`
	Restart bool   `json:"restart"`
}

func (h *Hub) updateDockerConfig(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerConfigPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	err = system.UpdateDockerConfigFromAgent(common.DockerConfigUpdateRequest{
		Content: payload.Content,
		Path:    payload.Path,
		Restart: payload.Restart,
	})
	status := dockerAuditStatusSuccess
	message := "update docker config"
	if err != nil {
		status = dockerAuditStatusFailed
		message = err.Error()
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     payload.System,
		UserID:       e.Auth.Id,
		Action:       "config.update",
		ResourceType: "config",
		ResourceID:   payload.System,
		Status:       status,
		Detail:       message,
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

type dockerServiceConfigPayload struct {
	System string `json:"system"`
	Name   string `json:"name"`
	URL    string `json:"url"`
	Token  string `json:"token"`
}

type dockerServiceConfigUpdatePayload struct {
	ID    string  `json:"id"`
	Name  *string `json:"name"`
	URL   *string `json:"url"`
	Token *string `json:"token"`
}

type dockerServiceConfigContentPayload struct {
	System  string `json:"system"`
	ID      string `json:"id"`
	Content string `json:"content"`
}

type serviceConfigContentResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Content string `json:"content"`
	} `json:"data"`
}

type serviceConfigStatusResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (h *Hub) listDockerServiceConfigs(e *core.RequestEvent) error {
	systemID := strings.TrimSpace(e.Request.URL.Query().Get("system"))
	if _, err := h.resolveSystemRecordForUser(e, systemID); err != nil {
		return respondSystemAccessError(e, err)
	}
	records, err := h.FindRecordsByFilter(
		"docker_service_configs",
		"system = {:system}",
		"-created",
		-1,
		0,
		map[string]any{"system": systemID},
	)
	if err != nil {
		h.logServiceConfigError("list service configs failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, map[string]any{
			"id":      record.Id,
			"system":  record.GetString("system"),
			"name":    record.GetString("name"),
			"url":     record.GetString("url"),
			"created": record.Get("created"),
			"updated": record.Get("updated"),
		})
	}
	return e.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Hub) createDockerServiceConfig(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerServiceConfigPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if _, err := h.resolveSystemRecordForUser(e, payload.System); err != nil {
		return respondSystemAccessError(e, err)
	}
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}
	urlValue, err := validateServiceConfigURL(payload.URL)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	token := strings.TrimSpace(payload.Token)
	if token == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "token is required"})
	}
	collection, err := h.FindCollectionByNameOrId("docker_service_configs")
	if err != nil {
		h.logServiceConfigError("find service configs collection failed", err, "system", payload.System)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	record := core.NewRecord(collection)
	record.Set("system", strings.TrimSpace(payload.System))
	record.Set("name", name)
	record.Set("url", urlValue)
	record.Set("token", token)
	if err := h.Save(record); err != nil {
		h.logServiceConfigError("create service config failed", err, "system", payload.System, "name", name, "url", urlValue)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	h.Logger().Info(
		"service config created",
		"logger",
		"hub",
		"system",
		record.GetString("system"),
		"id",
		record.Id,
		"name",
		name,
		"url",
		urlValue,
	)
	return e.JSON(http.StatusOK, map[string]any{"id": record.Id})
}

func (h *Hub) updateDockerServiceConfig(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerServiceConfigUpdatePayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if strings.TrimSpace(payload.ID) == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}
	if payload.Token != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "token cannot be updated"})
	}
	if payload.Name == nil && payload.URL == nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "name or url is required"})
	}
	record, err := h.FindRecordById("docker_service_configs", payload.ID)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "service config not found"})
	}
	if _, err := h.resolveSystemRecordForUser(e, record.GetString("system")); err != nil {
		return respondSystemAccessError(e, err)
	}
	if payload.Name != nil {
		name := strings.TrimSpace(*payload.Name)
		if name == "" {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
		}
		record.Set("name", name)
	}
	if payload.URL != nil {
		urlValue, err := validateServiceConfigURL(*payload.URL)
		if err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		record.Set("url", urlValue)
	}
	if err := h.Save(record); err != nil {
		h.logServiceConfigError("update service config failed", err, "id", payload.ID, "system", record.GetString("system"))
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	h.Logger().Info(
		"service config updated",
		"logger",
		"hub",
		"system",
		record.GetString("system"),
		"id",
		record.Id,
	)
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Hub) deleteDockerServiceConfig(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	id := strings.TrimSpace(e.Request.URL.Query().Get("id"))
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}
	record, err := h.FindRecordById("docker_service_configs", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "service config not found"})
	}
	if _, err := h.resolveSystemRecordForUser(e, record.GetString("system")); err != nil {
		return respondSystemAccessError(e, err)
	}
	if err := h.Delete(record); err != nil {
		h.logServiceConfigError("delete service config failed", err, "id", id, "system", record.GetString("system"))
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	h.Logger().Info(
		"service config deleted",
		"logger",
		"hub",
		"system",
		record.GetString("system"),
		"id",
		record.Id,
	)
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Hub) getDockerServiceConfigContent(e *core.RequestEvent) error {
	query := e.Request.URL.Query()
	systemID := strings.TrimSpace(query.Get("system"))
	configID := strings.TrimSpace(query.Get("id"))
	if systemID == "" || configID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system and id are required"})
	}
	if _, err := h.resolveSystemRecordForUser(e, systemID); err != nil {
		return respondSystemAccessError(e, err)
	}
	record, err := h.FindRecordById("docker_service_configs", configID)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "service config not found"})
	}
	if record.GetString("system") != systemID {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system mismatch"})
	}
	targetURL := record.GetString("url")
	token := record.GetString("token")
	if strings.TrimSpace(targetURL) == "" || strings.TrimSpace(token) == "" {
		h.logServiceConfigError(
			"service config missing url or token",
			errors.New("service config missing url or token"),
			"system", systemID,
			"id", configID,
		)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "service config missing url or token"})
	}
	ctx, cancel := context.WithTimeout(e.Request.Context(), 10*time.Second)
	defer cancel()
	body, status, err := h.requestServiceConfig(ctx, http.MethodGet, targetURL, token, nil)
	if err != nil {
		h.logServiceConfigError(
			"service config fetch failed",
			err,
			"system", systemID,
			"id", configID,
			"url", targetURL,
			"status", status,
			"response_size", len(body),
		)
		if status == http.StatusForbidden {
			return e.JSON(http.StatusBadGateway, map[string]string{"error": "upstream rejected token"})
		}
		if status != http.StatusOK && status != 0 {
			return e.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("upstream status %d", status)})
		}
		return e.JSON(http.StatusBadGateway, map[string]string{"error": "failed to fetch config content"})
	}
	if status != http.StatusOK {
		h.logServiceConfigError(
			"service config fetch unexpected status",
			fmt.Errorf("status %d", status),
			"system", systemID,
			"id", configID,
			"url", targetURL,
			"response_size", len(body),
		)
		return e.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("upstream status %d", status)})
	}
	var response serviceConfigContentResponse
	if err := json.Unmarshal(body, &response); err != nil {
		h.logServiceConfigError(
			"service config fetch invalid response",
			err,
			"system", systemID,
			"id", configID,
			"url", targetURL,
		)
		return e.JSON(http.StatusBadGateway, map[string]string{"error": "invalid upstream response"})
	}
	if response.Code != http.StatusOK {
		message := strings.TrimSpace(response.Message)
		if message == "" {
			message = fmt.Sprintf("upstream error code %d", response.Code)
		}
		h.logServiceConfigError(
			"service config fetch upstream error",
			fmt.Errorf("upstream code %d", response.Code),
			"system", systemID,
			"id", configID,
			"url", targetURL,
			"message", message,
		)
		return e.JSON(http.StatusBadGateway, map[string]string{"error": message})
	}
	return e.JSON(http.StatusOK, map[string]any{"content": response.Data.Content})
}

func (h *Hub) updateDockerServiceConfigContent(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerServiceConfigContentPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	systemID := strings.TrimSpace(payload.System)
	if systemID == "" || strings.TrimSpace(payload.ID) == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system and id are required"})
	}
	if _, err := h.resolveSystemRecordForUser(e, systemID); err != nil {
		return respondSystemAccessError(e, err)
	}
	if strings.TrimSpace(payload.Content) == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "content is required"})
	}
	record, err := h.FindRecordById("docker_service_configs", payload.ID)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "service config not found"})
	}
	if record.GetString("system") != systemID {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system mismatch"})
	}
	targetURL := record.GetString("url")
	token := record.GetString("token")
	if strings.TrimSpace(targetURL) == "" || strings.TrimSpace(token) == "" {
		h.logServiceConfigError(
			"service config missing url or token",
			errors.New("service config missing url or token"),
			"system", systemID,
			"id", payload.ID,
		)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "service config missing url or token"})
	}
	requestBody, err := json.Marshal(map[string]string{"content": payload.Content})
	if err != nil {
		h.logServiceConfigError(
			"service config marshal failed",
			err,
			"system", systemID,
			"id", payload.ID,
			"url", targetURL,
			"content_len", len(payload.Content),
		)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to encode content"})
	}
	ctx, cancel := context.WithTimeout(e.Request.Context(), 10*time.Second)
	defer cancel()
	body, status, err := h.requestServiceConfig(ctx, http.MethodPut, targetURL, token, requestBody)
	if err != nil {
		h.logServiceConfigError(
			"service config update failed",
			err,
			"system", systemID,
			"id", payload.ID,
			"url", targetURL,
			"status", status,
			"response_size", len(body),
		)
		if status == http.StatusForbidden {
			return e.JSON(http.StatusBadGateway, map[string]string{"error": "upstream rejected token"})
		}
		if status != http.StatusOK && status != 0 {
			return e.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("upstream status %d", status)})
		}
		return e.JSON(http.StatusBadGateway, map[string]string{"error": "failed to update config content"})
	}
	if status != http.StatusOK {
		h.logServiceConfigError(
			"service config update unexpected status",
			fmt.Errorf("status %d", status),
			"system", systemID,
			"id", payload.ID,
			"url", targetURL,
			"response_size", len(body),
		)
		return e.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("upstream status %d", status)})
	}
	var response serviceConfigStatusResponse
	if err := json.Unmarshal(body, &response); err != nil {
		h.logServiceConfigError(
			"service config update invalid response",
			err,
			"system", systemID,
			"id", payload.ID,
			"url", targetURL,
		)
		return e.JSON(http.StatusBadGateway, map[string]string{"error": "invalid upstream response"})
	}
	if response.Code != http.StatusOK {
		message := strings.TrimSpace(response.Message)
		if message == "" {
			message = fmt.Sprintf("upstream error code %d", response.Code)
		}
		h.logServiceConfigError(
			"service config update upstream error",
			fmt.Errorf("upstream code %d", response.Code),
			"system", systemID,
			"id", payload.ID,
			"url", targetURL,
			"message", message,
		)
		return e.JSON(http.StatusBadGateway, map[string]string{"error": message})
	}
	h.Logger().Info(
		"service config content updated",
		"logger",
		"hub",
		"system",
		systemID,
		"id",
		payload.ID,
		"url",
		targetURL,
		"content_len",
		len(payload.Content),
	)
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Hub) requestServiceConfig(
	ctx context.Context,
	method string,
	endpoint string,
	token string,
	payload []byte,
) ([]byte, int, error) {
	requestBody := bytes.NewReader(payload)
	req, err := http.NewRequestWithContext(ctx, method, endpoint, requestBody)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-Config-Token", token)
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPut {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func (h *Hub) listDockerRegistries(e *core.RequestEvent) error {
	records, err := h.FindRecordsByFilter("docker_registries", "", "-created", -1, 0)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, map[string]any{
			"id":       record.Id,
			"name":     record.GetString("name"),
			"server":   record.GetString("server"),
			"username": record.GetString("username"),
			"created":  record.Get("created"),
			"updated":  record.Get("updated"),
		})
	}
	return e.JSON(http.StatusOK, map[string]any{"items": items})
}

type dockerRegistryPayload struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Server   string `json:"server"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type dockerRegistryUpdatePayload struct {
	ID       string  `json:"id"`
	Name     *string `json:"name"`
	Server   *string `json:"server"`
	Username *string `json:"username"`
	Password *string `json:"password"`
}

func (h *Hub) createDockerRegistry(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerRegistryPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if strings.TrimSpace(payload.Name) == "" || strings.TrimSpace(payload.Server) == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "name and server are required"})
	}
	collection, err := h.FindCollectionByNameOrId("docker_registries")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	record := core.NewRecord(collection)
	record.Set("name", payload.Name)
	record.Set("server", payload.Server)
	record.Set("username", payload.Username)
	record.Set("password", payload.Password)
	record.Set("created_by", e.Auth.Id)
	if err := h.Save(record); err != nil {
		if auditErr := h.recordDockerAudit(dockerAuditEntry{
			UserID:       e.Auth.Id,
			Action:       "registry.create",
			ResourceType: "registry",
			ResourceID:   record.Id,
			Status:       dockerAuditStatusFailed,
			Detail:       err.Error(),
		}); auditErr != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
		}
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		UserID:       e.Auth.Id,
		Action:       "registry.create",
		ResourceType: "registry",
		ResourceID:   record.Id,
		Status:       dockerAuditStatusSuccess,
		Detail:       "create registry",
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"id": record.Id})
}

func (h *Hub) updateDockerRegistry(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerRegistryUpdatePayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if strings.TrimSpace(payload.ID) == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}
	record, err := h.FindRecordById("docker_registries", payload.ID)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "registry not found"})
	}
	if payload.Name != nil {
		record.Set("name", strings.TrimSpace(*payload.Name))
	}
	if payload.Server != nil {
		record.Set("server", strings.TrimSpace(*payload.Server))
	}
	if payload.Username != nil {
		record.Set("username", strings.TrimSpace(*payload.Username))
	}
	if payload.Password != nil {
		record.Set("password", *payload.Password)
	}
	if err := h.Save(record); err != nil {
		if auditErr := h.recordDockerAudit(dockerAuditEntry{
			UserID:       e.Auth.Id,
			Action:       "registry.update",
			ResourceType: "registry",
			ResourceID:   payload.ID,
			Status:       dockerAuditStatusFailed,
			Detail:       err.Error(),
		}); auditErr != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
		}
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		UserID:       e.Auth.Id,
		Action:       "registry.update",
		ResourceType: "registry",
		ResourceID:   payload.ID,
		Status:       dockerAuditStatusSuccess,
		Detail:       "update registry",
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Hub) deleteDockerRegistry(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	id := e.Request.URL.Query().Get("id")
	if strings.TrimSpace(id) == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}
	record, err := h.FindRecordById("docker_registries", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "registry not found"})
	}
	if err := h.Delete(record); err != nil {
		if auditErr := h.recordDockerAudit(dockerAuditEntry{
			UserID:       e.Auth.Id,
			Action:       "registry.delete",
			ResourceType: "registry",
			ResourceID:   id,
			Status:       dockerAuditStatusFailed,
			Detail:       err.Error(),
		}); auditErr != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
		}
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		UserID:       e.Auth.Id,
		Action:       "registry.delete",
		ResourceType: "registry",
		ResourceID:   id,
		Status:       dockerAuditStatusSuccess,
		Detail:       "delete registry",
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Hub) listDockerComposeTemplates(e *core.RequestEvent) error {
	records, err := h.FindRecordsByFilter("docker_compose_templates", "", "-created", -1, 0)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, map[string]any{
			"id":          record.Id,
			"name":        record.GetString("name"),
			"description": record.GetString("description"),
			"content":     record.GetString("content"),
			"env":         record.GetString("env"),
			"created":     record.Get("created"),
			"updated":     record.Get("updated"),
		})
	}
	return e.JSON(http.StatusOK, map[string]any{"items": items})
}

type dockerComposeTemplatePayload struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	Env         string `json:"env"`
}

type dockerComposeTemplateUpdatePayload struct {
	ID          string  `json:"id"`
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Content     *string `json:"content"`
	Env         *string `json:"env"`
}

func (h *Hub) createDockerComposeTemplate(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerComposeTemplatePayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if strings.TrimSpace(payload.Name) == "" || strings.TrimSpace(payload.Content) == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "name and content are required"})
	}
	if err := validateComposeTemplate(payload.Content); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	collection, err := h.FindCollectionByNameOrId("docker_compose_templates")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	record := core.NewRecord(collection)
	record.Set("name", payload.Name)
	record.Set("description", payload.Description)
	record.Set("content", payload.Content)
	record.Set("env", payload.Env)
	record.Set("created_by", e.Auth.Id)
	if err := h.Save(record); err != nil {
		if auditErr := h.recordDockerAudit(dockerAuditEntry{
			UserID:       e.Auth.Id,
			Action:       "compose_template.create",
			ResourceType: "compose_template",
			ResourceID:   record.Id,
			Status:       dockerAuditStatusFailed,
			Detail:       err.Error(),
		}); auditErr != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
		}
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		UserID:       e.Auth.Id,
		Action:       "compose_template.create",
		ResourceType: "compose_template",
		ResourceID:   record.Id,
		Status:       dockerAuditStatusSuccess,
		Detail:       "create compose template",
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"id": record.Id})
}

func (h *Hub) updateDockerComposeTemplate(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dockerComposeTemplateUpdatePayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if strings.TrimSpace(payload.ID) == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}
	record, err := h.FindRecordById("docker_compose_templates", payload.ID)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "template not found"})
	}
	if payload.Name != nil {
		record.Set("name", strings.TrimSpace(*payload.Name))
	}
	if payload.Description != nil {
		record.Set("description", strings.TrimSpace(*payload.Description))
	}
	if payload.Content != nil {
		if err := validateComposeTemplate(*payload.Content); err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		record.Set("content", *payload.Content)
	}
	if payload.Env != nil {
		record.Set("env", *payload.Env)
	}
	if err := h.Save(record); err != nil {
		if auditErr := h.recordDockerAudit(dockerAuditEntry{
			UserID:       e.Auth.Id,
			Action:       "compose_template.update",
			ResourceType: "compose_template",
			ResourceID:   payload.ID,
			Status:       dockerAuditStatusFailed,
			Detail:       err.Error(),
		}); auditErr != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
		}
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		UserID:       e.Auth.Id,
		Action:       "compose_template.update",
		ResourceType: "compose_template",
		ResourceID:   payload.ID,
		Status:       dockerAuditStatusSuccess,
		Detail:       "update compose template",
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Hub) deleteDockerComposeTemplate(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	id := e.Request.URL.Query().Get("id")
	if strings.TrimSpace(id) == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}
	record, err := h.FindRecordById("docker_compose_templates", id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "template not found"})
	}
	if err := h.Delete(record); err != nil {
		if auditErr := h.recordDockerAudit(dockerAuditEntry{
			UserID:       e.Auth.Id,
			Action:       "compose_template.delete",
			ResourceType: "compose_template",
			ResourceID:   id,
			Status:       dockerAuditStatusFailed,
			Detail:       err.Error(),
		}); auditErr != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
		}
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		UserID:       e.Auth.Id,
		Action:       "compose_template.delete",
		ResourceType: "compose_template",
		ResourceID:   id,
		Status:       dockerAuditStatusSuccess,
		Detail:       "delete compose template",
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"status": "ok"})
}

func (h *Hub) listDockerAudits(e *core.RequestEvent) error {
	query := e.Request.URL.Query()
	systemID := strings.TrimSpace(query.Get("system"))
	startRaw := strings.TrimSpace(query.Get("start"))
	endRaw := strings.TrimSpace(query.Get("end"))
	pageRaw := strings.TrimSpace(query.Get("page"))
	perPageRaw := strings.TrimSpace(query.Get("perPage"))

	filters := make([]string, 0, 3)
	params := map[string]any{}
	if systemID != "" {
		filters = append(filters, "system = {:system}")
		params["system"] = systemID
	}

	var startTime time.Time
	var endTime time.Time
	if startRaw != "" {
		parsed, err := time.Parse(time.RFC3339, startRaw)
		if err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "start must be RFC3339"})
		}
		startTime = parsed
		startDate, err := types.ParseDateTime(startTime)
		if err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid start time"})
		}
		filters = append(filters, "created >= {:start}")
		params["start"] = startDate
	}
	if endRaw != "" {
		parsed, err := time.Parse(time.RFC3339, endRaw)
		if err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "end must be RFC3339"})
		}
		endTime = parsed
		endDate, err := types.ParseDateTime(endTime)
		if err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid end time"})
		}
		filters = append(filters, "created <= {:end}")
		params["end"] = endDate
	}
	if !startTime.IsZero() && !endTime.IsZero() && startTime.After(endTime) {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "start must be before end"})
	}

	limit := -1
	offset := 0
	if pageRaw != "" || perPageRaw != "" {
		if pageRaw == "" || perPageRaw == "" {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "page and perPage are required"})
		}
		page, err := strconv.Atoi(pageRaw)
		if err != nil || page <= 0 {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "page must be a positive integer"})
		}
		perPage, err := strconv.Atoi(perPageRaw)
		if err != nil || perPage <= 0 {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "perPage must be a positive integer"})
		}
		limit = perPage
		offset = (page - 1) * perPage
	}

	filter := strings.Join(filters, " && ")
	records, err := h.FindRecordsByFilter("docker_audits", filter, "-created", limit, offset, params)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, map[string]any{
			"id":            record.Id,
			"system":        record.GetString("system"),
			"user":          record.GetString("user"),
			"action":        record.GetString("action"),
			"resource_type": record.GetString("resource_type"),
			"resource_id":   record.GetString("resource_id"),
			"status":        record.GetString("status"),
			"detail":        record.GetString("detail"),
			"created":       record.Get("created"),
		})
	}
	return e.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Hub) getRegistryAuth(registryID string) (*common.DockerRegistryAuth, error) {
	if strings.TrimSpace(registryID) == "" {
		return nil, nil
	}
	record, err := h.FindRecordById("docker_registries", registryID)
	if err != nil {
		return nil, errors.New("registry not found")
	}
	return &common.DockerRegistryAuth{
		Server:   record.GetString("server"),
		Username: record.GetString("username"),
		Password: record.GetString("password"),
	}, nil
}

func validateComposeTemplate(content string) error {
	var payload struct {
		Services map[string]any `yaml:"services"`
	}
	if err := yaml.Unmarshal([]byte(content), &payload); err != nil {
		return err
	}
	if len(payload.Services) == 0 {
		return errors.New("compose services is required")
	}
	return nil
}

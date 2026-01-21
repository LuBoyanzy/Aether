// handlers.go 定义 Agent 侧的请求处理器与路由逻辑。
// 负责 WebSocket 请求解码、调用 Agent 能力并返回响应。
package agent

import (
	"errors"
	"fmt"
	"time"

	"aether/internal/common"
	"aether/internal/entities/repo"
	"aether/internal/entities/smart"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/fxamacker/cbor/v2"

	"golang.org/x/exp/slog"
)

// HandlerContext provides context for request handlers
type HandlerContext struct {
	Client      *WebSocketClient
	Agent       *Agent
	Request     *common.HubRequest[cbor.RawMessage]
	RequestID   *uint32
	HubVerified bool
	// SendResponse abstracts how a handler sends responses (WS or SSH)
	SendResponse func(data any, requestID *uint32) error
}

// RequestHandler defines the interface for handling specific websocket request types
type RequestHandler interface {
	// Handle processes the request and returns an error if unsuccessful
	Handle(hctx *HandlerContext) error
}

// Responder sends handler responses back to the hub (over WS or SSH)
type Responder interface {
	SendResponse(data any, requestID *uint32) error
}

// HandlerRegistry manages the mapping between actions and their handlers
type HandlerRegistry struct {
	handlers map[common.WebSocketAction]RequestHandler
}

// NewHandlerRegistry creates a new handler registry with default handlers
func NewHandlerRegistry() *HandlerRegistry {
	registry := &HandlerRegistry{
		handlers: make(map[common.WebSocketAction]RequestHandler),
	}

	registry.Register(common.GetData, &GetDataHandler{})
	registry.Register(common.CheckFingerprint, &CheckFingerprintHandler{})
	registry.Register(common.GetContainerLogs, &GetContainerLogsHandler{})
	registry.Register(common.GetContainerInfo, &GetContainerInfoHandler{})
	registry.Register(common.OperateContainer, &OperateContainerHandler{})
	registry.Register(common.GetDockerOverview, &GetDockerOverviewHandler{})
	registry.Register(common.ListDockerContainers, &ListDockerContainersHandler{})
	registry.Register(common.ListDockerImages, &ListDockerImagesHandler{})
	registry.Register(common.PullDockerImage, &PullDockerImageHandler{})
	registry.Register(common.PushDockerImage, &PushDockerImageHandler{})
	registry.Register(common.RemoveDockerImage, &RemoveDockerImageHandler{})
	registry.Register(common.ListDockerNetworks, &ListDockerNetworksHandler{})
	registry.Register(common.CreateDockerNetwork, &CreateDockerNetworkHandler{})
	registry.Register(common.RemoveDockerNetwork, &RemoveDockerNetworkHandler{})
	registry.Register(common.ListDockerVolumes, &ListDockerVolumesHandler{})
	registry.Register(common.CreateDockerVolume, &CreateDockerVolumeHandler{})
	registry.Register(common.RemoveDockerVolume, &RemoveDockerVolumeHandler{})
	registry.Register(common.ListDockerComposeProjects, &ListDockerComposeProjectsHandler{})
	registry.Register(common.CreateDockerComposeProject, &CreateDockerComposeProjectHandler{})
	registry.Register(common.UpdateDockerComposeProject, &UpdateDockerComposeProjectHandler{})
	registry.Register(common.OperateDockerComposeProject, &OperateDockerComposeProjectHandler{})
	registry.Register(common.DeleteDockerComposeProject, &DeleteDockerComposeProjectHandler{})
	registry.Register(common.GetDockerConfig, &GetDockerConfigHandler{})
	registry.Register(common.UpdateDockerConfig, &UpdateDockerConfigHandler{})
	registry.Register(common.GetSmartData, &GetSmartDataHandler{})
	registry.Register(common.GetSystemdInfo, &GetSystemdInfoHandler{})
	registry.Register(common.GetRepoSources, &GetRepoSourcesHandler{})
	registry.Register(common.DataCleanupMySQLDatabases, &DataCleanupMySQLDatabasesHandler{})
	registry.Register(common.DataCleanupMySQLTables, &DataCleanupMySQLTablesHandler{})
	registry.Register(common.DataCleanupMySQLDeleteTables, &DataCleanupMySQLDeleteTablesHandler{})
	registry.Register(common.DataCleanupRedisDatabases, &DataCleanupRedisDatabasesHandler{})
	registry.Register(common.DataCleanupRedisCleanup, &DataCleanupRedisCleanupHandler{})
	registry.Register(common.DataCleanupMinioBuckets, &DataCleanupMinioBucketsHandler{})
	registry.Register(common.DataCleanupMinioPrefixes, &DataCleanupMinioPrefixesHandler{})
	registry.Register(common.DataCleanupMinioCleanup, &DataCleanupMinioCleanupHandler{})
	registry.Register(common.DataCleanupESIndices, &DataCleanupESIndicesHandler{})
	registry.Register(common.DataCleanupESCleanup, &DataCleanupESCleanupHandler{})
	registry.Register(common.DataCleanupJobStatus, &DataCleanupJobStatusHandler{})

	return registry
}

// Register registers a handler for a specific action type
func (hr *HandlerRegistry) Register(action common.WebSocketAction, handler RequestHandler) {
	hr.handlers[action] = handler
}

// Handle routes the request to the appropriate handler
func (hr *HandlerRegistry) Handle(hctx *HandlerContext) error {
	handler, exists := hr.handlers[hctx.Request.Action]
	if !exists {
		return fmt.Errorf("unknown action: %d", hctx.Request.Action)
	}

	// Check verification requirement - default to requiring verification
	if hctx.Request.Action != common.CheckFingerprint && !hctx.HubVerified {
		return errors.New("hub not verified")
	}

	// Log handler execution for debugging
	// slog.Debug("Executing handler", "action", hctx.Request.Action)

	return handler.Handle(hctx)
}

// GetHandler returns the handler for a specific action
func (hr *HandlerRegistry) GetHandler(action common.WebSocketAction) (RequestHandler, bool) {
	handler, exists := hr.handlers[action]
	return handler, exists
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// GetDataHandler handles system data requests
type GetDataHandler struct{}

func (h *GetDataHandler) Handle(hctx *HandlerContext) error {
	var options common.DataRequestOptions
	_ = cbor.Unmarshal(hctx.Request.Data, &options)

	requestID := formatRequestID(hctx.RequestID)
	start := time.Now()
	slog.Debug("GetData start", "requestID", requestID, "cacheTimeMs", options.CacheTimeMs, "includeDetails", options.IncludeDetails)

	sysStats := hctx.Agent.gatherStats(options)
	slog.Info(
		"GetData done",
		"requestID",
		requestID,
		"cacheTimeMs",
		options.CacheTimeMs,
		"durationMs",
		time.Since(start).Milliseconds(),
		"containers",
		len(sysStats.Containers),
		"systemdServices",
		len(sysStats.SystemdServices),
	)
	return hctx.SendResponse(sysStats, hctx.RequestID)
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// CheckFingerprintHandler handles authentication challenges
type CheckFingerprintHandler struct{}

func (h *CheckFingerprintHandler) Handle(hctx *HandlerContext) error {
	return hctx.Client.handleAuthChallenge(hctx.Request, hctx.RequestID)
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// GetContainerLogsHandler handles container log requests
type GetContainerLogsHandler struct{}

func (h *GetContainerLogsHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}

	var req common.ContainerLogsRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}

	logContent, err := sdk.GetContainerLogs(req.ContainerID)
	if err != nil {
		return err
	}

	return hctx.SendResponse(logContent, hctx.RequestID)
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// GetContainerInfoHandler handles container info requests
type GetContainerInfoHandler struct{}

func (h *GetContainerInfoHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}

	var req common.ContainerInfoRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}

	info, err := sdk.GetContainerInfo(req.ContainerID)
	if err != nil {
		return err
	}

	return hctx.SendResponse(string(info), hctx.RequestID)
}

// //////////////////////////////////////////////////////////////////////////
// //////////////////////////////////////////////////////////////////////////
// OperateContainerHandler handles start/stop/restart/kill/pause/unpause
type OperateContainerHandler struct{}

func (h *OperateContainerHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}

	var req common.ContainerOperateRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	if req.ContainerID == "" || req.Operation == "" {
		return errors.New("container id and operation are required")
	}

	operateStart := time.Now()
	slog.Info("Operate container start", "operation", req.Operation, "containerID", req.ContainerID)
	if err := sdk.OperateContainer(req.ContainerID, req.Operation, req.Signal); err != nil {
		slog.Error("Operate container failed", "operation", req.Operation, "containerID", req.ContainerID, "durationMs", time.Since(operateStart).Milliseconds(), "err", err)
		return err
	}

	slog.Info("Operate container done", "operation", req.Operation, "containerID", req.ContainerID, "durationMs", time.Since(operateStart).Milliseconds())
	ack := fmt.Sprintf("%s ok", req.Operation)
	return hctx.SendResponse(ack, hctx.RequestID)
}

// //////////////////////////////////////////////////////////////////////////
// //////////////////////////////////////////////////////////////////////////
// GetDockerOverviewHandler handles Docker overview requests
type GetDockerOverviewHandler struct{}

func (h *GetDockerOverviewHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	overview, err := sdk.GetOverview()
	if err != nil {
		return err
	}
	return hctx.SendResponse(overview, hctx.RequestID)
}

// ListDockerContainersHandler handles Docker container list requests
type ListDockerContainersHandler struct{}

func (h *ListDockerContainersHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	var req common.DockerContainerListRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	containers, err := sdk.ListContainers(req.All)
	if err != nil {
		return err
	}
	return hctx.SendResponse(containers, hctx.RequestID)
}

// ListDockerImagesHandler handles Docker image list requests
type ListDockerImagesHandler struct{}

func (h *ListDockerImagesHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	var req common.DockerImageListRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	images, err := sdk.ListImages(req.All)
	if err != nil {
		return err
	}
	return hctx.SendResponse(images, hctx.RequestID)
}

// PullDockerImageHandler handles Docker image pull requests
type PullDockerImageHandler struct{}

func (h *PullDockerImageHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	var req common.DockerImagePullRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	operationStart := time.Now()
	slog.Info("Pull image start", "image", req.Image)
	logs, err := sdk.PullImage(req.Image, buildAuthConfig(req.Registry))
	if err != nil {
		slog.Error("Pull image failed", "image", req.Image, "durationMs", time.Since(operationStart).Milliseconds(), "err", err)
		return err
	}
	slog.Info("Pull image done", "image", req.Image, "durationMs", time.Since(operationStart).Milliseconds())
	return hctx.SendResponse(logs, hctx.RequestID)
}

// PushDockerImageHandler handles Docker image push requests
type PushDockerImageHandler struct{}

func (h *PushDockerImageHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	var req common.DockerImagePushRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	operationStart := time.Now()
	slog.Info("Push image start", "image", req.Image)
	logs, err := sdk.PushImage(req.Image, buildAuthConfig(req.Registry))
	if err != nil {
		slog.Error("Push image failed", "image", req.Image, "durationMs", time.Since(operationStart).Milliseconds(), "err", err)
		return err
	}
	slog.Info("Push image done", "image", req.Image, "durationMs", time.Since(operationStart).Milliseconds())
	return hctx.SendResponse(logs, hctx.RequestID)
}

// RemoveDockerImageHandler handles Docker image removal requests
type RemoveDockerImageHandler struct{}

func (h *RemoveDockerImageHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	var req common.DockerImageRemoveRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	operationStart := time.Now()
	slog.Info("Remove image start", "imageID", req.ImageID)
	if err := sdk.RemoveImage(req.ImageID, req.Force); err != nil {
		slog.Error("Remove image failed", "imageID", req.ImageID, "durationMs", time.Since(operationStart).Milliseconds(), "err", err)
		return err
	}
	slog.Info("Remove image done", "imageID", req.ImageID, "durationMs", time.Since(operationStart).Milliseconds())
	return hctx.SendResponse("ok", hctx.RequestID)
}

// ListDockerNetworksHandler handles Docker network list requests
type ListDockerNetworksHandler struct{}

func (h *ListDockerNetworksHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	networks, err := sdk.ListNetworks()
	if err != nil {
		return err
	}
	return hctx.SendResponse(networks, hctx.RequestID)
}

// CreateDockerNetworkHandler handles Docker network creation requests
type CreateDockerNetworkHandler struct{}

func (h *CreateDockerNetworkHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	var req common.DockerNetworkCreateRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	options := network.CreateOptions{
		Driver:     req.Driver,
		EnableIPv6: &req.EnableIPv6,
		Internal:   req.Internal,
		Attachable: req.Attachable,
		Labels:     req.Labels,
		Options:    req.Options,
	}
	operationStart := time.Now()
	slog.Info("Create network start", "name", req.Name)
	if err := sdk.CreateNetwork(options, req.Name); err != nil {
		slog.Error("Create network failed", "name", req.Name, "durationMs", time.Since(operationStart).Milliseconds(), "err", err)
		return err
	}
	slog.Info("Create network done", "name", req.Name, "durationMs", time.Since(operationStart).Milliseconds())
	return hctx.SendResponse("ok", hctx.RequestID)
}

// RemoveDockerNetworkHandler handles Docker network removal requests
type RemoveDockerNetworkHandler struct{}

func (h *RemoveDockerNetworkHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	var req common.DockerNetworkRemoveRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	operationStart := time.Now()
	slog.Info("Remove network start", "networkID", req.NetworkID)
	if err := sdk.RemoveNetwork(req.NetworkID); err != nil {
		slog.Error("Remove network failed", "networkID", req.NetworkID, "durationMs", time.Since(operationStart).Milliseconds(), "err", err)
		return err
	}
	slog.Info("Remove network done", "networkID", req.NetworkID, "durationMs", time.Since(operationStart).Milliseconds())
	return hctx.SendResponse("ok", hctx.RequestID)
}

// ListDockerVolumesHandler handles Docker volume list requests
type ListDockerVolumesHandler struct{}

func (h *ListDockerVolumesHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	volumes, err := sdk.ListVolumes()
	if err != nil {
		return err
	}
	return hctx.SendResponse(volumes, hctx.RequestID)
}

// CreateDockerVolumeHandler handles Docker volume creation requests
type CreateDockerVolumeHandler struct{}

func (h *CreateDockerVolumeHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	var req common.DockerVolumeCreateRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	options := volume.CreateOptions{
		Name:       req.Name,
		Driver:     req.Driver,
		Labels:     req.Labels,
		DriverOpts: req.Options,
	}
	operationStart := time.Now()
	slog.Info("Create volume start", "name", req.Name)
	_, err = sdk.CreateVolume(options)
	if err != nil {
		slog.Error("Create volume failed", "name", req.Name, "durationMs", time.Since(operationStart).Milliseconds(), "err", err)
		return err
	}
	slog.Info("Create volume done", "name", req.Name, "durationMs", time.Since(operationStart).Milliseconds())
	return hctx.SendResponse("ok", hctx.RequestID)
}

// RemoveDockerVolumeHandler handles Docker volume removal requests
type RemoveDockerVolumeHandler struct{}

func (h *RemoveDockerVolumeHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	var req common.DockerVolumeRemoveRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	operationStart := time.Now()
	slog.Info("Remove volume start", "name", req.Name)
	if err := sdk.RemoveVolume(req.Name, req.Force); err != nil {
		slog.Error("Remove volume failed", "name", req.Name, "durationMs", time.Since(operationStart).Milliseconds(), "err", err)
		return err
	}
	slog.Info("Remove volume done", "name", req.Name, "durationMs", time.Since(operationStart).Milliseconds())
	return hctx.SendResponse("ok", hctx.RequestID)
}

// ListDockerComposeProjectsHandler handles compose project listing
type ListDockerComposeProjectsHandler struct{}

func (h *ListDockerComposeProjectsHandler) Handle(hctx *HandlerContext) error {
	projects, err := hctx.Agent.ListComposeProjects()
	if err != nil {
		return err
	}
	return hctx.SendResponse(projects, hctx.RequestID)
}

// CreateDockerComposeProjectHandler handles compose project creation
type CreateDockerComposeProjectHandler struct{}

func (h *CreateDockerComposeProjectHandler) Handle(hctx *HandlerContext) error {
	var req common.DockerComposeProjectCreateRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	operationStart := time.Now()
	slog.Info("Create compose start", "name", req.Name)
	output, err := hctx.Agent.CreateComposeProject(req)
	if err != nil {
		slog.Error("Create compose failed", "name", req.Name, "durationMs", time.Since(operationStart).Milliseconds(), "err", err)
		return err
	}
	slog.Info("Create compose done", "name", req.Name, "durationMs", time.Since(operationStart).Milliseconds())
	return hctx.SendResponse(output, hctx.RequestID)
}

// UpdateDockerComposeProjectHandler handles compose project update
type UpdateDockerComposeProjectHandler struct{}

func (h *UpdateDockerComposeProjectHandler) Handle(hctx *HandlerContext) error {
	var req common.DockerComposeProjectUpdateRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	operationStart := time.Now()
	slog.Info("Update compose start", "name", req.Name)
	output, err := hctx.Agent.UpdateComposeProject(req)
	if err != nil {
		slog.Error("Update compose failed", "name", req.Name, "durationMs", time.Since(operationStart).Milliseconds(), "err", err)
		return err
	}
	slog.Info("Update compose done", "name", req.Name, "durationMs", time.Since(operationStart).Milliseconds())
	return hctx.SendResponse(output, hctx.RequestID)
}

// OperateDockerComposeProjectHandler handles compose operations
type OperateDockerComposeProjectHandler struct{}

func (h *OperateDockerComposeProjectHandler) Handle(hctx *HandlerContext) error {
	var req common.DockerComposeProjectOperateRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	operationStart := time.Now()
	slog.Info("Operate compose start", "name", req.Name, "operation", req.Operation)
	output, err := hctx.Agent.OperateComposeProject(req)
	if err != nil {
		slog.Error("Operate compose failed", "name", req.Name, "operation", req.Operation, "durationMs", time.Since(operationStart).Milliseconds(), "err", err)
		return err
	}
	slog.Info("Operate compose done", "name", req.Name, "operation", req.Operation, "durationMs", time.Since(operationStart).Milliseconds())
	return hctx.SendResponse(output, hctx.RequestID)
}

// DeleteDockerComposeProjectHandler handles compose deletion
type DeleteDockerComposeProjectHandler struct{}

func (h *DeleteDockerComposeProjectHandler) Handle(hctx *HandlerContext) error {
	var req common.DockerComposeProjectDeleteRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	operationStart := time.Now()
	slog.Info("Delete compose start", "name", req.Name)
	output, err := hctx.Agent.DeleteComposeProject(req)
	if err != nil {
		slog.Error("Delete compose failed", "name", req.Name, "durationMs", time.Since(operationStart).Milliseconds(), "err", err)
		return err
	}
	slog.Info("Delete compose done", "name", req.Name, "durationMs", time.Since(operationStart).Milliseconds())
	return hctx.SendResponse(output, hctx.RequestID)
}

// GetDockerConfigHandler handles daemon.json read requests
type GetDockerConfigHandler struct{}

func (h *GetDockerConfigHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	var req common.DockerConfigRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	config, err := sdk.ReadDaemonConfig("")
	if err != nil {
		return err
	}
	return hctx.SendResponse(config, hctx.RequestID)
}

// UpdateDockerConfigHandler handles daemon.json update requests
type UpdateDockerConfigHandler struct{}

func (h *UpdateDockerConfigHandler) Handle(hctx *HandlerContext) error {
	sdk, err := hctx.Agent.getDockerSDK()
	if err != nil {
		return err
	}
	var req common.DockerConfigUpdateRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	operationStart := time.Now()
	slog.Info("Update docker config start", "restart", req.Restart)
	if err := sdk.UpdateDaemonConfig(req.Content, req.Path, req.Restart); err != nil {
		slog.Error("Update docker config failed", "restart", req.Restart, "durationMs", time.Since(operationStart).Milliseconds(), "err", err)
		return err
	}
	slog.Info("Update docker config done", "restart", req.Restart, "durationMs", time.Since(operationStart).Milliseconds())
	return hctx.SendResponse("ok", hctx.RequestID)
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// GetSmartDataHandler handles SMART data requests
type GetSmartDataHandler struct{}

func (h *GetSmartDataHandler) Handle(hctx *HandlerContext) error {
	if hctx.Agent.smartManager == nil {
		// return empty map to indicate no data
		return hctx.SendResponse(map[string]smart.SmartData{}, hctx.RequestID)
	}
	if err := hctx.Agent.smartManager.Refresh(false); err != nil {
		slog.Debug("smart refresh failed", "err", err)
	}
	data := hctx.Agent.smartManager.GetCurrentData()
	return hctx.SendResponse(data, hctx.RequestID)
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// GetSystemdInfoHandler handles detailed systemd service info requests
type GetSystemdInfoHandler struct{}

func (h *GetSystemdInfoHandler) Handle(hctx *HandlerContext) error {
	if hctx.Agent.systemdManager == nil {
		return errors.ErrUnsupported
	}

	var req common.SystemdInfoRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return err
	}
	if req.ServiceName == "" {
		return errors.New("service name is required")
	}

	details, err := hctx.Agent.systemdManager.getServiceDetails(req.ServiceName)
	if err != nil {
		return err
	}

	return hctx.SendResponse(details, hctx.RequestID)
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// GetRepoSourcesHandler handles repository source requests
type GetRepoSourcesHandler struct{}

func (h *GetRepoSourcesHandler) Handle(hctx *HandlerContext) error {
	var req common.RepoSourcesRequest
	if len(hctx.Request.Data) > 0 {
		if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
			return err
		}
	}
	sources, err := hctx.Agent.collectRepoSources(repoSourcesOptions{Check: req.Check})
	if err != nil {
		return err
	}
	if sources == nil {
		sources = []repo.Source{}
	}
	return hctx.SendResponse(sources, hctx.RequestID)
}

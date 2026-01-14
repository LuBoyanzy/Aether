package ws

import (
	"context"
	"errors"
	"time"

	"aether/internal/common"
	"aether/internal/entities/docker"
	"aether/internal/entities/repo"
	"aether/internal/entities/smart"
	"aether/internal/entities/system"
	"aether/internal/entities/systemd"
	"github.com/fxamacker/cbor/v2"
	"github.com/lxzan/gws"
	"golang.org/x/crypto/ssh"
)

// ResponseHandler defines interface for handling agent responses
type ResponseHandler interface {
	Handle(agentResponse common.AgentResponse) error
	HandleLegacy(rawData []byte) error
}

// BaseHandler provides a default implementation that can be embedded to make HandleLegacy optional
type BaseHandler struct{}

func (h *BaseHandler) HandleLegacy(rawData []byte) error {
	return errors.New("legacy format not supported")
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// systemDataHandler implements ResponseHandler for system data requests
type systemDataHandler struct {
	data *system.CombinedData
}

func (h *systemDataHandler) HandleLegacy(rawData []byte) error {
	return cbor.Unmarshal(rawData, h.data)
}

func (h *systemDataHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.SystemData != nil {
		*h.data = *agentResponse.SystemData
	}
	return nil
}

// RequestSystemData requests system metrics from the agent and unmarshals the response.
func (ws *WsConn) RequestSystemData(ctx context.Context, data *system.CombinedData, options common.DataRequestOptions) error {
	if !ws.IsConnected() {
		return gws.ErrConnClosed
	}

	req, err := ws.requestManager.SendRequest(ctx, common.GetData, options)
	if err != nil {
		return err
	}

	handler := &systemDataHandler{data: data}
	return ws.handleAgentRequest(req, handler)
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// stringResponseHandler is a generic handler for string responses from agents
type stringResponseHandler struct {
	BaseHandler
	value    string
	errorMsg string
}

func (h *stringResponseHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.String == nil {
		return errors.New(h.errorMsg)
	}
	h.value = *agentResponse.String
	return nil
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// requestContainerStringViaWS is a generic function to request container-related strings via WebSocket
func (ws *WsConn) requestContainerStringViaWS(ctx context.Context, action common.WebSocketAction, requestData any, errorMsg string) (string, error) {
	if !ws.IsConnected() {
		return "", gws.ErrConnClosed
	}

	req, err := ws.requestManager.SendRequest(ctx, action, requestData)
	if err != nil {
		return "", err
	}

	handler := &stringResponseHandler{errorMsg: errorMsg}
	if err := ws.handleAgentRequest(req, handler); err != nil {
		return "", err
	}

	return handler.value, nil
}

// RequestContainerLogs requests logs for a specific container via WebSocket.
func (ws *WsConn) RequestContainerLogs(ctx context.Context, containerID string) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.GetContainerLogs, common.ContainerLogsRequest{ContainerID: containerID}, "no logs in response")
}

// RequestContainerInfo requests information about a specific container via WebSocket.
func (ws *WsConn) RequestContainerInfo(ctx context.Context, containerID string) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.GetContainerInfo, common.ContainerInfoRequest{ContainerID: containerID}, "no info in response")
}

// RequestContainerOperate executes a container operation (start/stop/restart/kill/pause/unpause) via WebSocket.
func (ws *WsConn) RequestContainerOperate(ctx context.Context, req common.ContainerOperateRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.OperateContainer, req, "operation failed")
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// RequestDockerOverview requests Docker overview information via WebSocket.
func (ws *WsConn) RequestDockerOverview(ctx context.Context) (docker.Overview, error) {
	if !ws.IsConnected() {
		return docker.Overview{}, gws.ErrConnClosed
	}
	req, err := ws.requestManager.SendRequest(ctx, common.GetDockerOverview, common.DockerOverviewRequest{})
	if err != nil {
		return docker.Overview{}, err
	}
	var result docker.Overview
	handler := &dockerOverviewHandler{result: &result}
	if err := ws.handleAgentRequest(req, handler); err != nil {
		return docker.Overview{}, err
	}
	return result, nil
}

type dockerOverviewHandler struct {
	BaseHandler
	result *docker.Overview
}

func (h *dockerOverviewHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.DockerInfo == nil {
		return errors.New("no docker overview in response")
	}
	*h.result = *agentResponse.DockerInfo
	return nil
}

// RequestDockerContainers requests Docker container list via WebSocket.
func (ws *WsConn) RequestDockerContainers(ctx context.Context, req common.DockerContainerListRequest) ([]docker.Container, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequest(ctx, common.ListDockerContainers, req)
	if err != nil {
		return nil, err
	}
	var result []docker.Container
	handler := &dockerContainersHandler{result: &result}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return nil, err
	}
	return result, nil
}

type dockerContainersHandler struct {
	BaseHandler
	result *[]docker.Container
}

func (h *dockerContainersHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.DockerContainers == nil {
		return errors.New("no docker containers in response")
	}
	*h.result = agentResponse.DockerContainers
	return nil
}

// RequestDockerImages requests Docker image list via WebSocket.
func (ws *WsConn) RequestDockerImages(ctx context.Context, req common.DockerImageListRequest) ([]docker.Image, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequest(ctx, common.ListDockerImages, req)
	if err != nil {
		return nil, err
	}
	var result []docker.Image
	handler := &dockerImagesHandler{result: &result}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return nil, err
	}
	return result, nil
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// RequestRepoSources requests package repository sources via WebSocket.
func (ws *WsConn) RequestRepoSources(ctx context.Context, req common.RepoSourcesRequest) ([]repo.Source, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequest(ctx, common.GetRepoSources, req)
	if err != nil {
		return nil, err
	}
	var result []repo.Source
	handler := &repoSourcesHandler{result: &result}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return nil, err
	}
	return result, nil
}

type repoSourcesHandler struct {
	BaseHandler
	result *[]repo.Source
}

func (h *repoSourcesHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.RepoSources == nil {
		return errors.New("no repo sources in response")
	}
	*h.result = agentResponse.RepoSources
	return nil
}

type dataCleanupListHandler struct {
	BaseHandler
	result   *common.DockerDataCleanupList
	errorMsg string
}

func (h *dataCleanupListHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.Error != "" {
		return errors.New(agentResponse.Error)
	}
	if agentResponse.DataCleanupList == nil {
		return errors.New(h.errorMsg)
	}
	*h.result = *agentResponse.DataCleanupList
	return nil
}

type dataCleanupResultHandler struct {
	BaseHandler
	result   *common.DockerDataCleanupResult
	errorMsg string
}

func (h *dataCleanupResultHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.Error != "" {
		return errors.New(agentResponse.Error)
	}
	if agentResponse.DataCleanupResult == nil {
		return errors.New(h.errorMsg)
	}
	*h.result = *agentResponse.DataCleanupResult
	return nil
}

const (
	dataCleanupListTimeout   = 20 * time.Second
	dataCleanupActionTimeout = 30 * time.Minute
)

func (ws *WsConn) RequestDataCleanupMySQLDatabases(
	ctx context.Context,
	req common.DataCleanupMySQLDatabasesRequest,
) ([]string, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequestWithTimeout(ctx, common.DataCleanupMySQLDatabases, req, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	var result common.DockerDataCleanupList
	handler := &dataCleanupListHandler{result: &result, errorMsg: "no mysql databases in response"}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return nil, err
	}
	return result.Databases, nil
}

func (ws *WsConn) RequestDataCleanupMySQLTables(
	ctx context.Context,
	req common.DataCleanupMySQLTablesRequest,
) ([]string, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequestWithTimeout(ctx, common.DataCleanupMySQLTables, req, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	var result common.DockerDataCleanupList
	handler := &dataCleanupListHandler{result: &result, errorMsg: "no mysql tables in response"}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return nil, err
	}
	return result.Tables, nil
}

func (ws *WsConn) RequestDataCleanupMySQLDeleteTables(
	ctx context.Context,
	req common.DataCleanupMySQLDeleteTablesRequest,
) (common.DockerDataCleanupResult, error) {
	if !ws.IsConnected() {
		return common.DockerDataCleanupResult{}, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequestWithTimeout(ctx, common.DataCleanupMySQLDeleteTables, req, dataCleanupActionTimeout)
	if err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	var result common.DockerDataCleanupResult
	handler := &dataCleanupResultHandler{result: &result, errorMsg: "no mysql cleanup result in response"}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	return result, nil
}

func (ws *WsConn) RequestDataCleanupRedisDatabases(
	ctx context.Context,
	req common.DataCleanupRedisDatabasesRequest,
) ([]int, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequestWithTimeout(ctx, common.DataCleanupRedisDatabases, req, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	var result common.DockerDataCleanupList
	handler := &dataCleanupListHandler{result: &result, errorMsg: "no redis databases in response"}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return nil, err
	}
	return result.RedisDBs, nil
}

func (ws *WsConn) RequestDataCleanupRedisCleanup(
	ctx context.Context,
	req common.DataCleanupRedisCleanupRequest,
) (common.DockerDataCleanupResult, error) {
	if !ws.IsConnected() {
		return common.DockerDataCleanupResult{}, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequestWithTimeout(ctx, common.DataCleanupRedisCleanup, req, dataCleanupActionTimeout)
	if err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	var result common.DockerDataCleanupResult
	handler := &dataCleanupResultHandler{result: &result, errorMsg: "no redis cleanup result in response"}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	return result, nil
}

func (ws *WsConn) RequestDataCleanupMinioBuckets(
	ctx context.Context,
	req common.DataCleanupMinioBucketsRequest,
) ([]string, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequestWithTimeout(ctx, common.DataCleanupMinioBuckets, req, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	var result common.DockerDataCleanupList
	handler := &dataCleanupListHandler{result: &result, errorMsg: "no minio buckets in response"}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return nil, err
	}
	return result.Buckets, nil
}

func (ws *WsConn) RequestDataCleanupMinioPrefixes(
	ctx context.Context,
	req common.DataCleanupMinioPrefixesRequest,
) ([]string, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequestWithTimeout(ctx, common.DataCleanupMinioPrefixes, req, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	var result common.DockerDataCleanupList
	handler := &dataCleanupListHandler{result: &result, errorMsg: "no minio prefixes in response"}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return nil, err
	}
	return result.Prefixes, nil
}

func (ws *WsConn) RequestDataCleanupMinioCleanup(
	ctx context.Context,
	req common.DataCleanupMinioCleanupRequest,
) (common.DockerDataCleanupResult, error) {
	if !ws.IsConnected() {
		return common.DockerDataCleanupResult{}, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequestWithTimeout(ctx, common.DataCleanupMinioCleanup, req, dataCleanupActionTimeout)
	if err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	var result common.DockerDataCleanupResult
	handler := &dataCleanupResultHandler{result: &result, errorMsg: "no minio cleanup result in response"}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	return result, nil
}

func (ws *WsConn) RequestDataCleanupESIndices(
	ctx context.Context,
	req common.DataCleanupESIndicesRequest,
) ([]string, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequestWithTimeout(ctx, common.DataCleanupESIndices, req, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	var result common.DockerDataCleanupList
	handler := &dataCleanupListHandler{result: &result, errorMsg: "no es indices in response"}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return nil, err
	}
	return result.Indices, nil
}

func (ws *WsConn) RequestDataCleanupESCleanup(
	ctx context.Context,
	req common.DataCleanupESCleanupRequest,
) (common.DockerDataCleanupResult, error) {
	if !ws.IsConnected() {
		return common.DockerDataCleanupResult{}, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequestWithTimeout(ctx, common.DataCleanupESCleanup, req, dataCleanupActionTimeout)
	if err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	var result common.DockerDataCleanupResult
	handler := &dataCleanupResultHandler{result: &result, errorMsg: "no es cleanup result in response"}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	return result, nil
}

type dockerImagesHandler struct {
	BaseHandler
	result *[]docker.Image
}

func (h *dockerImagesHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.DockerImages == nil {
		return errors.New("no docker images in response")
	}
	*h.result = agentResponse.DockerImages
	return nil
}

// RequestDockerNetworks requests Docker network list via WebSocket.
func (ws *WsConn) RequestDockerNetworks(ctx context.Context) ([]docker.Network, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequest(ctx, common.ListDockerNetworks, nil)
	if err != nil {
		return nil, err
	}
	var result []docker.Network
	handler := &dockerNetworksHandler{result: &result}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return nil, err
	}
	return result, nil
}

type dockerNetworksHandler struct {
	BaseHandler
	result *[]docker.Network
}

func (h *dockerNetworksHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.DockerNetworks == nil {
		return errors.New("no docker networks in response")
	}
	*h.result = agentResponse.DockerNetworks
	return nil
}

// RequestDockerVolumes requests Docker volume list via WebSocket.
func (ws *WsConn) RequestDockerVolumes(ctx context.Context) ([]docker.Volume, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequest(ctx, common.ListDockerVolumes, nil)
	if err != nil {
		return nil, err
	}
	var result []docker.Volume
	handler := &dockerVolumesHandler{result: &result}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return nil, err
	}
	return result, nil
}

type dockerVolumesHandler struct {
	BaseHandler
	result *[]docker.Volume
}

func (h *dockerVolumesHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.DockerVolumes == nil {
		return errors.New("no docker volumes in response")
	}
	*h.result = agentResponse.DockerVolumes
	return nil
}

// RequestDockerComposeProjects requests Docker compose projects via WebSocket.
func (ws *WsConn) RequestDockerComposeProjects(ctx context.Context) ([]docker.ComposeProject, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequest(ctx, common.ListDockerComposeProjects, common.DockerComposeProjectListRequest{})
	if err != nil {
		return nil, err
	}
	var result []docker.ComposeProject
	handler := &dockerComposeProjectsHandler{result: &result}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return nil, err
	}
	return result, nil
}

type dockerComposeProjectsHandler struct {
	BaseHandler
	result *[]docker.ComposeProject
}

func (h *dockerComposeProjectsHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.DockerComposeProjects == nil {
		return errors.New("no docker compose projects in response")
	}
	*h.result = agentResponse.DockerComposeProjects
	return nil
}

// RequestDockerConfig requests Docker daemon config via WebSocket.
func (ws *WsConn) RequestDockerConfig(ctx context.Context) (docker.DaemonConfig, error) {
	if !ws.IsConnected() {
		return docker.DaemonConfig{}, gws.ErrConnClosed
	}
	handleReq, err := ws.requestManager.SendRequest(ctx, common.GetDockerConfig, common.DockerConfigRequest{})
	if err != nil {
		return docker.DaemonConfig{}, err
	}
	var result docker.DaemonConfig
	handler := &dockerConfigHandler{result: &result}
	if err := ws.handleAgentRequest(handleReq, handler); err != nil {
		return docker.DaemonConfig{}, err
	}
	return result, nil
}

type dockerConfigHandler struct {
	BaseHandler
	result *docker.DaemonConfig
}

func (h *dockerConfigHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.DockerConfig == nil {
		return errors.New("no docker config in response")
	}
	*h.result = *agentResponse.DockerConfig
	return nil
}

// RequestDockerImagePull triggers docker image pull via WebSocket.
func (ws *WsConn) RequestDockerImagePull(ctx context.Context, req common.DockerImagePullRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.PullDockerImage, req, "docker image pull failed")
}

// RequestDockerImagePush triggers docker image push via WebSocket.
func (ws *WsConn) RequestDockerImagePush(ctx context.Context, req common.DockerImagePushRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.PushDockerImage, req, "docker image push failed")
}

// RequestDockerImageRemove removes docker image via WebSocket.
func (ws *WsConn) RequestDockerImageRemove(ctx context.Context, req common.DockerImageRemoveRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.RemoveDockerImage, req, "docker image remove failed")
}

// RequestDockerNetworkCreate creates docker network via WebSocket.
func (ws *WsConn) RequestDockerNetworkCreate(ctx context.Context, req common.DockerNetworkCreateRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.CreateDockerNetwork, req, "docker network create failed")
}

// RequestDockerNetworkRemove removes docker network via WebSocket.
func (ws *WsConn) RequestDockerNetworkRemove(ctx context.Context, req common.DockerNetworkRemoveRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.RemoveDockerNetwork, req, "docker network remove failed")
}

// RequestDockerVolumeCreate creates docker volume via WebSocket.
func (ws *WsConn) RequestDockerVolumeCreate(ctx context.Context, req common.DockerVolumeCreateRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.CreateDockerVolume, req, "docker volume create failed")
}

// RequestDockerVolumeRemove removes docker volume via WebSocket.
func (ws *WsConn) RequestDockerVolumeRemove(ctx context.Context, req common.DockerVolumeRemoveRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.RemoveDockerVolume, req, "docker volume remove failed")
}

// RequestDockerComposeCreate creates compose project via WebSocket.
func (ws *WsConn) RequestDockerComposeCreate(ctx context.Context, req common.DockerComposeProjectCreateRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.CreateDockerComposeProject, req, "docker compose create failed")
}

// RequestDockerComposeUpdate updates compose project via WebSocket.
func (ws *WsConn) RequestDockerComposeUpdate(ctx context.Context, req common.DockerComposeProjectUpdateRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.UpdateDockerComposeProject, req, "docker compose update failed")
}

// RequestDockerComposeOperate operates compose project via WebSocket.
func (ws *WsConn) RequestDockerComposeOperate(ctx context.Context, req common.DockerComposeProjectOperateRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.OperateDockerComposeProject, req, "docker compose operation failed")
}

// RequestDockerComposeDelete deletes compose project via WebSocket.
func (ws *WsConn) RequestDockerComposeDelete(ctx context.Context, req common.DockerComposeProjectDeleteRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.DeleteDockerComposeProject, req, "docker compose delete failed")
}

// RequestDockerConfigUpdate updates docker daemon config via WebSocket.
func (ws *WsConn) RequestDockerConfigUpdate(ctx context.Context, req common.DockerConfigUpdateRequest) (string, error) {
	return ws.requestContainerStringViaWS(ctx, common.UpdateDockerConfig, req, "docker config update failed")
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// RequestSystemdInfo requests detailed information about a systemd service via WebSocket.
func (ws *WsConn) RequestSystemdInfo(ctx context.Context, serviceName string) (systemd.ServiceDetails, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}

	req, err := ws.requestManager.SendRequest(ctx, common.GetSystemdInfo, common.SystemdInfoRequest{ServiceName: serviceName})
	if err != nil {
		return nil, err
	}

	var result systemd.ServiceDetails
	handler := &systemdInfoHandler{result: &result}
	if err := ws.handleAgentRequest(req, handler); err != nil {
		return nil, err
	}

	return result, nil
}

// systemdInfoHandler parses ServiceDetails from AgentResponse
type systemdInfoHandler struct {
	BaseHandler
	result *systemd.ServiceDetails
}

func (h *systemdInfoHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.ServiceInfo == nil {
		return errors.New("no systemd info in response")
	}
	*h.result = *agentResponse.ServiceInfo
	return nil
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// RequestSmartData requests SMART data via WebSocket.
func (ws *WsConn) RequestSmartData(ctx context.Context) (map[string]smart.SmartData, error) {
	if !ws.IsConnected() {
		return nil, gws.ErrConnClosed
	}
	req, err := ws.requestManager.SendRequest(ctx, common.GetSmartData, nil)
	if err != nil {
		return nil, err
	}
	var result map[string]smart.SmartData
	handler := ResponseHandler(&smartDataHandler{result: &result})
	if err := ws.handleAgentRequest(req, handler); err != nil {
		return nil, err
	}
	return result, nil
}

// smartDataHandler parses SMART data map from AgentResponse
type smartDataHandler struct {
	BaseHandler
	result *map[string]smart.SmartData
}

func (h *smartDataHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.SmartData == nil {
		return errors.New("no SMART data in response")
	}
	*h.result = agentResponse.SmartData
	return nil
}

////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////
////////////////////////////////////////////////////////////////////////////

// fingerprintHandler implements ResponseHandler for fingerprint requests
type fingerprintHandler struct {
	result *common.FingerprintResponse
}

func (h *fingerprintHandler) HandleLegacy(rawData []byte) error {
	return cbor.Unmarshal(rawData, h.result)
}

func (h *fingerprintHandler) Handle(agentResponse common.AgentResponse) error {
	if agentResponse.Fingerprint != nil {
		*h.result = *agentResponse.Fingerprint
		return nil
	}
	return errors.New("no fingerprint data in response")
}

// GetFingerprint authenticates with the agent using SSH signature and returns the agent's fingerprint.
func (ws *WsConn) GetFingerprint(ctx context.Context, token string, signer ssh.Signer, needSysInfo bool) (common.FingerprintResponse, error) {
	if !ws.IsConnected() {
		return common.FingerprintResponse{}, gws.ErrConnClosed
	}

	challenge := []byte(token)
	signature, err := signer.Sign(nil, challenge)
	if err != nil {
		return common.FingerprintResponse{}, err
	}

	req, err := ws.requestManager.SendRequest(ctx, common.CheckFingerprint, common.FingerprintRequest{
		Signature:   signature.Blob,
		NeedSysInfo: needSysInfo,
	})
	if err != nil {
		return common.FingerprintResponse{}, err
	}

	var result common.FingerprintResponse
	handler := &fingerprintHandler{result: &result}
	err = ws.handleAgentRequest(req, handler)
	return result, err
}

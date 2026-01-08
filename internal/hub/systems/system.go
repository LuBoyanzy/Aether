package systems

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math/rand"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"aether/internal/common"
	"aether/internal/hub/ws"

	"aether/internal/entities/container"
	"aether/internal/entities/docker"
	"aether/internal/entities/system"
	"aether/internal/entities/systemd"

	"aether"

	"github.com/blang/semver"
	"github.com/fxamacker/cbor/v2"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"golang.org/x/crypto/ssh"
)

type System struct {
	Id      string               `db:"id"`
	Host    string               `db:"host"`
	Port    string               `db:"port"`
	Status  string               `db:"status"`
	manager *SystemManager       // Manager that this system belongs to
	client  *ssh.Client          // SSH client for fetching data
	data    *system.CombinedData // system data from agent
	ctx     context.Context      // Context for stopping the updater
	cancel  context.CancelFunc   // Stops and removes system from updater
	// test overrides (set only in tests)
	operateOverride   func(containerID, op, signal string) error
	updateNowOverride func() error
	WsConn            *ws.WsConn     // Handler for agent WebSocket connection
	agentVersion      semver.Version // Agent version
	updateTicker      *time.Ticker   // Ticker for updating the system
	detailsFetched    atomic.Bool    // True if static system details have been fetched and saved
	smartFetching     atomic.Bool    // True if SMART devices are currently being fetched
	smartInterval     time.Duration  // Interval for periodic SMART data updates
	lastSmartFetch    atomic.Int64   // Unix milliseconds of last SMART data fetch
}

func (sm *SystemManager) NewSystem(systemId string) *System {
	system := &System{
		Id:   systemId,
		data: &system.CombinedData{},
	}
	system.ctx, system.cancel = system.getContext()
	return system
}

// StartUpdater starts the system updater.
// It first fetches the data from the agent then updates the records.
// If the data is not found or the system is down, it sets the system down.
func (sys *System) StartUpdater() {
	// Channel that can be used to set the system down. Currently only used to
	// allow a short delay for reconnection after websocket connection is closed.
	var downChan chan struct{}

	// Add random jitter to first WebSocket connection to prevent
	// clustering if all agents are started at the same time.
	// SSH connections during hub startup are already staggered.
	var jitter <-chan time.Time
	if sys.WsConn != nil {
		jitter = getJitter()
		// use the websocket connection's down channel to set the system down
		downChan = sys.WsConn.DownChan
	} else {
		// if the system does not have a websocket connection, wait before updating
		// to allow the agent to connect via websocket (makes sure fingerprint is set).
		time.Sleep(11 * time.Second)
	}

	// update immediately if system is not paused (only for ws connections)
	// we'll wait a minute before connecting via SSH to prioritize ws connections
	if sys.Status != paused && sys.ctx.Err() == nil {
		if err := sys.update(); err != nil {
			_ = sys.setDown(err)
		}
	}

	sys.updateTicker = time.NewTicker(time.Duration(interval) * time.Millisecond)
	// Go 1.23+ will automatically stop the ticker when the system is garbage collected, however we seem to need this or testing/synctest will block even if calling runtime.GC()
	defer sys.updateTicker.Stop()

	for {
		select {
		case <-sys.ctx.Done():
			return
		case <-sys.updateTicker.C:
			if err := sys.update(); err != nil {
				_ = sys.setDown(err)
			}
		case <-downChan:
			sys.WsConn = nil
			downChan = nil
			_ = sys.setDown(nil)
		case <-jitter:
			sys.updateTicker.Reset(time.Duration(interval) * time.Millisecond)
			if err := sys.update(); err != nil {
				_ = sys.setDown(err)
			}
		}
	}
}

// update updates the system data and records.
func (sys *System) update() error {
	if sys.Status == paused {
		sys.handlePaused()
		return nil
	}
	options := common.DataRequestOptions{
		CacheTimeMs: uint16(interval),
	}
	// fetch system details if not already fetched
	if !sys.detailsFetched.Load() {
		options.IncludeDetails = true
	}

	data, err := sys.fetchDataFromAgent(options)
	if err != nil {
		return err
	}

	// create system records
	_, err = sys.createRecords(data)

	// Fetch and save SMART devices when system first comes online or at intervals
	if backgroundSmartFetchEnabled() {
		if sys.smartInterval <= 0 {
			sys.smartInterval = time.Hour
		}
		lastFetch := sys.lastSmartFetch.Load()
		if time.Since(time.UnixMilli(lastFetch)) >= sys.smartInterval && sys.smartFetching.CompareAndSwap(false, true) {
			go func() {
				defer sys.smartFetching.Store(false)
				sys.lastSmartFetch.Store(time.Now().UnixMilli())
				_ = sys.FetchAndSaveSmartDevices()
			}()
		}
	}

	return err
}

func (sys *System) handlePaused() {
	if sys.WsConn == nil {
		// if the system is paused and there's no websocket connection, remove the system
		_ = sys.manager.RemoveSystem(sys.Id)
	} else {
		// Send a ping to the agent to keep the connection alive if the system is paused
		if err := sys.WsConn.Ping(); err != nil {
			sys.manager.hub.Logger().Warn("Failed to ping agent", "logger", "systems", "system", sys.Id, "err", err)
			_ = sys.manager.RemoveSystem(sys.Id)
		}
	}
}

// createRecords updates the system record and adds system_stats and container_stats records
func (sys *System) createRecords(data *system.CombinedData) (*core.Record, error) {
	systemRecord, err := sys.getRecord()
	if err != nil {
		return nil, err
	}
	hub := sys.manager.hub
	err = hub.RunInTransaction(func(txApp core.App) error {
		// add system_stats record
		systemStatsCollection, err := txApp.FindCachedCollectionByNameOrId("system_stats")
		if err != nil {
			return err
		}
		systemStatsRecord := core.NewRecord(systemStatsCollection)
		systemStatsRecord.Set("system", systemRecord.Id)
		systemStatsRecord.Set("stats", data.Stats)
		systemStatsRecord.Set("type", "1m")
		if err := txApp.SaveNoValidate(systemStatsRecord); err != nil {
			return err
		}

		// add containers and container_stats records
		if len(data.Containers) > 0 {
			if data.Containers[0].Id != "" {
				if err := createContainerRecords(txApp, data.Containers, sys.Id); err != nil {
					return err
				}
			}
			containerStatsCollection, err := txApp.FindCachedCollectionByNameOrId("container_stats")
			if err != nil {
				return err
			}
			containerStatsRecord := core.NewRecord(containerStatsCollection)
			containerStatsRecord.Set("system", systemRecord.Id)
			containerStatsRecord.Set("stats", data.Containers)
			containerStatsRecord.Set("type", "1m")
			if err := txApp.SaveNoValidate(containerStatsRecord); err != nil {
				return err
			}
		}

		// add new systemd_stats record
		if len(data.SystemdServices) > 0 {
			if err := createSystemdStatsRecords(txApp, data.SystemdServices, sys.Id); err != nil {
				return err
			}
		}

		// add system details record
		if data.Details != nil {
			if err := createSystemDetailsRecord(txApp, data.Details, sys.Id); err != nil {
				return err
			}
			sys.detailsFetched.Store(true)
			// update smart interval if it's set on the agent side
			if data.Details.SmartInterval > 0 {
				sys.smartInterval = data.Details.SmartInterval
			}
		}

		// update system record (do this last because it triggers alerts and we need above records to be inserted first)
		systemRecord.Set("status", up)
		systemRecord.Set("info", data.Info)
		if err := txApp.SaveNoValidate(systemRecord); err != nil {
			return err
		}
		return nil
	})

	return systemRecord, err
}

func createSystemDetailsRecord(app core.App, data *system.Details, systemId string) error {
	collectionName := "system_details"
	params := dbx.Params{
		"id":           systemId,
		"system":       systemId,
		"hostname":     data.Hostname,
		"kernel":       data.Kernel,
		"cores":        data.Cores,
		"threads":      data.Threads,
		"cpu":          data.CpuModel,
		"os":           data.Os,
		"os_name":      data.OsName,
		"arch":         data.Arch,
		"memory":       data.MemoryTotal,
		"podman":       data.Podman,
		"cuda_version": data.CudaVersion,
		"nvidia_ctk":   data.NvidiaCTK,
		"updated":      time.Now().UTC(),
	}
	result, err := app.DB().Update(collectionName, params, dbx.HashExp{"id": systemId}).Execute()
	rowsAffected, _ := result.RowsAffected()
	if err != nil || rowsAffected == 0 {
		_, err = app.DB().Insert(collectionName, params).Execute()
	}
	return err
}

func createSystemdStatsRecords(app core.App, data []*systemd.Service, systemId string) error {
	if len(data) == 0 {
		return nil
	}
	// shared params for all records
	params := dbx.Params{
		"system":  systemId,
		"updated": time.Now().UTC().UnixMilli(),
	}

	valueStrings := make([]string, 0, len(data))
	for i, service := range data {
		suffix := fmt.Sprintf("%d", i)
		valueStrings = append(valueStrings, fmt.Sprintf("({:id%[1]s}, {:system}, {:name%[1]s}, {:state%[1]s}, {:sub%[1]s}, {:cpu%[1]s}, {:cpuPeak%[1]s}, {:memory%[1]s}, {:memPeak%[1]s}, {:updated})", suffix))
		params["id"+suffix] = makeStableHashId(systemId, service.Name)
		params["name"+suffix] = service.Name
		params["state"+suffix] = service.State
		params["sub"+suffix] = service.Sub
		params["cpu"+suffix] = service.Cpu
		params["cpuPeak"+suffix] = service.CpuPeak
		params["memory"+suffix] = service.Mem
		params["memPeak"+suffix] = service.MemPeak
	}
	queryString := fmt.Sprintf(
		"INSERT INTO systemd_services (id, system, name, state, sub, cpu, cpuPeak, memory, memPeak, updated) VALUES %s ON CONFLICT(id) DO UPDATE SET system = excluded.system, name = excluded.name, state = excluded.state, sub = excluded.sub, cpu = excluded.cpu, cpuPeak = excluded.cpuPeak, memory = excluded.memory, memPeak = excluded.memPeak, updated = excluded.updated",
		strings.Join(valueStrings, ","),
	)
	_, err := app.DB().NewQuery(queryString).Bind(params).Execute()
	return err
}

// createContainerRecords creates container records
func createContainerRecords(app core.App, data []*container.Stats, systemId string) error {
	if len(data) == 0 {
		return nil
	}
	// shared params for all records
	params := dbx.Params{
		"system":  systemId,
		"updated": time.Now().UTC().UnixMilli(),
	}
	valueStrings := make([]string, 0, len(data))
	for i, container := range data {
		suffix := fmt.Sprintf("%d", i)
		valueStrings = append(valueStrings, fmt.Sprintf("({:id%[1]s}, {:system}, {:name%[1]s}, {:image%[1]s}, {:status%[1]s}, {:uptime%[1]s}, {:cpu%[1]s}, {:memory%[1]s}, {:net%[1]s}, {:updated})", suffix))
		params["id"+suffix] = container.Id
		params["name"+suffix] = container.Name
		params["image"+suffix] = container.Image
		params["status"+suffix] = container.Status
		params["uptime"+suffix] = container.Uptime
		params["cpu"+suffix] = container.Cpu
		params["memory"+suffix] = container.Mem
		params["net"+suffix] = container.NetworkSent + container.NetworkRecv
	}
	queryString := fmt.Sprintf(
		"INSERT INTO containers (id, system, name, image, status, uptime, cpu, memory, net, updated) VALUES %s ON CONFLICT(id) DO UPDATE SET system = excluded.system, name = excluded.name, image = excluded.image, status = excluded.status, uptime = excluded.uptime, cpu = excluded.cpu, memory = excluded.memory, net = excluded.net, updated = excluded.updated",
		strings.Join(valueStrings, ","),
	)
	_, err := app.DB().NewQuery(queryString).Bind(params).Execute()
	return err
}

// getRecord retrieves the system record from the database.
// If the record is not found, it removes the system from the manager.
func (sys *System) getRecord() (*core.Record, error) {
	record, err := sys.manager.hub.FindRecordById("systems", sys.Id)
	if err != nil || record == nil {
		_ = sys.manager.RemoveSystem(sys.Id)
		return nil, err
	}
	return record, nil
}

// setDown marks a system as down in the database.
// It takes the original error that caused the system to go down and returns any error
// encountered during the process of updating the system status.
func (sys *System) setDown(originalError error) error {
	if sys.Status == down || sys.Status == paused {
		return nil
	}
	record, err := sys.getRecord()
	if err != nil {
		return err
	}
	if originalError != nil {
		sys.manager.hub.Logger().Error("System down", "logger", "systems", "system", record.GetString("name"), "err", originalError)
	}
	record.Set("status", down)
	return sys.manager.hub.SaveNoValidate(record)
}

func (sys *System) getContext() (context.Context, context.CancelFunc) {
	if sys.ctx == nil {
		sys.ctx, sys.cancel = context.WithCancel(context.Background())
	}
	return sys.ctx, sys.cancel
}

// fetchDataFromAgent attempts to fetch data from the agent,
// prioritizing WebSocket if available.
func (sys *System) fetchDataFromAgent(options common.DataRequestOptions) (*system.CombinedData, error) {
	if sys.data == nil {
		sys.data = &system.CombinedData{}
	}

	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		wsData, err := sys.fetchDataViaWebSocket(options)
		if err == nil {
			return wsData, nil
		}
		// close the WebSocket connection if error and try SSH
		sys.closeWebSocketConnection()
	}

	sshData, err := sys.fetchDataViaSSH(options)
	if err != nil {
		return nil, err
	}
	return sshData, nil
}

func (sys *System) fetchDataViaWebSocket(options common.DataRequestOptions) (*system.CombinedData, error) {
	if sys.WsConn == nil || !sys.WsConn.IsConnected() {
		return nil, errors.New("no websocket connection")
	}
	err := sys.WsConn.RequestSystemData(context.Background(), sys.data, options)
	if err != nil {
		return nil, err
	}
	return sys.data, nil
}

// fetchStringFromAgentViaSSH is a generic function to fetch strings via SSH
func (sys *System) fetchStringFromAgentViaSSH(action common.WebSocketAction, requestData any, errorMsg string) (string, error) {
	var result string
	err := sys.runSSHOperation(4*time.Second, 1, func(session *ssh.Session) (bool, error) {
		stdout, err := session.StdoutPipe()
		if err != nil {
			return false, err
		}
		stdin, stdinErr := session.StdinPipe()
		if stdinErr != nil {
			return false, stdinErr
		}
		if err := session.Shell(); err != nil {
			return false, err
		}
		req := common.HubRequest[any]{Action: action, Data: requestData}
		_ = cbor.NewEncoder(stdin).Encode(req)
		_ = stdin.Close()
		var resp common.AgentResponse
		err = cbor.NewDecoder(stdout).Decode(&resp)
		if err != nil {
			return false, err
		}
		if resp.String == nil {
			return false, errors.New(errorMsg)
		}
		result = *resp.String
		return false, nil
	})
	return result, err
}

// fetchDockerResponseViaSSH fetches a docker response via SSH and returns the raw AgentResponse.
func (sys *System) fetchDockerResponseViaSSH(action common.WebSocketAction, requestData any, timeout time.Duration) (common.AgentResponse, error) {
	var response common.AgentResponse
	err := sys.runSSHOperation(timeout, 1, func(session *ssh.Session) (bool, error) {
		stdout, err := session.StdoutPipe()
		if err != nil {
			return false, err
		}
		stdin, stdinErr := session.StdinPipe()
		if stdinErr != nil {
			return false, stdinErr
		}
		if err := session.Shell(); err != nil {
			return false, err
		}
		req := common.HubRequest[any]{Action: action, Data: requestData}
		if err := cbor.NewEncoder(stdin).Encode(req); err != nil {
			return false, err
		}
		_ = stdin.Close()
		if err := cbor.NewDecoder(stdout).Decode(&response); err != nil {
			return false, err
		}
		if response.Error != "" {
			return false, errors.New(response.Error)
		}
		return false, nil
	})
	return response, err
}

// FetchContainerInfoFromAgent fetches container info from the agent
func (sys *System) FetchContainerInfoFromAgent(containerID string) (string, error) {
	// fetch via websocket
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return sys.WsConn.RequestContainerInfo(ctx, containerID)
	}
	// fetch via SSH
	return sys.fetchStringFromAgentViaSSH(common.GetContainerInfo, common.ContainerInfoRequest{ContainerID: containerID}, "no info in response")
}

// FetchContainerLogsFromAgent fetches container logs from the agent
func (sys *System) FetchContainerLogsFromAgent(containerID string) (string, error) {
	// fetch via websocket
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return sys.WsConn.RequestContainerLogs(ctx, containerID)
	}
	// fetch via SSH
	return sys.fetchStringFromAgentViaSSH(common.GetContainerLogs, common.ContainerLogsRequest{ContainerID: containerID}, "no logs in response")
}

// UpdateNow triggers an immediate system update (containers/stats/etc).
func (sys *System) UpdateNow() error {
	if sys.updateNowOverride != nil {
		return sys.updateNowOverride()
	}
	data, err := sys.fetchDataFromAgent(common.DataRequestOptions{CacheTimeMs: 0})
	if err != nil {
		return err
	}
	_, err = sys.createRecords(data)
	return err
}

// OperateContainer sends start/stop/restart/kill/pause/unpause to agent.
func (sys *System) OperateContainer(containerID, op, signal string) error {
	if sys.operateOverride != nil {
		return sys.operateOverride(containerID, op, signal)
	}
	req := common.ContainerOperateRequest{ContainerID: containerID, Operation: op, Signal: signal}

	// websocket preferred
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		_, err := sys.WsConn.RequestContainerOperate(ctx, req)
		return err
	}

	// SSH fallback
	return sys.runSSHOperation(12*time.Second, 1, func(session *ssh.Session) (bool, error) {
		stdout, err := session.StdoutPipe()
		if err != nil {
			return false, err
		}
		stdin, stdinErr := session.StdinPipe()
		if stdinErr != nil {
			return false, stdinErr
		}

		if err := session.Start("aether-agent cbor"); err != nil {
			return false, err
		}

		encoder := cbor.NewEncoder(stdin)
		decoder := cbor.NewDecoder(stdout)

		if err := encoder.Encode(common.HubRequest[any]{Action: common.OperateContainer, Data: req}); err != nil {
			return false, err
		}

		var resp common.AgentResponse
		if err := decoder.Decode(&resp); err != nil {
			return false, err
		}

		if resp.Error != "" {
			return false, errors.New(resp.Error)
		}
		return true, nil
	})
}

// FetchDockerOverviewFromAgent fetches docker overview info from the agent.
func (sys *System) FetchDockerOverviewFromAgent() (docker.Overview, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return sys.WsConn.RequestDockerOverview(ctx)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.GetDockerOverview, common.DockerOverviewRequest{}, 5*time.Second)
	if err != nil {
		return docker.Overview{}, err
	}
	if resp.DockerInfo == nil {
		return docker.Overview{}, errors.New("no docker overview in response")
	}
	return *resp.DockerInfo, nil
}

// FetchDockerContainersFromAgent fetches docker container list from the agent.
func (sys *System) FetchDockerContainersFromAgent(all bool) ([]docker.Container, error) {
	req := common.DockerContainerListRequest{All: all}
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return sys.WsConn.RequestDockerContainers(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.ListDockerContainers, req, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if resp.DockerContainers == nil {
		return nil, errors.New("no docker containers in response")
	}
	return resp.DockerContainers, nil
}

// FetchDockerImagesFromAgent fetches docker image list from the agent.
func (sys *System) FetchDockerImagesFromAgent(all bool) ([]docker.Image, error) {
	req := common.DockerImageListRequest{All: all}
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return sys.WsConn.RequestDockerImages(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.ListDockerImages, req, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if resp.DockerImages == nil {
		return nil, errors.New("no docker images in response")
	}
	return resp.DockerImages, nil
}

// PullDockerImageFromAgent triggers docker image pull on the agent.
func (sys *System) PullDockerImageFromAgent(req common.DockerImagePullRequest) (string, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		return sys.WsConn.RequestDockerImagePull(ctx, req)
	}
	return sys.fetchStringFromAgentViaSSH(common.PullDockerImage, req, "docker image pull failed")
}

// PushDockerImageFromAgent triggers docker image push on the agent.
func (sys *System) PushDockerImageFromAgent(req common.DockerImagePushRequest) (string, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		return sys.WsConn.RequestDockerImagePush(ctx, req)
	}
	return sys.fetchStringFromAgentViaSSH(common.PushDockerImage, req, "docker image push failed")
}

// RemoveDockerImageFromAgent removes a docker image on the agent.
func (sys *System) RemoveDockerImageFromAgent(req common.DockerImageRemoveRequest) error {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := sys.WsConn.RequestDockerImageRemove(ctx, req)
		return err
	}
	_, err := sys.fetchStringFromAgentViaSSH(common.RemoveDockerImage, req, "docker image remove failed")
	return err
}

// FetchDockerNetworksFromAgent fetches docker network list from the agent.
func (sys *System) FetchDockerNetworksFromAgent() ([]docker.Network, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return sys.WsConn.RequestDockerNetworks(ctx)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.ListDockerNetworks, nil, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if resp.DockerNetworks == nil {
		return nil, errors.New("no docker networks in response")
	}
	return resp.DockerNetworks, nil
}

// CreateDockerNetworkFromAgent creates a docker network on the agent.
func (sys *System) CreateDockerNetworkFromAgent(req common.DockerNetworkCreateRequest) error {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := sys.WsConn.RequestDockerNetworkCreate(ctx, req)
		return err
	}
	_, err := sys.fetchStringFromAgentViaSSH(common.CreateDockerNetwork, req, "docker network create failed")
	return err
}

// RemoveDockerNetworkFromAgent removes a docker network on the agent.
func (sys *System) RemoveDockerNetworkFromAgent(req common.DockerNetworkRemoveRequest) error {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := sys.WsConn.RequestDockerNetworkRemove(ctx, req)
		return err
	}
	_, err := sys.fetchStringFromAgentViaSSH(common.RemoveDockerNetwork, req, "docker network remove failed")
	return err
}

// FetchDockerVolumesFromAgent fetches docker volume list from the agent.
func (sys *System) FetchDockerVolumesFromAgent() ([]docker.Volume, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return sys.WsConn.RequestDockerVolumes(ctx)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.ListDockerVolumes, nil, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if resp.DockerVolumes == nil {
		return nil, errors.New("no docker volumes in response")
	}
	return resp.DockerVolumes, nil
}

// CreateDockerVolumeFromAgent creates a docker volume on the agent.
func (sys *System) CreateDockerVolumeFromAgent(req common.DockerVolumeCreateRequest) error {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := sys.WsConn.RequestDockerVolumeCreate(ctx, req)
		return err
	}
	_, err := sys.fetchStringFromAgentViaSSH(common.CreateDockerVolume, req, "docker volume create failed")
	return err
}

// RemoveDockerVolumeFromAgent removes a docker volume on the agent.
func (sys *System) RemoveDockerVolumeFromAgent(req common.DockerVolumeRemoveRequest) error {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := sys.WsConn.RequestDockerVolumeRemove(ctx, req)
		return err
	}
	_, err := sys.fetchStringFromAgentViaSSH(common.RemoveDockerVolume, req, "docker volume remove failed")
	return err
}

// FetchDockerComposeProjectsFromAgent fetches compose projects from the agent.
func (sys *System) FetchDockerComposeProjectsFromAgent() ([]docker.ComposeProject, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return sys.WsConn.RequestDockerComposeProjects(ctx)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.ListDockerComposeProjects, common.DockerComposeProjectListRequest{}, 10*time.Second)
	if err != nil {
		return nil, err
	}
	if resp.DockerComposeProjects == nil {
		return nil, errors.New("no docker compose projects in response")
	}
	return resp.DockerComposeProjects, nil
}

// CreateDockerComposeProjectFromAgent creates a compose project on the agent.
func (sys *System) CreateDockerComposeProjectFromAgent(req common.DockerComposeProjectCreateRequest) (string, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		return sys.WsConn.RequestDockerComposeCreate(ctx, req)
	}
	return sys.fetchStringFromAgentViaSSH(common.CreateDockerComposeProject, req, "docker compose create failed")
}

// UpdateDockerComposeProjectFromAgent updates a compose project on the agent.
func (sys *System) UpdateDockerComposeProjectFromAgent(req common.DockerComposeProjectUpdateRequest) (string, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		return sys.WsConn.RequestDockerComposeUpdate(ctx, req)
	}
	return sys.fetchStringFromAgentViaSSH(common.UpdateDockerComposeProject, req, "docker compose update failed")
}

// OperateDockerComposeProjectFromAgent operates a compose project on the agent.
func (sys *System) OperateDockerComposeProjectFromAgent(req common.DockerComposeProjectOperateRequest) (string, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		return sys.WsConn.RequestDockerComposeOperate(ctx, req)
	}
	return sys.fetchStringFromAgentViaSSH(common.OperateDockerComposeProject, req, "docker compose operation failed")
}

// DeleteDockerComposeProjectFromAgent deletes a compose project on the agent.
func (sys *System) DeleteDockerComposeProjectFromAgent(req common.DockerComposeProjectDeleteRequest) (string, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		return sys.WsConn.RequestDockerComposeDelete(ctx, req)
	}
	return sys.fetchStringFromAgentViaSSH(common.DeleteDockerComposeProject, req, "docker compose delete failed")
}

// FetchDockerConfigFromAgent fetches docker daemon config from the agent.
func (sys *System) FetchDockerConfigFromAgent() (docker.DaemonConfig, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return sys.WsConn.RequestDockerConfig(ctx)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.GetDockerConfig, common.DockerConfigRequest{}, 5*time.Second)
	if err != nil {
		return docker.DaemonConfig{}, err
	}
	if resp.DockerConfig == nil {
		return docker.DaemonConfig{}, errors.New("no docker config in response")
	}
	return *resp.DockerConfig, nil
}

// UpdateDockerConfigFromAgent updates docker daemon config on the agent.
func (sys *System) UpdateDockerConfigFromAgent(req common.DockerConfigUpdateRequest) error {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, err := sys.WsConn.RequestDockerConfigUpdate(ctx, req)
		return err
	}
	_, err := sys.fetchStringFromAgentViaSSH(common.UpdateDockerConfig, req, "docker config update failed")
	return err
}

// SetOperateOverride sets a test hook to override container operations.
func (sys *System) SetOperateOverride(fn func(containerID, op, signal string) error) {
	sys.operateOverride = fn
}

// SetUpdateNowOverride sets a test hook to override UpdateNow.
func (sys *System) SetUpdateNowOverride(fn func() error) {
	sys.updateNowOverride = fn
}

// FetchSystemdInfoFromAgent fetches detailed systemd service information from the agent
func (sys *System) FetchSystemdInfoFromAgent(serviceName string) (systemd.ServiceDetails, error) {
	// fetch via websocket
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return sys.WsConn.RequestSystemdInfo(ctx, serviceName)
	}

	var result systemd.ServiceDetails
	err := sys.runSSHOperation(5*time.Second, 1, func(session *ssh.Session) (bool, error) {
		stdout, err := session.StdoutPipe()
		if err != nil {
			return false, err
		}
		stdin, stdinErr := session.StdinPipe()
		if stdinErr != nil {
			return false, stdinErr
		}
		if err := session.Shell(); err != nil {
			return false, err
		}

		req := common.HubRequest[any]{Action: common.GetSystemdInfo, Data: common.SystemdInfoRequest{ServiceName: serviceName}}
		if err := cbor.NewEncoder(stdin).Encode(req); err != nil {
			return false, err
		}
		_ = stdin.Close()

		var resp common.AgentResponse
		if err := cbor.NewDecoder(stdout).Decode(&resp); err != nil {
			return false, err
		}
		if resp.ServiceInfo == nil {
			if resp.Error != "" {
				return false, errors.New(resp.Error)
			}
			return false, errors.New("no systemd info in response")
		}
		result = *resp.ServiceInfo
		return false, nil
	})

	return result, err
}

func makeStableHashId(strings ...string) string {
	hash := fnv.New32a()
	for _, str := range strings {
		hash.Write([]byte(str))
	}
	return fmt.Sprintf("%x", hash.Sum32())
}

// fetchDataViaSSH handles fetching data using SSH.
// This function encapsulates the original SSH logic.
// It updates sys.data directly upon successful fetch.
func (sys *System) fetchDataViaSSH(options common.DataRequestOptions) (*system.CombinedData, error) {
	err := sys.runSSHOperation(4*time.Second, 1, func(session *ssh.Session) (bool, error) {
		stdout, err := session.StdoutPipe()
		if err != nil {
			return false, err
		}
		stdin, stdinErr := session.StdinPipe()
		if err := session.Shell(); err != nil {
			return false, err
		}

		*sys.data = system.CombinedData{}

		if sys.agentVersion.GTE(aether.MinVersionAgentResponse) && stdinErr == nil {
			req := common.HubRequest[any]{Action: common.GetData, Data: options}
			_ = cbor.NewEncoder(stdin).Encode(req)
			_ = stdin.Close()

			var resp common.AgentResponse
			if decErr := cbor.NewDecoder(stdout).Decode(&resp); decErr == nil && resp.SystemData != nil {
				*sys.data = *resp.SystemData
				if err := session.Wait(); err != nil {
					return false, err
				}
				return false, nil
			}
		}

		var decodeErr error
		if sys.agentVersion.GTE(aether.MinVersionCbor) {
			decodeErr = cbor.NewDecoder(stdout).Decode(sys.data)
		} else {
			decodeErr = json.NewDecoder(stdout).Decode(sys.data)
		}

		if decodeErr != nil {
			return true, decodeErr
		}

		if err := session.Wait(); err != nil {
			return false, err
		}

		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return sys.data, nil
}

// runSSHOperation establishes an SSH session and executes the provided operation.
// The operation can request a retry by returning true as the first return value.
func (sys *System) runSSHOperation(timeout time.Duration, retries int, operation func(*ssh.Session) (bool, error)) error {
	for attempt := 0; attempt <= retries; attempt++ {
		if sys.client == nil || sys.Status == down {
			if err := sys.createSSHClient(); err != nil {
				return err
			}
		}

		session, err := sys.createSessionWithTimeout(timeout)
		if err != nil {
			if attempt >= retries {
				return err
			}
			sys.manager.hub.Logger().Warn("Session closed. Retrying...", "logger", "systems", "host", sys.Host, "port", sys.Port, "err", err)
			sys.closeSSHConnection()
			continue
		}

		retry, opErr := func() (bool, error) {
			defer session.Close()
			return operation(session)
		}()

		if opErr == nil {
			return nil
		}

		if retry {
			sys.closeSSHConnection()
			if attempt < retries {
				continue
			}
		}

		return opErr
	}

	return fmt.Errorf("ssh operation failed")
}

// createSSHClient creates a new SSH client for the system
func (s *System) createSSHClient() error {
	if s.manager.sshConfig == nil {
		if err := s.manager.createSSHClientConfig(); err != nil {
			return err
		}
	}
	network := "tcp"
	host := s.Host
	if strings.HasPrefix(host, "/") {
		network = "unix"
	} else {
		host = net.JoinHostPort(host, s.Port)
	}
	var err error
	s.client, err = ssh.Dial(network, host, s.manager.sshConfig)
	if err != nil {
		return err
	}
	s.agentVersion, _ = extractAgentVersion(string(s.client.Conn.ServerVersion()))
	return nil
}

// createSessionWithTimeout creates a new SSH session with a timeout to avoid hanging
// in case of network issues
func (sys *System) createSessionWithTimeout(timeout time.Duration) (*ssh.Session, error) {
	if sys.client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	ctx, cancel := context.WithTimeout(sys.ctx, timeout)
	defer cancel()

	sessionChan := make(chan *ssh.Session, 1)
	errChan := make(chan error, 1)

	go func() {
		if session, err := sys.client.NewSession(); err != nil {
			errChan <- err
		} else {
			sessionChan <- session
		}
	}()

	select {
	case session := <-sessionChan:
		return session, nil
	case err := <-errChan:
		return nil, err
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout")
	}
}

// closeSSHConnection closes the SSH connection but keeps the system in the manager
func (sys *System) closeSSHConnection() {
	if sys.client != nil {
		sys.client.Close()
		sys.client = nil
	}
}

// closeWebSocketConnection closes the WebSocket connection but keeps the system in the manager
// to allow updating via SSH. It will be removed if the WS connection is re-established.
// The system will be set as down a few seconds later if the connection is not re-established.
func (sys *System) closeWebSocketConnection() {
	if sys.WsConn != nil {
		sys.WsConn.Close(nil)
	}
}

// extractAgentVersion extracts the Aether version from SSH server version string
func extractAgentVersion(versionString string) (semver.Version, error) {
	_, after, _ := strings.Cut(versionString, "_")
	return semver.Parse(after)
}

// getJitter returns a channel that will be triggered after a random delay
// between 51% and 95% of the interval.
// This is used to stagger the initial WebSocket connections to prevent clustering.
func getJitter() <-chan time.Time {
	minPercent := 51
	maxPercent := 95
	jitterRange := maxPercent - minPercent
	msDelay := (interval * minPercent / 100) + rand.Intn(interval*jitterRange/100)
	return time.After(time.Duration(msDelay) * time.Millisecond)
}

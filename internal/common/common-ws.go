package common

import (
	"aether/internal/entities/docker"
	"aether/internal/entities/repo"
	"aether/internal/entities/smart"
	"aether/internal/entities/system"
	"aether/internal/entities/systemd"
)

type WebSocketAction = uint8

const (
	// Request system data from agent
	GetData WebSocketAction = iota
	// Check the fingerprint of the agent
	CheckFingerprint
	// Request container logs from agent
	GetContainerLogs
	// Request container info from agent
	GetContainerInfo
	// Request SMART data from agent
	GetSmartData
	// Request detailed systemd service info from agent
	GetSystemdInfo
	// Operate a container (start/stop/restart/kill/pause/unpause)
	OperateContainer
	// Request Docker overview data
	GetDockerOverview
	// Request Docker container list
	ListDockerContainers
	// Request Docker image list
	ListDockerImages
	// Pull Docker image
	PullDockerImage
	// Push Docker image
	PushDockerImage
	// Remove Docker image
	RemoveDockerImage
	// List Docker networks
	ListDockerNetworks
	// Create Docker network
	CreateDockerNetwork
	// Remove Docker network
	RemoveDockerNetwork
	// List Docker volumes
	ListDockerVolumes
	// Create Docker volume
	CreateDockerVolume
	// Remove Docker volume
	RemoveDockerVolume
	// List Docker compose projects
	ListDockerComposeProjects
	// Create Docker compose project
	CreateDockerComposeProject
	// Update Docker compose project
	UpdateDockerComposeProject
	// Operate Docker compose project
	OperateDockerComposeProject
	// Remove Docker compose project
	DeleteDockerComposeProject
	// Get Docker daemon config
	GetDockerConfig
	// Update Docker daemon config
	UpdateDockerConfig
	// Request package repository sources
	GetRepoSources
	// Add new actions here...
)

// HubRequest defines the structure for requests sent from hub to agent.
type HubRequest[T any] struct {
	Action WebSocketAction `cbor:"0,keyasint"`
	Data   T               `cbor:"1,keyasint,omitempty,omitzero"`
	Id     *uint32         `cbor:"2,keyasint,omitempty"`
}

// AgentResponse defines the structure for responses sent from agent to hub.
type AgentResponse struct {
	Id                    *uint32                    `cbor:"0,keyasint,omitempty"`
	SystemData            *system.CombinedData       `cbor:"1,keyasint,omitempty,omitzero"`
	Fingerprint           *FingerprintResponse       `cbor:"2,keyasint,omitempty,omitzero"`
	Error                 string                     `cbor:"3,keyasint,omitempty,omitzero"`
	String                *string                    `cbor:"4,keyasint,omitempty,omitzero"`
	SmartData             map[string]smart.SmartData `cbor:"5,keyasint,omitempty,omitzero"`
	ServiceInfo           *systemd.ServiceDetails    `cbor:"6,keyasint,omitempty,omitzero"`
	DockerInfo            *docker.Overview           `cbor:"7,keyasint,omitempty,omitzero"`
	DockerContainers      []docker.Container         `cbor:"8,keyasint,omitempty,omitzero"`
	DockerImages          []docker.Image             `cbor:"9,keyasint,omitempty,omitzero"`
	DockerNetworks        []docker.Network           `cbor:"10,keyasint,omitempty,omitzero"`
	DockerVolumes         []docker.Volume            `cbor:"11,keyasint,omitempty,omitzero"`
	DockerComposeProjects []docker.ComposeProject    `cbor:"12,keyasint,omitempty,omitzero"`
	DockerConfig          *docker.DaemonConfig       `cbor:"13,keyasint,omitempty,omitzero"`
	RepoSources           []repo.Source              `cbor:"14,keyasint,omitempty,omitzero"`
	// Logs        *LogsPayload         `cbor:"4,keyasint,omitempty,omitzero"`
	// RawBytes    []byte               `cbor:"4,keyasint,omitempty,omitzero"`
}

type FingerprintRequest struct {
	Signature   []byte `cbor:"0,keyasint"`
	NeedSysInfo bool   `cbor:"1,keyasint"` // For universal token system creation
}

type FingerprintResponse struct {
	Fingerprint string `cbor:"0,keyasint"`
	// Optional system info for universal token system creation
	Hostname string `cbor:"1,keyasint,omitzero"`
	Port     string `cbor:"2,keyasint,omitzero"`
	Name     string `cbor:"3,keyasint,omitzero"`
}

type DataRequestOptions struct {
	CacheTimeMs    uint16 `cbor:"0,keyasint"`
	IncludeDetails bool   `cbor:"1,keyasint"`
}

type ContainerLogsRequest struct {
	ContainerID string `cbor:"0,keyasint"`
}

type ContainerInfoRequest struct {
	ContainerID string `cbor:"0,keyasint"`
}

type ContainerOperateRequest struct {
	ContainerID string `cbor:"0,keyasint"`
	Operation   string `cbor:"1,keyasint"`
	Signal      string `cbor:"2,keyasint,omitempty"`
}

type DockerOverviewRequest struct{}

type DockerContainerListRequest struct {
	All bool `cbor:"0,keyasint,omitempty"`
}

type DockerImageListRequest struct {
	All bool `cbor:"0,keyasint,omitempty"`
}

type DockerRegistryAuth struct {
	Server   string `cbor:"0,keyasint"`
	Username string `cbor:"1,keyasint"`
	Password string `cbor:"2,keyasint"`
}

type DockerImagePullRequest struct {
	Image    string              `cbor:"0,keyasint"`
	Registry *DockerRegistryAuth `cbor:"1,keyasint,omitempty"`
}

type DockerImagePushRequest struct {
	Image    string              `cbor:"0,keyasint"`
	Registry *DockerRegistryAuth `cbor:"1,keyasint,omitempty"`
}

type DockerImageRemoveRequest struct {
	ImageID string `cbor:"0,keyasint"`
	Force   bool   `cbor:"1,keyasint,omitempty"`
}

type DockerNetworkCreateRequest struct {
	Name       string            `cbor:"0,keyasint"`
	Driver     string            `cbor:"1,keyasint,omitempty"`
	EnableIPv6 bool              `cbor:"2,keyasint,omitempty"`
	Internal   bool              `cbor:"3,keyasint,omitempty"`
	Attachable bool              `cbor:"4,keyasint,omitempty"`
	Labels     map[string]string `cbor:"5,keyasint,omitempty"`
	Options    map[string]string `cbor:"6,keyasint,omitempty"`
}

type DockerNetworkRemoveRequest struct {
	NetworkID string `cbor:"0,keyasint"`
}

type DockerVolumeCreateRequest struct {
	Name    string            `cbor:"0,keyasint"`
	Driver  string            `cbor:"1,keyasint,omitempty"`
	Labels  map[string]string `cbor:"2,keyasint,omitempty"`
	Options map[string]string `cbor:"3,keyasint,omitempty"`
}

type DockerVolumeRemoveRequest struct {
	Name  string `cbor:"0,keyasint"`
	Force bool   `cbor:"1,keyasint,omitempty"`
}

type DockerComposeProjectListRequest struct{}

type DockerComposeProjectCreateRequest struct {
	Name    string `cbor:"0,keyasint"`
	Content string `cbor:"1,keyasint"`
	Env     string `cbor:"2,keyasint,omitempty"`
}

type DockerComposeProjectUpdateRequest struct {
	Name    string `cbor:"0,keyasint"`
	Content string `cbor:"1,keyasint"`
	Env     string `cbor:"2,keyasint,omitempty"`
}

type DockerComposeProjectOperateRequest struct {
	Name       string `cbor:"0,keyasint"`
	Operation  string `cbor:"1,keyasint"`
	RemoveFile bool   `cbor:"2,keyasint,omitempty"`
}

type DockerComposeProjectDeleteRequest struct {
	Name       string `cbor:"0,keyasint"`
	RemoveFile bool   `cbor:"1,keyasint,omitempty"`
}

type DockerConfigRequest struct{}

type DockerConfigUpdateRequest struct {
	Content string `cbor:"0,keyasint"`
	Path    string `cbor:"1,keyasint,omitempty"`
	Restart bool   `cbor:"2,keyasint,omitempty"`
}

type SystemdInfoRequest struct {
	ServiceName string `cbor:"0,keyasint"`
}

type RepoSourcesRequest struct {
	Check bool `cbor:"0,keyasint,omitempty"`
}

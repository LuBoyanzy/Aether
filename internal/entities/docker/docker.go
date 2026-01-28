// Package docker 定义 Docker 相关的实体数据结构，用于 Agent 与 Hub 之间传输。
// 该模块只描述数据，不包含具体的 Docker 操作逻辑。
package docker

// Overview 描述 Docker 引擎的概览信息。
type Overview struct {
	ServerVersion     string `json:"serverVersion" cbor:"0,keyasint"`
	APIVersion        string `json:"apiVersion" cbor:"1,keyasint"`
	OperatingSystem   string `json:"operatingSystem" cbor:"2,keyasint"`
	KernelVersion     string `json:"kernelVersion" cbor:"3,keyasint"`
	Architecture      string `json:"architecture" cbor:"4,keyasint"`
	Containers        int    `json:"containers" cbor:"5,keyasint"`
	ContainersRunning int    `json:"containersRunning" cbor:"6,keyasint"`
	ContainersPaused  int    `json:"containersPaused" cbor:"7,keyasint"`
	ContainersStopped int    `json:"containersStopped" cbor:"8,keyasint"`
	Images            int    `json:"images" cbor:"9,keyasint"`
	StorageDriver     string `json:"storageDriver" cbor:"10,keyasint"`
	LoggingDriver     string `json:"loggingDriver" cbor:"11,keyasint"`
	CgroupDriver      string `json:"cgroupDriver" cbor:"12,keyasint"`
	DockerRootDir     string `json:"dockerRootDir" cbor:"13,keyasint"`
	CPUs              int    `json:"cpus" cbor:"14,keyasint"`
	MemTotal          uint64 `json:"memTotal" cbor:"15,keyasint"`
	ComposeVersion    string `json:"composeVersion" cbor:"16,keyasint"`
}

// Container 描述容器列表项。
type Container struct {
	ID        string            `json:"id" cbor:"0,keyasint"`
	Name      string            `json:"name" cbor:"1,keyasint"`
	Image     string            `json:"image" cbor:"2,keyasint"`
	ImageID   string            `json:"imageId" cbor:"11,keyasint,omitempty"`
	State     string            `json:"state" cbor:"3,keyasint"`
	Status    string            `json:"status" cbor:"4,keyasint"`
	Created   int64             `json:"created" cbor:"5,keyasint"`
	Ports     []Port            `json:"ports" cbor:"6,keyasint,omitempty"`
	Labels    map[string]string `json:"labels" cbor:"7,keyasint,omitempty"`
	Networks  []string          `json:"networks" cbor:"8,keyasint,omitempty"`
	Command   string            `json:"command" cbor:"9,keyasint,omitempty"`
	CreatedBy string            `json:"createdBy" cbor:"10,keyasint,omitempty"`
}

// Port 描述容器端口映射。
type Port struct {
	IP          string `json:"ip" cbor:"0,keyasint,omitempty"`
	PrivatePort uint16 `json:"privatePort" cbor:"1,keyasint"`
	PublicPort  uint16 `json:"publicPort" cbor:"2,keyasint,omitempty"`
	Type        string `json:"type" cbor:"3,keyasint"`
}

// Image 描述镜像信息。
type Image struct {
	ID          string            `json:"id" cbor:"0,keyasint"`
	RepoTags    []string          `json:"repoTags" cbor:"1,keyasint,omitempty"`
	RepoDigests []string          `json:"repoDigests" cbor:"2,keyasint,omitempty"`
	Created     int64             `json:"created" cbor:"3,keyasint"`
	Size        int64             `json:"size" cbor:"4,keyasint"`
	SharedSize  int64             `json:"sharedSize" cbor:"5,keyasint"`
	VirtualSize int64             `json:"virtualSize" cbor:"6,keyasint"`
	Containers  int64             `json:"containers" cbor:"7,keyasint"`
	Labels      map[string]string `json:"labels" cbor:"8,keyasint,omitempty"`
}

// Network 描述网络信息。
type Network struct {
	ID         string            `json:"id" cbor:"0,keyasint"`
	Name       string            `json:"name" cbor:"1,keyasint"`
	Driver     string            `json:"driver" cbor:"2,keyasint"`
	Scope      string            `json:"scope" cbor:"3,keyasint"`
	Internal   bool              `json:"internal" cbor:"4,keyasint"`
	Attachable bool              `json:"attachable" cbor:"5,keyasint"`
	Ingress    bool              `json:"ingress" cbor:"6,keyasint"`
	EnableIPv6 bool              `json:"enableIPv6" cbor:"7,keyasint"`
	Labels     map[string]string `json:"labels" cbor:"8,keyasint,omitempty"`
	Subnets    []string          `json:"subnets" cbor:"9,keyasint,omitempty"`
	Gateways   []string          `json:"gateways" cbor:"10,keyasint,omitempty"`
	Created    string            `json:"created" cbor:"11,keyasint,omitempty"`
}

// Volume 描述存储卷信息。
type Volume struct {
	Name       string            `json:"name" cbor:"0,keyasint"`
	Driver     string            `json:"driver" cbor:"1,keyasint"`
	Mountpoint string            `json:"mountpoint" cbor:"2,keyasint"`
	CreatedAt  string            `json:"createdAt" cbor:"3,keyasint"`
	Scope      string            `json:"scope" cbor:"4,keyasint"`
	Labels     map[string]string `json:"labels" cbor:"5,keyasint,omitempty"`
	Options    map[string]string `json:"options" cbor:"6,keyasint,omitempty"`
}

// ComposeProject 描述编排项目概要。
type ComposeProject struct {
	Name           string             `json:"name" cbor:"0,keyasint"`
	Workdir        string             `json:"workdir" cbor:"1,keyasint"`
	ConfigFiles    []string           `json:"configFiles" cbor:"2,keyasint,omitempty"`
	ContainerCount int                `json:"containerCount" cbor:"3,keyasint"`
	RunningCount   int                `json:"runningCount" cbor:"4,keyasint"`
	Status         string             `json:"status" cbor:"5,keyasint"`
	Services       []string           `json:"services" cbor:"6,keyasint,omitempty"`
	Containers     []ComposeContainer `json:"containers" cbor:"7,keyasint,omitempty"`
	CreatedAt      int64              `json:"createdAt" cbor:"8,keyasint,omitempty"`
	UpdatedAt      int64              `json:"updatedAt" cbor:"9,keyasint,omitempty"`
	Labels         map[string]string  `json:"labels" cbor:"10,keyasint,omitempty"`
}

// ComposeContainer 描述编排项目中的容器。
type ComposeContainer struct {
	ID      string `json:"id" cbor:"0,keyasint"`
	Name    string `json:"name" cbor:"1,keyasint"`
	Image   string `json:"image" cbor:"2,keyasint"`
	State   string `json:"state" cbor:"3,keyasint"`
	Status  string `json:"status" cbor:"4,keyasint"`
	Created int64  `json:"created" cbor:"5,keyasint"`
	Ports   []Port `json:"ports" cbor:"6,keyasint,omitempty"`
}

// DaemonConfig 描述 Docker daemon 配置文件。
type DaemonConfig struct {
	Path    string `json:"path" cbor:"0,keyasint"`
	Content string `json:"content" cbor:"1,keyasint"`
	Exists  bool   `json:"exists" cbor:"2,keyasint"`
}

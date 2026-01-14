import { pb } from "@/lib/api"
import type {
	DockerAuditItem,
	DockerComposeProject,
	DockerComposeTemplateItem,
	DockerContainer,
	DockerDaemonConfig,
	DockerDataCleanupConfig,
	DockerDataCleanupRun,
	DockerImage,
	DockerNetwork,
	DockerOverview,
	DockerRegistryItem,
	DockerServiceConfigItem,
	DockerVolume,
} from "@/types"

// 对接后端实现：internal/hub/docker.go、internal/hub/docker_data_cleanup.go

export const fetchDockerOverview = (system: string) =>
	pb.send<DockerOverview>("/api/aether/docker/overview", { query: { system } })

export const listDockerContainers = (system: string, all?: boolean) =>
	pb.send<DockerContainer[]>("/api/aether/docker/containers", {
		query: { system, ...(all ? { all: "1" } : {}) },
	})

export const listDockerImages = (system: string, all?: boolean) =>
	pb.send<DockerImage[]>("/api/aether/docker/images", {
		query: { system, ...(all ? { all: "1" } : {}) },
	})

export const pullDockerImage = (payload: { system: string; image: string; registryId?: string }) =>
	pb.send<{ status: string; logs: string }>("/api/aether/docker/images/pull", {
		method: "POST",
		body: payload,
	})

export const pushDockerImage = (payload: { system: string; image: string; registryId?: string }) =>
	pb.send<{ status: string; logs: string }>("/api/aether/docker/images/push", {
		method: "POST",
		body: payload,
	})

export const removeDockerImage = (payload: { system: string; image: string; force?: boolean }) =>
	pb.send<{ status: string }>("/api/aether/docker/images/remove", {
		method: "POST",
		body: payload,
	})

export const listDockerNetworks = (system: string) =>
	pb.send<DockerNetwork[]>("/api/aether/docker/networks", { query: { system } })

export const createDockerNetwork = (payload: {
	system: string
	name: string
	driver?: string
	enableIPv6?: boolean
	internal?: boolean
	attachable?: boolean
	labels?: Record<string, string>
	options?: Record<string, string>
}) =>
	pb.send<{ status: string }>("/api/aether/docker/networks", {
		method: "POST",
		body: payload,
	})

export const removeDockerNetwork = (payload: { system: string; networkId: string }) =>
	pb.send<{ status: string }>("/api/aether/docker/networks/remove", {
		method: "POST",
		body: payload,
	})

export const listDockerVolumes = (system: string) =>
	pb.send<DockerVolume[]>("/api/aether/docker/volumes", { query: { system } })

export const createDockerVolume = (payload: {
	system: string
	name: string
	driver?: string
	labels?: Record<string, string>
	options?: Record<string, string>
}) =>
	pb.send<{ status: string }>("/api/aether/docker/volumes", {
		method: "POST",
		body: payload,
	})

export const removeDockerVolume = (payload: { system: string; name: string; force?: boolean }) =>
	pb.send<{ status: string }>("/api/aether/docker/volumes/remove", {
		method: "POST",
		body: payload,
	})

export const listDockerComposeProjects = (system: string) =>
	pb.send<DockerComposeProject[]>("/api/aether/docker/compose/projects", { query: { system } })

export const createDockerComposeProject = (payload: {
	system: string
	name: string
	content: string
	env?: string
}) =>
	pb.send<{ status: string; logs: string }>("/api/aether/docker/compose/projects", {
		method: "POST",
		body: payload,
	})

export const updateDockerComposeProject = (payload: {
	system: string
	name: string
	content: string
	env?: string
}) =>
	pb.send<{ status: string; logs: string }>("/api/aether/docker/compose/projects/update", {
		method: "POST",
		body: payload,
	})

export const operateDockerComposeProject = (payload: {
	system: string
	name: string
	operation: string
	removeFile?: boolean
}) =>
	pb.send<{ status: string; logs: string }>("/api/aether/docker/compose/projects/operate", {
		method: "POST",
		body: payload,
	})

export const deleteDockerComposeProject = (payload: { system: string; name: string; removeFile?: boolean }) =>
	pb.send<{ status: string; logs: string }>("/api/aether/docker/compose/projects/delete", {
		method: "POST",
		body: payload,
	})

export const fetchDockerConfig = (system: string) =>
	pb.send<DockerDaemonConfig>("/api/aether/docker/config", { query: { system } })

export const updateDockerConfig = (payload: {
	system: string
	content: string
	path?: string
	restart?: boolean
}) =>
	pb.send<{ status: string }>("/api/aether/docker/config", {
		method: "POST",
		body: payload,
	})

export const listDockerRegistries = () =>
	pb.send<{ items: DockerRegistryItem[] }>("/api/aether/docker/registries", {})

export const createDockerRegistry = (payload: {
	name: string
	server: string
	username?: string
	password?: string
}) =>
	pb.send<{ id: string }>("/api/aether/docker/registries", {
		method: "POST",
		body: payload,
	})

export const updateDockerRegistry = (payload: {
	id: string
	name?: string
	server?: string
	username?: string
	password?: string
}) =>
	pb.send<{ status: string }>("/api/aether/docker/registries/update", {
		method: "POST",
		body: payload,
	})

export const deleteDockerRegistry = (id: string) =>
	pb.send<{ status: string }>("/api/aether/docker/registries/delete", {
		method: "POST",
		query: { id },
	})

export const listDockerServiceConfigs = (system: string) =>
	pb.send<{ items: DockerServiceConfigItem[] }>("/api/aether/docker/service-configs", {
		query: { system },
	})

export const createDockerServiceConfig = (payload: { system: string; name: string; url: string; token: string }) =>
	pb.send<{ id: string }>("/api/aether/docker/service-configs", {
		method: "POST",
		body: payload,
	})

export const updateDockerServiceConfig = (payload: { id: string; name?: string; url?: string }) =>
	pb.send<{ status: string }>("/api/aether/docker/service-configs/update", {
		method: "POST",
		body: payload,
	})

export const deleteDockerServiceConfig = (id: string) =>
	pb.send<{ status: string }>("/api/aether/docker/service-configs/delete", {
		method: "POST",
		query: { id },
	})

export const fetchDockerServiceConfigContent = (params: { system: string; id: string }) =>
	pb.send<{ content: string }>("/api/aether/docker/service-configs/content", {
		query: params,
	})

export const updateDockerServiceConfigContent = (payload: { system: string; id: string; content: string }) =>
	pb.send<{ status: string }>("/api/aether/docker/service-configs/content", {
		method: "PUT",
		body: payload,
	})

type DataCleanupListPayload = {
	system: string
	host: string
	port: number
	username?: string
	password?: string
	useStoredPassword?: boolean
	database?: string
}

type DataCleanupMinioListPayload = {
	system: string
	host: string
	port: number
	accessKey?: string
	secretKey?: string
	useStoredSecret?: boolean
	bucket?: string
}

export const fetchDockerDataCleanupConfig = (system: string) =>
	pb.send<DockerDataCleanupConfig>("/api/aether/docker/data-cleanup/config", { query: { system } })

export const upsertDockerDataCleanupConfig = (payload: DockerDataCleanupConfig) =>
	pb.send<{ id: string; status: string }>("/api/aether/docker/data-cleanup/config", {
		method: "POST",
		body: payload,
	})

export const listDockerDataCleanupMySQLDatabases = (payload: DataCleanupListPayload) =>
	pb.send<{ items: string[] }>("/api/aether/docker/data-cleanup/mysql/databases", {
		method: "POST",
		body: payload,
	})

export const listDockerDataCleanupMySQLTables = (payload: DataCleanupListPayload) =>
	pb.send<{ items: string[] }>("/api/aether/docker/data-cleanup/mysql/tables", {
		method: "POST",
		body: payload,
	})

export const listDockerDataCleanupRedisDatabases = (payload: DataCleanupListPayload) =>
	pb.send<{ items: number[] }>("/api/aether/docker/data-cleanup/redis/dbs", {
		method: "POST",
		body: payload,
	})

export const listDockerDataCleanupMinioBuckets = (payload: DataCleanupMinioListPayload) =>
	pb.send<{ items: string[] }>("/api/aether/docker/data-cleanup/minio/buckets", {
		method: "POST",
		body: payload,
	})

export const listDockerDataCleanupMinioPrefixes = (payload: DataCleanupMinioListPayload) =>
	pb.send<{ items: string[] }>("/api/aether/docker/data-cleanup/minio/prefixes", {
		method: "POST",
		body: payload,
	})

export const listDockerDataCleanupESIndices = (payload: DataCleanupListPayload) =>
	pb.send<{ items: string[] }>("/api/aether/docker/data-cleanup/es/indices", {
		method: "POST",
		body: payload,
	})

export const startDockerDataCleanupRun = (payload: { system: string }) =>
	pb.send<{ runId: string }>("/api/aether/docker/data-cleanup/run", {
		method: "POST",
		body: payload,
	})

export const fetchDockerDataCleanupRun = (runId: string) =>
	pb.send<DockerDataCleanupRun>("/api/aether/docker/data-cleanup/run", {
		query: { id: runId },
	})

export const retryDockerDataCleanupRun = (payload: { system: string }) =>
	pb.send<{ runId: string }>("/api/aether/docker/data-cleanup/retry", {
		method: "POST",
		body: payload,
	})

export const listDockerComposeTemplates = () =>
	pb.send<{ items: DockerComposeTemplateItem[] }>("/api/aether/docker/compose-templates", {})

export const createDockerComposeTemplate = (payload: {
	name: string
	description?: string
	content: string
	env?: string
}) =>
	pb.send<{ id: string }>("/api/aether/docker/compose-templates", {
		method: "POST",
		body: payload,
	})

export const updateDockerComposeTemplate = (payload: {
	id: string
	name?: string
	description?: string
	content?: string
	env?: string
}) =>
	pb.send<{ status: string }>("/api/aether/docker/compose-templates/update", {
		method: "POST",
		body: payload,
	})

export const deleteDockerComposeTemplate = (id: string) =>
	pb.send<{ status: string }>("/api/aether/docker/compose-templates/delete", {
		method: "POST",
		query: { id },
	})

export const listDockerAudits = (params?: {
	system?: string
	start?: string
	end?: string
	page?: number
	perPage?: number
}) =>
	pb.send<{ items: DockerAuditItem[] }>("/api/aether/docker/audits", {
		query: params ?? {},
	})

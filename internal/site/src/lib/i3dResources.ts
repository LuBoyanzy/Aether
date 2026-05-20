import { pb } from "@/lib/api"

export interface I3DResourceSummary {
	cpu_percent: number
	cpu_cores_used: number
	memory_bytes: number
	memory_usage_bytes?: number
	memory_rss_bytes?: number
	memory_cache_bytes?: number
	memory_anon_bytes?: number
	memory_inactive_file_bytes?: number
	disk_read_bps: number
	disk_write_bps: number
	network_rx_bps: number
	network_tx_bps: number
	gpu_util_percent: number
	gpu_memory_bytes: number
	pids: number
	threads: number
	abnormal_count: number
}

export interface I3DResourceGroup {
	id: string
	name: string
	cpu_percent: number
	memory_bytes: number
	memory_usage_bytes: number
	memory_rss_bytes: number
	memory_cache_bytes: number
	memory_anon_bytes: number
	memory_inactive_file_bytes: number
	pids: number
	threads: number
}

export interface I3DResourceTarget {
	id: string
	name: string
	group: string
	kind: "docker" | "process"
	status: "up" | "down" | "unknown"
	health_status: string
	cpu_percent: number
	cpu_cores_used: number
	memory_bytes: number
	memory_usage_bytes: number
	memory_rss_bytes: number
	memory_cache_bytes: number
	memory_anon_bytes: number
	memory_inactive_file_bytes: number
	memory_percent: number
	disk_read_bps: number
	disk_write_bps: number
	network_rx_bps: number
	network_tx_bps: number
	gpu_memory_bytes: number
	unit_count: number
	pids: number
	threads: number
	uptime_seconds: number
	restart_count: number
	container_name?: string
	pid_file?: string
	ports?: number[]
	working_dir?: string
	command_includes?: string[]
	updated_at: string
	diagnostic?: string
}

export interface I3DResourceOverview {
	environment: "local" | "release"
	sample_interval_seconds: number
	updated_at: string
	summary: I3DResourceSummary
	groups: I3DResourceGroup[]
	items: I3DResourceTarget[]
}

export interface I3DResourceTimeseriesPoint {
	timestamp: string
	cpu_percent: number
	cpu_cores_used: number
	memory_bytes: number
	memory_usage_bytes: number
	memory_rss_bytes: number
	memory_cache_bytes: number
	memory_anon_bytes: number
	memory_inactive_file_bytes: number
	disk_read_bps: number
	disk_write_bps: number
	network_rx_bps: number
	network_tx_bps: number
	gpu_memory_bytes: number
	groups?: Record<
		string,
		{
			cpu_percent: number
			memory_bytes: number
			memory_usage_bytes?: number
			memory_rss_bytes?: number
			memory_cache_bytes?: number
			memory_anon_bytes?: number
			memory_inactive_file_bytes?: number
		}
	>
	targets?: Record<
		string,
		{
			name: string
			cpu_percent: number
			memory_bytes: number
			memory_usage_bytes?: number
			memory_rss_bytes?: number
			memory_cache_bytes?: number
			memory_anon_bytes?: number
			memory_inactive_file_bytes?: number
			gpu_memory_bytes: number
		}
	>
}

export interface I3DResourceTimeseries {
	environment: "local" | "release"
	items: I3DResourceTimeseriesPoint[]
}

export const fetchI3DResourceOverview = () =>
	pb.send<I3DResourceOverview>("/api/aether/i3d/resources/overview", { requestKey: null })

export const fetchI3DResourceTimeseries = () =>
	pb.send<I3DResourceTimeseries>("/api/aether/i3d/resources/timeseries", { requestKey: null })

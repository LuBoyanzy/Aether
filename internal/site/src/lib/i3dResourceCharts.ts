import { i3dResourceMemoryPressureBytes } from "./i3dResourceMemory"
import type { I3DResourceTimeseries } from "./i3dResources"

const groupIDs = ["business", "middleware", "monitor"] as const

export interface I3DResourceChartPoint {
	[key: string]: number
	created: number
	cpu_percent: number
	cpu_cores_used: number
	memory_bytes: number
	memory_pressure_bytes: number
	gpu_memory_bytes: number
	disk_read_bps: number
	disk_write_bps: number
	disk_total_bps: number
	network_rx_bps: number
	network_tx_bps: number
	network_total_bps: number
	business_cpu_percent: number
	business_memory_bytes: number
	business_memory_pressure_bytes: number
	middleware_cpu_percent: number
	middleware_memory_bytes: number
	middleware_memory_pressure_bytes: number
	monitor_cpu_percent: number
	monitor_memory_bytes: number
	monitor_memory_pressure_bytes: number
}

export function buildI3DResourceChartPoints(timeseries: I3DResourceTimeseries | null): I3DResourceChartPoint[] {
	const targetIDs = Array.from(new Set((timeseries?.items ?? []).flatMap((item) => Object.keys(item.targets ?? {}))))
	return (timeseries?.items ?? [])
		.map((item) => {
			const created = new Date(item.timestamp).getTime()
			if (!Number.isFinite(created)) {
				return null
			}
			const point: I3DResourceChartPoint = {
				created,
				cpu_percent: item.cpu_percent || 0,
				cpu_cores_used: item.cpu_cores_used || 0,
				memory_bytes: item.memory_bytes || 0,
				memory_pressure_bytes: i3dResourceMemoryPressureBytes(item),
				gpu_memory_bytes: item.gpu_memory_bytes || 0,
				disk_read_bps: item.disk_read_bps || 0,
				disk_write_bps: item.disk_write_bps || 0,
				disk_total_bps: (item.disk_read_bps || 0) + (item.disk_write_bps || 0),
				network_rx_bps: item.network_rx_bps || 0,
				network_tx_bps: item.network_tx_bps || 0,
				network_total_bps: (item.network_rx_bps || 0) + (item.network_tx_bps || 0),
				business_cpu_percent: 0,
				business_memory_bytes: 0,
				business_memory_pressure_bytes: 0,
				middleware_cpu_percent: 0,
				middleware_memory_bytes: 0,
				middleware_memory_pressure_bytes: 0,
				monitor_cpu_percent: 0,
				monitor_memory_bytes: 0,
				monitor_memory_pressure_bytes: 0,
			}
			for (const groupID of groupIDs) {
				point[`${groupID}_cpu_percent`] = item.groups?.[groupID]?.cpu_percent || 0
				point[`${groupID}_memory_bytes`] = item.groups?.[groupID]?.memory_bytes || 0
				point[`${groupID}_memory_pressure_bytes`] = i3dResourceMemoryPressureBytes(item.groups?.[groupID])
			}
			for (const targetID of targetIDs) {
				const key = targetChartKey(targetID)
				point[`${key}_cpu_percent`] = item.targets?.[targetID]?.cpu_percent || 0
				point[`${key}_memory_bytes`] = item.targets?.[targetID]?.memory_bytes || 0
				point[`${key}_memory_pressure_bytes`] = i3dResourceMemoryPressureBytes(item.targets?.[targetID])
				point[`${key}_gpu_memory_bytes`] = item.targets?.[targetID]?.gpu_memory_bytes || 0
			}
			return point
		})
		.filter((point): point is I3DResourceChartPoint => Boolean(point))
}

export function hasI3DResourceChartData(timeseries: I3DResourceTimeseries | null): boolean {
	return buildI3DResourceChartPoints(timeseries).length >= 2
}

export function targetChartKey(targetID: string): string {
	return `target_${targetID.replace(/[^a-zA-Z0-9]+/g, "_").replace(/^_+|_+$/g, "")}`
}

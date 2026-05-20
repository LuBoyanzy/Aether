export interface I3DResourceMemoryMetrics {
	memory_bytes?: number | null
	memory_usage_bytes?: number | null
	memory_rss_bytes?: number | null
	memory_anon_bytes?: number | null
	memory_cache_bytes?: number | null
}

function positiveBytes(value: number | null | undefined): number {
	return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : 0
}

export function i3dResourceMemoryPressureBytes(metrics: I3DResourceMemoryMetrics | null | undefined): number {
	return (
		positiveBytes(metrics?.memory_rss_bytes) ||
		positiveBytes(metrics?.memory_anon_bytes) ||
		positiveBytes(metrics?.memory_bytes)
	)
}

export function i3dResourceMemoryStatTotalBytes(metrics: I3DResourceMemoryMetrics | null | undefined): number {
	const usageBytes = positiveBytes(metrics?.memory_usage_bytes)
	const workingSetBytes = positiveBytes(metrics?.memory_bytes)
	const pressureBytes = i3dResourceMemoryPressureBytes(metrics)
	return Math.max(usageBytes, workingSetBytes, pressureBytes)
}

export function i3dResourceMemoryCacheBytes(metrics: I3DResourceMemoryMetrics | null | undefined): number {
	return positiveBytes(metrics?.memory_cache_bytes)
}

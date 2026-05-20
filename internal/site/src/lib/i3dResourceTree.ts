import type { I3DResourceTarget } from "./i3dResources"

export interface I3DResourceTreeRow {
	id: string
	name: string
	level: number
	kind: "group" | "target"
	target?: I3DResourceTarget
	cpu_percent: number
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
	unit_count: number
	pids: number
	threads: number
	abnormal_count: number
}

interface I3DResourceTreeGroup {
	id: string
	name: string
	matches: (item: I3DResourceTarget) => boolean
}

const groups: I3DResourceTreeGroup[] = [
	{
		id: "resource-group.service-cluster",
		name: "智能检索服务集群",
		matches: (item) => item.group === "business",
	},
	{
		id: "resource-group.infrastructure",
		name: "基础设施组件",
		matches: (item) => item.group === "middleware",
	},
	{
		id: "resource-group.observability",
		name: "监控组件",
		matches: (item) => item.group === "monitor",
	},
]

const targetOrder = [
	"search.web",
	"inference.web",
	"file.web",
	"file.worker",
	"file.consumer",
	"cad.web",
	"cad.batch_worker",
	"cad.query_worker",
	"cad.compare_worker",
	"middleware.postgres",
	"middleware.redis",
	"middleware.rabbitmq",
	"middleware.minio",
	"middleware.xxl_mysql",
	"middleware.xxl_admin",
	"aether",
]

function targetRank(item: I3DResourceTarget) {
	const normalizedID = item.id.replace(/^(local|release)\./, "")
	const index = targetOrder.indexOf(normalizedID)
	return index >= 0 ? index : targetOrder.length
}

function isAbnormal(item: I3DResourceTarget) {
	return item.status !== "up"
}

function toTargetRow(item: I3DResourceTarget): I3DResourceTreeRow {
	return {
		id: item.id,
		name: item.name,
		level: 1,
		kind: "target",
		target: item,
		cpu_percent: item.cpu_percent || 0,
		memory_bytes: item.memory_bytes || 0,
		memory_usage_bytes: item.memory_usage_bytes || 0,
		memory_rss_bytes: item.memory_rss_bytes || 0,
		memory_cache_bytes: item.memory_cache_bytes || 0,
		memory_anon_bytes: item.memory_anon_bytes || 0,
		memory_inactive_file_bytes: item.memory_inactive_file_bytes || 0,
		disk_read_bps: item.disk_read_bps || 0,
		disk_write_bps: item.disk_write_bps || 0,
		network_rx_bps: item.network_rx_bps || 0,
		network_tx_bps: item.network_tx_bps || 0,
		unit_count: item.unit_count || 0,
		pids: item.pids || 0,
		threads: item.threads || 0,
		abnormal_count: isAbnormal(item) ? 1 : 0,
	}
}

function toGroupRow(group: I3DResourceTreeGroup, children: I3DResourceTreeRow[]): I3DResourceTreeRow {
	return {
		id: group.id,
		name: group.name,
		level: 0,
		kind: "group",
		cpu_percent: children.reduce((total, item) => total + item.cpu_percent, 0),
		memory_bytes: children.reduce((total, item) => total + item.memory_bytes, 0),
		memory_usage_bytes: children.reduce((total, item) => total + item.memory_usage_bytes, 0),
		memory_rss_bytes: children.reduce((total, item) => total + item.memory_rss_bytes, 0),
		memory_cache_bytes: children.reduce((total, item) => total + item.memory_cache_bytes, 0),
		memory_anon_bytes: children.reduce((total, item) => total + item.memory_anon_bytes, 0),
		memory_inactive_file_bytes: children.reduce((total, item) => total + item.memory_inactive_file_bytes, 0),
		disk_read_bps: children.reduce((total, item) => total + item.disk_read_bps, 0),
		disk_write_bps: children.reduce((total, item) => total + item.disk_write_bps, 0),
		network_rx_bps: children.reduce((total, item) => total + item.network_rx_bps, 0),
		network_tx_bps: children.reduce((total, item) => total + item.network_tx_bps, 0),
		unit_count: children.reduce((total, item) => total + item.unit_count, 0),
		pids: children.reduce((total, item) => total + item.pids, 0),
		threads: children.reduce((total, item) => total + item.threads, 0),
		abnormal_count: children.reduce((total, item) => total + item.abnormal_count, 0),
	}
}

export function buildI3DResourceTreeRows(items: I3DResourceTarget[]) {
	const rows: I3DResourceTreeRow[] = []
	const visibleItems = items.filter((item) => item.group !== "frontend")

	for (const group of groups) {
		const children = visibleItems
			.filter(group.matches)
			.sort((left, right) => targetRank(left) - targetRank(right))
			.map(toTargetRow)
		if (children.length === 0) continue
		rows.push(toGroupRow(group, children), ...children)
	}

	return rows
}

export function visibleI3DResourceTreeRows(rows: I3DResourceTreeRow[], collapsedGroupIDs: Set<string>) {
	const visibleRows: I3DResourceTreeRow[] = []
	let hideChildren = false

	for (const row of rows) {
		if (row.kind === "group") {
			hideChildren = collapsedGroupIDs.has(row.id)
			visibleRows.push(row)
			continue
		}
		if (!hideChildren) {
			visibleRows.push(row)
		}
	}

	return visibleRows
}

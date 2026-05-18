import { describe, expect, test } from "bun:test"
import type { I3DResourceTarget } from "./i3dResources"
import { buildI3DResourceTreeRows, visibleI3DResourceTreeRows } from "./i3dResourceTree"

const target = (item: Partial<I3DResourceTarget> & Pick<I3DResourceTarget, "id" | "name" | "group">): I3DResourceTarget => ({
	kind: "process",
	status: "up",
	health_status: "up",
	cpu_percent: 0,
	cpu_cores_used: 0,
	memory_bytes: 0,
	memory_percent: 0,
	disk_read_bps: 0,
	disk_write_bps: 0,
	network_rx_bps: 0,
	network_tx_bps: 0,
	gpu_memory_bytes: 0,
	unit_count: 1,
	uptime_seconds: 0,
	restart_count: 0,
	updated_at: "2026-05-17T00:00:00Z",
	...item,
})

describe("i3d resource tree rows", () => {
	test("groups service targets into a single tree table without frontend targets", () => {
		const rows = buildI3DResourceTreeRows([
			target({ id: "local.search.web", name: "检索服务", group: "business", cpu_percent: 1, memory_bytes: 100 }),
			target({ id: "local.middleware.redis", name: "Redis", group: "middleware", cpu_percent: 2, memory_bytes: 200 }),
			target({ id: "local.aether", name: "本地 Aether", group: "monitor", cpu_percent: 3, memory_bytes: 300 }),
			target({ id: "local.frontend.webviews", name: "前端 Web Views dev server", group: "frontend", cpu_percent: 99, memory_bytes: 999 }),
		])

		expect(rows.map((row) => row.id)).toEqual([
			"resource-group.service-cluster",
			"local.search.web",
			"resource-group.infrastructure",
			"local.middleware.redis",
			"resource-group.observability",
			"local.aether",
		])
		expect(rows.find((row) => row.id === "resource-group.service-cluster")?.name).toBe("智能检索服务集群")
		expect(rows.find((row) => row.id === "resource-group.infrastructure")?.name).toBe("基础设施组件")
		expect(rows.find((row) => row.id === "resource-group.observability")?.name).toBe("监控组件")
		expect(rows.some((row) => row.id.includes("frontend"))).toBe(false)
	})

	test("hides only child rows for collapsed parent groups", () => {
		const rows = buildI3DResourceTreeRows([
			target({ id: "local.search.web", name: "检索服务", group: "business" }),
			target({ id: "local.middleware.redis", name: "Redis", group: "middleware" }),
			target({ id: "local.aether", name: "本地 Aether", group: "monitor" }),
		])

		expect(visibleI3DResourceTreeRows(rows, new Set(["resource-group.infrastructure"])).map((row) => row.id)).toEqual([
			"resource-group.service-cluster",
			"local.search.web",
			"resource-group.infrastructure",
			"resource-group.observability",
			"local.aether",
		])
	})

	test("does not mark parent groups abnormal for running health status when target status is up", () => {
		const rows = buildI3DResourceTreeRows([
			target({ id: "local.middleware.redis", name: "Redis", group: "middleware", status: "up", health_status: "running" }),
		])

		expect(rows.find((row) => row.id === "resource-group.infrastructure")?.abnormal_count).toBe(0)
		expect(rows.find((row) => row.id === "local.middleware.redis")?.abnormal_count).toBe(0)
	})
})

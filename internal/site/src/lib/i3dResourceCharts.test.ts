import { describe, expect, test } from "bun:test"
import type { I3DResourceTimeseries } from "./i3dResources"
import { buildI3DResourceChartPoints, hasI3DResourceChartData } from "./i3dResourceCharts"

const timeseries: I3DResourceTimeseries = {
	environment: "local",
	items: [
		{
			timestamp: "2026-05-18T09:00:00+08:00",
			cpu_percent: 12.5,
			cpu_cores_used: 0.125,
			memory_bytes: 1024,
			disk_read_bps: 10,
			disk_write_bps: 20,
			network_rx_bps: 30,
			network_tx_bps: 40,
			gpu_memory_bytes: 256,
			groups: {
				business: { cpu_percent: 7.5, memory_bytes: 512 },
				middleware: { cpu_percent: 5, memory_bytes: 512 },
			},
			targets: {
				"local.inference.web": { name: "推理服务", cpu_percent: 7.5, memory_bytes: 512, gpu_memory_bytes: 256 },
				"local.middleware.redis": { name: "Redis", cpu_percent: 5, memory_bytes: 512, gpu_memory_bytes: 0 },
			},
		},
		{
			timestamp: "2026-05-18T09:00:05+08:00",
			cpu_percent: 20,
			cpu_cores_used: 0.2,
			memory_bytes: 2048,
			disk_read_bps: 100,
			disk_write_bps: 200,
			network_rx_bps: 300,
			network_tx_bps: 400,
			gpu_memory_bytes: 512,
			groups: {
				business: { cpu_percent: 15, memory_bytes: 1536 },
				monitor: { cpu_percent: 5, memory_bytes: 512 },
			},
			targets: {
				"local.inference.web": { name: "推理服务", cpu_percent: 15, memory_bytes: 1536, gpu_memory_bytes: 512 },
				"local.aether": { name: "本地 Aether", cpu_percent: 5, memory_bytes: 512, gpu_memory_bytes: 0 },
			},
		},
	],
}

describe("i3d resource charts", () => {
	test("normalizes timeseries into chart points with timestamps and grouped values", () => {
		const points = buildI3DResourceChartPoints(timeseries)

		expect(points).toHaveLength(2)
		expect(points[0].created).toBe(new Date("2026-05-18T09:00:00+08:00").getTime())
		expect(points[0].cpu_percent).toBe(12.5)
		expect(points[0].disk_total_bps).toBe(30)
		expect(points[0].network_total_bps).toBe(70)
		expect(points[0].business_cpu_percent).toBe(7.5)
		expect(points[1].middleware_memory_bytes).toBe(0)
		expect(points[1].monitor_memory_bytes).toBe(512)
		expect(points[0]["target_local_inference_web_gpu_memory_bytes"]).toBe(256)
		expect(points[1]["target_local_inference_web_gpu_memory_bytes"]).toBe(512)
		expect(points[1]["target_local_middleware_redis_cpu_percent"]).toBe(0)
	})

	test("reports empty chart data when there are fewer than two valid points", () => {
		expect(hasI3DResourceChartData({ environment: "local", items: [] })).toBe(false)
		expect(hasI3DResourceChartData({ environment: "local", items: [timeseries.items[0]] })).toBe(false)
		expect(hasI3DResourceChartData(timeseries)).toBe(true)
	})
})

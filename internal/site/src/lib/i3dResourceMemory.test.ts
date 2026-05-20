import { describe, expect, test } from "bun:test"
import {
	i3dResourceMemoryCacheBytes,
	i3dResourceMemoryPressureBytes,
	i3dResourceMemoryStatTotalBytes,
} from "./i3dResourceMemory"

describe("i3d resource memory metrics", () => {
	test("uses RSS as the primary memory pressure value", () => {
		const metrics = {
			memory_bytes: 2048,
			memory_usage_bytes: 4096,
			memory_rss_bytes: 512,
			memory_cache_bytes: 3000,
		}

		expect(i3dResourceMemoryPressureBytes(metrics)).toBe(512)
		expect(i3dResourceMemoryCacheBytes(metrics)).toBe(3000)
		expect(i3dResourceMemoryStatTotalBytes(metrics)).toBe(4096)
	})

	test("falls back to anon and working-set memory for older samples", () => {
		expect(i3dResourceMemoryPressureBytes({ memory_anon_bytes: 900, memory_bytes: 1200 })).toBe(900)
		expect(i3dResourceMemoryPressureBytes({ memory_bytes: 1200 })).toBe(1200)
		expect(i3dResourceMemoryStatTotalBytes({ memory_bytes: 1200 })).toBe(1200)
	})
})

import { describe, expect, test } from "bun:test"
import { formatI3DBytesPerSecond, formatI3DBytesValue } from "./i3dResourceFormatters"

describe("i3d resource formatters", () => {
	test("formats API byte values without treating them as megabytes", () => {
		expect(formatI3DBytesValue(507_645_952)).toBe("484.1 MB")
		expect(formatI3DBytesValue(12_258_045_952)).toBe("11.4 GB")
	})

	test("formats API bytes-per-second values once", () => {
		expect(formatI3DBytesPerSecond(84_377)).toBe("82.4 KB/s")
		expect(formatI3DBytesPerSecond(0)).toBe("0.00 B/s")
		expect(formatI3DBytesPerSecond(84_377)).not.toContain("/s/s")
	})
})

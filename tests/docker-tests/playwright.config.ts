import { defineConfig } from "@playwright/test"

export default defineConfig({
	testDir: ".",
	testMatch: /.*\.spec\.ts/,
	timeout: 180_000,
	fullyParallel: false,
	workers: 1,
	outputDir: "test-results",
	reporter: [
		["list"],
		["html", { outputFolder: "playwright-report", open: "never" }],
		["json", { outputFile: "test-results/results.json" }],
	],
	use: {
		actionTimeout: 30_000,
		navigationTimeout: 60_000,
		trace: "retain-on-failure",
		// 需要在 HTML 报告中看到截图/视频时，必须对通过用例也产出附件。
		// 注意：会显著增加 test-results 与 report 的体积。
		screenshot: "on",
		video: "on",
	},
})

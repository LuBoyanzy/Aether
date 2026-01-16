import { defineConfig } from "@playwright/test"

export default defineConfig({
	testDir: ".",
	testMatch: /.*\.spec\.ts/,
	timeout: 120_000,
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
		screenshot: "only-on-failure",
		video: "retain-on-failure",
	},
})


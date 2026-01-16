import { defineConfig } from "@playwright/test"

export default defineConfig({
	testDir: "../../tests/api-tests",
	testMatch: /.*\.spec\.ts/,
	timeout: 120_000,
})

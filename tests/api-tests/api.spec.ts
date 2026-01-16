import { expect, test } from "@playwright/test"
import {
	createCase,
	createCollection,
	deleteRecordIfExists,
	env,
	getSchedule,
	listRuns,
	login,
	runAll,
	runCase,
	updateSchedule,
} from "./support"

test.setTimeout(120_000)

test("API Tests API 覆盖", async ({ request }) => {
	const auth = await login(request)
	const caseOrigin = new URL(env.caseUrl).origin
	const collectionName = `Auto Collection ${Date.now()}`
	const caseName = `Auto Case ${Date.now()}`
	let collectionId = ""
	let caseId = ""
	let scheduleSnapshot: {
		enabled: boolean
		intervalMinutes: number
		alertEnabled: boolean
		alertOnRecover: boolean
		historyRetentionDays: number
	} | null = null

	try {
		const collection = await createCollection(request, auth.token, {
			name: collectionName,
			base_url: caseOrigin,
		})
		collectionId = collection.id

		const caseRecord = await createCase(request, auth.token, {
			collection: collectionId,
			name: caseName,
			url: env.caseUrl,
		})
		caseId = caseRecord.id

		const schedule = await getSchedule(request, auth.token)
		scheduleSnapshot = {
			enabled: schedule.enabled,
			intervalMinutes: schedule.intervalMinutes,
			alertEnabled: schedule.alertEnabled,
			alertOnRecover: schedule.alertOnRecover,
			historyRetentionDays: schedule.historyRetentionDays,
		}

		const updatedSchedule = await updateSchedule(request, auth.token, {
			...scheduleSnapshot,
			alertOnRecover: !scheduleSnapshot.alertOnRecover,
		})
		expect(updatedSchedule.alertOnRecover).toBe(!scheduleSnapshot.alertOnRecover)

		const runCaseResult = await runCase(request, auth.token, caseId)
		expect(runCaseResult.caseId).toBe(caseId)
		expect(runCaseResult.name).toBe(caseName)

		const runAllResult = await runAll(request, auth.token)
		expect(runAllResult.cases).toBeGreaterThanOrEqual(1)
		expect(runAllResult.results.some((item) => item.name === caseName)).toBeTruthy()

		const runs = await listRuns(request, auth.token, { caseId, collectionId })
		expect(runs.items.length).toBeGreaterThan(0)
	} finally {
		if (scheduleSnapshot) {
			await updateSchedule(request, auth.token, scheduleSnapshot)
		}
		await deleteRecordIfExists(request, auth.token, "api_test_cases", caseId)
		await deleteRecordIfExists(request, auth.token, "api_test_collections", collectionId)
	}
})

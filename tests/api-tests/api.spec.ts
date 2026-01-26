import { expect, test, type APIRequestContext } from "@playwright/test"
import {
	createCase,
	createCollection,
	deleteRecordIfExists,
	env,
	exportApiTests,
	getRecord,
	importApiTests,
	getSchedule,
	listRuns,
	login,
	runAll,
	runCase,
	runCollection,
	updateSchedule,
} from "./support"

test.setTimeout(120_000)

const createCollectionAndCase = async (request: APIRequestContext, token: string) => {
	const caseOrigin = new URL(env.caseUrl).origin
	const collectionName = `Auto Collection ${Date.now()}`
	const caseName = `Auto Case ${Date.now()}`
	const collection = await createCollection(request, token, {
		name: collectionName,
		base_url: caseOrigin,
	})
	const caseRecord = await createCase(request, token, {
		collection: collection.id,
		name: caseName,
		url: env.caseUrl,
	})
	return {
		collectionId: collection.id,
		caseId: caseRecord.id,
		collectionName,
		caseName,
	}
}

test("接口管理 API：合集/用例 创建与清理", async ({ request }) => {
	const auth = await login(request)
	let collectionId = ""
	let caseId = ""

	try {
		const created = await createCollectionAndCase(request, auth.token)
		collectionId = created.collectionId
		caseId = created.caseId

		expect(collectionId).toBeTruthy()
		expect(caseId).toBeTruthy()
	} finally {
		await deleteRecordIfExists(request, auth.token, "api_test_cases", caseId)
		await deleteRecordIfExists(request, auth.token, "api_test_collections", collectionId)
	}
})

test("接口管理 API：计划 读取/更新/恢复", async ({ request }) => {
	const auth = await login(request)
	let snapshot: {
		enabled: boolean
		intervalMinutes: number
		alertEnabled: boolean
		alertOnRecover: boolean
		historyRetentionDays: number
	} | null = null

	try {
		const schedule = await getSchedule(request, auth.token)
		snapshot = {
			enabled: schedule.enabled,
			intervalMinutes: schedule.intervalMinutes,
			alertEnabled: schedule.alertEnabled,
			alertOnRecover: schedule.alertOnRecover,
			historyRetentionDays: schedule.historyRetentionDays,
		}

		const updated = await updateSchedule(request, auth.token, {
			...snapshot,
			alertOnRecover: !snapshot.alertOnRecover,
		})
		expect(updated.alertOnRecover).toBe(!snapshot.alertOnRecover)
	} finally {
		if (snapshot) {
			await updateSchedule(request, auth.token, snapshot)
		}
	}
})

test("接口管理 API：导入/导出 合集+用例", async ({ request }) => {
	const auth = await login(request)
	let collectionId = ""
	let caseId = ""
	let collectionName = ""
	let caseName = ""

	try {
		const created = await createCollectionAndCase(request, auth.token)
		collectionId = created.collectionId
		caseId = created.caseId
		collectionName = created.collectionName
		caseName = created.caseName

		const exported = await exportApiTests(request, auth.token)
		const exportedCollection = exported.collections.find(
			(item) => typeof item.name === "string" && item.name === collectionName
		) as Record<string, unknown> | undefined
		const exportedCase = exported.cases.find(
			(item) =>
				typeof item.name === "string" &&
				item.name === caseName &&
				typeof item.collection === "string" &&
				item.collection === collectionName
		) as Record<string, unknown> | undefined

		expect(exportedCollection).toBeTruthy()
		expect(exportedCase).toBeTruthy()
		if (!exportedCollection || !exportedCase) {
			throw new Error("Exported payload missing target collection or case.")
		}

		const nextTimeout = Number(exportedCase.timeout_ms ?? 15000) + 1000
		const skipPayload = {
			mode: "skip" as const,
			data: {
				collections: [
					{
						...exportedCollection,
						description: "skip description update",
					},
				],
				cases: [
					{
						...exportedCase,
						timeout_ms: nextTimeout,
					},
				],
			},
		}
		await importApiTests(request, auth.token, skipPayload)

		const collectionRecord = await getRecord<{ description: string }>(
			request,
			auth.token,
			"api_test_collections",
			collectionId
		)
		const caseRecord = await getRecord<{ timeout_ms: number }>(
			request,
			auth.token,
			"api_test_cases",
			caseId
		)
		expect(collectionRecord.description).not.toBe("skip description update")
		expect(caseRecord.timeout_ms).not.toBe(nextTimeout)

		const overwritePayload = {
			mode: "overwrite" as const,
			data: skipPayload.data,
		}
		await importApiTests(request, auth.token, overwritePayload)

		const updatedCollection = await getRecord<{ description: string }>(
			request,
			auth.token,
			"api_test_collections",
			collectionId
		)
		const updatedCase = await getRecord<{ timeout_ms: number }>(
			request,
			auth.token,
			"api_test_cases",
			caseId
		)
		expect(updatedCollection.description).toBe("skip description update")
		expect(updatedCase.timeout_ms).toBe(nextTimeout)
	} finally {
		await deleteRecordIfExists(request, auth.token, "api_test_cases", caseId)
		await deleteRecordIfExists(request, auth.token, "api_test_collections", collectionId)
	}
})

test("接口管理 API：执行与历史（runCase/runAll/runs）", async ({ request }) => {
	const auth = await login(request)
	let collectionId = ""
	let caseId = ""
	let caseName = ""

	try {
		const created = await createCollectionAndCase(request, auth.token)
		collectionId = created.collectionId
		caseId = created.caseId
		caseName = created.caseName

		const runCaseResult = await runCase(request, auth.token, caseId)
		expect(runCaseResult.caseId).toBe(caseId)
		expect(runCaseResult.name).toBe(caseName)

		const runCollectionResult = await runCollection(request, auth.token, collectionId)
		expect(runCollectionResult.collectionId).toBe(collectionId)
		expect(runCollectionResult.cases).toBeGreaterThanOrEqual(1)
		expect(runCollectionResult.results.some((item) => item.name === caseName)).toBeTruthy()

		const runAllResult = await runAll(request, auth.token)
		expect(runAllResult.cases).toBeGreaterThanOrEqual(1)
		expect(runAllResult.results.some((item) => item.name === caseName)).toBeTruthy()

		const runs = await listRuns(request, auth.token, { caseId, collectionId })
		expect(runs.items.length).toBeGreaterThan(0)
		expect(runs.items.some((item) => item.caseId === caseId)).toBeTruthy()
	} finally {
		await deleteRecordIfExists(request, auth.token, "api_test_cases", caseId)
		await deleteRecordIfExists(request, auth.token, "api_test_collections", collectionId)
	}
})

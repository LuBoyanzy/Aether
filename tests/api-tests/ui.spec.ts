import { expect, test, type APIRequestContext, type Locator, type Page } from "@playwright/test"
import {
	createCase,
	createCollection,
	deleteRecordIfExists,
	env,
	findRecordIdByName,
	login,
	runCase,
	setAuthStorage,
} from "./support"

const fillInputByText = async (scope: Locator, label: string, value: string) => {
	const field = scope.getByText(label, { exact: true }).locator("..").getByRole("textbox")
	await field.fill(value)
}

const removeRowByName = async (page: Page, name: string) => {
	const row = page.locator("tr", { hasText: name }).first()
	await expect(row).toBeVisible()
	await row.getByRole("button", { name: "删除" }).click()
	await expect(page.locator("tr", { hasText: name })).toHaveCount(0)
}

test.setTimeout(120_000)

const openApiTestsPage = async (page: Page, request: APIRequestContext) => {
	page.on("dialog", (dialog) => dialog.accept())
	const auth = await login(request)
	await page.context().clearCookies()
	await setAuthStorage(page, auth)
	await page.goto(env.apiTestsUrl, { waitUntil: "domcontentloaded" })
	return auth
}

test("接口管理 UI：页面基础与 Tab 文案", async ({ page, request }) => {
	await openApiTestsPage(page, request)

	await expect(page.getByRole("heading", { name: "接口管理" })).toBeVisible()
	await expect(page.getByText("管理并运行接口测试。")).toBeVisible()

	await expect(page.getByRole("button", { name: "刷新" })).toBeVisible()
	await expect(page.getByRole("button", { name: "全部运行" })).toBeVisible()

	await expect(page.getByRole("tab", { name: "合集" })).toBeVisible()
	await expect(page.getByRole("tab", { name: "用例" })).toBeVisible()
	await expect(page.getByRole("tab", { name: "计划" })).toBeVisible()
	await expect(page.getByRole("tab", { name: "历史记录" })).toBeVisible()
})

test("接口管理 UI：合集 新建/编辑/删除", async ({ page, request }) => {
	const auth = await openApiTestsPage(page, request)
	const caseOrigin = new URL(env.caseUrl).origin
	let collectionName = `Auto Collection UI ${Date.now()}`
	let originalCollectionName = collectionName

	try {
		await page.getByRole("tab", { name: "合集" }).click()
		await page.getByRole("button", { name: "新建合集" }).click()
		const dialog = page.getByRole("dialog", { name: "新建合集" })
		await fillInputByText(dialog, "名称", collectionName)
		await fillInputByText(dialog, "基础 URL", caseOrigin)
		await dialog.getByRole("button", { name: "保存" }).click()
		await expect(page.locator("tr", { hasText: collectionName }).first()).toBeVisible()

		const row = page.locator("tr", { hasText: collectionName }).first()
		await row.getByRole("button", { name: "编辑" }).click()
		const editDialog = page.getByRole("dialog", { name: "编辑合集" })
		collectionName = `${collectionName} Updated`
		await fillInputByText(editDialog, "名称", collectionName)
		await editDialog.getByRole("button", { name: "保存" }).click()
		await expect(page.locator("tr", { hasText: collectionName }).first()).toBeVisible()

		await removeRowByName(page, collectionName)
	} finally {
		const maybeIds = [
			await findRecordIdByName(request, auth.token, "api_test_collections", collectionName),
			await findRecordIdByName(request, auth.token, "api_test_collections", originalCollectionName),
		].filter(Boolean)
		for (const id of maybeIds) {
			await deleteRecordIfExists(request, auth.token, "api_test_collections", id)
		}
	}
})

test("接口管理 UI：用例 新建/编辑/删除", async ({ page, request }) => {
	const auth = await login(request)
	const caseOrigin = new URL(env.caseUrl).origin
	const collectionName = `Auto Collection ${Date.now()}`
	let collectionId = ""
	let caseName = `Auto Case UI ${Date.now()}`
	let updatedCaseName = ""

	try {
		const collection = await createCollection(request, auth.token, { name: collectionName, base_url: caseOrigin })
		collectionId = collection.id

		page.on("dialog", (dialog) => dialog.accept())
		await page.context().clearCookies()
		await setAuthStorage(page, auth)
		await page.goto(env.apiTestsUrl, { waitUntil: "domcontentloaded" })

		await page.getByRole("tab", { name: "用例" }).click()
		const panel = page.getByRole("tabpanel", { name: "用例" })
		const filter = panel.getByRole("combobox").first()
		await filter.click()
		await page.getByRole("option", { name: collectionName }).click()

		await page.getByRole("button", { name: "新建用例" }).click()
		const dialog = page.getByRole("dialog", { name: "新建用例" })
		await fillInputByText(dialog, "名称", caseName)
		await fillInputByText(dialog, "URL", env.caseUrl)
		await dialog.getByRole("button", { name: "保存" }).click()
		await expect(page.locator("tr", { hasText: caseName }).first()).toBeVisible()

		const row = page.locator("tr", { hasText: caseName }).first()
		await row.getByRole("button", { name: "编辑" }).click()
		const editDialog = page.getByRole("dialog", { name: "编辑用例" })
		updatedCaseName = `${caseName} Updated`
		await fillInputByText(editDialog, "名称", updatedCaseName)
		await editDialog.getByRole("button", { name: "保存" }).click()
		await expect(page.locator("tr", { hasText: updatedCaseName }).first()).toBeVisible()

		await removeRowByName(page, updatedCaseName)
	} finally {
		const maybeCaseIds = []
		if (updatedCaseName) {
			maybeCaseIds.push(await findRecordIdByName(request, auth.token, "api_test_cases", updatedCaseName))
		}
		if (caseName) {
			maybeCaseIds.push(await findRecordIdByName(request, auth.token, "api_test_cases", caseName))
		}
		for (const id of maybeCaseIds.filter(Boolean)) {
			await deleteRecordIfExists(request, auth.token, "api_test_cases", id)
		}
		await deleteRecordIfExists(request, auth.token, "api_test_collections", collectionId)
	}
})

test("接口管理 UI：计划 保存提示", async ({ page, request }) => {
	await openApiTestsPage(page, request)
	await page.getByRole("tab", { name: "计划" }).click()
	const schedulePanel = page.getByRole("tabpanel", { name: "计划" })
	await expect(schedulePanel.getByText("启用定时", { exact: true })).toBeVisible()
	await schedulePanel.getByRole("button", { name: "保存" }).click()
	await expect(page.getByText("计划已保存", { exact: true })).toBeVisible()
})

test("接口管理 UI：历史记录 列表展示", async ({ page, request }) => {
	const auth = await login(request)
	const caseOrigin = new URL(env.caseUrl).origin
	const collectionName = `Auto Collection ${Date.now()}`
	const caseName = `Auto Case ${Date.now()}`
	let collectionId = ""
	let caseId = ""

	try {
		const collection = await createCollection(request, auth.token, { name: collectionName, base_url: caseOrigin })
		collectionId = collection.id

		// 直接通过 API 触发一次执行，确保历史记录面板有数据可展示。
		const createdCase = await createCase(request, auth.token, {
			collection: collectionId,
			name: caseName,
			url: env.caseUrl,
		})
		caseId = createdCase.id
		await runCase(request, auth.token, caseId)

		page.on("dialog", (dialog) => dialog.accept())
		await page.context().clearCookies()
		await setAuthStorage(page, auth)
		await page.goto(env.apiTestsUrl, { waitUntil: "domcontentloaded" })

		await page.getByRole("tab", { name: "历史记录" }).click()
		await expect(page.getByRole("heading", { name: "历史记录" })).toBeVisible()

		const panel = page.getByRole("tabpanel", { name: "历史记录" })
		const filters = panel.getByRole("combobox")
		await expect(filters).toHaveCount(2)
		await filters.nth(0).click()
		await page.getByRole("option", { name: collectionName }).click()
		await filters.nth(1).click()
		await page.getByRole("option", { name: caseName }).click()

		await expect(page.locator("tr", { hasText: caseName }).first()).toBeVisible()
	} finally {
		await deleteRecordIfExists(request, auth.token, "api_test_cases", caseId)
		await deleteRecordIfExists(request, auth.token, "api_test_collections", collectionId)
	}
})

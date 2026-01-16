import { expect, test, type Locator, type Page } from "@playwright/test"
import { createCase, createCollection, deleteRecordIfExists, env, login, setAuthStorage } from "./support"

const fillInputByText = async (scope: Locator, label: string, value: string) => {
	const field = scope.getByText(label, { exact: true }).locator("..").getByRole("textbox")
	await field.fill(value)
}

const removeRowByName = async (page: Page, name: string) => {
	const row = page.locator("tr", { hasText: name }).first()
	await expect(row).toBeVisible()
	await row.getByRole("button", { name: "删除" }).click()
	await expect(page.getByText(name, { exact: true })).toHaveCount(0)
}

test.setTimeout(120_000)

test("API Tests UI 覆盖", async ({ page, request }) => {
	const auth = await login(request)
	const caseOrigin = new URL(env.caseUrl).origin
	let collectionName = `Auto Collection ${Date.now()}`
	let caseName = `Auto Case ${Date.now()}`
	let collectionId = ""
	let caseId = ""

	try {
		page.on("dialog", (dialog) => dialog.accept())
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

		await page.context().clearCookies()
		await setAuthStorage(page, auth)
		await page.goto(env.apiTestsUrl, { waitUntil: "domcontentloaded" })

		await expect(page.getByRole("heading", { name: "接口管理" })).toBeVisible()
		await expect(page.getByText("管理并运行接口测试。")).toBeVisible()

		await expect(page.getByRole("button", { name: "刷新" })).toBeVisible()
		await expect(page.getByRole("button", { name: "全部运行" })).toBeVisible()

		await expect(page.getByRole("tab", { name: "合集" })).toBeVisible()
		await expect(page.getByRole("tab", { name: "用例" })).toBeVisible()
		await expect(page.getByRole("tab", { name: "计划" })).toBeVisible()
		await expect(page.getByRole("tab", { name: "历史记录" })).toBeVisible()

		await page.getByRole("tab", { name: "合集" }).click()
		const collectionRow = page.locator("tr", { hasText: collectionName }).first()
		await expect(collectionRow).toBeVisible()

		await collectionRow.getByRole("button", { name: "编辑" }).click()
		const collectionDialog = page.getByRole("dialog", { name: "编辑合集" })
		collectionName = `${collectionName} Updated`
		await fillInputByText(collectionDialog, "名称", collectionName)
		await collectionDialog.getByRole("button", { name: "保存" }).click()
		await expect(page.getByText(collectionName, { exact: true })).toBeVisible()

		await page.getByRole("tab", { name: "用例" }).click()
		const casesPanel = page.getByRole("tabpanel", { name: "用例" })
		const casesFilter = casesPanel.getByRole("combobox").first()
		await casesFilter.click()
		await page.getByRole("option", { name: "全部合集" }).click()

		const caseRow = page.locator("tr", { hasText: caseName }).first()
		await expect(caseRow).toBeVisible()
		await caseRow.getByRole("button", { name: "编辑" }).click()
		const caseDialog = page.getByRole("dialog", { name: "编辑用例" })
		caseName = `${caseName} Updated`
		await fillInputByText(caseDialog, "名称", caseName)
		await caseDialog.getByRole("button", { name: "保存" }).click()
		await expect(page.getByText(caseName, { exact: true })).toBeVisible()

		await page.getByRole("tab", { name: "计划" }).click()
		const schedulePanel = page.getByRole("tabpanel", { name: "计划" })
		await expect(schedulePanel.getByText("启用定时", { exact: true })).toBeVisible()
		await expect(schedulePanel.getByText("历史保留（天）")).toBeVisible()
		await schedulePanel.getByRole("button", { name: "保存" }).click()
		await expect(page.getByText("计划已保存", { exact: true })).toBeVisible()

		await page.getByRole("tab", { name: "历史记录" }).click()
		await expect(page.getByRole("heading", { name: "历史记录" })).toBeVisible()
		const historyPanel = page.getByRole("tabpanel", { name: "历史记录" })
		await expect(historyPanel.getByRole("combobox")).toHaveCount(2)
		await expect(historyPanel.getByText("状态")).toBeVisible()
		await expect(historyPanel.getByRole("columnheader", { name: "用例" })).toBeVisible()

		await page.getByRole("tab", { name: "用例" }).click()
		await removeRowByName(page, caseName)

		await page.getByRole("tab", { name: "合集" }).click()
		await removeRowByName(page, collectionName)
	} finally {
		await deleteRecordIfExists(request, auth.token, "api_test_cases", caseId)
		await deleteRecordIfExists(request, auth.token, "api_test_collections", collectionId)
	}
})

import { expect, test, type APIRequestContext, type Locator, type Page } from "@playwright/test"
import { authHeaders, buildApiUrl, env, getSystemRecord, login, setAuthStorage } from "./support"

test.setTimeout(180_000)

const openDockerPage = async (page: Page, request: APIRequestContext) => {
	const auth = await login(request)
	await page.context().clearCookies()
	await setAuthStorage(page, auth, env.systemId)
	await page.goto(env.dockerUrl, { waitUntil: "domcontentloaded" })
	await expect(page.getByRole("heading", { name: "Docker" })).toBeVisible()

	// 确保系统已选中（避免默认选到其他系统导致接口失败）
	const system = await getSystemRecord(request, auth.token, env.systemId)
	await page.locator("#docker-system-select").click()
	await page.getByRole("option", { name: system.name }).click()

	return { auth, system }
}

const fillByLabel = async (scope: Locator, label: string, value: string) => {
	const field = scope.getByLabel(label, { exact: true })
	await expect(field).toBeVisible()
	await field.fill(value)
}

const clickRowAction = async (page: Page, row: Locator, actionName: string) => {
	// 行内操作按钮无 aria-label，直接点击第一个按钮以展开菜单
	await row.locator("button").first().click()
	await page.getByRole("menuitem", { name: actionName }).click()
}

test("Docker UI：页面入口与 11 个 Tab 可见", async ({ page, request }) => {
	await openDockerPage(page, request)

	const tabs = [
		"概览",
		"容器",
		"编排",
		"镜像",
		"网络",
		"存储卷",
		"仓库",
		"编排模板",
		"配置",
		"服务配置",
		"数据清理",
	]
	for (const name of tabs) {
		// Radix Tabs 的 name 采用“包含匹配”，例如“编排”会匹配到“编排模板”，这里强制精确匹配避免 strict mode 冲突。
		await expect(page.getByRole("tab", { name, exact: true })).toBeVisible()
	}
})

test("Docker UI：概览面板基础展示", async ({ page, request }) => {
	await openDockerPage(page, request)

	await page.getByRole("tab", { name: "概览", exact: true }).click()
	await expect(page.getByRole("heading", { name: "概览" })).toBeVisible()
	await expect(page.getByRole("button", { name: "刷新" })).toBeVisible()
	await expect(page.getByText("引擎详情")).toBeVisible()
})

test("Docker UI：容器 关注规则 新增/删除", async ({ page, request }) => {
	const { auth } = await openDockerPage(page, request)

	await page.getByRole("tab", { name: "容器", exact: true }).click()

	// 打开关注规则对话框
	const focusBtn = page.getByRole("button", { name: /关注规则/ })
	await expect(focusBtn).toBeVisible()
	await focusBtn.click()

	const dialog = page.getByRole("dialog", { name: /关注规则/ })
	await expect(dialog).toBeVisible()

	// 新增规则：容器名称
	const ruleValue = `pw-container-${Date.now()}`
	await dialog.locator("#docker-focus-type").click()
	await page.getByRole("option", { name: "容器名称" }).click()
	await fillByLabel(dialog, "容器名称", ruleValue)
	await dialog.getByRole("button", { name: "添加规则" }).click()

	// 表格中应出现刚刚创建的规则（按 value 定位，避免同名 cell 造成 strict mode 冲突）
	const createdRow = dialog.locator("tbody tr", { hasText: ruleValue }).first()
	await expect(createdRow).toBeVisible()
	await expect(createdRow.getByRole("cell", { name: "容器名称" }).first()).toBeVisible()

	// 删除刚刚创建的规则（按 value 定位）
	await clickRowAction(page, createdRow, "删除")
	const confirm = page.getByRole("alertdialog")
	await expect(confirm).toBeVisible()
	await confirm.getByRole("button", { name: "继续", exact: true }).click()
	await expect(dialog.getByRole("cell", { name: ruleValue })).toHaveCount(0)

	// 兜底清理：通过 API 删除残留（按 system 过滤）
	const listUrl = new URL(buildApiUrl("/api/collections/docker_focus_services/records"))
	listUrl.searchParams.set("perPage", "50")
	listUrl.searchParams.set("page", "1")
	listUrl.searchParams.set("filter", `system=${JSON.stringify(env.systemId)}`)
	const list = await request.get(listUrl.toString(), {
		headers: authHeaders(auth.token),
	})
	if (list.ok()) {
		const data = (await list.json()) as { items?: Array<{ id: string; match_type: string; value: string }> }
		for (const item of data.items || []) {
			if (typeof item.value === "string" && item.value.startsWith("pw-container-")) {
				await request.delete(buildApiUrl(`/api/collections/docker_focus_services/records/${item.id}`), {
					headers: authHeaders(auth.token),
				})
			}
		}
	}
})

test("Docker UI：仓库 新建/编辑/删除", async ({ page, request }) => {
	const { auth } = await openDockerPage(page, request)

	await page.getByRole("tab", { name: "仓库", exact: true }).click()
	await expect(page.getByRole("heading", { name: "仓库" })).toBeVisible()

	const name = `PW Registry UI ${Date.now()}`
	const updatedName = `${name} Updated`

	let createdId = ""

	try {
		await page.getByRole("button", { name: "创建仓库" }).click()
		const dialog = page.getByRole("dialog", { name: "创建仓库" })
		await fillByLabel(dialog, "名称", name)
		await fillByLabel(dialog, "服务器", "localhost")
		await dialog.getByRole("button", { name: "创建", exact: true }).click()
		await expect(page.locator("tr", { hasText: name }).first()).toBeVisible()

		// 编辑
		const row = page.locator("tr", { hasText: name }).first()
		await clickRowAction(page, row, "编辑")
		const editDialog = page.getByRole("dialog", { name: "编辑仓库" })
		await fillByLabel(editDialog, "名称", updatedName)
		await editDialog.getByRole("button", { name: "保存" }).click()
		await expect(page.locator("tr", { hasText: updatedName }).first()).toBeVisible()

		// 删除
		const updatedRow = page.locator("tr", { hasText: updatedName }).first()
		await clickRowAction(page, updatedRow, "删除")
		const confirm = page.getByRole("alertdialog")
		await expect(confirm).toBeVisible()
		await confirm.getByRole("button", { name: "继续", exact: true }).click()
		await expect(page.locator("tr", { hasText: updatedName })).toHaveCount(0)
	} finally {
		// API 兜底清理
		const listResp = await request.get(buildApiUrl("/api/aether/docker/registries"), {
			headers: authHeaders(auth.token),
		})
		if (listResp.ok()) {
			const list = (await listResp.json()) as { items?: Array<{ id: string; name: string }> }
			createdId = (list.items || []).find((item) => item.name === name || item.name === updatedName)?.id || ""
		}
		if (createdId) {
			const deleteUrl = new URL(buildApiUrl("/api/aether/docker/registries/delete"))
			deleteUrl.searchParams.set("id", createdId)
			await request.post(deleteUrl.toString(), {
				headers: authHeaders(auth.token),
			})
		}
	}
})

test("Docker UI：编排模板 新建/编辑/删除", async ({ page, request }) => {
	const { auth } = await openDockerPage(page, request)

	await page.getByRole("tab", { name: "编排模板", exact: true }).click()
	await expect(page.getByRole("heading", { name: "编排模板" })).toBeVisible()

	const name = `PW Template UI ${Date.now()}`
	const updatedName = `${name} Updated`
	const content = "services:\n  app:\n    image: nginx:alpine\n"

	let createdId = ""

	try {
		await page.getByRole("button", { name: "创建模板" }).click()
		const dialog = page.getByRole("dialog", { name: "创建模板" })
		await fillByLabel(dialog, "名称", name)
		await fillByLabel(dialog, "编排内容", content)
		await dialog.getByRole("button", { name: "创建", exact: true }).click()
		await expect(page.locator("tr", { hasText: name }).first()).toBeVisible()

		const row = page.locator("tr", { hasText: name }).first()
		await clickRowAction(page, row, "编辑")
		const editDialog = page.getByRole("dialog", { name: "编辑模板" })
		await fillByLabel(editDialog, "名称", updatedName)
		await editDialog.getByRole("button", { name: "保存" }).click()
		await expect(page.locator("tr", { hasText: updatedName }).first()).toBeVisible()

		const updatedRow = page.locator("tr", { hasText: updatedName }).first()
		await clickRowAction(page, updatedRow, "删除")
		const confirm = page.getByRole("alertdialog")
		await expect(confirm).toBeVisible()
		await confirm.getByRole("button", { name: "继续", exact: true }).click()
		await expect(page.locator("tr", { hasText: updatedName })).toHaveCount(0)
	} finally {
		const listResp = await request.get(buildApiUrl("/api/aether/docker/compose-templates"), {
			headers: authHeaders(auth.token),
		})
		if (listResp.ok()) {
			const list = (await listResp.json()) as { items?: Array<{ id: string; name: string }> }
			createdId = (list.items || []).find((item) => item.name === name || item.name === updatedName)?.id || ""
		}
		if (createdId) {
			const deleteUrl = new URL(buildApiUrl("/api/aether/docker/compose-templates/delete"))
			deleteUrl.searchParams.set("id", createdId)
			await request.post(deleteUrl.toString(), {
				headers: authHeaders(auth.token),
			})
		}
	}
})

test("Docker UI：服务配置 新建/编辑/删除（依赖 system.host）", async ({ page, request }) => {
	const { auth, system } = await openDockerPage(page, request)
	if (!system.host) {
		test.skip(true, "当前 system 缺少 host 字段，无法通过 UI 组合服务 URL")
	}

	await page.getByRole("tab", { name: "服务配置", exact: true }).click()
	await expect(page.getByRole("heading", { name: "服务配置" })).toBeVisible()

	const name = `PW Service UI ${Date.now()}`
	const updatedName = `${name} Updated`
	let createdId = ""

	try {
		await page.getByRole("button", { name: "新增服务" }).click()
		const dialog = page.getByRole("dialog", { name: "新增服务" })
		await fillByLabel(dialog, "服务名称", name)
		await dialog.getByPlaceholder("端口").fill("1")
		await fillByLabel(dialog, "接口 URL", "/")
		await fillByLabel(dialog, "X-Config-Token", "pw-token")
		await dialog.getByRole("button", { name: "创建" }).click()

		const card = page.locator('[role="button"]', { hasText: name }).first()
		await expect(card).toBeVisible()

		await clickRowAction(page, card, "编辑")
		const editDialog = page.getByRole("dialog", { name: "编辑服务" })
		await fillByLabel(editDialog, "服务名称", updatedName)
		// 该面板会把根路径 "/" 解析为空字符串，但保存时又要求必填；这里显式填回根路径以保证稳定回归。
		await fillByLabel(editDialog, "接口 URL", "/")
		await editDialog.getByRole("button", { name: "保存" }).click()
		await expect(editDialog).toBeHidden({ timeout: 30_000 })
		await expect(page.locator('[role="button"]', { hasText: updatedName }).first()).toBeVisible()

		const updatedCard = page.locator('[role="button"]', { hasText: updatedName }).first()
		await clickRowAction(page, updatedCard, "删除")
		const confirm = page.getByRole("alertdialog")
		await expect(confirm).toBeVisible()
		await confirm.getByRole("button", { name: "删除", exact: true }).click()
		await expect(page.locator('[role="button"]', { hasText: updatedName })).toHaveCount(0)
	} finally {
		const listUrl = new URL(buildApiUrl("/api/aether/docker/service-configs"))
		listUrl.searchParams.set("system", env.systemId)
		const listResp = await request.get(listUrl.toString(), { headers: authHeaders(auth.token) })
		if (listResp.ok()) {
			const list = (await listResp.json()) as { items?: Array<{ id: string; name: string }> }
			createdId = (list.items || []).find((item) => item.name === name || item.name === updatedName)?.id || ""
		}
		if (createdId) {
			const deleteUrl = new URL(buildApiUrl("/api/aether/docker/service-configs/delete"))
			deleteUrl.searchParams.set("id", createdId)
			await request.post(deleteUrl.toString(), { headers: authHeaders(auth.token) })
		}
	}
})

test("Docker UI：数据清理 面板基础展示（不执行清理）", async ({ page, request }) => {
	await openDockerPage(page, request)

	await page.getByRole("tab", { name: "数据清理", exact: true }).click()
	await expect(page.getByRole("heading", { name: "数据清理" })).toBeVisible()
	await expect(page.getByRole("button", { name: "刷新配置" })).toBeVisible()
	await expect(page.getByRole("button", { name: "保存配置" })).toBeVisible()
	await expect(page.getByRole("button", { name: "开始清理" })).toBeVisible()
	await expect(page.getByRole("heading", { name: "MySQL", exact: true })).toBeVisible()
	await expect(page.getByRole("heading", { name: "Redis", exact: true })).toBeVisible()
	await expect(page.getByRole("heading", { name: "MinIO", exact: true })).toBeVisible()
	await expect(page.getByRole("heading", { name: "Elasticsearch", exact: true })).toBeVisible()
})

test("Docker UI：编排/镜像/网络/存储卷/配置 Tab 基础展示（不执行写操作）", async ({ page, request }) => {
	await openDockerPage(page, request)

	// 编排
	await page.getByRole("tab", { name: "编排", exact: true }).click()
	await expect(page.getByRole("heading", { name: "编排" })).toBeVisible()
	await expect(page.getByRole("button", { name: "创建编排" })).toBeVisible()
	await expect(page.getByRole("button", { name: "刷新" })).toBeVisible()

	// 镜像
	await page.getByRole("tab", { name: "镜像", exact: true }).click()
	await expect(page.getByRole("heading", { name: "镜像" })).toBeVisible()
	await expect(page.getByRole("button", { name: "拉取镜像" })).toBeVisible()
	await expect(page.getByRole("button", { name: "推送镜像" })).toBeVisible()

	// 网络
	await page.getByRole("tab", { name: "网络", exact: true }).click()
	await expect(page.getByRole("heading", { name: "网络" })).toBeVisible()
	await expect(page.getByRole("button", { name: "创建网络" })).toBeVisible()

	// 存储卷
	await page.getByRole("tab", { name: "存储卷", exact: true }).click()
	await expect(page.getByRole("heading", { name: "存储卷" })).toBeVisible()
	await expect(page.getByRole("button", { name: "创建存储卷" })).toBeVisible()

	// 配置
	await page.getByRole("tab", { name: "配置", exact: true }).click()
	await expect(page.getByRole("heading", { name: "配置", exact: true })).toBeVisible()
	await expect(page.getByRole("button", { name: "保存" })).toBeVisible()
})

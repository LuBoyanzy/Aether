import fs from "node:fs"

const readJson = (fileUrl) => {
	const content = fs.readFileSync(fileUrl, "utf8")
	return JSON.parse(content)
}

const formatMs = (ms) => {
	if (ms < 1000) return `${ms}ms`
	const s = ms / 1000
	if (s < 60) return `${s.toFixed(1)}s`
	const m = Math.floor(s / 60)
	const rem = s % 60
	return `${m}m${rem.toFixed(0)}s`
}

const collectTests = (suite, fileTitle, out) => {
	if (suite.specs) {
		for (const spec of suite.specs) {
			for (const testRun of spec.tests || []) {
				const results = testRun.results || []
				const last = results[results.length - 1]
				out.push({
					file: fileTitle || suite.file || suite.title || "unknown",
					title: spec.title || "unknown",
					ok: spec.ok === true,
					status: last?.status || "unknown",
					duration: typeof last?.duration === "number" ? last.duration : 0,
					startTime: last?.startTime || "",
					location: {
						file: spec.file || suite.file || "",
						line: spec.line ?? 0,
						column: spec.column ?? 0,
					},
					errors: (last?.errors || []).map((e) => e?.message || "").filter(Boolean),
					attachments: (last?.attachments || []).map((a) => ({
						name: a?.name || "",
						contentType: a?.contentType || "",
						path: a?.path || "",
					})),
				})
			}
		}
	}
	if (suite.suites) {
		for (const child of suite.suites) {
			collectTests(child, fileTitle, out)
		}
	}
}

const main = () => {
	const resultsUrl = new URL("../test-results/results.json", import.meta.url)
	if (!fs.existsSync(resultsUrl)) {
		console.error("未找到 Playwright JSON 报告：tests/api-tests/test-results/results.json")
		console.error("请先执行：cd tests && npm run test:api-tests")
		process.exit(1)
	}

	const data = readJson(resultsUrl)
	const tests = []
	for (const suite of data.suites || []) {
		collectTests(suite, suite.title || suite.file, tests)
	}

	// 功能覆盖矩阵：人工维护（必须与用例标题保持一致），用于把“功能”与“用例结果”对应起来。
	const testCatalog = {
		"接口管理 UI：页面基础与 Tab 文案": ["页面标题与描述", "顶部操作按钮（刷新/全部运行）", "Tab：合集/用例/计划/历史记录"],
		"接口管理 UI：合集 新建/编辑/删除": ["新建合集（名称/基础 URL）", "编辑合集（修改名称）", "删除合集（UI 操作）"],
		"接口管理 UI：用例 新建/编辑/删除": ["新建用例（选择合集/名称/URL）", "编辑用例（修改名称）", "删除用例（UI 操作）"],
		"接口管理 UI：计划 保存提示": ["计划页面基本字段可见", "保存计划并提示成功 Toast"],
		"接口管理 UI：历史记录 列表展示": ["通过 API 触发一次执行产生历史", "历史记录筛选（合集/用例）", "历史记录列表展示用例名称"],
		"接口管理 API：合集/用例 创建与清理": ["通过 API 创建合集", "通过 API 创建用例", "测试结束清理数据"],
		"接口管理 API：计划 读取/更新/恢复": ["读取计划配置", "更新计划字段（并验证返回）", "恢复原配置（避免影响其他用例）"],
		"接口管理 API：执行与历史（runCase/runAll/runs）": ["执行单用例 run-case", "执行合集 run-collection", "执行全部 run-all", "查询执行历史 runs"],
	}

	const featureMatrix = [
		{
			feature: "页面入口与 Tab（合集/用例/计划/历史记录）",
			testTitles: ["接口管理 UI：页面基础与 Tab 文案"],
		},
		{
			feature: "合集管理（新建/编辑/删除）",
			testTitles: ["接口管理 UI：合集 新建/编辑/删除", "接口管理 API：合集/用例 创建与清理"],
		},
		{
			feature: "用例管理（新建/编辑/删除）",
			testTitles: ["接口管理 UI：用例 新建/编辑/删除", "接口管理 API：合集/用例 创建与清理"],
		},
		{
			feature: "计划管理（读取/更新/保存/恢复）",
			testTitles: ["接口管理 API：计划 读取/更新/恢复", "接口管理 UI：计划 保存提示"],
		},
		{
			feature: "执行与历史（单用例/合集/全部/查询/页面展示）",
			testTitles: ["接口管理 API：执行与历史（runCase/runAll/runs）", "接口管理 UI：历史记录 列表展示"],
		},
	]

	const statusByTitle = new Map(tests.map((t) => [t.title, t.status]))

	const total = tests.length
	const passed = tests.filter((t) => t.status === "passed").length
	const failed = tests.filter((t) => t.status === "failed").length
	const skipped = tests.filter((t) => t.status === "skipped").length
	const other = total - passed - failed - skipped

	const earliest = tests
		.map((t) => (t.startTime ? Date.parse(t.startTime) : NaN))
		.filter((n) => Number.isFinite(n))
		.sort((a, b) => a - b)[0]
	const durationTotalMs = tests.reduce((sum, t) => sum + (t.duration || 0), 0)

	const startedAt = Number.isFinite(earliest) ? new Date(earliest).toLocaleString("zh-CN") : "未知"
	const configFile = data?.config?.configFile || "未知"
	const playwrightVersion = data?.config?.version || "未知"

	const lines = []
	lines.push("# API Tests 测试报告（简体中文）")
	lines.push("")
	lines.push("## 概览")
	lines.push(`- 生成时间：${new Date().toLocaleString("zh-CN")}`)
	lines.push(`- 开始时间：${startedAt}`)
	lines.push(`- Playwright 版本：${playwrightVersion}`)
	lines.push(`- 配置文件：${configFile}`)
	lines.push(`- 用例统计：共 ${total} 条，✅ 通过 ${passed}，❌ 失败 ${failed}，⏭️ 跳过 ${skipped}${other ? `，❓ 其他 ${other}` : ""}`)
	lines.push(`- 总耗时（累计）：${formatMs(durationTotalMs)}`)
	lines.push("")
	lines.push("## 为什么会看到“用例统计：共 X 条”")
	lines.push("- Playwright 的统计口径是：每个 `test()` 计为 1 条“用例”。")
	lines.push("- 为了让失败定位更清晰，本模块将“接口断言(API)”与“页面断言(UI)”分层，并按功能点拆分，所以用例数会 > 1。")
	lines.push("")
	lines.push("## 功能覆盖与结果")
	for (const item of featureMatrix) {
		const statuses = item.testTitles.map((title) => statusByTitle.get(title) || "missing")
		const hasMissing = statuses.includes("missing")
		const hasFailed = statuses.some((s) => s === "failed")
		const hasSkipped = statuses.some((s) => s === "skipped")
		const ok = !hasMissing && !hasFailed && !hasSkipped && statuses.every((s) => s === "passed")
		const icon = ok ? "✅" : hasFailed ? "❌" : hasMissing ? "❓" : "⚠️"
		lines.push(`- ${icon} ${item.feature}`)
		lines.push(`  - 对应用例：${item.testTitles.join("；")}`)
	}
	lines.push("")
	lines.push("## 详情")

	const byFile = new Map()
	for (const t of tests) {
		const key = t.file || "unknown"
		const arr = byFile.get(key) || []
		arr.push(t)
		byFile.set(key, arr)
	}

	for (const [file, items] of byFile.entries()) {
		lines.push("")
		lines.push(`### ${file}`)
		for (const t of items) {
			const icon = t.status === "passed" ? "✅" : t.status === "failed" ? "❌" : t.status === "skipped" ? "⏭️" : "❓"
			const loc = t.location?.file ? `${t.location.file}:${t.location.line || 0}` : ""
			lines.push(`- ${icon} ${t.title}（${formatMs(t.duration)}）${loc ? ` @ ${loc}` : ""}`)
			const catalog = testCatalog[t.title]
			if (catalog?.length) {
				lines.push(`  - 覆盖点：${catalog.join("；")}`)
			}
			if (t.status === "failed") {
				for (const msg of t.errors.slice(0, 3)) {
					lines.push(`  - 错误：${msg.replace(/\r?\n/g, " ").slice(0, 500)}`)
				}
				const attachments = t.attachments.filter((a) => a.path).slice(0, 10)
				if (attachments.length) {
					lines.push("  - 附件：")
					for (const a of attachments) {
						lines.push(`    - ${a.name || "attachment"} ${a.contentType ? `(${a.contentType})` : ""}: ${a.path}`)
					}
				}
			}
		}
	}

	lines.push("")
	lines.push("## 如何查看可视化报告")
	lines.push("- HTML 报告目录：`tests/api-tests/playwright-report/`")
	lines.push("- 查看命令：`cd tests && npm run report:api-tests`")
	lines.push("")

	const outUrl = new URL("../test-results/report.zh-CN.md", import.meta.url)
	fs.writeFileSync(outUrl, lines.join("\n"), "utf8")
	console.log(`已生成：${outUrl.pathname}`)
}

main()

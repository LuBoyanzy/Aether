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
		console.error("未找到 Playwright JSON 报告：tests/docker-tests/test-results/results.json")
		console.error("请先执行：cd tests && npm run test:docker-tests")
		process.exit(1)
	}

	const data = readJson(resultsUrl)
	const tests = []
	for (const suite of data.suites || []) {
		collectTests(suite, suite.title || suite.file, tests)
	}

	const featureMatrix = [
		{ feature: "Docker 页面入口与 Tab", testTitles: ["Docker UI：页面入口与 11 个 Tab 可见"] },
		{ feature: "编排/镜像/网络/存储卷/配置 基础展示", testTitles: ["Docker UI：编排/镜像/网络/存储卷/配置 Tab 基础展示（不执行写操作）"] },
		{ feature: "概览（Overview）", testTitles: ["Docker UI：概览面板基础展示", "Docker API：概览与列表接口（基础返回结构）"] },
		{ feature: "容器（Containers）关注规则", testTitles: ["Docker UI：容器 关注规则 新增/删除"] },
		{ feature: "仓库（Registries）管理 + 审计", testTitles: ["Docker UI：仓库 新建/编辑/删除", "Docker API：仓库（Registries）创建/更新/删除 + 审计"] },
		{ feature: "编排模板（Compose Templates）管理 + 审计", testTitles: ["Docker UI：编排模板 新建/编辑/删除", "Docker API：编排模板（Compose Templates）创建/更新/删除 + YAML 校验 + 审计"] },
		{ feature: "服务配置（Service Configs）管理", testTitles: ["Docker UI：服务配置 新建/编辑/删除（依赖 system.host）", "Docker API：服务配置（Service Configs）创建/更新/删除 + 字段校验"] },
		{ feature: "数据清理（Data Cleanup）入口", testTitles: ["Docker UI：数据清理 面板基础展示（不执行清理）"] },
		{ feature: "审计查询参数校验", testTitles: ["Docker API：审计查询参数校验（start/end/page/perPage）"] },
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
	lines.push("# Docker Tests 测试报告（简体中文）")
	lines.push("")
	lines.push("## 概览")
	lines.push(`- 生成时间：${new Date().toLocaleString("zh-CN")}`)
	lines.push(`- 开始时间：${startedAt}`)
	lines.push(`- Playwright 版本：${playwrightVersion}`)
	lines.push(`- 配置文件：${configFile}`)
	lines.push(`- 用例统计：共 ${total} 条，✅ 通过 ${passed}，❌ 失败 ${failed}，⏭️ 跳过 ${skipped}${other ? `，❓ 其他 ${other}` : ""}`)
	lines.push(`- 总耗时（累计）：${formatMs(durationTotalMs)}`)
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
	lines.push("- HTML 报告目录：`tests/docker-tests/playwright-report/`")
	lines.push("- 查看命令：`cd tests && npm run report:docker-tests`")
	lines.push("")

	const outUrl = new URL("../test-results/report.zh-CN.md", import.meta.url)
	fs.writeFileSync(outUrl, lines.join("\n"), "utf8")
	console.log(`已生成：${outUrl.pathname}`)
}

main()

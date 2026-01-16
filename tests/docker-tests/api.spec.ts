import { expect, test, type APIRequestContext, type APIResponse } from "@playwright/test"
import { buildApiUrl, env, expectOkJson, getSystemName, listDockerAudits, login, loginReadonlyIfConfigured, authHeaders } from "./support"

test.setTimeout(180_000)

const expectJson = async (response: APIResponse) => {
	const text = await response.text()
	try {
		return JSON.parse(text) as any
	} catch {
		return { raw: text }
	}
}

const getWithAuth = async (request: APIRequestContext, token: string, path: string, query?: Record<string, string>) => {
	const url = new URL(buildApiUrl(path))
	for (const [k, v] of Object.entries(query || {})) {
		url.searchParams.set(k, v)
	}
	return request.get(url.toString(), { headers: authHeaders(token) })
}

const postWithAuth = async (request: APIRequestContext, token: string, path: string, payload?: unknown, query?: Record<string, string>) => {
	const url = new URL(buildApiUrl(path))
	for (const [k, v] of Object.entries(query || {})) {
		url.searchParams.set(k, v)
	}
	return request.post(url.toString(), { headers: authHeaders(token), data: payload })
}

test("Docker API：system 参数校验（overview/containers/images/networks/volumes/compose/config）", async ({ request }) => {
	const auth = await login(request)
	const paths = [
		"/api/aether/docker/overview",
		"/api/aether/docker/containers",
		"/api/aether/docker/images",
		"/api/aether/docker/networks",
		"/api/aether/docker/volumes",
		"/api/aether/docker/compose/projects",
		"/api/aether/docker/config",
	]

	for (const path of paths) {
		const response = await getWithAuth(request, auth.token, path)
		expect(response.status(), `${path} 未传 system 应返回 400`).toBe(400)
		const body = await expectJson(response)
		expect(body.error, `${path} 错误信息应明确说明缺少 system`).toBe("system is required")
	}
})

test("Docker API：概览与列表接口（基础返回结构）", async ({ request }) => {
	const auth = await login(request)
	// 先读一下 system 名称，确保 systemId 可用（也方便后续排查）
	expect(await getSystemName(request, auth.token, env.systemId)).toBeTruthy()

	const overviewResp = await getWithAuth(request, auth.token, "/api/aether/docker/overview", { system: env.systemId })
	if (overviewResp.status() === 502) {
		test.skip(true, "当前 system 的 Docker 概览接口返回 502（可能未安装/未启用 Docker 或 Agent 不可达）")
	}
	const overview = await expectOkJson<Record<string, unknown>>(overviewResp)
	expect(typeof overview.serverVersion).toBe("string")
	expect(typeof overview.apiVersion).toBe("string")
	expect(typeof overview.operatingSystem).toBe("string")

	const containersResp = await getWithAuth(request, auth.token, "/api/aether/docker/containers", { system: env.systemId })
	const containers = await expectOkJson<unknown>(containersResp)
	expect(Array.isArray(containers)).toBeTruthy()

	const imagesResp = await getWithAuth(request, auth.token, "/api/aether/docker/images", { system: env.systemId })
	const images = await expectOkJson<unknown>(imagesResp)
	expect(Array.isArray(images)).toBeTruthy()

	const networksResp = await getWithAuth(request, auth.token, "/api/aether/docker/networks", { system: env.systemId })
	const networks = await expectOkJson<unknown>(networksResp)
	expect(Array.isArray(networks)).toBeTruthy()

	const volumesResp = await getWithAuth(request, auth.token, "/api/aether/docker/volumes", { system: env.systemId })
	const volumes = await expectOkJson<unknown>(volumesResp)
	expect(Array.isArray(volumes)).toBeTruthy()

	const composeResp = await getWithAuth(request, auth.token, "/api/aether/docker/compose/projects", { system: env.systemId })
	const projects = await expectOkJson<unknown>(composeResp)
	expect(Array.isArray(projects)).toBeTruthy()

	const configResp = await getWithAuth(request, auth.token, "/api/aether/docker/config", { system: env.systemId })
	const config = await expectOkJson<Record<string, unknown>>(configResp)
	expect(typeof config.exists).toBe("boolean")
})

test("Docker API：仓库（Registries）创建/更新/删除 + 审计", async ({ request }) => {
	const auth = await login(request)

	const name = `PW Registry ${Date.now()}`
	const updatedName = `${name} Updated`
	const createResp = await postWithAuth(request, auth.token, "/api/aether/docker/registries", {
		name,
		server: "localhost",
		username: "",
		password: "",
	})
	const created = await expectOkJson<{ id: string }>(createResp)
	expect(created.id).toBeTruthy()

	try {
		const listResp = await getWithAuth(request, auth.token, "/api/aether/docker/registries")
		const list = await expectOkJson<{ items: Array<{ id: string; name: string }> }>(listResp)
		expect(list.items.some((item) => item.id === created.id && item.name === name)).toBeTruthy()

		const updateResp = await postWithAuth(request, auth.token, "/api/aether/docker/registries/update", {
			id: created.id,
			name: updatedName,
			server: "127.0.0.1",
		})
		expect((await expectOkJson<{ status: string }>(updateResp)).status).toBe("ok")

		const deleteResp = await postWithAuth(request, auth.token, "/api/aether/docker/registries/delete", undefined, {
			id: created.id,
		})
		expect((await expectOkJson<{ status: string }>(deleteResp)).status).toBe("ok")

		const windowStart = new Date(Date.now() - 2 * 60_000).toISOString()
		const windowEnd = new Date(Date.now() + 2 * 60_000).toISOString()
		const audits = await listDockerAudits(request, auth.token, { start: windowStart, end: windowEnd, page: 1, perPage: 100 })
		const related = audits.items.filter((a) => a.resource_type === "registry" && a.resource_id === created.id)
		expect(related.some((a) => a.action === "registry.create" && a.status === "success")).toBeTruthy()
		expect(related.some((a) => a.action === "registry.update" && a.status === "success")).toBeTruthy()
		expect(related.some((a) => a.action === "registry.delete" && a.status === "success")).toBeTruthy()
	} finally {
		// 即便删除已执行，也再尝试一次清理，避免中途失败残留。
		await postWithAuth(request, auth.token, "/api/aether/docker/registries/delete", undefined, { id: created.id })
	}
})

test("Docker API：编排模板（Compose Templates）创建/更新/删除 + YAML 校验 + 审计", async ({ request }) => {
	const auth = await login(request)

	const invalidResp = await postWithAuth(request, auth.token, "/api/aether/docker/compose-templates", {
		name: `PW Template Invalid ${Date.now()}`,
		content: "version: '3'",
	})
	expect(invalidResp.status()).toBe(400)
	expect((await expectJson(invalidResp)).error).toBe("compose services is required")

	const name = `PW Template ${Date.now()}`
	const createResp = await postWithAuth(request, auth.token, "/api/aether/docker/compose-templates", {
		name,
		description: "pw",
		content: "services:\n  app:\n    image: nginx:alpine\n",
		env: "",
	})
	const created = await expectOkJson<{ id: string }>(createResp)
	expect(created.id).toBeTruthy()

	try {
		const listResp = await getWithAuth(request, auth.token, "/api/aether/docker/compose-templates")
		const list = await expectOkJson<{ items: Array<{ id: string; name: string; content: string }> }>(listResp)
		expect(list.items.some((item) => item.id === created.id && item.name === name)).toBeTruthy()

		const updateResp = await postWithAuth(request, auth.token, "/api/aether/docker/compose-templates/update", {
			id: created.id,
			description: "pw-updated",
		})
		expect((await expectOkJson<{ status: string }>(updateResp)).status).toBe("ok")

		const deleteResp = await postWithAuth(request, auth.token, "/api/aether/docker/compose-templates/delete", undefined, {
			id: created.id,
		})
		expect((await expectOkJson<{ status: string }>(deleteResp)).status).toBe("ok")

		const windowStart = new Date(Date.now() - 2 * 60_000).toISOString()
		const windowEnd = new Date(Date.now() + 2 * 60_000).toISOString()
		const audits = await listDockerAudits(request, auth.token, { start: windowStart, end: windowEnd, page: 1, perPage: 100 })
		const related = audits.items.filter((a) => a.resource_type === "compose_template" && a.resource_id === created.id)
		expect(related.some((a) => a.action === "compose_template.create" && a.status === "success")).toBeTruthy()
		expect(related.some((a) => a.action === "compose_template.update" && a.status === "success")).toBeTruthy()
		expect(related.some((a) => a.action === "compose_template.delete" && a.status === "success")).toBeTruthy()
	} finally {
		await postWithAuth(request, auth.token, "/api/aether/docker/compose-templates/delete", undefined, { id: created.id })
	}
})

test("Docker API：服务配置（Service Configs）创建/更新/删除 + 字段校验", async ({ request }) => {
	const auth = await login(request)
	expect(await getSystemName(request, auth.token, env.systemId)).toBeTruthy()

	const listResp = await getWithAuth(request, auth.token, "/api/aether/docker/service-configs", { system: env.systemId })
	const list = await expectOkJson<{ items: Array<{ id: string; name: string; system: string; url: string }> }>(listResp)
	expect(Array.isArray(list.items)).toBeTruthy()

	const name = `PW Service ${Date.now()}`
	const createResp = await postWithAuth(request, auth.token, "/api/aether/docker/service-configs", {
		system: env.systemId,
		name,
		url: "http://127.0.0.1:1",
		token: "pw-token",
	})
	const created = await expectOkJson<{ id: string }>(createResp)
	expect(created.id).toBeTruthy()

	try {
		const updateResp = await postWithAuth(request, auth.token, "/api/aether/docker/service-configs/update", {
			id: created.id,
			name: `${name} Updated`,
		})
		expect((await expectOkJson<{ status: string }>(updateResp)).status).toBe("ok")

		const updateTokenResp = await postWithAuth(request, auth.token, "/api/aether/docker/service-configs/update", {
			id: created.id,
			token: "new",
		})
		expect(updateTokenResp.status()).toBe(400)
		expect((await expectJson(updateTokenResp)).error).toBe("token cannot be updated")

		const delResp = await postWithAuth(request, auth.token, "/api/aether/docker/service-configs/delete", undefined, {
			id: created.id,
		})
		expect((await expectOkJson<{ status: string }>(delResp)).status).toBe("ok")
	} finally {
		await postWithAuth(request, auth.token, "/api/aether/docker/service-configs/delete", undefined, { id: created.id })
	}
})

test("Docker API：服务配置内容（Content）缺参/上游失败", async ({ request }) => {
	if (!env.enableServiceConfigContentTests) {
		test.skip(true, "未开启服务配置内容用例：设置 PLAYWRIGHT_DOCKER_ENABLE_SERVICE_CONFIG_CONTENT=true")
	}

	const auth = await login(request)

	const missingResp = await getWithAuth(request, auth.token, "/api/aether/docker/service-configs/content")
	expect(missingResp.status()).toBe(400)
	expect((await expectJson(missingResp)).error).toBe("system and id are required")

	// 通过创建一个不可达的 upstream URL，验证 Hub 的错误映射（不会影响真实服务）。
	const createResp = await postWithAuth(request, auth.token, "/api/aether/docker/service-configs", {
		system: env.systemId,
		name: `PW Service Content ${Date.now()}`,
		url: "http://127.0.0.1:1",
		token: "pw-token",
	})
	const created = await expectOkJson<{ id: string }>(createResp)

	try {
		const contentResp = await getWithAuth(request, auth.token, "/api/aether/docker/service-configs/content", {
			system: env.systemId,
			id: created.id,
		})
		expect(contentResp.status()).toBe(502)
		expect((await expectJson(contentResp)).error).toBe("failed to fetch config content")

		const updateResp = await request.put(buildApiUrl("/api/aether/docker/service-configs/content"), {
			headers: authHeaders(auth.token),
			data: { system: env.systemId, id: created.id, content: "pw" },
		})
		expect(updateResp.status()).toBe(502)
		expect((await expectJson(updateResp)).error).toBe("failed to update config content")
	} finally {
		await postWithAuth(request, auth.token, "/api/aether/docker/service-configs/delete", undefined, { id: created.id })
	}
})

test("Docker API：审计查询参数校验（start/end/page/perPage）", async ({ request }) => {
	const auth = await login(request)

	const invalidStartResp = await getWithAuth(request, auth.token, "/api/aether/docker/audits", { start: "nope" })
	expect(invalidStartResp.status()).toBe(400)
	expect((await expectJson(invalidStartResp)).error).toBe("start must be RFC3339")

	const invalidEndResp = await getWithAuth(request, auth.token, "/api/aether/docker/audits", { end: "nope" })
	expect(invalidEndResp.status()).toBe(400)
	expect((await expectJson(invalidEndResp)).error).toBe("end must be RFC3339")

	const missingPagePairResp = await getWithAuth(request, auth.token, "/api/aether/docker/audits", { page: "1" })
	expect(missingPagePairResp.status()).toBe(400)
	expect((await expectJson(missingPagePairResp)).error).toBe("page and perPage are required")

	const invalidPageResp = await getWithAuth(request, auth.token, "/api/aether/docker/audits", { page: "0", perPage: "10" })
	expect(invalidPageResp.status()).toBe(400)
	expect((await expectJson(invalidPageResp)).error).toBe("page must be a positive integer")

	const invalidPerPageResp = await getWithAuth(request, auth.token, "/api/aether/docker/audits", { page: "1", perPage: "0" })
	expect(invalidPerPageResp.status()).toBe(400)
	expect((await expectJson(invalidPerPageResp)).error).toBe("perPage must be a positive integer")
})

test("Docker API：数据清理（Data Cleanup）配置读取与参数校验（不执行清理/不写入配置）", async ({ request }) => {
	const auth = await login(request)

	const missingSystemResp = await getWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/config")
	expect(missingSystemResp.status()).toBe(400)
	expect((await expectJson(missingSystemResp)).error).toBe("system is required")

	const configResp = await getWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/config", { system: env.systemId })
	const config = await expectOkJson<{
		id?: string
		system: string
		redis: { patterns: string[] }
	}>(configResp)
	expect(config.system).toBe(env.systemId)
	expect(Array.isArray(config.redis.patterns)).toBeTruthy()
	expect(config.redis.patterns.length).toBeGreaterThan(0)

	const runMissingSystem = await postWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/run", {})
	expect(runMissingSystem.status()).toBe(400)
	expect((await expectJson(runMissingSystem)).error).toBe("system is required")

	const runGetMissingId = await getWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/run")
	expect(runGetMissingId.status()).toBe(400)
	expect((await expectJson(runGetMissingId)).error).toBe("id is required")

	const mysqlDbMissing = await postWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/mysql/databases", {})
	expect(mysqlDbMissing.status()).toBe(400)
	expect((await expectJson(mysqlDbMissing)).error).toBe("system, host and port are required")

	const mysqlTablesMissing = await postWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/mysql/tables", {})
	expect(mysqlTablesMissing.status()).toBe(400)
	expect((await expectJson(mysqlTablesMissing)).error).toBe("system, host, port, database are required")

	const redisDbsMissing = await postWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/redis/dbs", {})
	expect(redisDbsMissing.status()).toBe(400)
	expect((await expectJson(redisDbsMissing)).error).toBe("system, host and port are required")

	const minioBucketsMissing = await postWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/minio/buckets", {})
	expect(minioBucketsMissing.status()).toBe(400)
	expect((await expectJson(minioBucketsMissing)).error).toBe("system, host and port are required")

	const minioPrefixesMissing = await postWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/minio/prefixes", {})
	expect(minioPrefixesMissing.status()).toBe(400)
	expect((await expectJson(minioPrefixesMissing)).error).toBe("system, host, port, bucket are required")

	const esIndicesMissing = await postWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/es/indices", {})
	expect(esIndicesMissing.status()).toBe(400)
	expect((await expectJson(esIndicesMissing)).error).toBe("system, host and port are required")
})

test("Docker API：容器详情接口参数校验（logs/info）", async ({ request }) => {
	const auth = await login(request)

	const logsResp = await getWithAuth(request, auth.token, "/api/aether/containers/logs")
	if (logsResp.status() === 404) {
		test.skip(true, "当前环境未启用 CONTAINER_DETAILS，/api/aether/containers/* 接口不可用")
	}
	expect(logsResp.status()).toBe(400)
	expect((await expectJson(logsResp)).error).toBe("system and container parameters are required")

	const infoResp = await getWithAuth(request, auth.token, "/api/aether/containers/info")
	expect(infoResp.status()).toBe(400)
	expect((await expectJson(infoResp)).error).toBe("system and container parameters are required")
})

test("Docker API：写接口参数校验（images/compose/config）", async ({ request }) => {
	const auth = await login(request)

	const paths = [
		"/api/aether/docker/images/pull",
		"/api/aether/docker/images/push",
		"/api/aether/docker/images/remove",
		"/api/aether/docker/compose/projects",
		"/api/aether/docker/compose/projects/update",
		"/api/aether/docker/compose/projects/operate",
		"/api/aether/docker/compose/projects/delete",
		"/api/aether/docker/config",
	]

	for (const path of paths) {
		const resp = await postWithAuth(request, auth.token, path, {})
		if (resp.status() === 403) {
			test.skip(true, `当前账号可能为只读，无法覆盖写接口：${path}`)
		}
		expect(resp.status(), `${path} body.system 为空应返回 400`).toBe(400)
		const body = await expectJson(resp)
		expect(body.error).toBe("system is required")
	}
})

test.describe("Docker API：可回收写操作（Networks/Volumes）", () => {
	test("Docker API：网络创建/删除 + 审计（可选：需要 Docker 可用）", async ({ request }) => {
		if (!env.enableDestructiveDockerTests) {
			test.skip(true, "未开启可回收写操作：设置 PLAYWRIGHT_DOCKER_ENABLE_DESTRUCTIVE=true")
		}
		const auth = await login(request)
		const networkName = `pw-net-${Date.now()}`

		const createResp = await postWithAuth(request, auth.token, "/api/aether/docker/networks", {
			system: env.systemId,
			name: networkName,
			driver: "bridge",
			enableIPv6: false,
			internal: false,
			attachable: false,
			labels: { "pw": "1" },
			options: {},
		})
		if (createResp.status() === 502) {
			test.skip(true, "当前 system 无法创建 Docker 网络（可能 Docker/权限/Agent 状态异常）")
		}
		expect((await expectOkJson<{ status: string }>(createResp)).status).toBe("ok")

		const listResp = await getWithAuth(request, auth.token, "/api/aether/docker/networks", { system: env.systemId })
		const networks = await expectOkJson<Array<{ id: string; name: string }>>(listResp)
		const created = networks.find((n) => n.name === networkName)
		expect(created, "创建后应能在 networks 列表中找到该网络").toBeTruthy()

		const removeResp = await postWithAuth(request, auth.token, "/api/aether/docker/networks/remove", {
			system: env.systemId,
			networkId: created!.id,
		})
		expect((await expectOkJson<{ status: string }>(removeResp)).status).toBe("ok")

		const windowStart = new Date(Date.now() - 2 * 60_000).toISOString()
		const windowEnd = new Date(Date.now() + 2 * 60_000).toISOString()
		const audits = await listDockerAudits(request, auth.token, { system: env.systemId, start: windowStart, end: windowEnd, page: 1, perPage: 100 })
		const related = audits.items.filter((a) => a.resource_type === "network" && (a.resource_id === networkName || a.resource_id === created!.id))
		expect(related.some((a) => a.action === "network.create")).toBeTruthy()
		expect(related.some((a) => a.action === "network.remove")).toBeTruthy()
	})

	test("Docker API：存储卷创建/删除 + 审计（可选：需要 Docker 可用）", async ({ request }) => {
		if (!env.enableDestructiveDockerTests) {
			test.skip(true, "未开启可回收写操作：设置 PLAYWRIGHT_DOCKER_ENABLE_DESTRUCTIVE=true")
		}
		const auth = await login(request)
		const volumeName = `pw-vol-${Date.now()}`

		const createResp = await postWithAuth(request, auth.token, "/api/aether/docker/volumes", {
			system: env.systemId,
			name: volumeName,
			driver: "local",
			labels: { "pw": "1" },
			options: {},
		})
		if (createResp.status() === 502) {
			test.skip(true, "当前 system 无法创建 Docker 存储卷（可能 Docker/权限/Agent 状态异常）")
		}
		expect((await expectOkJson<{ status: string }>(createResp)).status).toBe("ok")

		const removeResp = await postWithAuth(request, auth.token, "/api/aether/docker/volumes/remove", {
			system: env.systemId,
			name: volumeName,
			force: true,
		})
		expect((await expectOkJson<{ status: string }>(removeResp)).status).toBe("ok")

		const windowStart = new Date(Date.now() - 2 * 60_000).toISOString()
		const windowEnd = new Date(Date.now() + 2 * 60_000).toISOString()
		const audits = await listDockerAudits(request, auth.token, { system: env.systemId, start: windowStart, end: windowEnd, page: 1, perPage: 100 })
		const related = audits.items.filter((a) => a.resource_type === "volume" && a.resource_id === volumeName)
		expect(related.some((a) => a.action === "volume.create")).toBeTruthy()
		expect(related.some((a) => a.action === "volume.remove")).toBeTruthy()
	})
})

test.describe("Docker API：高风险/依赖外部状态写操作（可选）", () => {
	test("Docker API：Data Cleanup 配置 更新/恢复（不覆盖密码）", async ({ request }) => {
		if (!env.enableDataCleanupConfigUpdateTests) {
			test.skip(true, "未开启 Data Cleanup 配置更新用例：设置 PLAYWRIGHT_DOCKER_ENABLE_DATA_CLEANUP_CONFIG_UPDATE=true")
		}
		const auth = await login(request)
		const getResp = await getWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/config", { system: env.systemId })
		const current = await expectOkJson<any>(getResp)
		if (!current?.id) {
			test.skip(true, "当前 system 尚未创建 data-cleanup 配置记录（无删除接口，避免用例自动创建）")
		}

		const snapshot = JSON.parse(JSON.stringify(current))
		const nextDb = typeof current.redis?.db === "number" ? (current.redis.db === 0 ? 1 : 0) : 0
		const updateResp = await postWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/config", {
			...current,
			redis: { ...current.redis, db: nextDb, password: "" },
			mysql: { ...current.mysql, password: "" },
			minio: { ...current.minio, secretKey: "" },
			es: { ...current.es, password: "" },
		})
		if (updateResp.status() === 403) {
			test.skip(true, "当前账号可能为只读，无法更新 data-cleanup 配置")
		}
		expect((await expectOkJson<{ status: string }>(updateResp)).status).toBe("ok")

		try {
			const afterResp = await getWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/config", { system: env.systemId })
			const after = await expectOkJson<any>(afterResp)
			expect(after.redis?.db).toBe(nextDb)
		} finally {
			// 恢复原配置（不传密码字段，避免覆盖已加密的密钥）
			const restoreResp = await postWithAuth(request, auth.token, "/api/aether/docker/data-cleanup/config", {
				...snapshot,
				mysql: { ...snapshot.mysql, password: "" },
				redis: { ...snapshot.redis, password: "" },
				minio: { ...snapshot.minio, secretKey: "" },
				es: { ...snapshot.es, password: "" },
			})
			if (!restoreResp.ok()) {
				throw new Error(`恢复 data-cleanup 配置失败：${restoreResp.status()} ${await restoreResp.text()}`)
			}
		}
	})

	test("Docker API：Docker daemon 配置更新（复写原内容，可选）", async ({ request }) => {
		if (!env.enableDaemonConfigTests) {
			test.skip(true, "未开启 Docker daemon 配置用例：设置 PLAYWRIGHT_DOCKER_ENABLE_DAEMON_CONFIG=true")
		}
		const auth = await login(request)
		const configResp = await getWithAuth(request, auth.token, "/api/aether/docker/config", { system: env.systemId })
		if (configResp.status() === 502) {
			test.skip(true, "当前 system 无法读取 Docker daemon 配置（可能未启用 Docker 或 Agent 不可达）")
		}
		const current = await expectOkJson<{ path: string; content: string; exists: boolean }>(configResp)
		if (!current.exists || !current.content.trim()) {
			test.skip(true, "当前 daemon.json 不存在或内容为空；避免用例自动创建新配置文件")
		}

		const updateResp = await postWithAuth(request, auth.token, "/api/aether/docker/config", {
			system: env.systemId,
			content: current.content,
			path: current.path,
			restart: false,
		})
		if (updateResp.status() === 403) {
			test.skip(true, "当前账号可能为只读，无法更新 Docker daemon 配置")
		}
		if (updateResp.status() === 502) {
			const body = await updateResp.text()
			if (body.includes("context deadline exceeded")) {
				test.skip(true, "更新 Docker daemon 配置超时（502 context deadline exceeded），当前环境可能 Agent/Docker 不可达")
			}
			throw new Error(`更新 Docker daemon 配置失败：502 ${body}`)
		}
		expect((await expectOkJson<{ status: string }>(updateResp)).status).toBe("ok")

		const windowStart = new Date(Date.now() - 2 * 60_000).toISOString()
		const windowEnd = new Date(Date.now() + 2 * 60_000).toISOString()
		const audits = await listDockerAudits(request, auth.token, { system: env.systemId, start: windowStart, end: windowEnd, page: 1, perPage: 100 })
		expect(audits.items.some((a) => a.action === "config.update")).toBeTruthy()
	})

	test("Docker API：编排项目（Compose Projects）创建/操作/删除（可选，需提供可运行镜像）", async ({ request }) => {
		if (!env.enableComposeProjectTests) {
			test.skip(true, "未开启编排项目用例：设置 PLAYWRIGHT_DOCKER_ENABLE_COMPOSE=true")
		}
		if (!env.enableDestructiveDockerTests) {
			test.skip(true, "未开启可回收写操作：设置 PLAYWRIGHT_DOCKER_ENABLE_DESTRUCTIVE=true")
		}
		if (!env.testImage) {
			test.skip(true, "未提供测试镜像：设置 PLAYWRIGHT_DOCKER_TEST_IMAGE（需为本机已存在且可运行的镜像标签）")
		}

		const auth = await login(request)
		const name = `pw-${Date.now().toString(36)}`
		const compose = `services:\n  app:\n    image: ${env.testImage}\n    command: [\"sh\", \"-c\", \"sleep 30\"]\n`

		const createResp = await postWithAuth(request, auth.token, "/api/aether/docker/compose/projects", {
			system: env.systemId,
			name,
			content: compose,
			env: "",
		})
		if (createResp.status() === 403) {
			test.skip(true, "当前账号可能为只读，无法创建编排项目")
		}
		if (createResp.status() === 502) {
			test.skip(true, "创建编排项目失败（可能 docker compose 不可用 / 镜像不可运行 / Docker 不可用）")
		}
		expect((await expectOkJson<{ status: string }>(createResp)).status).toBe("ok")

		try {
			const stopResp = await postWithAuth(request, auth.token, "/api/aether/docker/compose/projects/operate", {
				system: env.systemId,
				name,
				operation: "stop",
			})
			expect((await expectOkJson<{ status: string }>(stopResp)).status).toBe("ok")
		} finally {
			const deleteResp = await postWithAuth(request, auth.token, "/api/aether/docker/compose/projects/delete", {
				system: env.systemId,
				name,
				removeFile: true,
			})
			if (!deleteResp.ok()) {
				throw new Error(`删除编排项目失败：${deleteResp.status()} ${await deleteResp.text()}`)
			}
		}

		const windowStart = new Date(Date.now() - 5 * 60_000).toISOString()
		const windowEnd = new Date(Date.now() + 5 * 60_000).toISOString()
		const audits = await listDockerAudits(request, auth.token, { system: env.systemId, start: windowStart, end: windowEnd, page: 1, perPage: 200 })
		const related = audits.items.filter((a) => a.resource_type === "compose" && a.resource_id === name)
		expect(related.some((a) => a.action === "compose.create")).toBeTruthy()
		expect(related.some((a) => a.action === "compose.operate")).toBeTruthy()
		expect(related.some((a) => a.action === "compose.delete")).toBeTruthy()
	})
})

test.describe("Docker API：只读/权限用例（可选）", () => {
	test("Docker API：只读账号写入应返回 403（以 Registries 为例）", async ({ request }) => {
		const readonly = await loginReadonlyIfConfigured(request)
		if (!readonly) {
			test.skip(true, "未配置只读账号：设置 PLAYWRIGHT_READONLY_EMAIL / PLAYWRIGHT_READONLY_PASSWORD")
		}

		const resp = await request.post(buildApiUrl("/api/aether/docker/registries"), {
			headers: authHeaders(readonly!.token),
			data: { name: `PW RO ${Date.now()}`, server: "localhost" },
		})
		expect(resp.status()).toBe(403)
		expect((await expectJson(resp)).error).toBe("forbidden")
	})
})

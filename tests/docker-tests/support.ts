import { type APIRequestContext, type APIResponse, type Page } from "@playwright/test"
import fs from "node:fs"

const loadEnvFile = () => {
	// 与当前工作目录无关：始终读取同目录下的 `.env`（tests/docker-tests/.env）
	const envUrl = new URL("./.env", import.meta.url)
	if (!fs.existsSync(envUrl)) {
		return
	}
	const content = fs.readFileSync(envUrl, "utf8")
	for (const line of content.split(/\r?\n/)) {
		const trimmed = line.trim()
		if (!trimmed || trimmed.startsWith("#")) {
			continue
		}
		const separatorIndex = trimmed.indexOf("=")
		if (separatorIndex <= 0) {
			continue
		}
		let key = trimmed.slice(0, separatorIndex).trim()
		if (key.startsWith("export ")) {
			key = key.slice("export ".length).trim()
		}
		let value = trimmed.slice(separatorIndex + 1).trim()
		if (
			(value.startsWith("\"") && value.endsWith("\"")) ||
			(value.startsWith("'") && value.endsWith("'"))
		) {
			value = value.slice(1, -1)
		}
		if (process.env[key] === undefined) {
			process.env[key] = value
		}
	}
}

loadEnvFile()

const requiredEnv = (name: string) => {
	const value = process.env[name]
	if (!value) {
		throw new Error(`缺少环境变量：${name}。请在运行测试前设置该变量。`)
	}
	return value
}

const dockerUrl = requiredEnv("PLAYWRIGHT_DOCKER_URL")
const email = requiredEnv("PLAYWRIGHT_EMAIL")
const password = requiredEnv("PLAYWRIGHT_PASSWORD")
const systemId = requiredEnv("PLAYWRIGHT_DOCKER_SYSTEM_ID")

const readonlyEmail = process.env.PLAYWRIGHT_READONLY_EMAIL || ""
const readonlyPassword = process.env.PLAYWRIGHT_READONLY_PASSWORD || ""

const dockerUrlObj = new URL(dockerUrl)
const dockerBasePath = (() => {
	const trimmed = dockerUrlObj.pathname.replace(/\/containers\/?$/, "")
	if (!trimmed || trimmed === "/") {
		return ""
	}
	return trimmed.endsWith("/") ? trimmed.slice(0, -1) : trimmed
})()

if (dockerUrlObj.pathname === dockerBasePath) {
	throw new Error("PLAYWRIGHT_DOCKER_URL 必须以 /containers 结尾，例如：https://example.com/containers")
}

const apiBaseUrl = `${dockerUrlObj.origin}${dockerBasePath}`

export const env = {
	dockerUrl,
	apiBaseUrl,
	email,
	password,
	systemId,
	readonlyEmail,
	readonlyPassword,
	enableDestructiveDockerTests: process.env.PLAYWRIGHT_DOCKER_ENABLE_DESTRUCTIVE === "true",
	enableComposeProjectTests: process.env.PLAYWRIGHT_DOCKER_ENABLE_COMPOSE === "true",
	testImage: process.env.PLAYWRIGHT_DOCKER_TEST_IMAGE || "",
	enableDaemonConfigTests: process.env.PLAYWRIGHT_DOCKER_ENABLE_DAEMON_CONFIG === "true",
	enableServiceConfigContentTests: process.env.PLAYWRIGHT_DOCKER_ENABLE_SERVICE_CONFIG_CONTENT === "true",
	enableDataCleanupConfigUpdateTests: process.env.PLAYWRIGHT_DOCKER_ENABLE_DATA_CLEANUP_CONFIG_UPDATE === "true",
}

export type AuthResponse = {
	token: string
	record: Record<string, unknown>
}

export const buildApiUrl = (path: string) => `${apiBaseUrl}${path.startsWith("/") ? path : `/${path}`}`

export const expectOkJson = async <T>(response: APIResponse) => {
	if (!response.ok()) {
		const body = await response.text()
		throw new Error(`API 请求失败：${response.status()} ${response.statusText()} ${body}`)
	}
	return (await response.json()) as T
}

export const authHeaders = (token: string) => ({
	Authorization: token,
})

export const login = async (request: APIRequestContext, opts?: { email?: string; password?: string }) => {
	const response = await request.post(buildApiUrl("/api/collections/users/auth-with-password"), {
		data: {
			identity: opts?.email || email,
			password: opts?.password || password,
		},
	})
	const data = await expectOkJson<AuthResponse>(response)
	if (!data.token || !data.record) {
		throw new Error("登录响应缺少 token 或 record。")
	}
	return data
}

export const loginReadonlyIfConfigured = async (request: APIRequestContext) => {
	if (!env.readonlyEmail || !env.readonlyPassword) {
		return null
	}
	return login(request, { email: env.readonlyEmail, password: env.readonlyPassword })
}

export const setAuthStorage = async (page: Page, auth: AuthResponse, dockerSystemId: string) => {
	await page.addInitScript(
		(payload) => {
			localStorage.clear()
			sessionStorage.clear()
			localStorage.setItem("lang", "zh-CN")
			localStorage.setItem("docker-system", payload.dockerSystemId)
			localStorage.setItem("pocketbase_auth", JSON.stringify({ token: payload.token, record: payload.record }))
		},
		{ token: auth.token, record: auth.record, dockerSystemId }
	)
}

export const getSystemName = async (request: APIRequestContext, token: string, id: string) => {
	const response = await request.get(buildApiUrl(`/api/collections/systems/records/${encodeURIComponent(id)}`), {
		headers: authHeaders(token),
	})
	const data = await expectOkJson<{ id: string; name?: string }>(response)
	const name = typeof data.name === "string" ? data.name : ""
	if (!name) {
		throw new Error("无法读取 system 名称：systems 记录缺少 name 字段。")
	}
	return name
}

export const getSystemRecord = async (request: APIRequestContext, token: string, id: string) => {
	const response = await request.get(buildApiUrl(`/api/collections/systems/records/${encodeURIComponent(id)}`), {
		headers: authHeaders(token),
	})
	const data = await expectOkJson<{ id: string; name?: string; host?: string; port?: string }>(response)
	return {
		id: data.id,
		name: typeof data.name === "string" ? data.name : "",
		host: typeof data.host === "string" ? data.host : "",
		port: typeof data.port === "string" ? data.port : "",
	}
}

export const listDockerAudits = async (
	request: APIRequestContext,
	token: string,
	params: { system?: string; start?: string; end?: string; page?: number; perPage?: number }
) => {
	const url = new URL(buildApiUrl("/api/aether/docker/audits"))
	if (params.system) url.searchParams.set("system", params.system)
	if (params.start) url.searchParams.set("start", params.start)
	if (params.end) url.searchParams.set("end", params.end)
	if (params.page) url.searchParams.set("page", String(params.page))
	if (params.perPage) url.searchParams.set("perPage", String(params.perPage))
	const response = await request.get(url.toString(), { headers: authHeaders(token) })
	return expectOkJson<{
		items: Array<{
			id: string
			system?: string
			user: string
			user_name?: string
			user_email?: string
			action: string
			resource_type: string
			resource_id: string
			status: string
			detail: string
			created: string
		}>
	}>(response)
}

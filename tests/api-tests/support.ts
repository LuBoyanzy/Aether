import { type APIRequestContext, type APIResponse, type Page } from "@playwright/test"
import fs from "node:fs"

const loadEnvFile = () => {
	// 与当前工作目录无关：始终读取同目录下的 `.env`（tests/api-tests/.env）
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
		throw new Error(`${name} is required. Set ${name} before running this test.`)
	}
	return value
}

const baseUrl = requiredEnv("PLAYWRIGHT_BASE_URL")
const email = requiredEnv("PLAYWRIGHT_EMAIL")
const password = requiredEnv("PLAYWRIGHT_PASSWORD")
const caseUrl = requiredEnv("PLAYWRIGHT_CASE_URL")

const apiTestsUrl = (() => {
	if (!baseUrl.includes("/api-tests")) {
		throw new Error("PLAYWRIGHT_BASE_URL must include /api-tests")
	}
	return baseUrl
})()

const apiTestsUrlObj = new URL(apiTestsUrl)
const apiBasePath = (() => {
	const trimmed = apiTestsUrlObj.pathname.replace(/\/api-tests\/?$/, "")
	if (!trimmed || trimmed === "/") {
		return ""
	}
	return trimmed.endsWith("/") ? trimmed.slice(0, -1) : trimmed
})()
const apiBaseUrl = `${apiTestsUrlObj.origin}${apiBasePath}`

export const env = {
	apiTestsUrl,
	email,
	password,
	caseUrl,
	apiBaseUrl,
}

export type AuthResponse = {
	token: string
	record: Record<string, unknown>
}

export const buildApiUrl = (path: string) => `${apiBaseUrl}${path.startsWith("/") ? path : `/${path}`}`

export const expectOkJson = async <T>(response: APIResponse) => {
	if (!response.ok()) {
		const body = await response.text()
		throw new Error(`API request failed: ${response.status()} ${response.statusText()} ${body}`)
	}
	return (await response.json()) as T
}

export const authHeaders = (token: string) => ({
	Authorization: token,
})

export const login = async (request: APIRequestContext) => {
	const response = await request.post(buildApiUrl("/api/collections/users/auth-with-password"), {
		data: {
			identity: email,
			password,
		},
	})
	const data = await expectOkJson<AuthResponse>(response)
	if (!data.token || !data.record) {
		throw new Error("Auth response missing token or record.")
	}
	return data
}

export const setAuthStorage = async (page: Page, auth: AuthResponse) => {
	await page.addInitScript(
		(payload) => {
			localStorage.clear()
			sessionStorage.clear()
			localStorage.setItem("lang", "zh-CN")
			localStorage.setItem("pocketbase_auth", JSON.stringify({ token: payload.token, record: payload.record }))
		},
		{ token: auth.token, record: auth.record }
	)
}

export const createCollection = async (
	request: APIRequestContext,
	token: string,
	payload: {
		name: string
		base_url: string
		description?: string
		sort_order?: number
		tags?: string[]
	}
) => {
	const response = await request.post(buildApiUrl("/api/collections/api_test_collections/records"), {
		headers: authHeaders(token),
		data: {
			description: "",
			sort_order: 0,
			tags: [],
			...payload,
		},
	})
	return expectOkJson<{ id: string; name: string }>(response)
}

export const createCase = async (
	request: APIRequestContext,
	token: string,
	payload: {
		collection: string
		name: string
		url: string
		method?: string
		body_type?: string
		description?: string
		headers?: Array<{ key: string; value: string; enabled: boolean }>
		params?: Array<{ key: string; value: string; enabled: boolean }>
		expected_status?: number
		timeout_ms?: number
		schedule_enabled?: boolean
		schedule_minutes?: number
		sort_order?: number
		tags?: string[]
		alert_threshold?: number
	}
) => {
	const response = await request.post(buildApiUrl("/api/collections/api_test_cases/records"), {
		headers: authHeaders(token),
		data: {
			method: "GET",
			body_type: "json",
			description: "",
			headers: [],
			params: [],
			expected_status: 200,
			timeout_ms: 15000,
			schedule_enabled: false,
			schedule_minutes: 5,
			sort_order: 0,
			tags: [],
			alert_threshold: 1,
			...payload,
		},
	})
	return expectOkJson<{ id: string; name: string }>(response)
}

export const exportApiTests = async (request: APIRequestContext, token: string) => {
	const response = await request.get(buildApiUrl("/api/aether/api-tests/export"), {
		headers: authHeaders(token),
	})
	return expectOkJson<{
		collections: Array<Record<string, unknown>>
		cases: Array<Record<string, unknown>>
	}>(response)
}

export const importApiTests = async (
	request: APIRequestContext,
	token: string,
	payload: {
		mode: "skip" | "overwrite"
		data: Record<string, unknown>
	}
) => {
	const response = await request.post(buildApiUrl("/api/aether/api-tests/import"), {
		headers: authHeaders(token),
		data: payload,
	})
	return expectOkJson<{
		collections: { created: number; updated: number; skipped: number }
		cases: { created: number; updated: number; skipped: number }
	}>(response)
}

export const getRecord = async <T>(
	request: APIRequestContext,
	token: string,
	collection: string,
	recordId: string
) => {
	if (!recordId) {
		throw new Error("recordId is required")
	}
	const recordUrl = buildApiUrl(`/api/collections/${encodeURIComponent(collection)}/records/${recordId}`)
	const response = await request.get(recordUrl, { headers: authHeaders(token) })
	return expectOkJson<T>(response)
}

export const deleteRecordIfExists = async (
	request: APIRequestContext,
	token: string,
	collection: string,
	recordId?: string
) => {
	if (!recordId) {
		return
	}
	const recordUrl = buildApiUrl(`/api/collections/${encodeURIComponent(collection)}/records/${recordId}`)
	const checkResponse = await request.get(recordUrl, { headers: authHeaders(token) })
	if (checkResponse.status() === 404) {
		return
	}
	if (!checkResponse.ok()) {
		throw new Error(`Failed to check record: ${checkResponse.status()} ${await checkResponse.text()}`)
	}
	const deleteResponse = await request.delete(recordUrl, { headers: authHeaders(token) })
	if (!deleteResponse.ok()) {
		throw new Error(`Failed to delete record: ${deleteResponse.status()} ${await deleteResponse.text()}`)
	}
}

export const findRecordIdByName = async (
	request: APIRequestContext,
	token: string,
	collection: string,
	name: string
) => {
	const url = new URL(buildApiUrl(`/api/collections/${encodeURIComponent(collection)}/records`))
	url.searchParams.set("perPage", "1")
	url.searchParams.set("page", "1")
	url.searchParams.set("filter", `name=${JSON.stringify(name)}`)

	const response = await request.get(url.toString(), { headers: authHeaders(token) })
	const data = await expectOkJson<{
		items: Array<{ id: string }>
		page: number
		perPage: number
		totalItems: number
		totalPages: number
	}>(response)
	return data.items?.[0]?.id || ""
}

export const getSchedule = async (request: APIRequestContext, token: string) => {
	const response = await request.get(buildApiUrl("/api/aether/api-tests/schedule"), { headers: authHeaders(token) })
	return expectOkJson<{
		id: string
		enabled: boolean
		intervalMinutes: number
		lastRunAt: string
		nextRunAt: string
		lastError: string
		alertEnabled: boolean
		alertOnRecover: boolean
		historyRetentionDays: number
	}>(response)
}

export const updateSchedule = async (
	request: APIRequestContext,
	token: string,
	payload: {
		enabled: boolean
		intervalMinutes: number
		alertEnabled: boolean
		alertOnRecover: boolean
		historyRetentionDays: number
	}
) => {
	const response = await request.put(buildApiUrl("/api/aether/api-tests/schedule"), {
		headers: authHeaders(token),
		data: payload,
	})
	return expectOkJson<{
		id: string
		enabled: boolean
		intervalMinutes: number
		lastRunAt: string
		nextRunAt: string
		lastError: string
		alertEnabled: boolean
		alertOnRecover: boolean
		historyRetentionDays: number
	}>(response)
}

export const runCase = async (request: APIRequestContext, token: string, caseId: string) => {
	const response = await request.post(buildApiUrl("/api/aether/api-tests/run-case"), {
		headers: authHeaders(token),
		data: { caseId },
	})
	return expectOkJson<{
		caseId: string
		collectionId: string
		name: string
		status: number
		durationMs: number
		success: boolean
		error: string
		responseSnippet: string
		runAt: string
	}>(response)
}

export const runAll = async (request: APIRequestContext, token: string) => {
	const response = await request.post(buildApiUrl("/api/aether/api-tests/run-all"), {
		headers: authHeaders(token),
	})
	return expectOkJson<{
		collections: number
		cases: number
		success: number
		failed: number
		results: Array<{ name: string; status: number; durationMs: number; success: boolean }>
	}>(response)
}

export const runCollection = async (request: APIRequestContext, token: string, collectionId: string) => {
	const response = await request.post(buildApiUrl("/api/aether/api-tests/run-collection"), {
		headers: authHeaders(token),
		data: { collectionId },
	})
	return expectOkJson<{
		collectionId: string
		collection: string
		cases: number
		success: number
		failed: number
		results: Array<{ name: string; status: number; durationMs: number; success: boolean }>
	}>(response)
}

export const listRuns = async (
	request: APIRequestContext,
	token: string,
	params: { caseId?: string; collectionId?: string }
) => {
	const url = new URL(buildApiUrl("/api/aether/api-tests/runs"))
	if (params.caseId) {
		url.searchParams.set("case", params.caseId)
	}
	if (params.collectionId) {
		url.searchParams.set("collection", params.collectionId)
	}
	const response = await request.get(url.toString(), { headers: authHeaders(token) })
	return expectOkJson<{
		items: Array<{
			id: string
			caseId: string
			collectionId: string
			status: number
			durationMs: number
			success: boolean
			error: string
			responseSnippet: string
			source: string
			created: string
		}>
		page: number
		perPage: number
		totalItems: number
		totalPages: number
	}>(response)
}

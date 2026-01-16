// api-tests.ts 封装接口管理相关请求。
// 对接后端实现：internal/hub/api_tests.go
import { pb } from "@/lib/api"
import type {
	ApiTestCollectionRecord,
	ApiTestCaseRecord,
	ApiTestRunAllSummary,
	ApiTestCollectionRunSummary,
	ApiTestRunResult,
	ApiTestScheduleConfig,
	ApiTestRunList,
} from "@/types"

export const listApiTestCollections = () =>
	pb.collection("api_test_collections").getFullList<ApiTestCollectionRecord>({
		sort: "sort_order,created",
	})

export const listApiTestCases = (collectionId?: string) =>
	pb.collection("api_test_cases").getFullList<ApiTestCaseRecord>({
		filter: collectionId ? `collection = "${collectionId}"` : "",
		sort: "collection,sort_order,created",
	})

export const createApiTestCollection = (payload: Partial<ApiTestCollectionRecord>) =>
	pb.collection("api_test_collections").create<ApiTestCollectionRecord>(payload)

export const updateApiTestCollection = (id: string, payload: Partial<ApiTestCollectionRecord>) =>
	pb.collection("api_test_collections").update<ApiTestCollectionRecord>(id, payload)

export const deleteApiTestCollection = (id: string) => pb.collection("api_test_collections").delete(id)

export const createApiTestCase = (payload: Partial<ApiTestCaseRecord>) =>
	pb.collection("api_test_cases").create<ApiTestCaseRecord>(payload)

export const updateApiTestCase = (id: string, payload: Partial<ApiTestCaseRecord>) =>
	pb.collection("api_test_cases").update<ApiTestCaseRecord>(id, payload)

export const deleteApiTestCase = (id: string) => pb.collection("api_test_cases").delete(id)

export const fetchApiTestSchedule = () => pb.send<ApiTestScheduleConfig>("/api/aether/api-tests/schedule", {})

export const updateApiTestSchedule = (payload: {
	enabled?: boolean
	intervalMinutes?: number
	alertEnabled?: boolean
	alertOnRecover?: boolean
	historyRetentionDays?: number
}) =>
	pb.send<ApiTestScheduleConfig>("/api/aether/api-tests/schedule", {
		method: "PUT",
		body: payload,
	})

export const runApiTestCase = (caseId: string) =>
	pb.send<ApiTestRunResult>("/api/aether/api-tests/run-case", {
		method: "POST",
		body: { caseId },
	})

export const runApiTestCollection = (collectionId: string) =>
	pb.send<ApiTestCollectionRunSummary>("/api/aether/api-tests/run-collection", {
		method: "POST",
		body: { collectionId },
	})

export const runAllApiTests = () =>
	pb.send<ApiTestRunAllSummary>("/api/aether/api-tests/run-all", {
		method: "POST",
	})

export const listApiTestRuns = (params: {
	caseId?: string
	collectionId?: string
	page?: number
	perPage?: number
}) =>
	pb.send<ApiTestRunList>("/api/aether/api-tests/runs", {
		query: {
			...(params.caseId ? { case: params.caseId } : {}),
			...(params.collectionId ? { collection: params.collectionId } : {}),
			...(params.page ? { page: String(params.page) } : {}),
			...(params.perPage ? { perPage: String(params.perPage) } : {}),
		},
	})

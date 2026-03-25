import { pb } from "@/lib/api"

export type IngestMonitorScope = {
	tenant: string
	recordType: string
}

export type IngestMonitorSummaryCounts = {
	total: number
	success: number
	failure: number
	pending: number
	unknown: number
}

export type IngestMonitorRecord = {
	itemCode: string
	productName: string
	status: "success" | "failure" | "pending" | "unknown"
	isComplete?: number
	isTemporary: boolean
	errorMsg: string
	sourceFilePath: string
	convertedFilePath: string
	pcAddress: string
	glbAddress: string
	hasSourceFilePath: boolean
	hasConvertedFile: boolean
	hasPcAddress: boolean
	hasGlbAddress: boolean
	pathReadyCount: number
	pathReadyTotal: number
	inferenceTypes: number[]
	updateTime: string
	createTime: string
}

export type IngestMonitorSummaryResponse = {
	scope: IngestMonitorScope
	summary: IngestMonitorSummaryCounts
	recent: IngestMonitorRecord[]
	failures: IngestMonitorRecord[]
}

export type IngestMonitorDetailResponse = {
	scope: IngestMonitorScope
	item: IngestMonitorRecord
}

export const fetchIngestMonitorSummary = () =>
	pb.send<IngestMonitorSummaryResponse>("/api/aether/ingest-monitor/summary", { method: "GET" })

export const fetchIngestMonitorDetail = (itemCode: string) =>
	pb.send<IngestMonitorDetailResponse>("/api/aether/ingest-monitor/detail", {
		method: "GET",
		query: { itemCode },
	})

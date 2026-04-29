import { pb } from "@/lib/api"

export type IngestMonitorScope = {
	tenant: string
	recordType: string
}

export type IngestMonitorSummaryCounts = {
	total: number
	success: number
	failure: number
	processing: number
}

export type IngestMonitorRecord = {
	itemCode: string
	productName: string
	status: "success" | "failure" | "processing"
	isComplete?: number
	isTemporary: boolean
	hasFormalRecord: boolean
	recordSource: string
	cadNumber: string
	fileName: string
	processStatus: string
	batchRunId: string
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
	stageStatus: string
	diagnosticMessage: string
	missingPaths: string[]
	processStartTime: string
	processEndTime: string
	productUpdateTime: string
	isStalled: boolean
	stalledMinutes?: number
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

export type IngestMonitorBatch = {
	batchRunId: string
	sourceType: string
	xxlJobId: string
	xxlLogId: string
	scanPaths: string[]
	fileType?: number
	batchSize: number
	force: boolean
	status: "pending" | "running" | "completed" | "failed" | string
	errorMessage: string
	scanStartedAt: string
	scanFinishedAt: string
	xxlScanElapsedSeconds?: number
	finalIngestElapsedSeconds?: number
	totalDirsScanned: number
	totalFilesScanned: number
	totalFilesFiltered: number
	totalFilesLargeFiltered: number
	totalFilesRegistered: number
	totalFilesRegisterFailed: number
	totalFilesEnqueued: number
	totalFilesEnqueueFailed: number
	totalFilesProcessed: number
	totalBatches: number
	totalTracked: number
	successCount: number
	failureCount: number
	processingCount: number
}

export type IngestMonitorBatchItem = {
	itemCode: string
	cadNumber: string
	fileName: string
	processStatus: string
	ingestStatus: "success" | "failure" | "processing"
	productName: string
	isComplete?: number
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
	stageStatus: string
	diagnosticMessage: string
	missingPaths: string[]
	hasFormalRecord: boolean
	isStalled: boolean
	stalledMinutes?: number
	processStartTime: string
	processEndTime: string
	productUpdateTime: string
	updateTime: string
	createTime: string
}

export type IngestMonitorBatchListResponse = {
	scope: IngestMonitorScope
	batches: IngestMonitorBatch[]
}

export type IngestMonitorBatchDetailResponse = {
	scope: IngestMonitorScope
	batch: IngestMonitorBatch
	items: IngestMonitorBatchItem[]
	totalItems: number
	pageSize: number
	nextCursor: string
	hasMore: boolean
}

export const fetchIngestMonitorSummary = () =>
	pb.send<IngestMonitorSummaryResponse>("/api/aether/ingest-monitor/summary", {
		method: "GET",
		requestKey: null,
	})

export const fetchIngestMonitorDetail = (itemCode: string) =>
	pb.send<IngestMonitorDetailResponse>("/api/aether/ingest-monitor/detail", {
		method: "GET",
		query: { itemCode },
		requestKey: null,
	})

export const fetchIngestMonitorBatches = () =>
	pb.send<IngestMonitorBatchListResponse>("/api/aether/ingest-monitor/batches", {
		method: "GET",
		requestKey: null,
	})

export const fetchIngestMonitorBatchDetail = (batchRunId: string, cursor = "") =>
	pb.send<IngestMonitorBatchDetailResponse>("/api/aether/ingest-monitor/batch-detail", {
		method: "GET",
		query: cursor ? { batchRunId, cursor } : { batchRunId },
		requestKey: null,
	})

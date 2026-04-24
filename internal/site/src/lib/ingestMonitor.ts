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
	pendingCount: number
	formalPendingCount: number
	localProcessingCount: number
	localCompletedCount: number
	localFailedCount: number
	queuedCount: number
}

export type IngestMonitorBatchItem = {
	itemCode: string
	cadNumber: string
	fileName: string
	processStatus: string
	ingestStatus: "success" | "failure" | "pending" | "unknown"
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
}

export const fetchIngestMonitorSummary = () =>
	pb.send<IngestMonitorSummaryResponse>("/api/aether/ingest-monitor/summary", { method: "GET" })

export const fetchIngestMonitorDetail = (itemCode: string) =>
	pb.send<IngestMonitorDetailResponse>("/api/aether/ingest-monitor/detail", {
		method: "GET",
		query: { itemCode },
	})

export const fetchIngestMonitorBatches = () =>
	pb.send<IngestMonitorBatchListResponse>("/api/aether/ingest-monitor/batches", { method: "GET" })

export const fetchIngestMonitorBatchDetail = (batchRunId: string) =>
	pb.send<IngestMonitorBatchDetailResponse>("/api/aether/ingest-monitor/batch-detail", {
		method: "GET",
		query: { batchRunId },
	})

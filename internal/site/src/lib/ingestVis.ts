import { pb } from "@/lib/api"

export type IngestVisEvent = {
	id: string
	timestamp: string
	service: string
	action: string
	outcome: string
	itemCode: string
	traceId?: string
	message: string
	processType?: number
	ingest?: string
	insertType?: number[]
	force?: number
	fileType?: number
	taskId?: string
	inferTaskId?: string
	inferResultPath?: string
	inferType?: number
	rabbitmqQueue?: string
	rabbitmqDeliveryTag?: number
	rabbitmqRedelivered?: boolean
	rabbitmqRequeue?: boolean
	errorMessage?: string
	errorType?: string
	errorStackTrace?: string
}

export type IngestVisRun = {
	key: string
	itemCode: string
	traceId?: string
	stage: "mq" | "minio" | "infer" | "es" | "out" | "trash" | "other"
	status: "running" | "success" | "failure"
	lastEvent: IngestVisEvent
}

export type IngestVisCacheStatus = {
	enabled: boolean
	started: boolean
	pollIntervalMs: number
	defaultWindowSec: number
	cacheTtlSec: number
	maxEvents: number
	index: string
	lastQueryAt: string
	lastPollAt: string
	lastErrorAt: string
	lastError: string
	truncated: boolean
	runsCount: number
	seenCount: number
}

export const fetchIngestVisRuns = (params?: { windowSec?: number; limit?: number }) =>
	pb.send<{ items: IngestVisRun[] }>("/api/aether/ingest-vis/runs", {
		query: params ?? {},
	})

export const fetchIngestVisEvents = (params: { itemCode: string; traceId?: string; windowSec?: number; limit?: number }) =>
	pb.send<{ items: IngestVisEvent[]; truncated: boolean }>("/api/aether/ingest-vis/events", {
		query: params,
	})

export const fetchIngestVisCacheStatus = () =>
	pb.send<IngestVisCacheStatus>("/api/aether/ingest-vis/cache/status", { method: "GET" })

export const clearIngestVisCache = () =>
	pb.send<{ status: "ok" }>("/api/aether/ingest-vis/cache/clear", { method: "POST" })

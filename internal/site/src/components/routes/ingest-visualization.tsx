// ingest-visualization.tsx 入库服务可视化（基于 logs-workflow-*）。
// - 实时：每 1s 拉取 runs（Hub 内存缓存聚合）
// - 回放：按 itemCode(+traceId) 查询事件列表并支持播放（弹窗）
import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react"
import { ActiveAlerts } from "@/components/active-alerts"
import { FooterRepoLink } from "@/components/footer-repo-link"
import { IngestPipeline, type IngestPipelineStage } from "@/components/ingest-vis/pipeline"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { toast } from "@/components/ui/use-toast"
import {
	clearIngestVisCache,
	fetchIngestVisCacheStatus,
	fetchIngestVisEvents,
	fetchIngestVisRuns,
	type IngestVisEvent,
	type IngestVisRun,
} from "@/lib/ingestVis"
import { BRAND_NAME, cn } from "@/lib/utils"
import { LoaderCircleIcon, PauseCircleIcon, PlayCircleIcon, RefreshCwIcon, Trash2Icon } from "lucide-react"

function pipelineNodeFromEvent(e: Pick<IngestVisEvent, "action" | "outcome" | "insertType">): IngestPipelineStage {
	switch (e.action) {
		case "mq.preprocess_message.consume":
			return "mq.preprocess_message.consume"
		case "mq.preprocess_message.validate":
			return "mq.preprocess_message.validate"
		case "mq.preprocess_message.ack":
			return "mq.preprocess_message.ack"
		case "mq.preprocess_message.nack":
			return "mq.preprocess_message.nack"
		case "mq.preprocess_message.parse":
			return "mq.preprocess_message.parse"
		case "minio.message.validate":
			return "minio.message.validate"
		case "minio.message.handle":
			return "minio.message.handle"
		case "minio.task.submit":
			return "minio.task.submit"
		case "minio.task.start":
			return "minio.task.start"
		case "minio.task.end":
			return "minio.task.end"
		case "minio.task.retry":
			return "minio.task.retry"
		case "minio.task.locked":
			return "minio.task.locked"
		case "minio.task.skip":
			return "minio.task.skip"
		case "minio.task.not_found":
			return "minio.task.not_found"
		case "minio.query.prepare":
			return "minio.query.prepare"
		case "minio.query.execute":
			return "minio.query.execute"
		case "minio.upload_only.execute":
			return "minio.upload_only.execute"
		case "minio.ingest.skip":
			return "minio.ingest.skip"
		case "infer.request":
			return "infer.request"
		default:
			break
	}
	return "other"
}

function pipelineStatusFromEvent(
	e: Pick<IngestVisEvent, "action" | "outcome" | "insertType">
): "running" | "success" | "failure" {
	if (e.outcome === "failure") return "failure"
	if (e.outcome === "success") {
		if (e.action === "minio.task.end") return (e.insertType ?? []).length === 0 ? "success" : "running"
		if (
			e.action === "mq.preprocess_message.ack" ||
			e.action === "minio.ingest.skip" ||
			e.action === "minio.upload_only.execute" ||
			e.action === "minio.query.execute"
		)
			return "success"
	}
	return "running"
}

function pipelineLabel(node: IngestPipelineStage): string {
	switch (node) {
		case "mq.preprocess_message.consume":
			return "MQ：消费"
		case "mq.preprocess_message.validate":
			return "MQ：校验"
		case "mq.preprocess_message.ack":
			return "MQ：确认"
		case "mq.preprocess_message.nack":
			return "MQ：拒收"
		case "mq.preprocess_message.parse":
			return "MQ：解析失败"
		case "minio.message.validate":
			return "MinIO：消息校验"
		case "minio.message.handle":
			return "MinIO：消息异常"
		case "minio.task.submit":
			return "MinIO：任务提交"
		case "minio.task.start":
			return "MinIO：任务开始"
		case "minio.task.end":
			return "MinIO：任务结束"
		case "minio.task.retry":
			return "MinIO：任务重试"
		case "minio.task.locked":
			return "MinIO：任务锁定"
		case "minio.task.skip":
			return "MinIO：状态跳过"
		case "minio.task.not_found":
			return "MinIO：任务不存在"
		case "minio.query.prepare":
			return "MinIO：查询准备"
		case "minio.query.execute":
			return "MinIO：查询执行"
		case "minio.upload_only.execute":
			return "MinIO：仅上传"
		case "minio.ingest.skip":
			return "MinIO：智能跳过"
		case "infer.request":
			return "3D Search：推理请求"
		case "other":
			return "未知事件"
		default:
			return "未知事件"
	}
}

function statusBadgeVariant(status: IngestVisRun["status"]) {
	if (status === "success") return "success"
	if (status === "failure") return "danger"
	return "secondary"
}

function statusText(status: IngestVisRun["status"]) {
	if (status === "success") return "成功"
	if (status === "failure") return "失败"
	return "进行中"
}

function formatTs(ts: string) {
	const d = new Date(ts)
	if (Number.isNaN(d.getTime())) return ts
	return d.toLocaleString()
}

function formatWindowSec(sec: number) {
	if (sec >= 3600) return `${Math.round(sec / 3600)} 小时`
	if (sec >= 60) return `${Math.round(sec / 60)} 分钟`
	return `${sec} 秒`
}

export default memo(() => {
	const runsLoadedOnce = useRef(false)
	const serviceLanes = [
		{ key: "xxl_job_executor", label: "xxl_job_executor", status: "未接入日志" },
		{ key: "minio-api", label: "minio-api", status: "已接入" },
		{ key: "3d-search-core", label: "3d-search-core", status: "未接入日志" },
		{ key: "3d-elasticsearch", label: "3d-elasticsearch", status: "未接入日志" },
	]

	// realtime
	const [realtimeWindowSec, setRealtimeWindowSec] = useState(10 * 60)
	const [runs, setRuns] = useState<IngestVisRun[]>([])
	const [runsLoading, setRunsLoading] = useState(false)
	const [runsError, setRunsError] = useState<string>("")

	// replay
	const [replayItemCode, setReplayItemCode] = useState("")
	const [replayTraceId, setReplayTraceId] = useState("")
	const [replayWindowSec, setReplayWindowSec] = useState(24 * 60 * 60)
	const [events, setEvents] = useState<IngestVisEvent[]>([])
	const [eventsTruncated, setEventsTruncated] = useState(false)
	const [eventsLoading, setEventsLoading] = useState(false)
	const [cursor, setCursor] = useState(0)
	const [playing, setPlaying] = useState(false)
	const [speed, setSpeed] = useState<0.5 | 1 | 2 | 4>(1)
	const [eventsDialogOpen, setEventsDialogOpen] = useState(false)
	const [replayDialogOpen, setReplayDialogOpen] = useState(false)
	const [pipelineDialogOpen, setPipelineDialogOpen] = useState(false)

	// cache status
	const [cacheStatus, setCacheStatus] = useState<{
		lastError?: string
		truncated?: boolean
	}>({})

	useEffect(() => {
		document.title = `${BRAND_NAME} - 入库服务可视化`
	}, [])

	const openReplay = useCallback((run: IngestVisRun) => {
		setReplayItemCode(run.itemCode)
		setReplayTraceId(run.traceId ?? "")
		setReplayWindowSec(24 * 60 * 60)
		setEvents([])
		setEventsTruncated(false)
		setCursor(0)
		setPlaying(false)
		toast({
			title: "已选中回放对象",
			description: `${run.itemCode}${run.traceId ? " · trace" : ""}（点击“查询”查看事件列表，点击“播放”弹窗回放动画）`,
		})
	}, [])

	const refreshCacheStatus = useCallback(async () => {
		try {
			const status = await fetchIngestVisCacheStatus()
			setCacheStatus({ lastError: status.lastError, truncated: status.truncated })
		} catch (err) {
			console.error("fetch ingest-vis cache status failed", err)
		}
	}, [])

	const pollRuns = useCallback(
		async ({ showLoading }: { showLoading: boolean }) => {
			if (showLoading) setRunsLoading(true)
			setRunsError("")
			try {
				const res = await fetchIngestVisRuns({ windowSec: realtimeWindowSec, limit: 800 })
				setRuns(res.items ?? [])
			} catch (err) {
				console.error("fetch ingest-vis runs failed", err)
				setRunsError(err instanceof Error ? err.message : String(err))
			} finally {
				if (showLoading) setRunsLoading(false)
			}
		},
		[realtimeWindowSec]
	)

	useEffect(() => {
		let cancelled = false
		let timer: number | undefined

		const run = async (showLoading: boolean) => {
			if (cancelled) return
			await pollRuns({ showLoading })
			await refreshCacheStatus()
		}

		const first = !runsLoadedOnce.current
		runsLoadedOnce.current = true

		void run(first)
		timer = window.setInterval(() => void run(false), 1000)
		return () => {
			cancelled = true
			if (timer) window.clearInterval(timer)
		}
	}, [pollRuns, refreshCacheStatus])

	const realtimeBoard = useMemo(() => {
		const cols: Record<"mq" | "processing" | "out" | "trash", IngestVisRun[]> = {
			mq: [],
			processing: [],
			out: [],
			trash: [],
		}

		for (const r of runs) {
			if (r.stage === "mq") cols.mq.push(r)
			else if (r.stage === "out") cols.out.push(r)
			else if (r.stage === "trash") cols.trash.push(r)
			else cols.processing.push(r)
		}

		const byTimeDesc = (a: IngestVisRun, b: IngestVisRun) => (a.lastEvent.timestamp < b.lastEvent.timestamp ? 1 : -1)
		cols.mq.sort(byTimeDesc)
		cols.processing.sort(byTimeDesc)
		cols.out.sort(byTimeDesc)
		cols.trash.sort(byTimeDesc)

		return cols
	}, [runs])

	const fetchReplay = useCallback(async () => {
		const itemCode = replayItemCode.trim()
		const traceId = replayTraceId.trim()
		if (!itemCode) {
			toast({ variant: "destructive", title: "参数错误", description: "itemCode 为必填" })
			return
		}

		setEventsLoading(true)
		setEvents([])
		setEventsTruncated(false)
		setCursor(0)
		setPlaying(false)
		try {
			const res = await fetchIngestVisEvents({
				itemCode,
				traceId: traceId || undefined,
				windowSec: replayWindowSec,
				limit: 5000,
			})
			setEvents(res.items ?? [])
			setEventsTruncated(!!res.truncated)
		} catch (err) {
			console.error("fetch ingest-vis events failed", err)
			toast({
				variant: "destructive",
				title: "查询失败",
				description: err instanceof Error ? err.message : String(err),
			})
			throw err
		} finally {
			setEventsLoading(false)
		}
	}, [replayItemCode, replayTraceId, replayWindowSec])

	const openEventsDialog = useCallback(async () => {
		try {
			await fetchReplay()
			setEventsDialogOpen(true)
		} catch {
			// toast 已提示
		}
	}, [fetchReplay])

	useEffect(() => {
		if (!playing) return
		if (!events.length) return
		const interval = Math.max(120, Math.floor(600 / speed))
		const timer = window.setInterval(() => {
			setCursor((cur) => {
				const next = cur + 1
				if (next >= events.length) {
					setPlaying(false)
					return cur
				}
				return next
			})
		}, interval)
		return () => window.clearInterval(timer)
	}, [playing, events.length, speed])

	const clearCache = useCallback(async () => {
		try {
			await clearIngestVisCache()
			toast({ title: "已清空缓存", description: "已重置 ingest-vis 的内存状态（不会影响 ES）" })
			await pollRuns({ showLoading: true })
			await refreshCacheStatus()
		} catch (err) {
			console.error("clear ingest-vis cache failed", err)
			toast({
				variant: "destructive",
				title: "清空失败",
				description: err instanceof Error ? err.message : String(err),
			})
		}
	}, [pollRuns, refreshCacheStatus])

	const currentEvent = events[cursor]
	const currentPipelineNode = currentEvent ? pipelineNodeFromEvent(currentEvent) : "other"
	const replayTokenStatus = currentEvent ? pipelineStatusFromEvent(currentEvent) : "running"

	const replayPipelineTokens = useMemo(
		() =>
			replayItemCode.trim()
				? [{ key: "replay", label: replayItemCode.trim(), stage: currentPipelineNode, status: replayTokenStatus }]
				: [],
		[replayItemCode, currentPipelineNode, replayTokenStatus]
	)

	const realtimePipelineTokens = useMemo(
		() =>
			(runs ?? []).slice(0, 12).map((r) => ({
				key: r.key,
				label: r.traceId ? `${r.itemCode} · trace` : r.itemCode,
				stage: pipelineNodeFromEvent(r.lastEvent),
				status: pipelineStatusFromEvent(r.lastEvent),
			})),
		[runs]
	)

	return (
		<>
			<div className="grid gap-4">
				<ActiveAlerts />

				<Card className="border-border/60 bg-gradient-to-r from-background via-background to-muted/30">
					<CardHeader className="pb-3">
						<CardTitle className="text-2xl">入库服务可视化</CardTitle>
						<CardDescription>
							节点与 <code className="text-xs">event.action</code> 完全对齐，专注{" "}
							<code className="text-xs">logs-workflow-*</code> 日志链路（支持{" "}
							<code className="text-xs">workflow.trace_id</code>）。
						</CardDescription>
					</CardHeader>
					<CardContent className="flex flex-wrap items-center justify-between gap-3">
						<div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
							<Badge variant="outline" className="bg-background/70">
								实时窗口：{formatWindowSec(realtimeWindowSec)}
							</Badge>
							<Badge variant="outline" className="bg-background/70">
								回放窗口：{formatWindowSec(replayWindowSec)}
							</Badge>
							<Badge variant="outline" className="bg-background/70">
								运行项：{runs.length}
							</Badge>
							{runsError && <Badge variant="danger">加载失败：{runsError}</Badge>}
						</div>
						<div className="flex flex-wrap items-center gap-2">
							<Button onClick={() => void pollRuns({ showLoading: true })} variant="outline" disabled={runsLoading}>
								{runsLoading ? (
									<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
								) : (
									<RefreshCwIcon className="me-2 h-4 w-4" />
								)}
								刷新
							</Button>
							<Button onClick={() => void clearCache()} variant="outline">
								<Trash2Icon className="me-2 h-4 w-4" />
								清空缓存
							</Button>
							{cacheStatus.truncated && (
								<Badge variant="warning">⚠️ 采样被截断（建议调大 WORKFLOW_VIS_MAX_EVENTS）</Badge>
							)}
							{cacheStatus.lastError && <Badge variant="danger">ES 轮询错误：{cacheStatus.lastError}</Badge>}
						</div>
					</CardContent>
				</Card>

				<div className="grid gap-4 lg:grid-cols-[1.4fr,1fr]">
					<Card className="relative overflow-hidden">
						<CardHeader className="pb-3">
							<CardTitle className="text-base">流水线动画（实时）</CardTitle>
							<CardDescription>
								主线展示入库任务流转，分支展示查询/仅上传/智能跳过，异常分支覆盖 nack/消息异常/锁定与跳过。
							</CardDescription>
						</CardHeader>
						<CardContent className="grid gap-3">
							<div className="flex flex-wrap items-center gap-3 text-xs text-muted-foreground">
								<span className="inline-flex items-center gap-1">
									<span className="h-2 w-2 rounded-full bg-primary" />
									主线
								</span>
								<span className="inline-flex items-center gap-1">
									<span className="h-2 w-2 rounded-full bg-orange-500" />
									分支
								</span>
								<span className="inline-flex items-center gap-1">
									<span className="h-2 w-2 rounded-full bg-destructive" />
									异常
								</span>
								<span className="inline-flex items-center gap-1">
									<span className="h-2 w-2 rounded-full bg-primary animate-pulse" />
									进行中
								</span>
								<span className="inline-flex items-center gap-1">
									<span className="h-2 w-2 rounded-full bg-emerald-500" />
									成功
								</span>
								<span className="inline-flex items-center gap-1">
									<span className="h-2 w-2 rounded-full bg-destructive" />
									失败
								</span>
							</div>
							<div className="flex flex-col gap-3 rounded-lg border bg-muted/20 p-4">
								<div className="text-sm text-muted-foreground">
									当前画布较小，已改为全屏查看以提升节点与连线的可读性。
								</div>
								<div className="flex flex-wrap items-center gap-2">
									<Button onClick={() => setPipelineDialogOpen(true)}>
										全屏查看流水线
									</Button>
									<Badge variant="secondary">标记 {realtimePipelineTokens.length}</Badge>
								</div>
							</div>
						</CardContent>
					</Card>

					<Card>
						<CardHeader className="pb-3">
							<CardTitle className="text-base">控制台</CardTitle>
							<CardDescription>实时监控 + 事后回放，所有控件集中在右侧。</CardDescription>
						</CardHeader>
						<CardContent className="grid gap-4">
							<div className="grid gap-3 rounded-lg border bg-muted/20 p-3">
								<div className="flex items-center justify-between gap-2">
									<div className="text-sm font-semibold">实时监控</div>
									<Badge variant="secondary">1s 刷新</Badge>
								</div>
								<div className="grid gap-3 sm:grid-cols-2">
									<div className="space-y-2">
										<Label className="text-muted-foreground">时间窗</Label>
										<Select value={String(realtimeWindowSec)} onValueChange={(v) => setRealtimeWindowSec(Number(v))}>
											<SelectTrigger>
												<SelectValue />
											</SelectTrigger>
											<SelectContent>
												<SelectItem value={String(5 * 60)}>最近 5 分钟</SelectItem>
												<SelectItem value={String(10 * 60)}>最近 10 分钟</SelectItem>
												<SelectItem value={String(30 * 60)}>最近 30 分钟</SelectItem>
												<SelectItem value={String(60 * 60)}>最近 1 小时</SelectItem>
											</SelectContent>
										</Select>
									</div>
									<div className="space-y-2">
										<Label className="text-muted-foreground">运行统计</Label>
										<div className="flex flex-wrap items-center gap-2 text-xs">
											<Badge variant="secondary">入口 {realtimeBoard.mq.length}</Badge>
											<Badge variant="secondary">处理中 {realtimeBoard.processing.length}</Badge>
											<Badge variant="secondary">完成 {realtimeBoard.out.length}</Badge>
											<Badge variant="secondary">异常 {realtimeBoard.trash.length}</Badge>
										</div>
									</div>
								</div>
							</div>

							<div className="grid gap-3 rounded-lg border bg-muted/20 p-3">
								<div className="flex items-center justify-between gap-2">
									<div className="text-sm font-semibold">回放控制</div>
									<Badge variant="secondary">事件 {events.length}</Badge>
								</div>
								<div className="grid gap-3 sm:grid-cols-2">
									<div className="space-y-2 sm:col-span-2">
										<Label>itemCode</Label>
										<Input
											value={replayItemCode}
											onChange={(e) => setReplayItemCode(e.target.value)}
											placeholder="例如：7cd3-... 或业务编码"
										/>
									</div>
									<div className="space-y-2">
										<Label>trace_id（可选）</Label>
										<Input
											value={replayTraceId}
											onChange={(e) => setReplayTraceId(e.target.value)}
											placeholder="workflow.trace_id"
										/>
									</div>
									<div className="space-y-2">
										<Label>回放时间窗</Label>
										<Select value={String(replayWindowSec)} onValueChange={(v) => setReplayWindowSec(Number(v))}>
											<SelectTrigger>
												<SelectValue />
											</SelectTrigger>
											<SelectContent>
												<SelectItem value={String(15 * 60)}>最近 15 分钟</SelectItem>
												<SelectItem value={String(60 * 60)}>最近 1 小时</SelectItem>
												<SelectItem value={String(6 * 60 * 60)}>最近 6 小时</SelectItem>
												<SelectItem value={String(24 * 60 * 60)}>最近 24 小时</SelectItem>
											</SelectContent>
										</Select>
									</div>
								</div>

								<div className="flex flex-wrap items-center gap-2">
									<Button onClick={() => void openEventsDialog()} disabled={eventsLoading}>
										{eventsLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : null}
										查询
									</Button>
									<Button
										variant="outline"
										onClick={() => {
											setReplayDialogOpen(true)
											setPlaying(true)
										}}
										disabled={!events.length}
									>
										<PlayCircleIcon className="me-2 h-4 w-4" />
										播放
									</Button>
									<Button
										variant="outline"
										onClick={() => {
											setCursor(0)
											setPlaying(false)
										}}
										disabled={!events.length}
									>
										重置
									</Button>
									<div className="flex items-center gap-2">
										<Label className="text-muted-foreground">倍速</Label>
										<Select value={String(speed)} onValueChange={(v) => setSpeed(Number(v) as 0.5 | 1 | 2 | 4)}>
											<SelectTrigger className="w-24">
												<SelectValue />
											</SelectTrigger>
											<SelectContent>
												<SelectItem value="0.5">0.5x</SelectItem>
												<SelectItem value="1">1x</SelectItem>
												<SelectItem value="2">2x</SelectItem>
												<SelectItem value="4">4x</SelectItem>
											</SelectContent>
										</Select>
									</div>
									<Badge variant="secondary">当前阶段：{pipelineLabel(currentPipelineNode)}</Badge>
									<Badge variant="secondary">进度：{events.length ? `${cursor + 1}/${events.length}` : "0/0"}</Badge>
									{eventsTruncated && <Badge variant="warning">⚠️ 查询结果被截断</Badge>}
								</div>
							</div>
						</CardContent>
					</Card>
				</div>

				<div className="grid gap-4 lg:grid-cols-4">
					{[
						{ key: "mq", title: "入口(MQ)", items: realtimeBoard.mq },
						{ key: "processing", title: "处理中", items: realtimeBoard.processing },
						{ key: "out", title: "完成", items: realtimeBoard.out },
						{ key: "trash", title: "异常", items: realtimeBoard.trash },
					].map((col) => (
						<Card key={col.key}>
							<CardHeader className="pb-3">
								<CardTitle className="text-base flex items-center justify-between gap-2">
									<span>{col.title}</span>
									<Badge variant="secondary">{col.items.length}</Badge>
								</CardTitle>
							</CardHeader>
							<CardContent className="space-y-2">
								{col.items.slice(0, 30).map((r) => (
									<button
										key={r.key}
										type="button"
										onClick={() => openReplay(r)}
										className={cn(
											"w-full rounded-md border bg-background/40 px-3 py-2 text-left hover:bg-accent/40 transition-colors",
											"flex items-start gap-2"
										)}
									>
										<div className="min-w-0 flex-1">
											<div className="font-mono text-sm truncate">
												{r.itemCode}
												{r.traceId ? " · trace" : ""}
											</div>
											<div className="text-xs text-muted-foreground truncate">
												{r.lastEvent.action} · {formatTs(r.lastEvent.timestamp)}
											</div>
										</div>
										<div className="shrink-0 flex flex-col items-end gap-1">
											<Badge variant={statusBadgeVariant(r.status)}>{statusText(r.status)}</Badge>
											<Badge variant="outline">{pipelineLabel(pipelineNodeFromEvent(r.lastEvent))}</Badge>
										</div>
									</button>
								))}
								{!col.items.length && <div className="text-sm text-muted-foreground">暂无数据</div>}
								{col.items.length > 30 && (
									<div className="text-xs text-muted-foreground">仅展示前 30 条（按最新事件排序）</div>
								)}
							</CardContent>
						</Card>
					))}
				</div>

			<Dialog open={eventsDialogOpen} onOpenChange={setEventsDialogOpen}>
					<DialogContent className="max-w-5xl">
						<DialogHeader>
							<DialogTitle>事件列表</DialogTitle>
							<DialogDescription>
								{replayItemCode.trim() ? (
									<span className="font-mono text-xs">
										itemCode={replayItemCode.trim()}
										{replayTraceId.trim() ? ` · traceId=${replayTraceId.trim()}` : ""}
									</span>
								) : (
									<span>请先输入 itemCode 并点击“查询”</span>
								)}
							</DialogDescription>
						</DialogHeader>

						<div className="flex flex-wrap items-center gap-2">
							<Badge variant="secondary">共 {events.length} 条</Badge>
							<Badge variant="secondary">当前阶段：{pipelineLabel(currentPipelineNode)}</Badge>
							<Badge variant="secondary">进度：{events.length ? `${cursor + 1}/${events.length}` : "0/0"}</Badge>
							{eventsTruncated && <Badge variant="warning">⚠️ 查询结果被截断</Badge>}
							<Button
								variant="outline"
								onClick={() => {
									setReplayDialogOpen(true)
									setPlaying(true)
								}}
								disabled={!events.length}
							>
								<PlayCircleIcon className="me-2 h-4 w-4" />
								播放
							</Button>
						</div>

						<div className="max-h-[60vh] overflow-auto rounded-md border">
							<Table>
								<TableHeader>
									<TableRow>
										<TableHead className="w-[210px]">时间</TableHead>
										<TableHead className="w-[160px]">服务</TableHead>
										<TableHead className="w-[220px]">action</TableHead>
										<TableHead className="w-[110px]">outcome</TableHead>
										<TableHead>message</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{events.map((ev, idx) => (
										<TableRow
											key={ev.id}
											className={cn(idx === cursor ? "bg-muted/60" : "", "cursor-pointer")}
											onClick={() => {
												setCursor(idx)
												setPlaying(false)
											}}
										>
											<TableCell className="font-mono text-xs">{formatTs(ev.timestamp)}</TableCell>
											<TableCell className="text-xs">{ev.service}</TableCell>
											<TableCell className="font-mono text-xs">{ev.action}</TableCell>
											<TableCell>
												<Badge
													variant={
														ev.outcome === "failure" ? "danger" : ev.outcome === "success" ? "success" : "secondary"
													}
												>
													{ev.outcome}
												</Badge>
											</TableCell>
											<TableCell className="text-xs">
												<div className="truncate max-w-[680px]">{ev.message || ev.errorMessage || "-"}</div>
											</TableCell>
										</TableRow>
									))}
									{!events.length && (
										<TableRow>
											<TableCell colSpan={5} className="text-sm text-muted-foreground">
												暂无数据
											</TableCell>
										</TableRow>
									)}
								</TableBody>
							</Table>
						</div>
					</DialogContent>
			</Dialog>

			<Dialog open={pipelineDialogOpen} onOpenChange={setPipelineDialogOpen}>
				<DialogContent className="h-[92vh] w-[96vw] max-w-[96vw] overflow-hidden p-0">
					<div className="flex h-full flex-col">
						<DialogHeader className="px-6 pb-3 pt-6">
							<DialogTitle>流水线动画（全屏）</DialogTitle>
							<DialogDescription>节点与连线已按真实日志链路对齐，支持悬停查看节点详情。</DialogDescription>
						</DialogHeader>
						<div className="flex-1 px-6 pb-6">
							<div className="mb-4 grid grid-cols-2 gap-2 rounded-lg border bg-muted/20 p-3 text-xs text-muted-foreground md:grid-cols-4">
								{serviceLanes.map((lane) => (
									<div
										key={lane.key}
										className={cn(
											"flex items-center justify-between rounded-md border px-3 py-2",
											lane.label === "minio-api" ? "bg-primary/10 text-primary border-primary/30" : "bg-background/60"
										)}
									>
										<span className="font-semibold">{lane.label}</span>
										<Badge variant={lane.label === "minio-api" ? "secondary" : "outline"}>{lane.status}</Badge>
									</div>
								))}
							</div>
							<IngestPipeline tokens={realtimePipelineTokens} className="h-full" />
						</div>
					</div>
				</DialogContent>
			</Dialog>

				<Dialog
					open={replayDialogOpen}
					onOpenChange={(open) => {
						setReplayDialogOpen(open)
						if (!open) setPlaying(false)
					}}
				>
					<DialogContent className="max-w-5xl">
						<DialogHeader>
							<DialogTitle>流水线回放</DialogTitle>
							<DialogDescription>
								{replayItemCode.trim() ? (
									<span className="font-mono text-xs">
										itemCode={replayItemCode.trim()}
										{replayTraceId.trim() ? ` · traceId=${replayTraceId.trim()}` : ""}
									</span>
								) : (
									<span>请先查询并输入 itemCode</span>
								)}
							</DialogDescription>
						</DialogHeader>

						<div className="grid gap-4">
							<IngestPipeline tokens={replayPipelineTokens} className="h-[260px]" />

							<div className="flex flex-wrap items-center gap-2">
								<Button variant="outline" onClick={() => setPlaying((v) => !v)} disabled={!events.length}>
									{playing ? <PauseCircleIcon className="me-2 h-4 w-4" /> : <PlayCircleIcon className="me-2 h-4 w-4" />}
									{playing ? "暂停" : "播放"}
								</Button>
								<Button
									variant="outline"
									onClick={() => {
										setCursor(0)
										setPlaying(false)
									}}
									disabled={!events.length}
								>
									重置
								</Button>
								<div className="flex items-center gap-2">
									<Label className="text-muted-foreground">倍速</Label>
									<Select value={String(speed)} onValueChange={(v) => setSpeed(Number(v) as 0.5 | 1 | 2 | 4)}>
										<SelectTrigger className="w-28">
											<SelectValue />
										</SelectTrigger>
										<SelectContent>
											<SelectItem value="0.5">0.5x</SelectItem>
											<SelectItem value="1">1x</SelectItem>
											<SelectItem value="2">2x</SelectItem>
											<SelectItem value="4">4x</SelectItem>
										</SelectContent>
									</Select>
								</div>
								<Badge variant="secondary">当前阶段：{pipelineLabel(currentPipelineNode)}</Badge>
								<Badge variant="secondary">进度：{events.length ? `${cursor + 1}/${events.length}` : "0/0"}</Badge>
							</div>

							<Card>
								<CardHeader className="pb-3">
									<CardTitle className="text-base">当前事件</CardTitle>
								</CardHeader>
								<CardContent className="grid gap-2 text-sm">
									{currentEvent ? (
										<>
											<div className="flex flex-wrap items-center gap-2">
												<Badge variant="outline" className="font-mono text-xs">
													{formatTs(currentEvent.timestamp)}
												</Badge>
												<Badge variant="outline" className="text-xs">
													{currentEvent.service}
												</Badge>
												<Badge
													variant={
														currentEvent.outcome === "failure"
															? "danger"
															: currentEvent.outcome === "success"
																? "success"
																: "secondary"
													}
													className="text-xs"
												>
													{currentEvent.outcome}
												</Badge>
											</div>
											<div className="font-mono text-xs break-all">{currentEvent.action}</div>
											<div className="text-xs text-muted-foreground break-words">
												{currentEvent.message || currentEvent.errorMessage || "-"}
											</div>
										</>
									) : (
										<div className="text-sm text-muted-foreground">暂无事件</div>
									)}
								</CardContent>
							</Card>
						</div>
					</DialogContent>
				</Dialog>

				<FooterRepoLink />
			</div>
		</>
	)
})

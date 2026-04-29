import { 
	EyeIcon, 
	LoaderCircleIcon, 
	RefreshCwIcon, 
	DatabaseIcon, 
	CheckCircleIcon, 
	ChevronLeftIcon,
	ChevronRightIcon,
	XCircleIcon, 
	ClockIcon, 
	HelpCircleIcon,
	WorkflowIcon,
	AlertTriangleIcon,
	FolderIcon,
	TimerIcon,
	BarChart3Icon,
	FileCodeIcon,
	LayersIcon,
	RouteIcon,
	FileIcon,
	FileBoxIcon,
	BoxIcon,
	CalendarIcon,
	SparklesIcon,
	PackageIcon
} from "lucide-react"
import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react"
import { ActiveAlerts } from "@/components/active-alerts"
import { FooterRepoLink } from "@/components/footer-repo-link"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
	fetchIngestMonitorBatchDetail,
	fetchIngestMonitorBatches,
	fetchIngestMonitorDetail,
	fetchIngestMonitorSummary,
	type IngestMonitorBatch,
	type IngestMonitorBatchDetailResponse,
	type IngestMonitorBatchItem,
	type IngestMonitorBatchListResponse,
	type IngestMonitorDetailResponse,
	type IngestMonitorRecord,
	type IngestMonitorSummaryResponse,
} from "@/lib/ingestMonitor"
import { BRAND_NAME, formatShortDate, cn } from "@/lib/utils"

const refreshIntervalMs = 15000

function statusBadgeVariant(status: IngestMonitorRecord["status"] | IngestMonitorBatchItem["ingestStatus"]) {
	switch (status) {
		case "success":
			return "success"
		case "failure":
			return "danger"
		case "processing":
			return "secondary"
		default:
			return "outline"
	}
}

function statusLabel(status: IngestMonitorRecord["status"] | IngestMonitorBatchItem["ingestStatus"]) {
	switch (status) {
		case "success":
			return "成功"
		case "failure":
			return "失败"
		case "processing":
			return "处理中"
		default:
			return status || "-"
	}
}

function batchStatusBadgeVariant(status: IngestMonitorBatch["status"]) {
	switch (status) {
		case "completed":
			return "success"
		case "failed":
			return "danger"
		case "running":
			return "secondary"
		default:
			return "outline"
	}
}

function batchStatusLabel(status: IngestMonitorBatch["status"]) {
	switch (status) {
		case "completed":
			return "已结束"
		case "failed":
			return "失败"
		case "running":
			return "扫描中"
		default:
			return "待执行"
	}
}

function recordSourceLabel(source: string) {
	return source === "batch_tracking" ? "批次跟踪记录" : "正式入库记录"
}

function formatMissingPaths(paths: string[]) {
	return paths.length ? paths.join("、") : "-"
}

function formatStalledMinutes(minutes?: number) {
	if (!minutes || minutes <= 0) {
		return "-"
	}
	if (minutes < 60) {
		return `${minutes} 分钟`
	}
	const hours = Math.floor(minutes / 60)
	const remainMinutes = minutes % 60
	return remainMinutes > 0 ? `${hours} 小时 ${remainMinutes} 分钟` : `${hours} 小时`
}

function formatInferenceTypes(record: IngestMonitorRecord) {
	return record.inferenceTypes?.length ? record.inferenceTypes.join(", ") : "-"
}

function formatTenantLabel(tenant: string) {
	return tenant === "guochuang" ? "国创（guochuang）" : tenant || "-"
}

function formatDisplayDate(value: string) {
	if (!value) {
		return "-"
	}
	const timestamp = new Date(value)
	if (Number.isNaN(timestamp.getTime())) {
		return value
	}
	return formatShortDate(value)
}

function formatRecordTime(record: IngestMonitorRecord) {
	return formatDisplayDate(record.updateTime || record.createTime || "")
}

function formatElapsedSeconds(value?: number) {
	if (value === undefined || Number.isNaN(value)) {
		return "-"
	}
	if (value < 60) {
		return `${value.toFixed(1)} 秒`
	}
	if (value < 3600) {
		const minutes = Math.floor(value / 60)
		return `${minutes} 分 ${(value % 60).toFixed(1)} 秒`
	}
	const hours = Math.floor(value / 3600)
	const minutes = Math.floor((value % 3600) / 60)
	return `${hours} 小时 ${minutes} 分 ${(value % 60).toFixed(1)} 秒`
}

function formatBatchFinalElapsed(batch: IngestMonitorBatch) {
	if (batch.finalIngestElapsedSeconds !== undefined) {
		return formatElapsedSeconds(batch.finalIngestElapsedSeconds)
	}
	if (batch.processingCount > 0 || batch.status === "running" || batch.status === "pending") {
		return "进行中"
	}
	if (batch.totalTracked > 0 && batch.successCount+batch.failureCount === batch.totalTracked) {
		return "终态时间缺失"
	}
	return "-"
}

function formatBatchPaths(paths: string[]) {
	if (!paths.length) {
		return "-"
	}
	if (paths.length === 1) {
		return truncateText(paths[0], 56)
	}
	return `${truncateText(paths[0], 40)} 等 ${paths.length} 个目录`
}

function truncateText(value: string, max = 64) {
	const trimmed = value.trim()
	if (!trimmed) {
		return "-"
	}
	if (trimmed.length <= max) {
		return trimmed
	}
	return `${trimmed.slice(0, max)}...`
}

function DetailField({ label, value }: { label: string; value: string }) {
	return (
		<div className="rounded-md border border-border/60 bg-muted/30 p-3">
			<div className="text-xs text-muted-foreground">{label}</div>
			<div className="mt-1 whitespace-pre-wrap break-all text-sm">{value || "-"}</div>
		</div>
	)
}

export default memo(() => {
	const [summaryData, setSummaryData] = useState<IngestMonitorSummaryResponse | null>(null)
	const [batchListData, setBatchListData] = useState<IngestMonitorBatchListResponse | null>(null)
	const [loading, setLoading] = useState(true)
	const [refreshing, setRefreshing] = useState(false)
	const [error, setError] = useState("")

	const [detailOpen, setDetailOpen] = useState(false)
	const [detailLoading, setDetailLoading] = useState(false)
	const [detailError, setDetailError] = useState("")
	const [detail, setDetail] = useState<IngestMonitorDetailResponse | null>(null)

	const [batchDetailOpen, setBatchDetailOpen] = useState(false)
	const [batchDetailLoading, setBatchDetailLoading] = useState(false)
	const [batchDetailError, setBatchDetailError] = useState("")
	const [batchDetail, setBatchDetail] = useState<IngestMonitorBatchDetailResponse | null>(null)
	const [selectedBatchRunId, setSelectedBatchRunId] = useState("")
	const [batchDetailCursorHistory, setBatchDetailCursorHistory] = useState<string[]>([])
	const [batchDetailCurrentCursor, setBatchDetailCurrentCursor] = useState("")
	const [batchDetailCurrentPage, setBatchDetailCurrentPage] = useState(1)
	const dashboardRequestInFlight = useRef(false)

	useEffect(() => {
		document.title = `${BRAND_NAME} - 入库服务可视化`
	}, [])

	const loadDashboard = useCallback(async ({ silent = false }: { silent?: boolean } = {}) => {
		if (dashboardRequestInFlight.current) {
			return
		}
		dashboardRequestInFlight.current = true
		if (silent) {
			setRefreshing(true)
		} else {
			setLoading(true)
		}
		setError("")

		try {
			const [summaryResponse, batchResponse] = await Promise.all([
				fetchIngestMonitorSummary(),
				fetchIngestMonitorBatches(),
			])
			setSummaryData(summaryResponse)
			setBatchListData(batchResponse)
		} catch (err) {
			setError(err instanceof Error ? err.message : String(err))
		} finally {
			dashboardRequestInFlight.current = false
			if (silent) {
				setRefreshing(false)
			} else {
				setLoading(false)
			}
		}
	}, [])

	useEffect(() => {
		loadDashboard()
		const timer = window.setInterval(() => {
			loadDashboard({ silent: true })
		}, refreshIntervalMs)
		return () => window.clearInterval(timer)
	}, [loadDashboard])

	const openDetail = useCallback(async (itemCode: string) => {
		setDetailOpen(true)
		setDetailLoading(true)
		setDetailError("")
		setDetail(null)
		try {
			setDetail(await fetchIngestMonitorDetail(itemCode))
		} catch (err) {
			setDetailError(err instanceof Error ? err.message : String(err))
		} finally {
			setDetailLoading(false)
		}
	}, [])

	const openBatchDetail = useCallback(async (
		batchRunId: string,
		{
			cursor = "",
			page = 1,
			history = [],
			reset = false,
		}: {
			cursor?: string
			page?: number
			history?: string[]
			reset?: boolean
		} = {},
	) => {
		setSelectedBatchRunId(batchRunId)
		setBatchDetailOpen(true)
		setBatchDetailLoading(true)
		setBatchDetailError("")
		if (reset) {
			setBatchDetail(null)
			setBatchDetailCurrentCursor("")
			setBatchDetailCurrentPage(1)
			setBatchDetailCursorHistory([])
		}
		setBatchDetailCurrentCursor(cursor)
		setBatchDetailCurrentPage(page)
		setBatchDetailCursorHistory(history)
		try {
			setBatchDetail(await fetchIngestMonitorBatchDetail(batchRunId, cursor))
		} catch (err) {
			setBatchDetailError(err instanceof Error ? err.message : String(err))
		} finally {
			setBatchDetailLoading(false)
		}
	}, [])

	const goToPreviousBatchDetailPage = useCallback(() => {
		if (!selectedBatchRunId || batchDetailCursorHistory.length === 0) {
			return
		}
		const previousCursor = batchDetailCursorHistory[batchDetailCursorHistory.length - 1]
		openBatchDetail(selectedBatchRunId, {
			cursor: previousCursor,
			page: Math.max(1, batchDetailCurrentPage - 1),
			history: batchDetailCursorHistory.slice(0, -1),
		})
	}, [batchDetailCurrentPage, batchDetailCursorHistory, openBatchDetail, selectedBatchRunId])

	const goToNextBatchDetailPage = useCallback(() => {
		if (!selectedBatchRunId || !batchDetail?.hasMore || !batchDetail.nextCursor) {
			return
		}
		openBatchDetail(selectedBatchRunId, {
			cursor: batchDetail.nextCursor,
			page: batchDetailCurrentPage + 1,
			history: [...batchDetailCursorHistory, batchDetailCurrentCursor],
		})
	}, [
		batchDetail?.hasMore,
		batchDetail?.nextCursor,
		batchDetailCurrentCursor,
		batchDetailCurrentPage,
		batchDetailCursorHistory,
		openBatchDetail,
		selectedBatchRunId,
	])

	const cards = useMemo(() => {
		const summary = summaryData?.summary
		return [
			{ 
				key: "total", 
				title: "正式入库总数", 
				value: summary?.total ?? 0, 
				description: "仅统计正式入库记录",
				icon: DatabaseIcon,
				color: "slate"
			},
			{ 
				key: "success", 
				title: "成功", 
				value: summary?.success ?? 0, 
				description: "完成且关键产物路径齐全",
				icon: CheckCircleIcon,
				color: "emerald"
			},
			{ 
				key: "failure", 
				title: "失败", 
				value: summary?.failure ?? 0, 
				description: "失败或存在错误信息",
				icon: XCircleIcon,
				color: "rose"
			},
			{ 
				key: "processing", 
				title: "处理中", 
				value: summary?.processing ?? 0, 
				description: "仍处于处理中状态",
				icon: ClockIcon,
				color: "amber"
			},
		]
	}, [summaryData])

	const scopeTenant = summaryData?.scope.tenant || batchListData?.scope.tenant || ""

	return (
		<div className="grid gap-4 pb-8">
			<Card className="overflow-hidden border-border/60 bg-gradient-to-br from-muted/50 via-background to-background shadow-sm">
				<CardHeader className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between pb-6">
					<div className="flex items-center gap-3">
						<WorkflowIcon className="h-6 w-6 text-primary" />
						<CardTitle className="text-2xl font-bold tracking-tight">入库服务可视化</CardTitle>
					</div>
					<div className="flex items-center gap-2">
						{scopeTenant && (
							<Badge className="bg-primary/10 text-primary border-primary/20 font-medium">
								{scopeTenant === "guochuang" ? "国创" : scopeTenant}
							</Badge>
						)}
						<Badge variant="outline" className="bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20 font-medium">
							{Math.round(refreshIntervalMs / 1000)} 秒自动刷新
						</Badge>
						<Button 
							variant="outline" 
							size="sm" 
							onClick={() => loadDashboard({ silent: true })} 
							disabled={loading || refreshing}
							className="gap-2"
						>
							{refreshing ? (
								<LoaderCircleIcon className="h-4 w-4 animate-spin" />
							) : (
								<RefreshCwIcon className="h-4 w-4" />
							)}
							刷新
						</Button>
					</div>
				</CardHeader>
			</Card>

			<div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
				{cards.map((card) => {
					const Icon = card.icon
					const colorClasses: Record<string, string> = {
						slate: "from-slate-500/10 via-transparent to-transparent border-slate-500/20",
						emerald: "from-emerald-500/12 via-transparent to-transparent border-emerald-500/20",
						rose: "from-rose-500/12 via-transparent to-transparent border-rose-500/20",
						amber: "from-amber-500/12 via-transparent to-transparent border-amber-500/20",
						sky: "from-sky-500/10 via-transparent to-transparent border-sky-500/20",
					}
					return (
						<Card 
							key={card.key} 
							className={`h-full border-border/60 bg-gradient-to-br ${colorClasses[card.color]} shadow-sm`}
						>
							<CardHeader className="flex h-full flex-col gap-4 space-y-0 pb-3">
								<div className="flex items-center justify-between">
									<CardDescription className="text-xs font-medium tracking-wide uppercase">{card.title}</CardDescription>
									<Icon className="h-4 w-4 shrink-0 text-muted-foreground/60" />
								</div>
								<CardTitle className="text-3xl leading-none">{card.value}</CardTitle>
								<CardContent className="mt-auto px-0 pt-0 text-xs leading-5 text-muted-foreground">
									{card.description}
								</CardContent>
							</CardHeader>
						</Card>
					)
				})}
			</div>

			{error ? (
				<Card className="border-rose-500/40 bg-rose-500/5">
					<CardHeader className="gap-2">
						<div className="flex items-center gap-2">
							<AlertTriangleIcon className="h-5 w-5 text-rose-500" />
							<CardTitle className="text-lg">数据加载失败</CardTitle>
						</div>
						<CardDescription className="text-rose-700 dark:text-rose-200">{error}</CardDescription>
					</CardHeader>
				</Card>
			) : null}

			<Card className="border-border/60">
				<CardHeader className="gap-3">
					<div className="flex items-center justify-between">
						<div className="flex items-center gap-2">
							<LayersIcon className="h-5 w-5 text-muted-foreground" />
							<CardTitle>最近扫描批次</CardTitle>
						</div>
						{!loading && batchListData?.batches.length ? (
							<Badge variant="outline" className="bg-background/80">{batchListData.batches.length} 个批次</Badge>
						) : null}
					</div>
					<CardDescription>按 XXL 大任务维度拆开展示扫描、过滤、登记、投递、本地处理和正式入库口径。</CardDescription>
				</CardHeader>
				<CardContent>
					{loading && !batchListData ? (
						<div className="flex items-center justify-center rounded-xl border border-dashed border-border/60 bg-muted/20 px-6 py-12 text-sm text-muted-foreground">
							<LoaderCircleIcon className="mr-2 h-4 w-4 animate-spin" />
							正在加载批次数据...
						</div>
					) : batchListData?.batches.length ? (
						<div className="rounded-lg border border-border/60 overflow-hidden">
							<Table>
								<TableHeader className="bg-muted/50">
									<TableRow className="hover:bg-transparent">
										<TableHead className="w-[120px]">
											<div className="flex items-center gap-1.5">
												<FileCodeIcon className="h-3.5 w-3.5" />
												批次
											</div>
										</TableHead>
										<TableHead className="w-[100px]">XXL 任务</TableHead>
										<TableHead className="w-[140px]">
											<div className="flex items-center gap-1.5">
												<FolderIcon className="h-3.5 w-3.5" />
												扫描目录
											</div>
										</TableHead>
										<TableHead className="w-[170px] text-right">
											<div className="flex items-center justify-end gap-1.5">
												<BarChart3Icon className="h-3.5 w-3.5" />
												统计
											</div>
										</TableHead>
										<TableHead className="w-[90px] text-right">扫描耗时</TableHead>
										<TableHead className="w-[90px] text-right">入库耗时</TableHead>
										<TableHead className="w-[220px] text-center">阶段结果</TableHead>
										<TableHead className="w-[90px] text-right">开始时间</TableHead>
										<TableHead className="w-[60px] text-center">详情</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{batchListData.batches.map((batch) => (
										<TableRow key={batch.batchRunId} className="hover:bg-muted/30 transition-colors">
											<TableCell>
												<div className="flex flex-col gap-1">
													<Badge variant={batchStatusBadgeVariant(batch.status)} className="w-fit text-[10px]">
														{batchStatusLabel(batch.status)}
													</Badge>
													<span className="font-mono text-[10px] text-muted-foreground truncate">{truncateText(batch.batchRunId, 12)}</span>
												</div>
											</TableCell>
											<TableCell>
												<div className="flex flex-col text-[10px] leading-tight">
													<span className="text-muted-foreground">J:{batch.xxlJobId || "-"}</span>
													<span className="text-muted-foreground">L:{batch.xxlLogId || "-"}</span>
												</div>
											</TableCell>
											<TableCell>
												<div className="max-w-[130px]">
													<p className="text-xs truncate" title={batch.scanPaths.join(', ')}>{formatBatchPaths(batch.scanPaths)}</p>
												</div>
											</TableCell>
											<TableCell className="text-right">
												<div className="flex flex-col gap-0.5 text-[11px] leading-tight">
													<span>扫 {batch.totalFilesScanned} / 目录 {batch.totalDirsScanned}</span>
													<span>扩展过滤 {batch.totalFilesFiltered} / 大文件 {batch.totalFilesLargeFiltered}</span>
													<span>登记 {batch.totalFilesRegistered} / 失败 {batch.totalFilesRegisterFailed}</span>
													<span>投递 {batch.totalFilesEnqueued} / 失败 {batch.totalFilesEnqueueFailed}</span>
												</div>
											</TableCell>
											<TableCell className="text-right text-xs">{formatElapsedSeconds(batch.xxlScanElapsedSeconds)}</TableCell>
											<TableCell className="text-right text-xs">{formatBatchFinalElapsed(batch)}</TableCell>
											<TableCell>
												<div className="grid gap-y-1 text-[10px]">
													<div className="flex items-center justify-between gap-2">
														<span className="text-muted-foreground">成功</span>
														<span className="font-medium text-emerald-600 dark:text-emerald-400">{batch.successCount}</span>
													</div>
													<div className="flex items-center justify-between gap-2">
														<span className="text-muted-foreground">处理中</span>
														<span className="font-medium text-amber-600 dark:text-amber-400">{batch.processingCount}</span>
													</div>
													<div className="flex items-center justify-between gap-2">
														<span className="text-muted-foreground">失败</span>
														<span className="font-medium text-rose-600 dark:text-rose-400">{batch.failureCount}</span>
													</div>
												</div>
											</TableCell>
											<TableCell className="text-right text-xs text-muted-foreground">{formatDisplayDate(batch.scanStartedAt)}</TableCell>
											<TableCell className="text-center">
												<Button
													variant="ghost"
													size="icon"
													className="h-7 w-7"
													onClick={() => openBatchDetail(batch.batchRunId, { reset: true, history: [], page: 1 })}
												>
													<EyeIcon className="h-3.5 w-3.5" />
												</Button>
											</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
						</div>
					) : (
						<div className="rounded-xl border border-dashed border-border/60 bg-muted/20 px-6 py-10 text-center">
							<div className="text-sm font-medium">暂无扫描批次</div>
							<div className="mt-2 text-xs text-muted-foreground">当前租户还没有追踪到 XXL 扫描批次</div>
						</div>
					)}
				</CardContent>
			</Card>

			<Card className="border-border/60">
				<CardHeader className="gap-3">
					<div className="flex items-center justify-between">
						<div className="flex items-center gap-2">
							<DatabaseIcon className="h-5 w-5 text-muted-foreground" />
							<CardTitle>最近正式入库记录</CardTitle>
						</div>
						{!loading && summaryData?.recent.length ? (
							<Badge variant="outline" className="bg-background/80">展示 {(summaryData.recent.slice(0, 10)).length} / {summaryData.recent.length} 条</Badge>
						) : null}
					</div>
					<CardDescription>按更新时间倒序展示最近 10 条正式入库记录，自动刷新间隔 {Math.round(refreshIntervalMs / 1000)} 秒。</CardDescription>
				</CardHeader>
				<CardContent>
					{loading && !summaryData ? (
						<div className="flex items-center justify-center rounded-xl border border-dashed border-border/60 bg-muted/20 px-6 py-12 text-sm text-muted-foreground">
							<LoaderCircleIcon className="mr-2 h-4 w-4 animate-spin" />
							正在加载入库状态...
						</div>
					) : summaryData?.recent.length ? (
						<div className="rounded-lg border border-border/60 overflow-hidden">
							<Table>
								<TableHeader className="bg-muted/50">
									<TableRow className="hover:bg-transparent">
										<TableHead className="w-[160px]">
											<div className="flex items-center gap-1.5">
												<FileCodeIcon className="h-3.5 w-3.5" />
												记录对象
											</div>
										</TableHead>
										<TableHead className="w-[90px]">状态</TableHead>
										<TableHead className="w-[100px] text-center">
											<div className="flex items-center justify-center gap-1.5">
												<RouteIcon className="h-3.5 w-3.5" />
												路径
											</div>
										</TableHead>
										<TableHead className="w-[100px]">推理</TableHead>
										<TableHead className="w-[110px] text-right">时间</TableHead>
										<TableHead className="w-[60px] text-center">详情</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{summaryData.recent.slice(0, 10).map((record) => (
										<TableRow key={`${record.itemCode}-${record.updateTime}`} className="hover:bg-muted/30 transition-colors">
											<TableCell>
												<div className="flex flex-col gap-0.5">
													<span className="font-mono text-[10px] text-muted-foreground truncate" title={record.itemCode}>{record.itemCode}</span>
													<span className="text-sm font-medium truncate" title={record.productName}>{record.productName || "-"}</span>
												</div>
											</TableCell>
											<TableCell>
												<div className="flex flex-col gap-1">
													<Badge variant={statusBadgeVariant(record.status)} className="w-fit text-[10px]">
														{statusLabel(record.status)}
													</Badge>
													<span className="text-[9px] text-muted-foreground">
														complete: {record.isComplete ?? "-"}
													</span>
												</div>
											</TableCell>
											<TableCell className="text-center">
												<div className={cn(
													"inline-flex h-6 items-center justify-center rounded px-2 text-xs font-medium",
													record.pathReadyCount === record.pathReadyTotal 
														? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400" 
														: "bg-muted text-muted-foreground"
												)}>
													{record.pathReadyCount}/{record.pathReadyTotal}
												</div>
											</TableCell>
											<TableCell>
												<div className="flex flex-wrap gap-1">
													{record.inferenceTypes?.length ? (
														record.inferenceTypes.slice(0, 2).map((type, idx) => (
															<span 
																key={idx}
																className="inline-flex h-5 items-center justify-center rounded bg-secondary px-1.5 text-[10px]"
															>
																{type === 1 ? "3D" : type === 2 ? "局部" : type === 3 ? "2D视图" : type === 4 ? "2D剖面" : type}
															</span>
														))
													) : (
														<span className="text-xs text-muted-foreground">-</span>
													)}
													{record.inferenceTypes && record.inferenceTypes.length > 2 && (
														<span className="text-[10px] text-muted-foreground">+{record.inferenceTypes.length - 2}</span>
													)}
												</div>
											</TableCell>
											<TableCell className="text-right">
												<div className="flex flex-col gap-0.5">
													<span className="text-xs">{formatRecordTime(record)}</span>
													<span className="text-[10px] text-muted-foreground">{formatDisplayDate(record.createTime)}</span>
												</div>
											</TableCell>
											<TableCell className="text-center">
												<Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openDetail(record.itemCode)}>
													<EyeIcon className="h-3.5 w-3.5" />
												</Button>
											</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
						</div>
					) : (
						<div className="rounded-xl border border-dashed border-border/60 bg-muted/20 px-6 py-10 text-center">
							<div className="text-sm font-medium">暂无正式入库记录</div>
							<div className="mt-2 text-xs text-muted-foreground">当前租户还没有正式入库记录</div>
						</div>
					)}
				</CardContent>
			</Card>

			<Card className="border-border/60">
				<CardHeader className="gap-3">
					<div className="flex items-center justify-between">
						<div className="flex items-center gap-2">
							<AlertTriangleIcon className="h-5 w-5 text-rose-500" />
							<CardTitle>异常记录</CardTitle>
							{summaryData?.failures.length ? (
								<Badge variant="danger" className="bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20">
									{summaryData.failures.length} 条
								</Badge>
							) : null}
						</div>
					</div>
					<CardDescription>优先关注失败记录和存在错误信息的正式入库记录。</CardDescription>
				</CardHeader>
				<CardContent>
					{summaryData?.failures.length ? (
						<>
						<div className="rounded-lg border border-amber-500/20 bg-amber-500/5 p-3 mb-4">
							<div className="flex items-center gap-2 text-sm text-amber-700 dark:text-amber-200">
								<AlertTriangleIcon className="h-4 w-4" />
								<span>发现 {summaryData.failures.length} 条异常记录，建议优先处理</span>
							</div>
						</div>
						<div className="rounded-lg border border-border/60 overflow-hidden">
							<Table>
								<TableHeader className="bg-muted/50">
									<TableRow className="hover:bg-transparent">
										<TableHead className="w-[180px]">
											<div className="flex items-center gap-1.5">
												<FileCodeIcon className="h-3.5 w-3.5" />
												记录对象
											</div>
										</TableHead>
										<TableHead className="w-[100px]">状态</TableHead>
										<TableHead>错误信息</TableHead>
										<TableHead className="w-[130px]">时间</TableHead>
										<TableHead className="w-[70px] text-center">详情</TableHead>
									</TableRow>
								</TableHeader>
								<TableBody>
									{summaryData.failures.map((record) => (
										<TableRow key={`${record.itemCode}-${record.updateTime}-failure`} className="hover:bg-rose-500/5 transition-colors">
											<TableCell>
												<div className="flex flex-col gap-1">
													<span className="font-mono text-xs text-muted-foreground truncate" title={record.itemCode}>{record.itemCode}</span>
													<span className="text-sm font-medium truncate" title={record.productName}>{record.productName || "-"}</span>
												</div>
											</TableCell>
											<TableCell>
												<Badge variant={statusBadgeVariant(record.status)} className="w-fit">
													{statusLabel(record.status)}
												</Badge>
											</TableCell>
											<TableCell>
												<div className="max-w-[400px]">
													<p className="text-sm text-rose-600 dark:text-rose-300 line-clamp-2" title={record.errorMsg}>
														{record.errorMsg || "无错误信息"}
													</p>
												</div>
											</TableCell>
											<TableCell>
												<div className="flex flex-col gap-0.5">
													<span className="text-sm">{formatRecordTime(record)}</span>
													<span className="text-xs text-muted-foreground">创建: {formatDisplayDate(record.createTime)}</span>
												</div>
											</TableCell>
											<TableCell className="text-center">
												<Button variant="ghost" size="icon" className="h-8 w-8" onClick={() => openDetail(record.itemCode)}>
													<EyeIcon className="h-4 w-4" />
												</Button>
											</TableCell>
										</TableRow>
									))}
								</TableBody>
							</Table>
						</div>
						</>
					) : (
						<div className="rounded-xl border border-dashed border-border/60 bg-muted/20 px-6 py-10 text-center">
							<div className="text-sm font-medium">当前没有异常记录</div>
							<div className="mt-2 text-xs text-muted-foreground">当前没有需要人工关注的异常正式入库记录</div>
						</div>
					)}
				</CardContent>
			</Card>

			<Dialog open={batchDetailOpen} onOpenChange={setBatchDetailOpen}>
				<DialogContent className="max-w-6xl max-h-[90vh] overflow-hidden p-0 sm:rounded-xl">
					<div className="flex flex-col h-full max-h-[90vh]">
						<div className="border-b border-border/60 px-6 py-4">
							<DialogHeader className="gap-2">
								<div className="flex items-center gap-2">
									<LayersIcon className="h-5 w-5 text-primary" />
									<DialogTitle>扫描批次详情</DialogTitle>
								</div>
								<DialogDescription>查看某次 XXL 大任务的扫描统计、入库耗时和批次内记录状态。</DialogDescription>
							</DialogHeader>
						</div>
						
						<div className="flex-1 overflow-y-auto px-6 py-4">
							{batchDetailLoading ? (
								<div className="flex items-center justify-center py-12 text-muted-foreground">
									<LoaderCircleIcon className="mr-2 h-4 w-4 animate-spin" />
									正在加载批次详情...
								</div>
							) : batchDetailError ? (
								<div className="rounded-lg border border-rose-500/40 bg-rose-500/5 p-4 text-sm">
									<div className="flex items-center gap-2 text-rose-600 dark:text-rose-400">
										<AlertTriangleIcon className="h-4 w-4" />
										<span>{batchDetailError}</span>
									</div>
								</div>
							) : batchDetail ? (
								<div className="grid gap-4">
									<div className="flex flex-wrap items-center gap-2">
										{batchDetail.scope.tenant && (
											<Badge className="bg-primary/10 text-primary border-primary/20">
												{batchDetail.scope.tenant === "guochuang" ? "国创" : batchDetail.scope.tenant}
											</Badge>
										)}
										<Badge variant={batchStatusBadgeVariant(batchDetail.batch.status)}>
											{batchStatusLabel(batchDetail.batch.status)}
										</Badge>
										<Badge variant="outline" className="font-mono text-[10px]">
											{truncateText(batchDetail.batch.batchRunId, 20)}
										</Badge>
									</div>

									<div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
										<Card className="border-sky-500/20 bg-gradient-to-br from-sky-500/10 via-transparent to-transparent">
											<CardHeader className="space-y-2 pb-3">
												<div className="flex items-center justify-between">
													<CardDescription className="text-xs">扫描与过滤</CardDescription>
													<FolderIcon className="h-3.5 w-3.5 text-sky-500/60" />
												</div>
												<div className="space-y-1 text-xs">
													<div>目录 {batchDetail.batch.totalDirsScanned} / 文件 {batchDetail.batch.totalFilesScanned}</div>
													<div>扩展过滤 {batchDetail.batch.totalFilesFiltered}</div>
													<div>大文件过滤 {batchDetail.batch.totalFilesLargeFiltered}</div>
												</div>
											</CardHeader>
										</Card>
										<Card className="border-violet-500/20 bg-gradient-to-br from-violet-500/10 via-transparent to-transparent">
											<CardHeader className="space-y-2 pb-3">
												<div className="flex items-center justify-between">
													<CardDescription className="text-xs">登记与投递</CardDescription>
													<TimerIcon className="h-3.5 w-3.5 text-violet-500/60" />
												</div>
												<div className="space-y-1 text-xs">
													<div>登记成功 {batchDetail.batch.totalFilesRegistered}</div>
													<div>登记失败 {batchDetail.batch.totalFilesRegisterFailed}</div>
													<div>投递成功 {batchDetail.batch.totalFilesEnqueued}</div>
													<div>投递失败 {batchDetail.batch.totalFilesEnqueueFailed}</div>
												</div>
											</CardHeader>
										</Card>
										<Card className="border-emerald-500/20 bg-gradient-to-br from-emerald-500/10 via-transparent to-transparent">
											<CardHeader className="space-y-2 pb-3">
												<div className="flex items-center justify-between">
													<CardDescription className="text-xs">入库结果</CardDescription>
													<BarChart3Icon className="h-3.5 w-3.5 text-emerald-500/60" />
												</div>
												<div className="space-y-1 text-xs">
													<div>成功 {batchDetail.batch.successCount}</div>
													<div>处理中 {batchDetail.batch.processingCount}</div>
													<div>失败 {batchDetail.batch.failureCount}</div>
												</div>
											</CardHeader>
										</Card>
										<Card className="border-amber-500/20 bg-gradient-to-br from-amber-500/10 via-transparent to-transparent">
											<CardHeader className="space-y-2 pb-3">
												<div className="flex items-center justify-between">
													<CardDescription className="text-xs">耗时与规模</CardDescription>
													<ClockIcon className="h-3.5 w-3.5 text-amber-500/60" />
												</div>
												<div className="space-y-1 text-xs">
													<div>追踪记录 {batchDetail.batch.totalTracked}</div>
													<div>批次数 {batchDetail.batch.totalBatches}</div>
													<div>最终耗时 {formatBatchFinalElapsed(batchDetail.batch)}</div>
												</div>
											</CardHeader>
										</Card>
									</div>

									<Card className="border-border/60">
										<CardHeader className="pb-3">
											<CardTitle className="text-sm">批次信息</CardTitle>
										</CardHeader>
										<CardContent className="grid gap-3 md:grid-cols-2">
											<div className="rounded-lg border border-border/60 bg-muted/30 p-3">
												<div className="text-[11px] text-muted-foreground mb-1">XXL 任务</div>
												<div className="flex gap-4 text-sm">
													<div>jobId: <span className="font-mono">{batchDetail.batch.xxlJobId || "-"}</span></div>
													<div>logId: <span className="font-mono">{batchDetail.batch.xxlLogId || "-"}</span></div>
												</div>
											</div>
											<div className="rounded-lg border border-border/60 bg-muted/30 p-3">
												<div className="text-[11px] text-muted-foreground mb-1">时间</div>
												<div className="flex gap-4 text-sm">
													<div>开始: <span className="text-muted-foreground">{formatDisplayDate(batchDetail.batch.scanStartedAt)}</span></div>
													<div>结束: <span className="text-muted-foreground">{formatDisplayDate(batchDetail.batch.scanFinishedAt)}</span></div>
												</div>
											</div>
											<div className="rounded-lg border border-border/60 bg-muted/30 p-3">
												<div className="text-[11px] text-muted-foreground mb-1">扫描路径</div>
												<div className="text-sm">
													{batchDetail.batch.scanPaths.map((path, idx) => (
														<div key={idx} className="font-mono text-xs truncate" title={path}>{path}</div>
													))}
												</div>
											</div>
											<div className="rounded-lg border border-border/60 bg-muted/30 p-3">
												<div className="text-[11px] text-muted-foreground mb-1">批次配置</div>
												<div className="flex flex-wrap gap-x-4 gap-y-1 text-sm">
													<div>fileType: <span className="font-mono">{batchDetail.batch.fileType ?? "-"}</span></div>
													<div>batchSize: <span className="font-mono">{batchDetail.batch.batchSize}</span></div>
													<div>force: <span className="font-mono">{batchDetail.batch.force ? "是" : "否"}</span></div>
													<div>tracked: <span className="font-mono">{batchDetail.batch.totalTracked}</span></div>
												</div>
											</div>
										</CardContent>
									</Card>

									{batchDetail.batch.errorMessage && (
										<div className="rounded-lg border border-rose-500/30 bg-rose-500/5 p-4">
											<div className="flex items-start gap-2">
												<AlertTriangleIcon className="h-4 w-4 text-rose-500 mt-0.5" />
												<div>
													<div className="text-sm font-medium text-rose-600 dark:text-rose-400 mb-1">批次错误信息</div>
													<div className="text-sm text-rose-700 dark:text-rose-300 whitespace-pre-wrap">{batchDetail.batch.errorMessage}</div>
												</div>
											</div>
										</div>
									)}

									<Card className="border-border/60">
										<CardHeader className="gap-3 pb-3">
											<div className="flex items-center justify-between">
												<div className="flex items-center gap-2">
													<DatabaseIcon className="h-4 w-4 text-muted-foreground" />
													<CardTitle className="text-base">批次内记录</CardTitle>
												</div>
												{batchDetail.totalItems ? (
													<Badge variant="outline" className="bg-background/80">
														展示 {batchDetail.items.length} / {batchDetail.totalItems} 条
													</Badge>
												) : null}
											</div>
											<CardDescription>默认每页加载 {batchDetail.pageSize} 条，优先展示失败和处理中记录。</CardDescription>
										</CardHeader>
										<CardContent>
											{batchDetail.items.length ? (
												<div className="grid gap-3">
													<div className="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/20 px-3 py-2 text-xs">
														<div className="text-muted-foreground">
															第 {batchDetailCurrentPage} / {Math.max(1, Math.ceil(batchDetail.totalItems / batchDetail.pageSize))} 页
														</div>
														<div className="flex items-center gap-2">
															<Button
																variant="outline"
																size="sm"
																className="h-8 gap-1.5"
																disabled={batchDetailLoading || batchDetailCursorHistory.length === 0}
																onClick={goToPreviousBatchDetailPage}
															>
																<ChevronLeftIcon className="h-3.5 w-3.5" />
																上一页
															</Button>
															<Button
																variant="outline"
																size="sm"
																className="h-8 gap-1.5"
																disabled={batchDetailLoading || !batchDetail.hasMore || !batchDetail.nextCursor}
																onClick={goToNextBatchDetailPage}
															>
																下一页
																<ChevronRightIcon className="h-3.5 w-3.5" />
															</Button>
														</div>
													</div>
													<div className="rounded-lg border border-border/60 overflow-hidden">
													<Table>
														<TableHeader className="bg-muted/50">
															<TableRow className="hover:bg-transparent">
																<TableHead className="w-[140px]">
																	<div className="flex items-center gap-1.5">
																		<FileCodeIcon className="h-3.5 w-3.5" />
																		记录对象
																	</div>
																</TableHead>
																<TableHead className="w-[90px]">入库状态</TableHead>
																<TableHead className="w-[90px] text-center">
																	<div className="flex items-center justify-center gap-1.5">
																		<RouteIcon className="h-3.5 w-3.5" />
																		路径
																	</div>
																</TableHead>
																<TableHead className="w-[110px] text-right">时间</TableHead>
																<TableHead className="w-[60px] text-center">详情</TableHead>
															</TableRow>
														</TableHeader>
														<TableBody>
															{batchDetail.items.map((item) => (
																<TableRow key={`${batchDetail.batch.batchRunId}-${item.itemCode}`} className="hover:bg-muted/30 transition-colors">
																	<TableCell>
																		<div className="flex flex-col gap-0.5">
																			<span className="font-mono text-[10px] text-muted-foreground truncate" title={item.itemCode}>{item.itemCode}</span>
																			<span className="text-xs truncate" title={item.fileName}>{truncateText(item.fileName, 18)}</span>
																			{item.cadNumber && (
																				<span className="font-mono text-[9px] text-muted-foreground truncate">CAD:{truncateText(item.cadNumber, 12)}</span>
																			)}
																		</div>
																	</TableCell>
																	<TableCell>
																		<Badge variant={statusBadgeVariant(item.ingestStatus)} className="text-[10px]">
																			{statusLabel(item.ingestStatus)}
																		</Badge>
																	</TableCell>
																	<TableCell className="text-center">
																		<div className={cn(
																			"inline-flex h-6 items-center justify-center rounded px-2 text-xs font-medium",
																			item.pathReadyCount === item.pathReadyTotal 
																				? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400" 
																				: "bg-muted text-muted-foreground"
																		)}>
																			{item.pathReadyCount}/{item.pathReadyTotal}
																		</div>
																	</TableCell>
																	<TableCell className="text-right">
																		<div className="flex flex-col gap-0.5">
																			<span className="text-xs text-muted-foreground">{formatDisplayDate(item.updateTime || item.createTime)}</span>
																			<span className="text-[10px] text-muted-foreground">{item.hasFormalRecord ? "已建正式记录" : "仅批次跟踪"}</span>
																		</div>
																	</TableCell>
																	<TableCell className="text-center">
																		<Button variant="ghost" size="icon" className="h-7 w-7" onClick={() => openDetail(item.itemCode)}>
																			<EyeIcon className="h-3.5 w-3.5" />
																		</Button>
																	</TableCell>
																</TableRow>
														))}
													</TableBody>
												</Table>
											</div>
												</div>
										) : (
											<div className="rounded-xl border border-dashed border-border/60 bg-muted/20 px-6 py-10 text-center">
												<div className="text-sm font-medium">暂无批次内记录</div>
												<div className="mt-2 text-xs text-muted-foreground">当前批次还没有追踪到记录</div>
											</div>
										)}
									</CardContent>
								</Card>
								</div>
							) : (
								<div className="rounded-xl border border-dashed border-border/60 bg-muted/20 px-6 py-10 text-center">
									<div className="text-sm font-medium">暂无批次详情</div>
									<div className="mt-2 text-xs text-muted-foreground">无法加载批次详情信息</div>
								</div>
							)}
						</div>
					</div>
				</DialogContent>
			</Dialog>

			<Dialog open={detailOpen} onOpenChange={setDetailOpen}>
				<DialogContent className="max-w-3xl max-h-[90vh] overflow-hidden p-0 sm:rounded-2xl border-border/60">
					<div className="flex flex-col h-full max-h-[90vh]">
						{/* 头部 - 渐变背景 */}
						<div className="bg-gradient-to-br from-primary/5 via-muted/30 to-background border-b border-border/60 px-6 py-5">
							<DialogHeader className="gap-2">
								<div className="flex items-center gap-3">
									<div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
										<FileCodeIcon className="h-5 w-5" />
									</div>
									<div>
										<DialogTitle className="text-xl">入库记录详情</DialogTitle>
										<DialogDescription className="text-xs mt-0.5">记录状态、关键路径和错误信息</DialogDescription>
									</div>
								</div>
							</DialogHeader>
						</div>
						
						<div className="flex-1 overflow-y-auto px-6 py-5">
							{detailLoading ? (
								<div className="flex items-center justify-center py-16 text-muted-foreground">
									<LoaderCircleIcon className="mr-2 h-5 w-5 animate-spin" />
									<span className="text-sm">正在加载详情...</span>
								</div>
							) : detailError ? (
								<div className="rounded-xl border border-rose-500/30 bg-rose-500/5 p-5">
									<div className="flex items-center gap-3">
										<div className="flex h-10 w-10 items-center justify-center rounded-full bg-rose-500/10">
											<AlertTriangleIcon className="h-5 w-5 text-rose-500" />
										</div>
										<div>
											<div className="text-sm font-medium text-rose-600 dark:text-rose-400">加载失败</div>
											<div className="text-sm text-rose-700/70 dark:text-rose-300/70 mt-0.5">{detailError}</div>
										</div>
									</div>
								</div>
							) : detail ? (
								<div className="grid gap-5">
									{/* 状态区域 - 使用卡片样式 */}
									<div className="flex flex-wrap items-center gap-2">
										{detail.scope.tenant && (
											<div className="inline-flex items-center gap-1.5 rounded-lg bg-primary/5 border border-primary/10 px-3 py-1.5">
												<span className="text-xs font-medium text-primary">{detail.scope.tenant === "guochuang" ? "国创" : detail.scope.tenant}</span>
											</div>
										)}
										<div className={cn(
											"inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5",
											detail.item.status === 'success' && "bg-emerald-500/10 border-emerald-500/20 text-emerald-700 dark:text-emerald-300",
											detail.item.status === 'failure' && "bg-rose-500/10 border-rose-500/20 text-rose-700 dark:text-rose-300",
											detail.item.status === 'processing' && "bg-amber-500/10 border-amber-500/20 text-amber-700 dark:text-amber-300"
										)}>
											{detail.item.status === 'success' && <CheckCircleIcon className="h-3.5 w-3.5" />}
											{detail.item.status === 'failure' && <XCircleIcon className="h-3.5 w-3.5" />}
											{detail.item.status === 'processing' && <ClockIcon className="h-3.5 w-3.5" />}
											<span className="text-xs font-medium">{statusLabel(detail.item.status)}</span>
										</div>
										<div className={cn(
											"inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-xs font-medium",
											detail.item.pathReadyCount === detail.item.pathReadyTotal 
												? "bg-emerald-50 text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-300 border-emerald-200 dark:border-emerald-500/20" 
												: "bg-muted/50 text-muted-foreground border-border"
										)}>
											<RouteIcon className="h-3.5 w-3.5" />
											<span>{detail.item.pathReadyCount}/{detail.item.pathReadyTotal}</span>
										</div>
										<Badge variant="outline">{recordSourceLabel(detail.item.recordSource)}</Badge>
										{detail.item.isStalled ? (
											<Badge variant="danger">异常等待 {formatStalledMinutes(detail.item.stalledMinutes)}</Badge>
										) : null}
									</div>

									{/* 核心信息 - 大卡片突出显示 */}
									<div className="rounded-2xl bg-gradient-to-br from-muted/60 via-muted/40 to-background border border-border/60 p-5">
										<div className="grid gap-4 md:grid-cols-2">
											<div>
												<div className="flex items-center gap-2 text-[11px] font-medium text-muted-foreground uppercase tracking-wider mb-1.5">
													<FileCodeIcon className="h-3 w-3" />
													itemCode
												</div>
												<div className="font-mono text-sm bg-background/60 rounded-lg px-3 py-2 border border-border/40 truncate">
													{detail.item.itemCode}
												</div>
											</div>
											<div>
												<div className="flex items-center gap-2 text-[11px] font-medium text-muted-foreground uppercase tracking-wider mb-1.5">
													<DatabaseIcon className="h-3 w-3" />
													产品名称
												</div>
												<div className="text-sm bg-background/60 rounded-lg px-3 py-2 border border-border/40 truncate">
													{detail.item.productName || "-"}
												</div>
											</div>
										</div>
										
										<div className="grid grid-cols-2 gap-4 mt-4 pt-4 border-t border-border/40">
											<div className="flex items-center gap-3">
												<div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/5 text-primary">
													<ClockIcon className="h-4 w-4" />
												</div>
												<div>
													<div className="text-[10px] text-muted-foreground">更新时间</div>
													<div className="text-xs font-medium">{formatDisplayDate(detail.item.updateTime)}</div>
												</div>
											</div>
											<div className="flex items-center gap-3">
												<div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/5 text-primary">
													<CalendarIcon className="h-4 w-4" />
												</div>
												<div>
													<div className="text-[10px] text-muted-foreground">创建时间</div>
													<div className="text-xs font-medium">{formatDisplayDate(detail.item.createTime)}</div>
												</div>
											</div>
										</div>

										{detail.item.recordSource === "batch_tracking" ? (
											<div className="grid gap-4 mt-4 pt-4 border-t border-border/40 md:grid-cols-2">
												<div className="rounded-lg bg-background/60 px-3 py-2 border border-border/40 text-sm">
													<div className="text-[10px] text-muted-foreground mb-1">批次来源</div>
													<div className="font-mono text-xs">{detail.item.batchRunId || "-"}</div>
												</div>
												<div className="rounded-lg bg-background/60 px-3 py-2 border border-border/40 text-sm">
													<div className="text-[10px] text-muted-foreground mb-1">文件标识</div>
													<div className="text-xs">
														CAD: {detail.item.cadNumber || "-"} / 文件: {detail.item.fileName || "-"}
													</div>
												</div>
											</div>
										) : null}
									</div>

									{/* 推理类型 */}
									{detail.item.inferenceTypes?.length > 0 && (
										<div className="flex items-center gap-3">
											<span className="text-xs text-muted-foreground">推理类型:</span>
											<div className="flex flex-wrap gap-2">
												{detail.item.inferenceTypes.map((type, idx) => (
													<div 
														key={idx}
														className="inline-flex items-center gap-1.5 rounded-lg bg-violet-500/10 text-violet-700 dark:text-violet-300 border border-violet-500/20 px-2.5 py-1"
													>
														<SparklesIcon className="h-3 w-3" />
														<span className="text-xs font-medium">
															{type === 1 ? "3D" : type === 2 ? "局部" : type === 3 ? "2D视图" : type === 4 ? "2D剖面" : type}
														</span>
													</div>
												))}
											</div>
										</div>
									)}

									{/* 关键路径 - 使用步骤条样式 */}
									<div className="rounded-2xl border border-border/60 bg-muted/20 p-5">
										<div className="flex items-center justify-between mb-4">
											<div className="flex items-center gap-2">
												<RouteIcon className="h-4 w-4 text-muted-foreground" />
												<span className="text-sm font-medium">关键路径准备度</span>
											</div>
											<div className="flex items-center gap-1.5 text-xs">
												<span className={cn(
													"font-bold",
													detail.item.pathReadyCount === detail.item.pathReadyTotal ? "text-emerald-600 dark:text-emerald-400" : "text-amber-600 dark:text-amber-400"
												)}>{detail.item.pathReadyCount}</span>
												<span className="text-muted-foreground">/</span>
												<span className="text-muted-foreground">{detail.item.pathReadyTotal}</span>
											</div>
										</div>
										
										<div className="grid grid-cols-4 gap-3">
											{[
												{ key: 'hasSourceFilePath', label: '源文件', path: detail.item.sourceFilePath, icon: FileIcon },
												{ key: 'hasConvertedFile', label: 'STEP', path: detail.item.convertedFilePath, icon: FileBoxIcon },
												{ key: 'hasPcAddress', label: 'PC', path: detail.item.pcAddress, icon: BoxIcon },
												{ key: 'hasGlbAddress', label: 'GLB', path: detail.item.glbAddress, icon: PackageIcon },
											].map((item, index) => {
												const isReady = detail.item[item.key as keyof typeof detail.item] as boolean;
												const Icon = item.icon;
												return (
													<div key={item.key} className="relative">
														<div className={cn(
															"flex flex-col items-center text-center p-3 rounded-xl border transition-all",
															isReady 
																? "bg-emerald-50/80 border-emerald-200 dark:bg-emerald-500/10 dark:border-emerald-500/30" 
																: "bg-background/50 border-border/60 opacity-70"
														)}>
															<div className={cn(
																"flex h-10 w-10 items-center justify-center rounded-xl mb-2 transition-colors",
																isReady 
																	? "bg-emerald-500 text-white shadow-sm shadow-emerald-500/20" 
																	: "bg-muted text-muted-foreground"
															)}>
																<Icon className="h-5 w-5" />
															</div>
															<span className={cn(
																"text-xs font-medium",
																isReady ? "text-emerald-700 dark:text-emerald-300" : "text-muted-foreground"
															)}>{item.label}</span>
															{isReady && item.path && (
																<div className="mt-1.5 text-[9px] text-emerald-600/70 dark:text-emerald-400/70 truncate max-w-full px-1" title={item.path}>
																	{truncateText(item.path, 15)}
																</div>
															)}
														</div>
														{index < 3 && (
															<div className="absolute top-1/2 -right-1.5 w-3 h-px bg-border hidden md:block" />
														)}
													</div>
												);
											})}
										</div>
									</div>

									{detail.item.missingPaths.length || detail.item.processStartTime || detail.item.processEndTime || detail.item.productUpdateTime ? (
										<div className="grid gap-3 md:grid-cols-2">
											<DetailField label="缺失产物" value={formatMissingPaths(detail.item.missingPaths)} />
											<DetailField label="最近更新时间链路" value={`本地开始: ${formatDisplayDate(detail.item.processStartTime)}\n本地结束: ${formatDisplayDate(detail.item.processEndTime)}\n正式更新时间: ${formatDisplayDate(detail.item.productUpdateTime)}`} />
										</div>
									) : null}

									{/* 错误信息 */}
									{detail.item.errorMsg && (
										<div className="rounded-xl border border-rose-200 dark:border-rose-500/30 bg-gradient-to-br from-rose-50 to-rose-100/50 dark:from-rose-500/10 dark:to-rose-500/5 p-4">
											<div className="flex items-start gap-3">
												<div className="flex h-8 w-8 items-center justify-center rounded-lg bg-rose-500 text-white flex-shrink-0">
													<AlertTriangleIcon className="h-4 w-4" />
												</div>
												<div className="min-w-0 flex-1">
													<div className="text-sm font-semibold text-rose-700 dark:text-rose-300 mb-1">错误信息</div>
													<div className="text-sm text-rose-600/90 dark:text-rose-200/80 whitespace-pre-wrap break-all leading-relaxed">{detail.item.errorMsg}</div>
												</div>
											</div>
										</div>
									)}
								</div>
							) : (
								<div className="flex flex-col items-center justify-center py-16 text-center">
									<div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted mb-3">
										<HelpCircleIcon className="h-6 w-6 text-muted-foreground" />
									</div>
									<div className="text-sm font-medium text-muted-foreground">暂无详情</div>
									<div className="text-xs text-muted-foreground/70 mt-1">无法加载记录详情信息</div>
								</div>
							)}
						</div>
					</div>
				</DialogContent>
			</Dialog>

			<ActiveAlerts />
			<FooterRepoLink />
		</div>
	)
})

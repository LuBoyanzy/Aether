import { EyeIcon, LoaderCircleIcon, RefreshCwIcon } from "lucide-react"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
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
import { BRAND_NAME, formatShortDate } from "@/lib/utils"

const refreshIntervalMs = 15000

function statusBadgeVariant(status: IngestMonitorRecord["status"] | IngestMonitorBatchItem["ingestStatus"]) {
	switch (status) {
		case "success":
			return "success"
		case "failure":
			return "danger"
		case "pending":
			return "secondary"
		default:
			return "outline"
	}
}

function statusLabel(status: IngestMonitorRecord["status"] | IngestMonitorBatchItem["ingestStatus"]) {
	switch (status) {
		case "success":
			return "正常"
		case "failure":
			return "异常"
		case "pending":
			return "处理中"
		default:
			return "待确认"
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

function processStatusBadgeVariant(status: string) {
	switch (status) {
		case "completed":
		case "success":
			return "success"
		case "failed":
		case "error":
			return "danger"
		case "processing":
			return "secondary"
		default:
			return "outline"
	}
}

function processStatusLabel(status: string) {
	switch (status) {
		case "completed":
			return "已投递 MQ"
		case "success":
			return "处理成功"
		case "failed":
			return "处理失败"
		case "error":
			return "处理异常"
		case "processing":
			return "处理中"
		case "pending":
			return "待处理"
		default:
			return status || "-"
	}
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
	if (batch.failureCount > 0 && batch.pendingCount === 0 && batch.successCount === 0) {
		return "失败未完成"
	}
	if (batch.status === "running" || batch.status === "pending") {
		return "进行中"
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

	useEffect(() => {
		document.title = `${BRAND_NAME} - 入库服务可视化`
	}, [])

	const loadDashboard = useCallback(async ({ silent = false }: { silent?: boolean } = {}) => {
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

	const openBatchDetail = useCallback(async (batchRunId: string) => {
		setBatchDetailOpen(true)
		setBatchDetailLoading(true)
		setBatchDetailError("")
		setBatchDetail(null)
		try {
			setBatchDetail(await fetchIngestMonitorBatchDetail(batchRunId))
		} catch (err) {
			setBatchDetailError(err instanceof Error ? err.message : String(err))
		} finally {
			setBatchDetailLoading(false)
		}
	}, [])

	const cards = useMemo(() => {
		const summary = summaryData?.summary
		return [
			{ key: "total", title: "正式入库总数", value: summary?.total ?? 0, description: "仅统计正式入库记录" },
			{ key: "success", title: "正常", value: summary?.success ?? 0, description: "完成且关键产物路径齐全" },
			{ key: "failure", title: "异常", value: summary?.failure ?? 0, description: "失败或存在错误信息" },
			{ key: "pending", title: "处理中", value: summary?.pending ?? 0, description: "仍处于处理中状态" },
			{ key: "unknown", title: "待确认", value: summary?.unknown ?? 0, description: "状态未落在明确口径内" },
		]
	}, [summaryData])

	const scopeTenant = summaryData?.scope.tenant || batchListData?.scope.tenant || ""
	const scopeText = scopeTenant
		? `租户：${formatTenantLabel(scopeTenant)} · 正式入库 + XXL 批次`
		: "正式入库 + XXL 批次"

	return (
		<div className="grid gap-4 pb-8">
			<Card className="border-border/60">
				<CardHeader className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
					<div>
						<CardTitle>入库服务可视化</CardTitle>
					</div>
					<div className="flex items-center gap-2">
						<Badge variant="outline">{scopeText}</Badge>
						<Button variant="outline" size="sm" onClick={() => loadDashboard({ silent: true })} disabled={loading || refreshing}>
							{refreshing ? (
								<LoaderCircleIcon className="mr-2 h-4 w-4 animate-spin" />
							) : (
								<RefreshCwIcon className="mr-2 h-4 w-4" />
							)}
							刷新
						</Button>
					</div>
				</CardHeader>
			</Card>

			<div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
				{cards.map((card) => (
					<Card key={card.key} className="border-border/60">
						<CardHeader className="space-y-1">
							<CardDescription>{card.title}</CardDescription>
							<CardTitle className="text-3xl">{card.value}</CardTitle>
						</CardHeader>
						<CardContent className="pt-0 text-sm text-muted-foreground">{card.description}</CardContent>
					</Card>
				))}
			</div>

			{error ? (
				<Card className="border-red-500/50">
					<CardHeader>
						<CardTitle>数据加载失败</CardTitle>
						<CardDescription>{error}</CardDescription>
					</CardHeader>
				</Card>
			) : null}

			<Card className="border-border/60">
				<CardHeader>
					<CardTitle>最近扫描批次</CardTitle>
					<CardDescription>按 XXL 大任务维度展示扫描统计、投递耗时和最终正式入库完成耗时。</CardDescription>
				</CardHeader>
				<CardContent>
					{loading && !batchListData ? (
						<div className="flex items-center justify-center py-12 text-muted-foreground">
							<LoaderCircleIcon className="mr-2 h-4 w-4 animate-spin" />
							正在加载批次数据...
						</div>
					) : batchListData?.batches.length ? (
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>批次 ID</TableHead>
									<TableHead>XXL 任务</TableHead>
									<TableHead>扫描目录</TableHead>
									<TableHead>扫描统计</TableHead>
									<TableHead>扫描投递耗时</TableHead>
									<TableHead>最终入库完成耗时</TableHead>
									<TableHead>成功/失败/处理中</TableHead>
									<TableHead>开始时间</TableHead>
									<TableHead className="w-[88px]">详情</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{batchListData.batches.map((batch) => (
									<TableRow key={batch.batchRunId}>
										<TableCell className="font-mono text-xs">{truncateText(batch.batchRunId, 16)}</TableCell>
										<TableCell>
											<div className="flex flex-col gap-1">
												<Badge variant={batchStatusBadgeVariant(batch.status)}>{batchStatusLabel(batch.status)}</Badge>
												<div className="text-xs text-muted-foreground">
													jobId={batch.xxlJobId || "-"} / logId={batch.xxlLogId || "-"}
												</div>
											</div>
										</TableCell>
										<TableCell className="max-w-[260px]">{formatBatchPaths(batch.scanPaths)}</TableCell>
										<TableCell className="text-sm">
											<div>{batch.totalDirsScanned} 目录</div>
											<div>{batch.totalFilesProcessed}/{batch.totalFilesScanned} 文件</div>
											<div>{batch.totalBatches} 批</div>
										</TableCell>
										<TableCell>{formatElapsedSeconds(batch.xxlScanElapsedSeconds)}</TableCell>
										<TableCell>{formatBatchFinalElapsed(batch)}</TableCell>
										<TableCell>
											<div className="flex flex-wrap gap-1">
												<Badge variant="success">{batch.successCount}</Badge>
												<Badge variant="danger">{batch.failureCount}</Badge>
												<Badge variant="secondary">{batch.pendingCount}</Badge>
											</div>
										</TableCell>
										<TableCell>{formatDisplayDate(batch.scanStartedAt)}</TableCell>
										<TableCell>
											<Button variant="ghost" size="icon" onClick={() => openBatchDetail(batch.batchRunId)}>
												<EyeIcon className="h-4 w-4" />
											</Button>
										</TableCell>
									</TableRow>
								))}
							</TableBody>
						</Table>
					) : (
						<div className="py-8 text-sm text-muted-foreground">暂无扫描批次记录。</div>
					)}
				</CardContent>
			</Card>

			<Card className="border-border/60">
				<CardHeader>
					<CardTitle>最近正式入库记录</CardTitle>
					<CardDescription>按更新时间倒序展示最近正式入库记录，自动刷新间隔 {Math.round(refreshIntervalMs / 1000)} 秒。</CardDescription>
				</CardHeader>
				<CardContent>
					{loading && !summaryData ? (
						<div className="flex items-center justify-center py-12 text-muted-foreground">
							<LoaderCircleIcon className="mr-2 h-4 w-4 animate-spin" />
							正在加载入库状态...
						</div>
					) : summaryData?.recent.length ? (
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>itemCode</TableHead>
									<TableHead>产品名称</TableHead>
									<TableHead>状态</TableHead>
									<TableHead>is_complete</TableHead>
									<TableHead>关键路径</TableHead>
									<TableHead>推理类型</TableHead>
									<TableHead>更新时间</TableHead>
									<TableHead className="w-[88px]">详情</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{summaryData.recent.map((record) => (
									<TableRow key={`${record.itemCode}-${record.updateTime}`}>
										<TableCell className="font-mono text-xs">{record.itemCode}</TableCell>
										<TableCell>{record.productName || "-"}</TableCell>
										<TableCell>
											<Badge variant={statusBadgeVariant(record.status)}>{statusLabel(record.status)}</Badge>
										</TableCell>
										<TableCell>{record.isComplete ?? "-"}</TableCell>
										<TableCell>
											<Badge variant={record.pathReadyCount === record.pathReadyTotal ? "success" : "outline"}>
												{record.pathReadyCount}/{record.pathReadyTotal}
											</Badge>
										</TableCell>
										<TableCell>{formatInferenceTypes(record)}</TableCell>
										<TableCell>{formatRecordTime(record)}</TableCell>
										<TableCell>
											<Button variant="ghost" size="icon" onClick={() => openDetail(record.itemCode)}>
												<EyeIcon className="h-4 w-4" />
											</Button>
										</TableCell>
									</TableRow>
								))}
							</TableBody>
						</Table>
					) : (
						<div className="py-8 text-sm text-muted-foreground">暂无正式入库记录。</div>
					)}
				</CardContent>
			</Card>

			<Card className="border-border/60">
				<CardHeader>
					<CardTitle>异常记录</CardTitle>
					<CardDescription>优先关注失败记录和存在错误信息的正式入库记录。</CardDescription>
				</CardHeader>
				<CardContent>
					{summaryData?.failures.length ? (
						<Table>
							<TableHeader>
								<TableRow>
									<TableHead>itemCode</TableHead>
									<TableHead>产品名称</TableHead>
									<TableHead>状态</TableHead>
									<TableHead>错误信息</TableHead>
									<TableHead>更新时间</TableHead>
									<TableHead className="w-[88px]">详情</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{summaryData.failures.map((record) => (
									<TableRow key={`${record.itemCode}-${record.updateTime}-failure`}>
										<TableCell className="font-mono text-xs">{record.itemCode}</TableCell>
										<TableCell>{record.productName || "-"}</TableCell>
										<TableCell>
											<Badge variant={statusBadgeVariant(record.status)}>{statusLabel(record.status)}</Badge>
										</TableCell>
										<TableCell className="max-w-[420px]">{truncateText(record.errorMsg, 96)}</TableCell>
										<TableCell>{formatRecordTime(record)}</TableCell>
										<TableCell>
											<Button variant="ghost" size="icon" onClick={() => openDetail(record.itemCode)}>
												<EyeIcon className="h-4 w-4" />
											</Button>
										</TableCell>
									</TableRow>
								))}
							</TableBody>
						</Table>
					) : (
						<div className="py-8 text-sm text-muted-foreground">当前没有异常记录。</div>
					)}
				</CardContent>
			</Card>

			<Dialog open={batchDetailOpen} onOpenChange={setBatchDetailOpen}>
				<DialogContent className="max-w-6xl max-h-[85vh] overflow-auto">
					<DialogHeader>
						<DialogTitle>扫描批次详情</DialogTitle>
						<DialogDescription>查看某次 XXL 大任务的扫描统计、入库耗时和批次内记录状态。</DialogDescription>
					</DialogHeader>

					{batchDetailLoading ? (
						<div className="flex items-center justify-center py-10 text-muted-foreground">
							<LoaderCircleIcon className="mr-2 h-4 w-4 animate-spin" />
							正在加载批次详情...
						</div>
					) : batchDetailError ? (
						<div className="rounded-md border border-red-500/40 bg-red-500/5 p-4 text-sm">{batchDetailError}</div>
					) : batchDetail ? (
						<div className="grid gap-4">
							<div className="flex flex-wrap items-center gap-2">
								<Badge variant="outline">租户：{formatTenantLabel(batchDetail.scope.tenant)}</Badge>
								<Badge variant={batchStatusBadgeVariant(batchDetail.batch.status)}>{batchStatusLabel(batchDetail.batch.status)}</Badge>
								<Badge variant="outline">batchRunId={batchDetail.batch.batchRunId}</Badge>
							</div>

							<div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
								<Card className="border-border/60">
									<CardHeader className="space-y-1">
										<CardDescription>扫描投递耗时</CardDescription>
										<CardTitle>{formatElapsedSeconds(batchDetail.batch.xxlScanElapsedSeconds)}</CardTitle>
									</CardHeader>
								</Card>
								<Card className="border-border/60">
									<CardHeader className="space-y-1">
										<CardDescription>最终入库完成耗时</CardDescription>
										<CardTitle>{formatBatchFinalElapsed(batchDetail.batch)}</CardTitle>
									</CardHeader>
								</Card>
								<Card className="border-border/60">
									<CardHeader className="space-y-1">
										<CardDescription>成功 / 失败 / 处理中</CardDescription>
										<CardTitle>
											{batchDetail.batch.successCount} / {batchDetail.batch.failureCount} / {batchDetail.batch.pendingCount}
										</CardTitle>
									</CardHeader>
								</Card>
								<Card className="border-border/60">
									<CardHeader className="space-y-1">
										<CardDescription>扫描目录 / 文件 / 批次</CardDescription>
										<CardTitle>
											{batchDetail.batch.totalDirsScanned} / {batchDetail.batch.totalFilesProcessed} / {batchDetail.batch.totalBatches}
										</CardTitle>
									</CardHeader>
								</Card>
							</div>

							<div className="grid gap-3 md:grid-cols-2">
								<DetailField label="XXL 任务" value={`jobId=${batchDetail.batch.xxlJobId || "-"} / logId=${batchDetail.batch.xxlLogId || "-"}`} />
								<DetailField label="扫描开始 / 结束" value={`${formatDisplayDate(batchDetail.batch.scanStartedAt)}\n${formatDisplayDate(batchDetail.batch.scanFinishedAt)}`} />
								<DetailField label="扫描路径" value={batchDetail.batch.scanPaths.join("\n")} />
								<DetailField label="批次配置" value={`fileType=${batchDetail.batch.fileType ?? "-"}\nbatchSize=${batchDetail.batch.batchSize}\nforce=${batchDetail.batch.force ? "是" : "否"}`} />
							</div>

							{batchDetail.batch.errorMessage ? <DetailField label="批次错误信息" value={batchDetail.batch.errorMessage} /> : null}

							<Card className="border-border/60">
								<CardHeader>
									<CardTitle>批次内记录</CardTitle>
									<CardDescription>按异常优先，其次处理中，再到已完成记录排序。</CardDescription>
								</CardHeader>
								<CardContent>
									{batchDetail.items.length ? (
										<Table>
											<TableHeader>
												<TableRow>
													<TableHead>itemCode</TableHead>
													<TableHead>cad_number</TableHead>
													<TableHead>文件名</TableHead>
													<TableHead>入库状态</TableHead>
													<TableHead>扫描状态</TableHead>
													<TableHead>关键路径</TableHead>
													<TableHead>更新时间</TableHead>
													<TableHead className="w-[88px]">详情</TableHead>
												</TableRow>
											</TableHeader>
											<TableBody>
												{batchDetail.items.map((item) => (
													<TableRow key={`${batchDetail.batch.batchRunId}-${item.itemCode}`}>
														<TableCell className="font-mono text-xs">{item.itemCode}</TableCell>
														<TableCell className="font-mono text-xs">{truncateText(item.cadNumber, 30)}</TableCell>
														<TableCell>{truncateText(item.fileName, 36)}</TableCell>
														<TableCell>
															<Badge variant={statusBadgeVariant(item.ingestStatus)}>{statusLabel(item.ingestStatus)}</Badge>
														</TableCell>
														<TableCell>
															<Badge variant={processStatusBadgeVariant(item.processStatus)}>{processStatusLabel(item.processStatus)}</Badge>
														</TableCell>
														<TableCell>
															<Badge variant={item.pathReadyCount === item.pathReadyTotal ? "success" : "outline"}>
																{item.pathReadyCount}/{item.pathReadyTotal}
															</Badge>
														</TableCell>
														<TableCell>{formatDisplayDate(item.updateTime || item.createTime)}</TableCell>
														<TableCell>
															<Button variant="ghost" size="icon" onClick={() => openDetail(item.itemCode)}>
																<EyeIcon className="h-4 w-4" />
															</Button>
														</TableCell>
													</TableRow>
												))}
											</TableBody>
										</Table>
									) : (
										<div className="py-8 text-sm text-muted-foreground">当前批次还没有追踪到记录。</div>
									)}
								</CardContent>
							</Card>
						</div>
					) : (
						<div className="py-8 text-sm text-muted-foreground">暂无批次详情。</div>
					)}
				</DialogContent>
			</Dialog>

			<Dialog open={detailOpen} onOpenChange={setDetailOpen}>
				<DialogContent className="max-w-4xl">
					<DialogHeader>
						<DialogTitle>入库记录详情</DialogTitle>
						<DialogDescription>展示单条正式入库记录的状态、关键路径和错误信息。</DialogDescription>
					</DialogHeader>

					{detailLoading ? (
						<div className="flex items-center justify-center py-10 text-muted-foreground">
							<LoaderCircleIcon className="mr-2 h-4 w-4 animate-spin" />
							正在加载详情...
						</div>
					) : detailError ? (
						<div className="rounded-md border border-red-500/40 bg-red-500/5 p-4 text-sm">{detailError}</div>
					) : detail ? (
						<div className="grid gap-4">
							<div className="flex flex-wrap items-center gap-2">
								<Badge variant="outline">租户：{formatTenantLabel(detail.scope.tenant)}</Badge>
								<Badge variant={statusBadgeVariant(detail.item.status)}>{statusLabel(detail.item.status)}</Badge>
								<Badge variant={detail.item.pathReadyCount === detail.item.pathReadyTotal ? "success" : "outline"}>
									关键路径 {detail.item.pathReadyCount}/{detail.item.pathReadyTotal}
								</Badge>
							</div>

							<div className="grid gap-3 md:grid-cols-2">
								<DetailField label="itemCode" value={detail.item.itemCode} />
								<DetailField label="产品名称" value={detail.item.productName} />
								<DetailField label="is_complete" value={detail.item.isComplete !== undefined ? String(detail.item.isComplete) : "-"} />
								<DetailField label="推理类型" value={formatInferenceTypes(detail.item)} />
								<DetailField label="更新时间" value={formatDisplayDate(detail.item.updateTime)} />
								<DetailField label="创建时间" value={formatDisplayDate(detail.item.createTime)} />
								<DetailField label="source_file_path" value={detail.item.sourceFilePath} />
								<DetailField label="converted_file_path" value={detail.item.convertedFilePath} />
								<DetailField label="pc_address" value={detail.item.pcAddress} />
								<DetailField label="glb_address" value={detail.item.glbAddress} />
							</div>

							<DetailField label="错误信息" value={detail.item.errorMsg} />
						</div>
					) : (
						<div className="py-8 text-sm text-muted-foreground">暂无详情。</div>
					)}
				</DialogContent>
			</Dialog>

			<ActiveAlerts />
			<FooterRepoLink />
		</div>
	)
})

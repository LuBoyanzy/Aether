// ingest-visualization.tsx 展示基于 product_info 的正式入库状态看板。
// 当前口径：仅统计 product_info.is_temporary = false 的正式入库记录。
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
	fetchIngestMonitorDetail,
	fetchIngestMonitorSummary,
	type IngestMonitorDetailResponse,
	type IngestMonitorRecord,
	type IngestMonitorSummaryResponse,
} from "@/lib/ingestMonitor"
import { BRAND_NAME, formatShortDate } from "@/lib/utils"

const refreshIntervalMs = 15000

function statusBadgeVariant(status: IngestMonitorRecord["status"]) {
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

function statusLabel(status: IngestMonitorRecord["status"]) {
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

function formatRecordTime(record: IngestMonitorRecord) {
	return formatDisplayDate(record.updateTime || record.createTime || "")
}

function formatInferenceTypes(record: IngestMonitorRecord) {
	if (!record.inferenceTypes || record.inferenceTypes.length === 0) {
		return "-"
	}
	return record.inferenceTypes.join(", ")
}

function truncateText(value: string, max = 64) {
	const trimmed = value.trim()
	if (trimmed.length <= max) {
		return trimmed || "-"
	}
	return `${trimmed.slice(0, max)}...`
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

function DetailField({ label, value }: { label: string; value: string }) {
	return (
		<div className="rounded-md border border-border/60 bg-muted/30 p-3">
			<div className="text-xs text-muted-foreground">{label}</div>
			<div className="mt-1 break-all text-sm">{value || "-"}</div>
		</div>
	)
}

export default memo(() => {
	const [data, setData] = useState<IngestMonitorSummaryResponse | null>(null)
	const [loading, setLoading] = useState(true)
	const [refreshing, setRefreshing] = useState(false)
	const [error, setError] = useState("")

	const [detailOpen, setDetailOpen] = useState(false)
	const [detailLoading, setDetailLoading] = useState(false)
	const [detailError, setDetailError] = useState("")
	const [detail, setDetail] = useState<IngestMonitorDetailResponse | null>(null)

	useEffect(() => {
		document.title = `${BRAND_NAME} - 入库服务可视化`
	}, [])

	const loadSummary = useCallback(async ({ silent = false }: { silent?: boolean } = {}) => {
		if (silent) {
			setRefreshing(true)
		} else {
			setLoading(true)
		}
		setError("")

		try {
			const response = await fetchIngestMonitorSummary()
			setData(response)
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
		loadSummary()
		const timer = window.setInterval(() => {
			loadSummary({ silent: true })
		}, refreshIntervalMs)
		return () => window.clearInterval(timer)
	}, [loadSummary])

	const openDetail = useCallback(async (itemCode: string) => {
		setDetailOpen(true)
		setDetailLoading(true)
		setDetailError("")
		setDetail(null)

		try {
			const response = await fetchIngestMonitorDetail(itemCode)
			setDetail(response)
		} catch (err) {
			setDetailError(err instanceof Error ? err.message : String(err))
		} finally {
			setDetailLoading(false)
		}
	}, [])

	const cards = useMemo(() => {
		const summary = data?.summary
		return [
			{
				key: "total",
				title: "正式入库总数",
				value: summary?.total ?? 0,
				description: "仅统计 product_info.is_temporary = false",
			},
			{ key: "success", title: "正常", value: summary?.success ?? 0, description: "完成且关键产物路径齐全" },
			{ key: "failure", title: "异常", value: summary?.failure ?? 0, description: "失败或存在错误信息" },
			{ key: "pending", title: "处理中", value: summary?.pending ?? 0, description: "仍处于处理中状态" },
			{ key: "unknown", title: "待确认", value: summary?.unknown ?? 0, description: "状态未落在明确口径内" },
		]
	}, [data])

	const scopeText = data ? `租户：${data.scope.tenant} · 口径：正式入库记录` : "口径：正式入库记录"

	return (
		<div className="grid gap-4 pb-8">
			<Card className="border-border/60">
				<CardHeader className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
					<div>
						<CardTitle>入库服务可视化</CardTitle>
					</div>
					<div className="flex items-center gap-2">
						<Badge variant="outline">{scopeText}</Badge>
						<Button
							variant="outline"
							size="sm"
							onClick={() => {
								loadSummary({ silent: true })
							}}
							disabled={loading || refreshing}
						>
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
					<CardTitle>最近正式入库记录</CardTitle>
					<CardDescription>
						按更新时间倒序展示最近记录，自动刷新间隔 {Math.round(refreshIntervalMs / 1000)} 秒。
					</CardDescription>
				</CardHeader>
				<CardContent>
					{loading && !data ? (
						<div className="flex items-center justify-center py-12 text-muted-foreground">
							<LoaderCircleIcon className="mr-2 h-4 w-4 animate-spin" />
							正在加载入库状态...
						</div>
					) : data?.recent.length ? (
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
								{data.recent.map((record) => (
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
											<Button
												variant="ghost"
												size="icon"
												onClick={() => {
													openDetail(record.itemCode)
												}}
											>
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
					{data?.failures.length ? (
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
								{data.failures.map((record) => (
									<TableRow key={`${record.itemCode}-${record.updateTime}-failure`}>
										<TableCell className="font-mono text-xs">{record.itemCode}</TableCell>
										<TableCell>{record.productName || "-"}</TableCell>
										<TableCell>
											<Badge variant={statusBadgeVariant(record.status)}>{statusLabel(record.status)}</Badge>
										</TableCell>
										<TableCell className="max-w-[420px]">{truncateText(record.errorMsg, 96)}</TableCell>
										<TableCell>{formatRecordTime(record)}</TableCell>
										<TableCell>
											<Button
												variant="ghost"
												size="icon"
												onClick={() => {
													openDetail(record.itemCode)
												}}
											>
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
								<Badge variant="outline">租户：{detail.scope.tenant}</Badge>
								<Badge variant={statusBadgeVariant(detail.item.status)}>{statusLabel(detail.item.status)}</Badge>
								<Badge variant={detail.item.pathReadyCount === detail.item.pathReadyTotal ? "success" : "outline"}>
									关键路径 {detail.item.pathReadyCount}/{detail.item.pathReadyTotal}
								</Badge>
							</div>

							<div className="grid gap-3 md:grid-cols-2">
								<DetailField label="itemCode" value={detail.item.itemCode} />
								<DetailField label="产品名称" value={detail.item.productName} />
								<DetailField
									label="is_complete"
									value={detail.item.isComplete !== undefined ? String(detail.item.isComplete) : "-"}
								/>
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

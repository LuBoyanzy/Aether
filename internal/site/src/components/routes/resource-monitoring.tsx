import {
	ActivityIcon,
	AlertTriangleIcon,
	ChevronRightIcon,
	CpuIcon,
	HardDriveIcon,
	LoaderCircleIcon,
	MemoryStickIcon,
	NetworkIcon,
	RefreshCwIcon,
	ServerIcon,
} from "lucide-react"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts"
import { ActiveAlerts } from "@/components/active-alerts"
import { FooterRepoLink } from "@/components/footer-repo-link"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ChartContainer, ChartTooltip, ChartTooltipContent, pinnedAxisDomain } from "@/components/ui/chart"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import {
	buildI3DResourceChartPoints,
	hasI3DResourceChartData,
	type I3DResourceChartPoint,
	targetChartKey,
} from "@/lib/i3dResourceCharts"
import { formatI3DBytesPerSecond, formatI3DBytesValue } from "@/lib/i3dResourceFormatters"
import {
	fetchI3DResourceOverview,
	fetchI3DResourceTimeseries,
	type I3DResourceOverview,
	type I3DResourceTarget,
	type I3DResourceTimeseries,
} from "@/lib/i3dResources"
import { buildI3DResourceTreeRows, type I3DResourceTreeRow, visibleI3DResourceTreeRows } from "@/lib/i3dResourceTree"
import { BRAND_NAME, cn, decimalString, formatSecondsToHuman, formatShortDate } from "@/lib/utils"

const refreshIntervalMs = 15000

function tooltipValueDescSorter(a: { value?: unknown }, b: { value?: unknown }) {
	const left = typeof a.value === "number" ? a.value : Number(a.value ?? 0)
	const right = typeof b.value === "number" ? b.value : Number(b.value ?? 0)
	return right - left
}

function environmentLabel(environment: I3DResourceOverview["environment"]) {
	return environment === "release" ? "交付 Docker" : "本地开发"
}

function statusLabel(status: I3DResourceTarget["status"]) {
	switch (status) {
		case "up":
			return "正常"
		case "down":
			return "未运行"
		default:
			return "未知"
	}
}

function statusBadgeClass(status: I3DResourceTarget["status"]) {
	switch (status) {
		case "up":
			return "border-green-500/30 bg-green-500/10 text-green-700 dark:text-green-300"
		case "down":
			return "border-red-500/30 bg-red-500/10 text-red-700 dark:text-red-300"
		default:
			return "border-yellow-500/30 bg-yellow-500/10 text-yellow-700 dark:text-yellow-300"
	}
}

function memoryDetailText(row: I3DResourceTreeRow) {
	const parts: string[] = []
	if (row.memory_rss_bytes > 0) {
		parts.push(`常驻 ${formatI3DBytesValue(row.memory_rss_bytes)}`)
	}
	if (row.memory_cache_bytes > 0) {
		parts.push(`缓存 ${formatI3DBytesValue(row.memory_cache_bytes)}`)
	}
	return parts.length > 0 ? parts.join(" / ") : ""
}

function unitDetailText(row: I3DResourceTreeRow) {
	const parts: string[] = []
	if (row.pids > 0) {
		parts.push(row.kind === "target" && row.target?.kind === "process" ? `进程 ${row.pids}` : `进程/线程 ${row.pids}`)
	}
	if (row.threads > 0) {
		parts.push(`线程 ${row.threads}`)
	}
	return parts.length > 0 ? parts.join(" / ") : ""
}

function ResourceStatCard({
	title,
	value,
	description,
	icon: Icon,
	tone = "default",
}: {
	title: string
	value: string
	description?: string
	icon: typeof CpuIcon
	tone?: "default" | "danger"
}) {
	return (
		<Card className={cn("border-border/60", tone === "danger" && "border-red-500/30")}>
			<CardHeader className="flex flex-row items-start justify-between gap-3 space-y-0 pb-2">
				<CardTitle className="text-sm text-muted-foreground">{title}</CardTitle>
				<Icon className={cn("h-4 w-4 text-muted-foreground", tone === "danger" && "text-red-500")} />
			</CardHeader>
			<CardContent>
				<div className="text-2xl font-semibold tabular-nums">{value}</div>
				{description && <p className="mt-1 text-xs text-muted-foreground">{description}</p>}
			</CardContent>
		</Card>
	)
}

function ResourceTreeTable({
	rows,
	collapsedGroupIDs,
	onToggleGroup,
}: {
	rows: I3DResourceTreeRow[]
	collapsedGroupIDs: Set<string>
	onToggleGroup: (id: string) => void
}) {
	const statusCell = (row: I3DResourceTreeRow) => {
		if (row.kind === "group") {
			return row.abnormal_count > 0 ? (
				<Badge variant="outline" className={statusBadgeClass("unknown")}>
					异常 {row.abnormal_count}
				</Badge>
			) : (
				<Badge variant="outline" className={statusBadgeClass("up")}>
					正常
				</Badge>
			)
		}
		const item = row.target
		if (!item) return null
		return (
			<Badge variant="outline" className={statusBadgeClass(item.status)}>
				{statusLabel(item.status)}
			</Badge>
		)
	}

	return (
		<div className="overflow-x-auto rounded-md border border-border/60">
			<Table>
				<TableHeader>
					<TableRow>
						<TableHead className="min-w-52">名称</TableHead>
						<TableHead>状态</TableHead>
						<TableHead className="text-right">CPU</TableHead>
						<TableHead className="min-w-44 text-right">内存</TableHead>
						<TableHead className="text-right">GPU 显存</TableHead>
						<TableHead className="text-right">磁盘 IO</TableHead>
						<TableHead className="text-right">网络 IO</TableHead>
						<TableHead className="min-w-28 text-right">实例</TableHead>
						<TableHead className="text-right">运行时长</TableHead>
					</TableRow>
				</TableHeader>
				<TableBody>
					{rows.length === 0 ? (
						<TableRow>
							<TableCell colSpan={9} className="h-24 text-center text-muted-foreground">
								暂无采集目标
							</TableCell>
						</TableRow>
					) : (
						rows.map((row) => {
							const isCollapsed = row.kind === "group" && collapsedGroupIDs.has(row.id)
							return (
								<TableRow
									key={row.id}
									className={cn(row.kind === "group" && "cursor-pointer bg-muted/45 hover:bg-muted/60")}
									onClick={row.kind === "group" ? () => onToggleGroup(row.id) : undefined}
								>
									<TableCell>
										<div className={cn("flex items-center gap-2 font-medium", row.level === 1 && "ps-6")}>
											{row.kind === "group" && (
												<ChevronRightIcon
													className={cn(
														"h-4 w-4 text-muted-foreground transition-transform",
														!isCollapsed && "rotate-90"
													)}
												/>
											)}
											<span>{row.name}</span>
										</div>
										<div className={cn("text-xs text-muted-foreground", row.level === 1 && "ps-6")}>
											{row.kind === "group"
												? `${isCollapsed ? "已收起" : "已展开"} · ${row.unit_count} 个实例`
												: row.id}
										</div>
									</TableCell>
									<TableCell>{statusCell(row)}</TableCell>
									<TableCell className="text-right tabular-nums">
										{decimalString(row.cpu_percent || 0, row.cpu_percent >= 10 ? 1 : 2)}%
									</TableCell>
									<TableCell className="text-right tabular-nums">
										<div>{formatI3DBytesValue(row.memory_bytes)}</div>
										{memoryDetailText(row) && (
											<div className="text-xs text-muted-foreground">{memoryDetailText(row)}</div>
										)}
									</TableCell>
									<TableCell className="text-right tabular-nums">
										{formatI3DBytesValue(row.target?.gpu_memory_bytes ?? 0)}
									</TableCell>
									<TableCell className="text-right tabular-nums">
										<div>{formatI3DBytesPerSecond(row.disk_read_bps + row.disk_write_bps)}</div>
										<div className="text-xs text-muted-foreground">
											读 {formatI3DBytesPerSecond(row.disk_read_bps)} / 写 {formatI3DBytesPerSecond(row.disk_write_bps)}
										</div>
									</TableCell>
									<TableCell className="text-right tabular-nums">
										<div>{formatI3DBytesPerSecond(row.network_rx_bps + row.network_tx_bps)}</div>
										<div className="text-xs text-muted-foreground">
											收 {formatI3DBytesPerSecond(row.network_rx_bps)} / 发{" "}
											{formatI3DBytesPerSecond(row.network_tx_bps)}
										</div>
									</TableCell>
									<TableCell className="text-right tabular-nums">
										<div>{row.unit_count || 0}</div>
										{unitDetailText(row) && <div className="text-xs text-muted-foreground">{unitDetailText(row)}</div>}
									</TableCell>
									<TableCell className="text-right tabular-nums">
										{row.target?.uptime_seconds ? formatSecondsToHuman(row.target.uptime_seconds) : "-"}
									</TableCell>
								</TableRow>
							)
						})
					)}
				</TableBody>
			</Table>
		</div>
	)
}

function ResourceTrendCard({
	title,
	description,
	points,
	series,
	valueFormatter,
	yTickFormatter,
	domain,
	showTotal = false,
}: {
	title: string
	description: string
	points: I3DResourceChartPoint[]
	series: {
		key: string
		name: string
		color: string
		opacity: number
		stackId?: string
	}[]
	valueFormatter: (value: number) => string
	yTickFormatter: (value: number) => string
	domain?: [number, number | ((dataMax: number) => number)]
	showTotal?: boolean
}) {
	return (
		<Card className="pb-2 sm:pb-4">
			<CardHeader className="pb-5 pt-4 gap-1 relative max-sm:py-3 max-sm:px-4">
				<CardTitle className="text-xl sm:text-2xl">{title}</CardTitle>
				<CardDescription>{description}</CardDescription>
			</CardHeader>
			<div className="ps-0 w-[calc(100%-1.3em)] relative h-48 md:h-52">
				{points.length < 2 ? (
					<div className="flex h-full items-center justify-center text-sm text-muted-foreground">等待足够的采样点</div>
				) : (
					<ChartContainer className="absolute h-full w-full bg-card">
						<AreaChart accessibilityLayer data={points} margin={{ top: 8, right: 8, bottom: 0, left: 8 }}>
							<CartesianGrid vertical={false} />
							<YAxis
								width={72}
								domain={domain ?? [0, "auto"]}
								tickFormatter={yTickFormatter}
								tickLine={false}
								axisLine={false}
							/>
							<XAxis
								dataKey="created"
								type="number"
								scale="time"
								domain={["dataMin", "dataMax"]}
								tickMargin={8}
								minTickGap={16}
								axisLine={false}
								tickFormatter={(value) =>
									new Date(value).toLocaleTimeString("zh-CN", { hour: "2-digit", minute: "2-digit" })
								}
							/>
							<ChartTooltip
								animationEasing="ease-out"
								animationDuration={150}
								// @ts-expect-error
								itemSorter={tooltipValueDescSorter}
								content={
									<ChartTooltipContent
										labelFormatter={(_, data) => formatShortDate(data[0].payload.created)}
										contentFormatter={({ value }) => valueFormatter(Number(value) || 0)}
										showTotal={showTotal}
										totalLabel="合计"
									/>
								}
							/>
							{series.map((item) => (
								<Area
									key={item.key}
									dataKey={item.key}
									name={item.name}
									type="monotoneX"
									fill={item.color}
									fillOpacity={item.opacity}
									stroke={item.color}
									isAnimationActive={false}
									stackId={item.stackId}
								/>
							))}
						</AreaChart>
					</ChartContainer>
				)}
			</div>
		</Card>
	)
}

function ResourceTrendSection({ timeseries, loading }: { timeseries: I3DResourceTimeseries | null; loading: boolean }) {
	const [breakdown, setBreakdown] = useState<"group" | "target">("group")
	const points = useMemo(() => buildI3DResourceChartPoints(timeseries), [timeseries])
	const hasData = hasI3DResourceChartData(timeseries)
	const cpuFormatter = (value: number) => `${decimalString(value, value >= 10 ? 1 : 2)}%`
	const bytesFormatter = (value: number) => formatI3DBytesValue(value)
	const rateFormatter = (value: number) => formatI3DBytesPerSecond(value)
	const groupSeries = {
		cpu: [
			{ key: "business_cpu_percent", name: "智能检索服务集群", color: "var(--chart-1)", opacity: 0.35, stackId: "cpu" },
			{ key: "middleware_cpu_percent", name: "基础设施组件", color: "var(--chart-2)", opacity: 0.3, stackId: "cpu" },
			{ key: "monitor_cpu_percent", name: "监控组件", color: "var(--chart-5)", opacity: 0.25, stackId: "cpu" },
		],
		memory: [
			{
				key: "business_memory_bytes",
				name: "智能检索服务集群",
				color: "var(--chart-1)",
				opacity: 0.35,
				stackId: "memory",
			},
			{
				key: "middleware_memory_bytes",
				name: "基础设施组件",
				color: "var(--chart-2)",
				opacity: 0.3,
				stackId: "memory",
			},
			{ key: "monitor_memory_bytes", name: "监控组件", color: "var(--chart-5)", opacity: 0.25, stackId: "memory" },
		],
		gpu: [{ key: "gpu_memory_bytes", name: "i3d 服务 GPU 显存", color: "var(--chart-4)", opacity: 0.35 }],
	}
	const targetSeries = useMemo(() => {
		const targets = new Map<string, string>()
		for (const item of timeseries?.items ?? []) {
			for (const [targetID, target] of Object.entries(item.targets ?? {})) {
				targets.set(targetID, target.name || targetID)
			}
		}
		return Array.from(targets.entries()).map(([targetID, name], index) => {
			const key = targetChartKey(targetID)
			const color = `var(--chart-${(index % 5) + 1})`
			return {
				cpu: { key: `${key}_cpu_percent`, name, color, opacity: 0.24, stackId: "cpu-target" },
				memory: { key: `${key}_memory_bytes`, name, color, opacity: 0.24, stackId: "memory-target" },
				gpu: { key: `${key}_gpu_memory_bytes`, name, color, opacity: 0.28, stackId: "gpu-target" },
			}
		})
	}, [timeseries])
	const cpuSeries = breakdown === "target" ? targetSeries.map((item) => item.cpu) : groupSeries.cpu
	const memorySeries = breakdown === "target" ? targetSeries.map((item) => item.memory) : groupSeries.memory
	const gpuSeries = breakdown === "target" ? targetSeries.map((item) => item.gpu) : groupSeries.gpu

	return (
		<section className="grid gap-4">
			<div className="flex flex-wrap items-start justify-between gap-3">
				<div>
					<h2 className="text-xl font-semibold">资源趋势</h2>
					<p className="mt-1 text-sm text-muted-foreground">查看最近采样的资源变化。</p>
				</div>
				<div className="flex rounded-md border border-border/60 p-1">
					<Button
						type="button"
						size="sm"
						variant={breakdown === "group" ? "secondary" : "ghost"}
						onClick={() => setBreakdown("group")}
					>
						按分组
					</Button>
					<Button
						type="button"
						size="sm"
						variant={breakdown === "target" ? "secondary" : "ghost"}
						onClick={() => setBreakdown("target")}
					>
						按服务
					</Button>
				</div>
			</div>
			{loading && !hasData ? (
				<div className="flex h-32 items-center justify-center rounded-md border border-border/60 text-muted-foreground">
					<LoaderCircleIcon className="me-2 h-5 w-5 animate-spin" />
					加载资源趋势
				</div>
			) : (
				<div className="grid xl:grid-cols-2 gap-4">
					<ResourceTrendCard
						title="CPU 趋势"
						description={breakdown === "target" ? "按服务查看 CPU 占用" : "按分组查看 CPU 占用"}
						points={points}
						series={cpuSeries}
						valueFormatter={cpuFormatter}
						yTickFormatter={cpuFormatter}
						domain={pinnedAxisDomain() as [number, (dataMax: number) => number]}
						showTotal={true}
					/>
					<ResourceTrendCard
						title="内存趋势"
						description={breakdown === "target" ? "按服务查看内存占用" : "按分组查看内存占用"}
						points={points}
						series={memorySeries}
						valueFormatter={bytesFormatter}
						yTickFormatter={bytesFormatter}
						showTotal={true}
					/>
					<ResourceTrendCard
						title="GPU 显存趋势"
						description={breakdown === "target" ? "按服务查看 GPU 显存占用" : "按分组查看 GPU 显存占用"}
						points={points}
						series={gpuSeries}
						valueFormatter={bytesFormatter}
						yTickFormatter={bytesFormatter}
						showTotal={true}
					/>
					<ResourceTrendCard
						title="磁盘 IO"
						description="磁盘读写吞吐"
						points={points}
						series={[
							{ key: "disk_read_bps", name: "读", color: "var(--chart-1)", opacity: 0.3 },
							{ key: "disk_write_bps", name: "写", color: "var(--chart-3)", opacity: 0.3 },
						]}
						valueFormatter={rateFormatter}
						yTickFormatter={rateFormatter}
						showTotal={true}
					/>
					<ResourceTrendCard
						title="网络 IO"
						description="网络收发吞吐"
						points={points}
						series={[
							{ key: "network_rx_bps", name: "收", color: "var(--chart-2)", opacity: 0.3 },
							{ key: "network_tx_bps", name: "发", color: "var(--chart-5)", opacity: 0.25 },
						]}
						valueFormatter={rateFormatter}
						yTickFormatter={rateFormatter}
						showTotal={true}
					/>
				</div>
			)}
		</section>
	)
}

export default memo(function ResourceMonitoringPage() {
	const [data, setData] = useState<I3DResourceOverview | null>(null)
	const [timeseries, setTimeseries] = useState<I3DResourceTimeseries | null>(null)
	const [loading, setLoading] = useState(true)
	const [refreshing, setRefreshing] = useState(false)
	const [trendLoading, setTrendLoading] = useState(false)
	const [error, setError] = useState("")
	const [collapsedGroupIDs, setCollapsedGroupIDs] = useState<Set<string>>(() => new Set())

	const loadData = useCallback(async (quiet = false) => {
		if (quiet) {
			setRefreshing(true)
		} else {
			setLoading(true)
		}
		setError("")
		try {
			const overview = await fetchI3DResourceOverview()
			setData({
				...overview,
				groups: overview.groups ?? [],
				items: overview.items ?? [],
			})
			setTrendLoading(true)
			fetchI3DResourceTimeseries()
				.then((value) => setTimeseries({ ...value, items: value.items ?? [] }))
				.catch((err) => console.error("load i3d resource timeseries failed", err))
				.finally(() => setTrendLoading(false))
		} catch (err) {
			console.error("load i3d resource overview failed", err)
			setError(err instanceof Error ? err.message : "资源监控数据加载失败")
		} finally {
			setLoading(false)
			setRefreshing(false)
		}
	}, [])

	const refreshData = useCallback(() => {
		loadData(true).catch((err) => console.error("refresh i3d resource overview failed", err))
	}, [loadData])

	useEffect(() => {
		document.title = `资源监控 - ${BRAND_NAME}`
		loadData().catch((err) => console.error("load i3d resource overview failed", err))
		const timer = window.setInterval(refreshData, refreshIntervalMs)
		return () => window.clearInterval(timer)
	}, [loadData, refreshData])

	const summary = data?.summary
	const summaryMemoryDetail = summary
		? [
				summary.memory_rss_bytes ? `常驻 ${formatI3DBytesValue(summary.memory_rss_bytes)}` : "",
				summary.memory_cache_bytes ? `缓存 ${formatI3DBytesValue(summary.memory_cache_bytes)}` : "",
			]
				.filter(Boolean)
				.join(" / ")
		: ""
	const treeRows = useMemo(() => buildI3DResourceTreeRows(data?.items ?? []), [data?.items])
	const visibleTreeRows = useMemo(
		() => visibleI3DResourceTreeRows(treeRows, collapsedGroupIDs),
		[treeRows, collapsedGroupIDs]
	)
	const toggleGroup = useCallback((id: string) => {
		setCollapsedGroupIDs((current) => {
			const next = new Set(current)
			if (next.has(id)) {
				next.delete(id)
			} else {
				next.add(id)
			}
			return next
		})
	}, [])

	return (
		<>
			<div className="grid gap-4">
				<ActiveAlerts />
				<Card className="border-border/60">
					<CardHeader className="gap-3">
						<div className="flex flex-wrap items-start justify-between gap-3">
							<div>
								<div className="flex items-center gap-2">
									<ActivityIcon className="h-5 w-5 text-primary" />
									<CardTitle>资源监控</CardTitle>
								</div>
								<CardDescription className="mt-2">关注 i3d 服务、中间件和监控组件的资源占用。</CardDescription>
							</div>
							<div className="flex items-center gap-2">
								<Badge variant="outline">{data ? environmentLabel(data.environment) : "环境识别中"}</Badge>
								<Badge variant="secondary">15 秒自动刷新</Badge>
								<Button variant="outline" size="sm" onClick={refreshData} disabled={loading || refreshing}>
									{refreshing ? (
										<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
									) : (
										<RefreshCwIcon className="me-2 h-4 w-4" />
									)}
									刷新
								</Button>
							</div>
						</div>
						{error && (
							<div className="rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-700 dark:text-red-300">
								{error}
							</div>
						)}
					</CardHeader>
					<CardContent className="space-y-4">
						{loading ? (
							<div className="flex h-48 items-center justify-center text-muted-foreground">
								<LoaderCircleIcon className="me-2 h-5 w-5 animate-spin" />
								加载资源监控数据
							</div>
						) : (
							<>
								<div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-6">
									<ResourceStatCard
										title="i3d CPU 总占用"
										value={`${decimalString(summary?.cpu_percent ?? 0, 1)}%`}
										description={`${decimalString(summary?.cpu_cores_used ?? 0, 2)} 核`}
										icon={CpuIcon}
									/>
									<ResourceStatCard
										title="i3d 内存总占用"
										value={formatI3DBytesValue(summary?.memory_bytes ?? 0)}
										description={summaryMemoryDetail || undefined}
										icon={MemoryStickIcon}
									/>
									<ResourceStatCard
										title="i3d 磁盘 IO"
										value={formatI3DBytesPerSecond((summary?.disk_read_bps ?? 0) + (summary?.disk_write_bps ?? 0))}
										description={`读 ${formatI3DBytesPerSecond(summary?.disk_read_bps ?? 0)} / 写 ${formatI3DBytesPerSecond(summary?.disk_write_bps ?? 0)}`}
										icon={HardDriveIcon}
									/>
									<ResourceStatCard
										title="i3d 网络 IO"
										value={formatI3DBytesPerSecond((summary?.network_rx_bps ?? 0) + (summary?.network_tx_bps ?? 0))}
										description={`收 ${formatI3DBytesPerSecond(summary?.network_rx_bps ?? 0)} / 发 ${formatI3DBytesPerSecond(summary?.network_tx_bps ?? 0)}`}
										icon={NetworkIcon}
									/>
									<ResourceStatCard
										title="i3d GPU 显存"
										value={formatI3DBytesValue(summary?.gpu_memory_bytes ?? 0)}
										description="显存占用"
										icon={ServerIcon}
									/>
									<ResourceStatCard
										title="异常目标"
										value={`${summary?.abnormal_count ?? 0}`}
										description="需要关注的目标"
										icon={AlertTriangleIcon}
										tone={(summary?.abnormal_count ?? 0) > 0 ? "danger" : "default"}
									/>
								</div>

								<ResourceTreeTable
									rows={visibleTreeRows}
									collapsedGroupIDs={collapsedGroupIDs}
									onToggleGroup={toggleGroup}
								/>
							</>
						)}
					</CardContent>
				</Card>
				{!loading && <ResourceTrendSection timeseries={timeseries} loading={trendLoading} />}
			</div>
			<FooterRepoLink />
		</>
	)
})

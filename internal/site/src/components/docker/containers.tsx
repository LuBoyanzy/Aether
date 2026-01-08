/**
 * Docker 容器列表与操作面板。
 * 提供容器筛选、状态展示与基础运维操作。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { toast } from "@/components/ui/use-toast"
import { listDockerContainers } from "@/lib/docker"
import { isReadOnlyUser, pb } from "@/lib/api"
import type { ContainerRecord, DockerContainer } from "@/types"
import { formatShortId, formatUnixSeconds, formatTagList } from "@/components/docker/utils"
import DockerEmptyState from "@/components/docker/empty-state"
import { AlertCircleIcon, LoaderCircleIcon, MoreHorizontalIcon, RefreshCwIcon } from "lucide-react"
import { MeterState } from "@/lib/enums"
import { cn, decimalString, formatBytes, formatSecondsToHuman, getMeterState } from "@/lib/utils"

const statusVariantMap: Record<string, "success" | "warning" | "danger" | "secondary"> = {
	running: "success",
	paused: "warning",
	restarting: "warning",
	dead: "danger",
	exited: "secondary",
	created: "secondary",
}
const usageWindowMs = 70_000

function formatPorts(ports?: DockerContainer["ports"]) {
	if (!ports || ports.length === 0) return "-"
	return ports
		.map((port) => {
			const host = port.publicPort ? `${port.ip || "0.0.0.0"}:${port.publicPort}` : "-"
			return `${host} -> ${port.privatePort}/${port.type}`
		})
		.join(", ")
}

export default memo(function DockerContainersPanel({ systemId }: { systemId?: string }) {
	const [loading, setLoading] = useState(true)
	const [showAll, setShowAll] = useState(false)
	const [data, setData] = useState<DockerContainer[]>([])
	const [filter, setFilter] = useState("")
	const [logOpen, setLogOpen] = useState(false)
	const [logContent, setLogContent] = useState("")
	const [logLoading, setLogLoading] = useState(false)
	const [inspectOpen, setInspectOpen] = useState(false)
	const [inspectContent, setInspectContent] = useState("")
	const [inspectLoading, setInspectLoading] = useState(false)
	const [activeContainer, setActiveContainer] = useState<DockerContainer | null>(null)
	const [usageMap, setUsageMap] = useState<Map<string, ContainerRecord>>(new Map())

	const loadContainers = useCallback(async () => {
		if (!systemId) return
		setLoading(true)
		try {
			const [items, usageList] = await Promise.all([
				listDockerContainers(systemId, showAll),
				pb.collection<ContainerRecord>("containers").getList(0, 2000, {
					fields: "id,cpu,memory,net,uptime,updated",
					filter: pb.filter("system={:system}", { system: systemId }),
				}),
			])
			const nextUsage = new Map<string, ContainerRecord>()
			const now = Date.now()
			for (const item of usageList.items) {
				const updated = item.updated < 1e11 ? item.updated * 1000 : item.updated
				if (now - updated <= usageWindowMs) {
					nextUsage.set(item.id, item)
				}
			}
			setUsageMap(nextUsage)
			setData(items)
		} catch (err) {
			console.error("load docker containers failed", err)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to load containers`,
			})
			throw err
		} finally {
			setLoading(false)
		}
	}, [systemId, showAll])

	useEffect(() => {
		if (systemId) {
			void loadContainers()
		}
	}, [systemId, showAll, loadContainers])

	const filtered = useMemo(() => {
		const term = filter.trim().toLowerCase()
		if (!term) return data
		return data.filter((item) => {
			const ports = formatPorts(item.ports)
			const networks = formatTagList(item.networks)
			return [item.name, item.image, item.state, item.status, ports, networks, item.command, item.createdBy]
				.filter(Boolean)
				.join(" ")
				.toLowerCase()
				.includes(term)
		})
	}, [data, filter])

	const handleOperate = useCallback(
		async (container: DockerContainer, operation: string) => {
			if (!systemId) return
			if (isReadOnlyUser()) {
				toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
				return
			}
			try {
				await pb.send<{ status: string }>("/api/aether/containers/operate", {
					method: "POST",
					body: {
						system: systemId,
						container: container.id,
						operation,
					},
				})
				toast({ title: t`Operation success`, description: `${operation} ${container.name}` })
				await loadContainers()
			} catch (err) {
				console.error("container operation failed", err)
				toast({
					variant: "destructive",
					title: t`Error`,
					description: t`Failed to operate container`,
				})
				throw err
			}
		},
		[systemId, loadContainers]
	)

	const openLogs = useCallback(
		async (container: DockerContainer) => {
			if (!systemId) return
			setActiveContainer(container)
			setLogOpen(true)
			setLogLoading(true)
			setLogContent("")
			try {
				const res = await pb.send<{ logs: string }>("/api/aether/containers/logs", {
					query: { system: systemId, container: container.id },
				})
				setLogContent(res.logs || "")
			} catch (err) {
				console.error("load container logs failed", err)
				toast({
					variant: "destructive",
					title: t`Error`,
					description: t`Failed to load container logs`,
				})
				throw err
			} finally {
				setLogLoading(false)
			}
		},
		[systemId]
	)

	const openInspect = useCallback(
		async (container: DockerContainer) => {
			if (!systemId) return
			setActiveContainer(container)
			setInspectOpen(true)
			setInspectLoading(true)
			setInspectContent("")
			try {
				const res = await pb.send<{ info: string }>("/api/aether/containers/info", {
					query: { system: systemId, container: container.id },
				})
				let payload = res.info || ""
				try {
					payload = JSON.stringify(JSON.parse(payload), null, 2)
				} catch (parseErr) {
					console.error("parse container inspect failed", parseErr)
				}
				setInspectContent(payload)
			} catch (err) {
				console.error("load container inspect failed", err)
				toast({
					variant: "destructive",
					title: t`Error`,
					description: t`Failed to load container details`,
				})
				throw err
			} finally {
				setInspectLoading(false)
			}
		},
		[systemId]
	)

	if (!systemId) {
		return <DockerEmptyState />
	}

	return (
		<div className="space-y-4">
			<div className="flex flex-wrap items-end justify-between gap-3">
				<div>
					<h2 className="text-lg font-semibold">
						<Trans>Containers</Trans>
					</h2>
					<p className="text-sm text-muted-foreground">
						<Trans>Manage running and stopped containers.</Trans>
					</p>
				</div>
				<div className="flex flex-wrap items-center gap-3">
					<div className="flex items-center gap-2">
						<Switch id="docker-containers-all" checked={showAll} onCheckedChange={setShowAll} />
						<Label htmlFor="docker-containers-all">
							<Trans>Show stopped</Trans>
						</Label>
					</div>
					<Input
						className="w-56"
						placeholder={t`Filter containers...`}
						value={filter}
						onChange={(event) => setFilter(event.target.value)}
					/>
					<Button variant="outline" size="sm" onClick={() => void loadContainers()} disabled={loading}>
						{loading ? (
							<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
						) : (
							<RefreshCwIcon className="me-2 h-4 w-4" />
						)}
						<Trans>Refresh</Trans>
					</Button>
				</div>
			</div>
			<div className="h-min max-h-[calc(100dvh-24rem)] max-w-full relative overflow-auto border rounded-md bg-card">
				<table className="w-full caption-bottom text-sm">
					<TableHeader className="sticky top-0 z-10 bg-card">
						<TableRow>
							<TableHead>
								<Trans>Name</Trans>
							</TableHead>
							<TableHead>
								<Trans>Image</Trans>
							</TableHead>
							<TableHead>
								<Trans>Status</Trans>
							</TableHead>
							<TableHead>
								<Trans>Usage</Trans>
							</TableHead>
							<TableHead>
								<Trans>Ports</Trans>
							</TableHead>
							<TableHead>
								<Trans>Networks</Trans>
							</TableHead>
							<TableHead>
								<Trans>Uptime</Trans>
							</TableHead>
							<TableHead>
								<Trans>Created</Trans>
							</TableHead>
							<TableHead className="text-right">
								<Trans>Actions</Trans>
							</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody
						className={
							loading
								? "opacity-50 pointer-events-none transition-opacity duration-300"
								: "opacity-100 transition-opacity duration-500"
						}
					>
						{loading && data.length === 0 ? (
							<TableRow>
								<TableCell colSpan={9} className="h-24 text-center">
									<div className="flex items-center justify-center gap-2 text-muted-foreground">
										<LoaderCircleIcon className="h-4 w-4 animate-spin" />
										<Trans>Loading...</Trans>
									</div>
								</TableCell>
							</TableRow>
						) : filtered.length === 0 ? (
							<TableRow>
								<TableCell colSpan={9} className="text-center text-sm text-muted-foreground">
									<Trans>No containers found.</Trans>
								</TableCell>
							</TableRow>
						) : (
							filtered.map((item) => (
								<TableRow key={item.id} className="group">
									<TableCell className="max-w-[100px] py-3">
										<div className="font-medium text-foreground truncate" title={item.name}>
											{item.name}
										</div>
										<div className="flex items-center gap-1.5 text-xs text-muted-foreground">
											<span className="truncate font-mono opacity-80">{formatShortId(item.id)}</span>
										</div>
										{item.createdBy && (
											<div className="text-xs text-muted-foreground truncate" title={item.createdBy}>
												<Trans>Compose</Trans>: {item.createdBy}
											</div>
										)}
									</TableCell>
									<TableCell className="max-w-[180px] py-3">
										<div className="truncate text-foreground/90" title={item.image}>
											{item.image}
										</div>
									</TableCell>
									<TableCell className="p-2">
										<div className="flex flex-col gap-1.5">
											<Badge
												variant={statusVariantMap[item.state] ?? "secondary"}
												className="w-fit px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider shadow-none"
											>
												{item.state}
											</Badge>
											<span className="text-xs font-medium text-muted-foreground capitalize">{item.status}</span>
										</div>
									</TableCell>
									<TableCell className="py-3">
										{(() => {
											const shortId = item.id ? item.id.slice(0, 12) : ""
											const usage = shortId ? usageMap.get(shortId) : undefined
											const cpu = usage?.cpu ?? 0
											const memory = usage?.memory ?? 0
											const net = usage?.net ?? 0
											const threshold = getMeterState(cpu)
											const meterClass = cn(
												"h-full rounded-full",
												(threshold === MeterState.Good && "bg-green-500") ||
													(threshold === MeterState.Warn && "bg-yellow-500") ||
													"bg-red-500"
											)
											const memFormatted = usage ? formatBytes(memory, false, undefined, true) : null
											const netFormatted = usage ? formatBytes(net, true, undefined, true) : null
											return (
												<div className="flex flex-col gap-1.5 w-full max-w-[200px] text-xs">
													<div className="flex items-center gap-2">
														<span className="font-semibold text-muted-foreground/70 w-8">CPU</span>
														<span className="tabular-nums font-medium w-12 text-foreground/90">
															{usage ? `${decimalString(cpu, cpu >= 10 ? 1 : 2)}%` : "-"}
														</span>
														<div className="h-1.5 flex-1 bg-muted/30 rounded-full overflow-hidden">
															<div className={meterClass} style={{ width: `${usage ? cpu : 0}%` }} />
														</div>
													</div>
													<div className="grid grid-cols-2 gap-4">
														<div className="flex items-center gap-2 overflow-hidden">
															<span className="font-semibold text-muted-foreground/70 w-8">MEM</span>
															<span className="tabular-nums truncate text-foreground/90">
																{memFormatted
																	? decimalString(memFormatted.value, memFormatted.value >= 10 ? 1 : 2)
																	: "-"}
																{memFormatted && (
																	<span className="text-muted-foreground/60 text-[10px] ms-0.5">
																		{memFormatted.unit}
																	</span>
																)}
															</span>
														</div>
														<div className="flex items-center gap-2 overflow-hidden">
															<span className="font-semibold text-muted-foreground/70 w-8">NET</span>
															<span className="tabular-nums truncate text-foreground/90">
																{netFormatted
																	? decimalString(netFormatted.value, netFormatted.value >= 10 ? 1 : 2)
																	: "-"}
																{netFormatted && (
																	<span className="text-muted-foreground/60 text-[10px] ms-0.5">
																		{netFormatted.unit}
																	</span>
																)}
															</span>
														</div>
													</div>
												</div>
											)
										})()}
									</TableCell>
									<TableCell className="max-w-[200px] text-xs text-muted-foreground py-3">
										<div className="line-clamp-2" title={formatPorts(item.ports)}>
											{formatPorts(item.ports)}
										</div>
									</TableCell>
									<TableCell className="max-w-[160px] text-xs text-muted-foreground py-3">
										<div className="truncate" title={formatTagList(item.networks)}>
											{formatTagList(item.networks)}
										</div>
									</TableCell>
									<TableCell className="text-xs text-muted-foreground whitespace-nowrap py-3">
										{(() => {
											const shortId = item.id ? item.id.slice(0, 12) : ""
											const usage = shortId ? usageMap.get(shortId) : undefined
											const formatted = usage ? formatSecondsToHuman(usage.uptime ?? 0) : ""
											return formatted || "-"
										})()}
									</TableCell>
									<TableCell className="text-xs text-muted-foreground whitespace-nowrap py-3">
										{formatUnixSeconds(item.created)}
									</TableCell>
									<TableCell className="text-right py-3">
										<DropdownMenu>
											<DropdownMenuTrigger asChild>
												<Button size="icon" variant="ghost">
													<MoreHorizontalIcon className="h-4 w-4" />
												</Button>
											</DropdownMenuTrigger>
											<DropdownMenuContent align="end">
												<DropdownMenuItem onSelect={() => void handleOperate(item, "start")}>
													<Trans>Start</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => void handleOperate(item, "stop")}>
													<Trans>Stop</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => void handleOperate(item, "restart")}>
													<Trans>Restart</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => void handleOperate(item, "kill")}>
													<Trans>Kill</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => void handleOperate(item, "pause")}>
													<Trans>Pause</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => void handleOperate(item, "unpause")}>
													<Trans>Unpause</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => void openLogs(item)}>
													<Trans>Logs</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => void openInspect(item)}>
													<Trans>Inspect</Trans>
												</DropdownMenuItem>
											</DropdownMenuContent>
										</DropdownMenu>
									</TableCell>
								</TableRow>
							))
						)}
					</TableBody>
				</table>
			</div>
			<Dialog open={logOpen} onOpenChange={setLogOpen}>
				<DialogContent className="max-w-3xl">
					<DialogHeader>
						<DialogTitle>
							<Trans>Container Logs</Trans>
						</DialogTitle>
						<DialogDescription>{activeContainer?.name || "-"}</DialogDescription>
					</DialogHeader>
					<div className="max-h-[60vh] overflow-auto rounded-md border bg-muted/30 p-3 text-xs font-mono">
						{logLoading ? (
							<div className="flex items-center gap-2 text-muted-foreground">
								<LoaderCircleIcon className="h-4 w-4 animate-spin" />
								<Trans>Loading</Trans>
							</div>
						) : logContent ? (
							<pre className="whitespace-pre-wrap break-words">{logContent}</pre>
						) : (
							<div className="flex items-center gap-2 text-muted-foreground">
								<AlertCircleIcon className="h-4 w-4" />
								<Trans>No logs available.</Trans>
							</div>
						)}
					</div>
				</DialogContent>
			</Dialog>
			<Dialog open={inspectOpen} onOpenChange={setInspectOpen}>
				<DialogContent className="max-w-3xl">
					<DialogHeader>
						<DialogTitle>
							<Trans>Container Details</Trans>
						</DialogTitle>
						<DialogDescription>{activeContainer?.name || "-"}</DialogDescription>
					</DialogHeader>
					<div className="max-h-[60vh] overflow-auto rounded-md border bg-muted/30 p-3 text-xs font-mono">
						{inspectLoading ? (
							<div className="flex items-center gap-2 text-muted-foreground">
								<LoaderCircleIcon className="h-4 w-4 animate-spin" />
								<Trans>Loading</Trans>
							</div>
						) : inspectContent ? (
							<pre className="whitespace-pre-wrap break-words">{inspectContent}</pre>
						) : (
							<div className="flex items-center gap-2 text-muted-foreground">
								<AlertCircleIcon className="h-4 w-4" />
								<Trans>No details available.</Trans>
							</div>
						)}
					</div>
				</DialogContent>
			</Dialog>
		</div>
	)
})

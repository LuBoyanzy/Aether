/**
 * Docker 容器列表与操作面板。
 * 提供容器筛选、状态展示与基础运维操作。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { useStore } from "@nanostores/react"
import { timeTicks } from "d3-time"
import { Fragment, memo, useCallback, useEffect, useMemo, useState, type ChangeEvent } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { toast } from "@/components/ui/use-toast"
import ChartTimeSelect from "@/components/charts/chart-time-select"
import ContainerChart from "@/components/charts/container-chart"
import { useContainerChartConfigs } from "@/components/charts/hooks"
import { listDockerContainers, listDockerImages } from "@/lib/docker"
import { listDockerFocusServices } from "@/lib/docker-focus"
import { getPbTimestamp, isReadOnlyUser, pb } from "@/lib/api"
import { formatContainerOperationError } from "@/lib/errors"
import type {
	ChartData,
	ChartTimes,
	ContainerRecord,
	ContainerStatsRecord,
	DockerContainer,
	DockerFocusServiceRecord,
	DockerImage,
	SystemRecord,
} from "@/types"
import { formatShortId, formatUnixSeconds, formatTagList } from "@/components/docker/utils"
import DockerEmptyState from "@/components/docker/empty-state"
import FocusAlertSettingsSheet from "@/components/docker/focus-alert-settings-sheet"
import DockerFocusRulesDialog from "@/components/docker/focus-rules-dialog"
import {
	AlertCircleIcon,
	ChevronRightIcon,
	LoaderCircleIcon,
	MoreHorizontalIcon,
	RefreshCwIcon,
	XIcon,
} from "lucide-react"
import { ChartType, MeterState, SystemStatus } from "@/lib/enums"
import { $allSystemsById, $chartTime, $containerFilter, $direction } from "@/lib/stores"
import { ChartCard } from "@/components/routes/system"
import {
	chartTimeData,
	cn,
	compareSemVer,
	decimalString,
	formatBytes,
	formatSecondsToHuman,
	getMeterState,
	parseSemVer,
} from "@/lib/utils"

const statusVariantMap: Record<string, "success" | "warning" | "danger" | "secondary"> = {
	running: "success",
	paused: "warning",
	restarting: "warning",
	dead: "danger",
	exited: "secondary",
	created: "secondary",
}
const usageWindowMs = 70_000
const composeProjectLabel = "com.docker.compose.project"
const composeServiceLabel = "com.docker.compose.service"
type FocusImageSummary = {
	name: string
	size?: number
}

type FocusUsageSummary = {
	cpu: number
	memory: number
	net: number
}
type ContainerStatsRecordLite = Pick<ContainerStatsRecord, "created" | "stats">

function getTimeData(chartTime: ChartTimes) {
	const now = new Date(Date.now())
	const startTime = chartTimeData[chartTime].getOffset(now)
	const ticks = timeTicks(startTime, now, chartTimeData[chartTime].ticks ?? 12).map((date) => date.getTime())
	return {
		ticks,
		domain: [startTime.getTime(), now.getTime()],
	}
}

function addEmptyValues<T extends { created: string | number | null }>(
	prevRecords: T[],
	newRecords: T[],
	expectedInterval: number
): T[] {
	const modifiedRecords: T[] = []
	let prevTime = (prevRecords.at(-1)?.created ?? 0) as number
	for (let i = 0; i < newRecords.length; i++) {
		const record = newRecords[i]
		if (record.created !== null) {
			record.created = new Date(record.created).getTime()
		}
		if (prevTime && record.created !== null) {
			const interval = (record.created as number) - prevTime
			if (interval > expectedInterval / 2 + expectedInterval) {
				modifiedRecords.push({ created: null, ...("stats" in record ? { stats: null } : {}) } as T)
			}
		}
		if (record.created !== null) {
			prevTime = record.created as number
		}
		modifiedRecords.push(record)
	}
	return modifiedRecords
}

function buildFocusContainerData(
	containers: ContainerStatsRecordLite[],
	focusNames: Set<string>
): ChartData["containerData"] {
	if (focusNames.size === 0) return []
	const containerData: ChartData["containerData"] = []
	for (let { created, stats } of containers) {
		if (!created) {
			containerData.push({ created: null } as ChartData["containerData"][0])
			continue
		}
		created = new Date(created).getTime()
		const containerStats: ChartData["containerData"][0] = { created } as ChartData["containerData"][0]
		if (Array.isArray(stats)) {
			for (const stat of stats) {
				if (!focusNames.has(stat.n)) continue
				containerStats[stat.n] = stat
			}
		}
		containerData.push(containerStats)
	}
	return containerData
}

function dockerOrPodman(str: string, isPodman: boolean): string {
	if (isPodman) {
		return str.replace("docker", "podman").replace("Docker", "Podman")
	}
	return str
}

function formatImageSizeLabel(size?: number) {
	if (size === undefined || size === null) return "-"
	const { value, unit } = formatBytes(size)
	const rounded = value >= 10 ? Math.round(value) : Math.round(value * 10) / 10
	return `${rounded}${unit}`
}

function buildFocusImageSummaries(containers: DockerContainer[], sizeMap: Map<string, number>): FocusImageSummary[] {
	const summaries: FocusImageSummary[] = []
	const seen = new Set<string>()
	for (const container of containers) {
		const imageName = container.image?.trim()
		if (!imageName || seen.has(imageName)) continue
		seen.add(imageName)
		const imageId = container.imageId?.trim()
		const size = imageId ? sizeMap.get(imageId) : undefined
		summaries.push({
			name: imageName,
			size: size ?? sizeMap.get(imageName),
		})
	}
	return summaries
}

function buildFocusUsageSummary(
	containers: DockerContainer[],
	usageMap: Map<string, ContainerRecord>
): FocusUsageSummary {
	let cpu = 0
	let memory = 0
	let net = 0
	for (const container of containers) {
		const shortId = container.id ? container.id.slice(0, 12) : ""
		const usage = shortId ? usageMap.get(shortId) : undefined
		if (!usage) continue
		cpu += usage.cpu ?? 0
		memory += usage.memory ?? 0
		net += usage.net ?? 0
	}
	return { cpu, memory, net }
}

function matchesFocusRule(container: DockerContainer, rule: DockerFocusServiceRecord): boolean {
	switch (rule.match_type) {
		case "container_name":
			return container.name === rule.value
		case "image":
			return container.image === rule.value
		case "compose_project": {
			const project = container.labels?.[composeProjectLabel] || container.createdBy || ""
			return project === rule.value
		}
		case "compose_service": {
			if (!rule.value2) return false
			const project = container.labels?.[composeProjectLabel] || container.createdBy || ""
			const service = container.labels?.[composeServiceLabel] || ""
			return project === rule.value && service === rule.value2
		}
		case "label": {
			if (!rule.value2) return false
			return container.labels?.[rule.value] === rule.value2
		}
		default:
			return false
	}
}

function formatPorts(ports?: DockerContainer["ports"]) {
	if (!ports || ports.length === 0) return "-"
	return ports
		.map((port) => {
			const host = port.publicPort ? `${port.ip || "0.0.0.0"}:${port.publicPort}` : "-"
			return `${host} -> ${port.privatePort}/${port.type}`
		})
		.join(", ")
}

function formatFocusRuleLabel(rule: DockerFocusServiceRecord) {
	switch (rule.match_type) {
		case "compose_service":
			return rule.value2 ? `${rule.value} / ${rule.value2}` : rule.value
		case "label":
			return rule.value2 ? `${rule.value}=${rule.value2}` : rule.value
		default:
			return rule.value
	}
}

function isContainerActive(container: DockerContainer) {
	return !["exited", "created", "dead"].includes(container.state)
}

function matchesSearchTerm(container: DockerContainer, term: string) {
	if (!term) return true
	const ports = formatPorts(container.ports)
	const networks = formatTagList(container.networks)
	return [
		container.name,
		container.image,
		container.state,
		container.status,
		ports,
		networks,
		container.command,
		container.createdBy,
	]
		.filter(Boolean)
		.join(" ")
		.toLowerCase()
		.includes(term)
}

type FocusGroupStatus = "running" | "partial" | "exited"

const focusGroupStatusVariantMap: Record<FocusGroupStatus, "success" | "warning" | "secondary"> = {
	running: "success",
	partial: "warning",
	exited: "secondary",
}

function getFocusGroupStatus(runningCount: number, totalCount: number): FocusGroupStatus {
	if (totalCount === 0 || runningCount === 0) return "exited"
	if (runningCount === totalCount) return "running"
	return "partial"
}

const FocusUsageFilterBar = memo(function FocusUsageFilterBar() {
	const storeValue = useStore($containerFilter)
	const [inputValue, setInputValue] = useState(storeValue)

	useEffect(() => {
		setInputValue(storeValue)
	}, [storeValue])

	useEffect(() => {
		if (inputValue === storeValue) {
			return
		}
		const handle = window.setTimeout(() => $containerFilter.set(inputValue), 80)
		return () => clearTimeout(handle)
	}, [inputValue, storeValue])

	const handleChange = useCallback((e: ChangeEvent<HTMLInputElement>) => {
		setInputValue(e.target.value)
	}, [])

	const handleClear = useCallback(() => {
		setInputValue("")
		$containerFilter.set("")
	}, [])

	return (
		<div className="relative w-full sm:w-44">
			<Input placeholder={t`Filter...`} className="ps-4 pe-8 w-full" onChange={handleChange} value={inputValue} />
			{inputValue && (
				<Button
					type="button"
					variant="ghost"
					size="icon"
					aria-label="Clear"
					className="absolute right-1 top-1/2 -translate-y-1/2 h-7 w-7 text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-gray-100"
					onClick={handleClear}
				>
					<XIcon className="h-4 w-4" />
				</Button>
			)}
		</div>
	)
})

export default memo(function DockerContainersPanel({ systemId }: { systemId?: string }) {
	const [loading, setLoading] = useState(true)
	const [showAll, setShowAll] = useState(true)
	const [focusOnly, setFocusOnly] = useState(true)
	const [data, setData] = useState<DockerContainer[]>([])
	const [images, setImages] = useState<DockerImage[]>([])
	const [focusRules, setFocusRules] = useState<DockerFocusServiceRecord[]>([])
	const [focusLoading, setFocusLoading] = useState(false)
	const [focusDialogOpen, setFocusDialogOpen] = useState(false)
	const [focusUsageOpen, setFocusUsageOpen] = useState(false)
	const [focusUsageLoading, setFocusUsageLoading] = useState(false)
	const [focusContainerData, setFocusContainerData] = useState<ChartData["containerData"]>([])
	const [alertSettingsOpen, setAlertSettingsOpen] = useState(false)
	const [filter, setFilter] = useState("")
	const [logOpen, setLogOpen] = useState(false)
	const [logContent, setLogContent] = useState("")
	const [logLoading, setLogLoading] = useState(false)
	const [inspectOpen, setInspectOpen] = useState(false)
	const [inspectContent, setInspectContent] = useState("")
	const [inspectLoading, setInspectLoading] = useState(false)
	const [activeContainer, setActiveContainer] = useState<DockerContainer | null>(null)
	const [usageMap, setUsageMap] = useState<Map<string, ContainerRecord>>(new Map())
	const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({})
	const chartTime = useStore($chartTime)
	const direction = useStore($direction)
	const systemsById = useStore($allSystemsById)
	const system = systemId ? (systemsById[systemId] as SystemRecord | undefined) : undefined
	const agentVersion = useMemo(() => parseSemVer(system?.info?.v), [system?.info?.v])
	const isPodman = system?.info?.p ?? false
	const canUse1m = compareSemVer(agentVersion, parseSemVer("0.13.0")) >= 0
	const searchTerm = useMemo(() => filter.trim().toLowerCase(), [filter])
	const focusContainerNames = useMemo(() => {
		if (focusRules.length === 0 || data.length === 0) return new Set<string>()
		const names = new Set<string>()
		for (const container of data) {
			for (const rule of focusRules) {
				if (matchesFocusRule(container, rule)) {
					names.add(container.name)
					break
				}
			}
		}
		return names
	}, [data, focusRules])
	const focusNamesKey = useMemo(() => Array.from(focusContainerNames).sort().join("|"), [focusContainerNames])

	const loadContainers = useCallback(async () => {
		if (!systemId) return
		setLoading(true)
		try {
			const requestAll = focusOnly ? true : showAll
			const [items, usageList] = await Promise.all([
				listDockerContainers(systemId, requestAll),
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
	}, [systemId, showAll, focusOnly])

	const loadFocusRules = useCallback(async () => {
		if (!systemId) return
		setFocusLoading(true)
		try {
			const items = await listDockerFocusServices(systemId)
			setFocusRules(items)
		} catch (err) {
			console.error("load docker focus rules failed", err)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to load focus rules`,
			})
			throw err
		} finally {
			setFocusLoading(false)
		}
	}, [systemId])

	const loadImages = useCallback(async () => {
		if (!systemId) return
		try {
			// 后端接口：internal/hub/docker.go listDockerImages -> internal/entities/docker/docker.go Image
			const items = await listDockerImages(systemId, true)
			setImages(items)
		} catch (err) {
			console.error("load docker images failed", err)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to load images`,
			})
			throw err
		}
	}, [systemId])

	const loadFocusUsage = useCallback(async () => {
		if (!systemId) return
		if (chartTime === "1m") return
		if (focusContainerNames.size === 0) {
			setFocusUsageLoading(false)
			setFocusContainerData([])
			return
		}
		setFocusUsageLoading(true)
		try {
			const records = await pb.collection<ContainerStatsRecord>("container_stats").getFullList({
				filter: pb.filter("system={:id} && created > {:created} && type={:type}", {
					id: systemId,
					created: getPbTimestamp(chartTime),
					type: chartTimeData[chartTime].type,
				}),
				fields: "created,stats",
				sort: "created",
			})
			const normalized = addEmptyValues([], records, chartTimeData[chartTime].expectedInterval)
			setFocusContainerData(buildFocusContainerData(normalized, focusContainerNames))
		} catch (err) {
			console.error("load focus usage failed", err, {
				systemId,
				chartTime,
				focusNames: Array.from(focusContainerNames),
			})
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to load containers`,
			})
			throw err
		} finally {
			setFocusUsageLoading(false)
		}
	}, [systemId, chartTime, focusContainerNames])

	useEffect(() => {
		if (systemId) {
			void loadContainers()
		}
	}, [systemId, showAll, loadContainers])

	useEffect(() => {
		if (systemId) {
			void loadFocusRules()
		}
	}, [systemId, loadFocusRules])

	useEffect(() => {
		if (systemId && focusOnly) {
			void loadImages()
		}
	}, [systemId, focusOnly, loadImages])

	useEffect(() => {
		setExpandedGroups({})
	}, [systemId, focusOnly])

	useEffect(() => {
		if (!canUse1m && chartTime === "1m") {
			$chartTime.set("1h")
		}
	}, [canUse1m, chartTime])

	useEffect(() => {
		if (!focusUsageOpen) {
			setFocusContainerData([])
			setFocusUsageLoading(false)
			return
		}
		if (chartTime !== "1m") {
			void loadFocusUsage()
		}
	}, [focusUsageOpen, chartTime, loadFocusUsage])

	useEffect(() => {
		let cancelled = false
		let unsub: (() => void) | null = null
		if (!focusUsageOpen || !systemId || chartTime !== "1m") {
			return () => {}
		}
		if (!canUse1m || system?.status !== SystemStatus.Up) {
			$chartTime.set("1h")
			return () => {}
		}
		if (focusContainerNames.size === 0) {
			setFocusContainerData([])
			return () => {}
		}
		setFocusContainerData([])
		setFocusUsageLoading(false)
		pb.realtime
			.subscribe(
				`rt_metrics`,
				(data: { container: ContainerStatsRecord["stats"] }) => {
					if (!data.container?.length) return
					const nextData = buildFocusContainerData(
						[{ created: Date.now(), stats: data.container }],
						focusContainerNames
					)
					if (nextData.length === 0) return
					setFocusContainerData((prev) => addEmptyValues(prev, prev.slice(-59).concat(nextData), 1000))
				},
				{ query: { system: systemId } }
			)
			.then((us) => {
				if (cancelled) {
					us?.()
					return
				}
				unsub = us
			})
		return () => {
			cancelled = true
			unsub?.()
		}
	}, [focusUsageOpen, systemId, chartTime, canUse1m, system?.status, focusNamesKey, focusContainerNames])

	const handleRefresh = useCallback(async () => {
		if (focusOnly) {
			await Promise.all([loadContainers(), loadFocusRules(), loadImages()])
			return
		}
		await Promise.all([loadContainers(), loadFocusRules()])
	}, [loadContainers, loadFocusRules, loadImages, focusOnly])

	const toggleGroup = useCallback((groupId: string) => {
		setExpandedGroups((prev) => ({ ...prev, [groupId]: !prev[groupId] }))
	}, [])

	const filteredContainers = useMemo(() => {
		if (focusOnly) return []
		if (!searchTerm) return data
		return data.filter((item) => matchesSearchTerm(item, searchTerm))
	}, [data, focusOnly, searchTerm])

	const imageSizeMap = useMemo(() => {
		const map = new Map<string, number>()
		for (const image of images) {
			if (image.id) {
				map.set(image.id, image.size)
			}
			for (const tag of image.repoTags || []) {
				if (!tag || tag === "<none>:<none>") continue
				map.set(tag, image.size)
			}
			for (const digest of image.repoDigests || []) {
				if (!digest || digest === "<none>@<none>") continue
				map.set(digest, image.size)
			}
		}
		return map
	}, [images])

	const focusGroups = useMemo(() => {
		if (!focusOnly) return []
		return focusRules.map((rule) => {
			const matched = data.filter((item) => matchesFocusRule(item, rule))
			const totalCount = matched.length
			const runningCount = matched.reduce((count, item) => count + (item.state === "running" ? 1 : 0), 0)
			const visibleContainers = matched.filter((item) => {
				if (!showAll && !isContainerActive(item)) return false
				return matchesSearchTerm(item, searchTerm)
			})
			return {
				id: rule.id,
				rule,
				label: formatFocusRuleLabel(rule),
				description: rule.description,
				totalCount,
				runningCount,
				status: getFocusGroupStatus(runningCount, totalCount),
				visibleContainers,
				imageSummaries: buildFocusImageSummaries(matched, imageSizeMap),
				usageSummary: buildFocusUsageSummary(matched, usageMap),
			}
		})
	}, [data, focusOnly, focusRules, searchTerm, showAll, imageSizeMap, usageMap])

	const focusChartData = useMemo(() => {
		return {
			systemStats: [],
			containerData: focusContainerData,
			chartTime,
			orientation: direction === "rtl" ? "right" : "left",
			...getTimeData(chartTime),
			agentVersion: agentVersion,
		} as ChartData
	}, [focusContainerData, chartTime, direction, agentVersion])

	const focusChartConfigs = useContainerChartConfigs(focusContainerData)
	const focusChartEmpty = !focusUsageLoading && focusContainerData.length === 0
	const focusFilterBar = focusContainerData.length ? <FocusUsageFilterBar /> : null

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
					description: formatContainerOperationError(err),
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

	const renderContainerRow = (item: DockerContainer, rowKey?: string, indent?: boolean) => (
		<TableRow
			key={rowKey ?? item.id}
			className={cn("group animate-in fade-in slide-in-from-top-1 duration-200", indent && "bg-muted/5")}
		>
			<TableCell className={cn("max-w-[100px] py-3", indent && "ps-8")}>
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
										{memFormatted ? decimalString(memFormatted.value, memFormatted.value >= 10 ? 1 : 2) : "-"}
										{memFormatted && (
											<span className="text-muted-foreground/60 text-[10px] ms-0.5">{memFormatted.unit}</span>
										)}
									</span>
								</div>
								<div className="flex items-center gap-2 overflow-hidden">
									<span className="font-semibold text-muted-foreground/70 w-8">NET</span>
									<span className="tabular-nums truncate text-foreground/90">
										{netFormatted ? decimalString(netFormatted.value, netFormatted.value >= 10 ? 1 : 2) : "-"}
										{netFormatted && (
											<span className="text-muted-foreground/60 text-[10px] ms-0.5">{netFormatted.unit}</span>
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
			<TableCell className="text-center py-3">
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
					<div className="flex items-center gap-2">
						<Switch id="docker-containers-focus" checked={focusOnly} onCheckedChange={setFocusOnly} />
						<Label htmlFor="docker-containers-focus">
							<Trans>Focus only</Trans>
						</Label>
					</div>
					<Button variant="outline" size="sm" onClick={() => setFocusDialogOpen(true)}>
						<Trans>Focus rules</Trans>
						<Badge variant="secondary" className="ms-2">
							{focusRules.length}
						</Badge>
					</Button>
					<Button variant="outline" size="sm" onClick={() => setFocusUsageOpen(true)}>
						<Trans>Focus usage</Trans>
					</Button>
					<Button variant="outline" size="sm" onClick={() => setAlertSettingsOpen(true)}>
						<Trans>Alert rules</Trans>
					</Button>
					<Input
						className="w-56"
						placeholder={t`Filter containers...`}
						value={filter}
						onChange={(event) => setFilter(event.target.value)}
					/>
					<Button variant="outline" size="sm" onClick={() => void handleRefresh()} disabled={loading || focusLoading}>
						{loading ? (
							<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
						) : (
							<RefreshCwIcon className="me-2 h-4 w-4" />
						)}
						<Trans>Refresh</Trans>
					</Button>
				</div>
			</div>
			{focusOnly && !focusLoading && focusRules.length === 0 ? (
				<div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-dashed bg-muted/10 p-3 text-sm text-muted-foreground">
					<span>
						<Trans>No focus rules configured for this system.</Trans>
					</span>
					<Button variant="outline" size="sm" onClick={() => setFocusDialogOpen(true)}>
						<Trans>Configure focus rules</Trans>
					</Button>
				</div>
			) : null}
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
							<TableHead className="text-center">
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
						) : focusOnly ? (
							focusGroups.length === 0 ? (
								<TableRow>
									<TableCell colSpan={9} className="text-center text-sm text-muted-foreground">
										<Trans>No containers found.</Trans>
									</TableCell>
								</TableRow>
							) : (
								focusGroups.map((group) => {
									const isExpanded = !!expandedGroups[group.id]
									const hasContainers = group.totalCount > 0
									return (
										<Fragment key={group.id}>
											<TableRow className="bg-muted/10">
												<TableCell className="max-w-[100px] py-3">
													<div className="flex items-start gap-2">
														<button
															type="button"
															onClick={() => toggleGroup(group.id)}
															disabled={!hasContainers}
															aria-label={t`Toggle focus group`}
															className="mt-0.5 inline-flex h-5 w-5 items-center justify-center rounded-sm text-muted-foreground/70 transition hover:text-foreground disabled:opacity-40"
														>
															<ChevronRightIcon
																className={cn(
																	"h-4 w-4 transition-transform duration-200",
																	isExpanded && "rotate-90",
																	!hasContainers && "opacity-40"
																)}
															/>
														</button>
														<div className="min-w-0">
															<div className="font-medium text-foreground truncate" title={group.label}>
																{group.label}
															</div>
															{group.description ? (
																<div className="text-xs text-muted-foreground line-clamp-2" title={group.description}>
																	{group.description}
																</div>
															) : null}
														</div>
													</div>
												</TableCell>
												<TableCell className="max-w-[220px] py-3 text-xs text-muted-foreground">
													{group.imageSummaries.length === 0 ? (
														<span>-</span>
													) : (
														<div className="flex flex-col gap-1">
															{group.imageSummaries.map((summary) => {
																const sizeLabel = formatImageSizeLabel(summary.size)
																const label = `${summary.name} (${sizeLabel})`
																return (
																	<div key={summary.name} className="truncate" title={label}>
																		<span className="text-foreground/90">{summary.name}</span>
																		<span className="text-muted-foreground"> ({sizeLabel})</span>
																	</div>
																)
															})}
														</div>
													)}
												</TableCell>
												<TableCell className="p-2">
													<div className="flex flex-col gap-1.5">
														<Badge
															variant={focusGroupStatusVariantMap[group.status] ?? "secondary"}
															className="w-fit px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider shadow-none"
														>
															{group.status}
														</Badge>
														<span className="text-xs font-medium text-muted-foreground">
															{group.runningCount}/{group.totalCount}
														</span>
													</div>
												</TableCell>
												<TableCell className="py-3">
													{(() => {
														const { cpu, memory, net } = group.usageSummary
														const threshold = getMeterState(cpu)
														const meterClass = cn(
															"h-full rounded-full",
															(threshold === MeterState.Good && "bg-green-500") ||
																(threshold === MeterState.Warn && "bg-yellow-500") ||
																"bg-red-500"
														)
														const memFormatted = formatBytes(memory, false, undefined, true)
														const netFormatted = formatBytes(net, true, undefined, true)
														return (
															<div className="flex flex-col gap-1.5 w-full max-w-[200px] text-xs">
																<div className="flex items-center gap-2">
																	<span className="font-semibold text-muted-foreground/70 w-8">CPU</span>
																	<span className="tabular-nums font-medium w-12 text-foreground/90">
																		{`${decimalString(cpu, cpu >= 10 ? 1 : 2)}%`}
																	</span>
																	<div className="h-1.5 flex-1 bg-muted/30 rounded-full overflow-hidden">
																		<div className={meterClass} style={{ width: `${cpu}%` }} />
																	</div>
																</div>
																<div className="grid grid-cols-2 gap-4">
																	<div className="flex items-center gap-2 overflow-hidden">
																		<span className="font-semibold text-muted-foreground/70 w-8">MEM</span>
																		<span className="tabular-nums truncate text-foreground/90">
																			{decimalString(memFormatted.value, memFormatted.value >= 10 ? 1 : 2)}
																			<span className="text-muted-foreground/60 text-[10px] ms-0.5">
																				{memFormatted.unit}
																			</span>
																		</span>
																	</div>
																	<div className="flex items-center gap-2 overflow-hidden">
																		<span className="font-semibold text-muted-foreground/70 w-8">NET</span>
																		<span className="tabular-nums truncate text-foreground/90">
																			{decimalString(netFormatted.value, netFormatted.value >= 10 ? 1 : 2)}
																			<span className="text-muted-foreground/60 text-[10px] ms-0.5">
																				{netFormatted.unit}
																			</span>
																		</span>
																	</div>
																</div>
															</div>
														)
													})()}
												</TableCell>
												<TableCell className="py-3 text-xs text-muted-foreground">-</TableCell>
												<TableCell className="py-3 text-xs text-muted-foreground">-</TableCell>
												<TableCell className="py-3 text-xs text-muted-foreground">-</TableCell>
												<TableCell className="py-3 text-xs text-muted-foreground">-</TableCell>
												<TableCell className="text-center py-3 text-xs text-muted-foreground">-</TableCell>
											</TableRow>
											{isExpanded
												? group.visibleContainers.map((item) =>
														renderContainerRow(item, `${group.id}-${item.id}`, true)
													)
												: null}
										</Fragment>
									)
								})
							)
						) : filteredContainers.length === 0 ? (
							<TableRow>
								<TableCell colSpan={9} className="text-center text-sm text-muted-foreground">
									<Trans>No containers found.</Trans>
								</TableCell>
							</TableRow>
						) : (
							filteredContainers.map((item) => renderContainerRow(item))
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
			<Dialog open={focusUsageOpen} onOpenChange={setFocusUsageOpen}>
				<DialogContent className="w-[90vw] max-w-[1600px]">
					<DialogHeader className="flex flex-row items-center justify-between space-y-0 pe-7">
						<DialogTitle>
							<Trans>Focus usage</Trans>
						</DialogTitle>
						{system ? <ChartTimeSelect className="h-8 w-32" agentVersion={agentVersion} /> : null}
					</DialogHeader>
					<div className="text-sm text-muted-foreground">
						{focusRules.length === 0 ? (
							<Trans>No focus rules configured for this system.</Trans>
						) : focusContainerNames.size === 0 ? (
							<Trans>No containers found.</Trans>
						) : null}
					</div>
					{focusRules.length === 0 || focusContainerNames.size === 0 ? null : (
						<div className="grid gap-4 grid-cols-1 lg:grid-cols-2">
							<ChartCard
								empty={focusChartEmpty}
								grid={true}
								title={dockerOrPodman(t`Docker CPU Usage`, isPodman)}
								description={t`Average CPU utilization of containers`}
								cornerEl={focusFilterBar}
							>
								<ContainerChart
									chartData={focusChartData}
									dataKey="c"
									chartType={ChartType.CPU}
									chartConfig={focusChartConfigs.cpu}
								/>
							</ChartCard>
							<ChartCard
								empty={focusChartEmpty}
								grid={true}
								title={dockerOrPodman(t`Docker Memory Usage`, isPodman)}
								description={dockerOrPodman(t`Memory usage of docker containers`, isPodman)}
								cornerEl={focusFilterBar}
							>
								<ContainerChart
									chartData={focusChartData}
									dataKey="m"
									chartType={ChartType.Memory}
									chartConfig={focusChartConfigs.memory}
								/>
							</ChartCard>
							<ChartCard
								empty={focusChartEmpty}
								grid={true}
								title={dockerOrPodman(t`Docker Network I/O`, isPodman)}
								description={dockerOrPodman(t`Network traffic of docker containers`, isPodman)}
								cornerEl={focusFilterBar}
							>
								<ContainerChart
									chartData={focusChartData}
									chartType={ChartType.Network}
									dataKey="n"
									chartConfig={focusChartConfigs.network}
								/>
							</ChartCard>
						</div>
					)}
				</DialogContent>
			</Dialog>
			<DockerFocusRulesDialog
				open={focusDialogOpen}
				onOpenChange={setFocusDialogOpen}
				systemId={systemId}
				rules={focusRules}
				loading={focusLoading}
				onReload={loadFocusRules}
			/>
			<FocusAlertSettingsSheet open={alertSettingsOpen} onOpenChange={setAlertSettingsOpen} systemId={systemId} />
		</div>
	)
})

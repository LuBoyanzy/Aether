import type { Column, ColumnDef } from "@tanstack/react-table"
import { Button } from "@/components/ui/button"
import { cn, decimalString, formatBytes, formatSecondsToHuman, hourWithSeconds, getMeterState } from "@/lib/utils"
import type { ContainerRecord } from "@/types"
import {
	ActivityIcon,
	ArrowUpDownIcon,
	ClockIcon,
	ContainerIcon,
	ServerIcon,
	LayersIcon,
	TimerIcon,
	PlayIcon,
	SquareIcon,
	RotateCcwIcon,
	PauseIcon,
	ZapIcon,
	PlayCircleIcon,
} from "lucide-react"
import { HourglassIcon } from "../ui/icons"
import { t } from "@lingui/core/macro"
import { $allSystemsById } from "@/lib/stores"
import { useStore } from "@nanostores/react"
import { MeterState } from "@/lib/enums"
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "../ui/dropdown-menu"
import { toast } from "../ui/use-toast"

const STATUS_COLORS = {
	up: "bg-green-500",
	down: "bg-red-500",
	pending: "bg-yellow-500",
	paused: "bg-primary/40",
} as const

function getStatusColor(status: string) {
	status = status.toLowerCase()
	if (status === "running" || status === "healthy" || status === "up") return STATUS_COLORS.up
	if (status === "exited" || status === "dead" || status === "unhealthy") return STATUS_COLORS.down
	if (status === "created" || status === "restarting" || status === "starting" || status === "recreating" || status === "creating")
		return STATUS_COLORS.pending
	if (status === "removing") return STATUS_COLORS.pending
	return STATUS_COLORS.paused
}

function getStatusLabel(status: string) {
	const labels: Record<string, string> = {
		running: "已启动",
		created: "已创建",
		restarting: "重启中",
		removing: "移除中",
		paused: "已暂停",
		exited: "已停止",
		dead: "已结束",
		starting: "启动中",
		recreating: "重建中",
		creating: "创建中",
		healthy: "正常",
		unhealthy: "异常",
	}
	const key = status.toLowerCase()
	return labels[key] || status || "未知"
}

type ContainerOp = "start" | "stop" | "restart" | "kill" | "pause" | "unpause"

function getAvailableOps(statusRaw: string): ContainerOp[] {
	const status = statusRaw.toLowerCase()
	if (status === "running" || status === "healthy") return ["stop", "restart", "kill", "pause"]
	if (status === "paused") return ["unpause", "stop", "kill"]
	if (status === "exited" || status === "dead" || status === "created") return ["start", "kill"]
	if (status === "restarting" || status === "starting" || status === "creating" || status === "recreating") return ["kill"]
	return ["start", "kill"]
}

function opIcon(op: ContainerOp) {
	switch (op) {
		case "start":
			return PlayIcon
		case "stop":
			return SquareIcon
		case "restart":
			return RotateCcwIcon
		case "kill":
			return ZapIcon
		case "pause":
			return PauseIcon
		case "unpause":
			return PlayCircleIcon
		default:
			return PlayIcon
	}
}

export const buildContainerChartCols = (onOperate: (record: ContainerRecord, op: ContainerOp) => Promise<void>): ColumnDef<ContainerRecord>[] => [
	{
		id: "name",
		sortingFn: (a, b) => a.original.name.localeCompare(b.original.name),
		accessorFn: (record) => record.name,
		header: ({ column }) => <HeaderButton column={column} name={t`Name`} Icon={ContainerIcon} />,
		cell: ({ getValue }) => {
			return <span className="ms-1.5 xl:w-48 block truncate font-medium text-foreground">{getValue() as string}</span>
		},
	},
	{
		id: "system",
		accessorFn: (record) => record.system,
		sortingFn: (a, b) => {
			const allSystems = $allSystemsById.get()
			const systemNameA = allSystems[a.original.system]?.name ?? ""
			const systemNameB = allSystems[b.original.system]?.name ?? ""
			return systemNameA.localeCompare(systemNameB)
		},
		header: ({ column }) => <HeaderButton column={column} name={t`System`} Icon={ServerIcon} />,
		cell: ({ getValue }) => {
			const allSystems = useStore($allSystemsById)
			return (
				<span className="ms-0 w-fit max-w-36 block truncate text-muted-foreground bg-muted/50 px-2 py-0.5 rounded-md text-xs border border-muted/50">
					{allSystems[getValue() as string]?.name ?? ""}
				</span>
			)
		},
	},
	// {
	// 	id: "id",
	// 	accessorFn: (record) => record.id,
	// 	sortingFn: (a, b) => a.original.id.localeCompare(b.original.id),
	// 	header: ({ column }) => <HeaderButton column={column} name="ID" Icon={HashIcon} />,
	// 	cell: ({ getValue }) => {
	// 		return <span className="ms-1.5 me-3 font-mono">{getValue() as string}</span>
	// 	},
	// },
	{
		id: "usage",
		accessorFn: (record) => record.cpu,
		invertSorting: true,
		header: ({ column }) => <HeaderButton column={column} name={t`Usage`} Icon={ActivityIcon} />,
		cell: ({ row }) => {
			const { cpu, memory, net } = row.original
			// CPU
			const threshold = getMeterState(cpu)
			const meterClass = cn(
				"h-full rounded-full",
				(threshold === MeterState.Good && STATUS_COLORS.up) ||
					(threshold === MeterState.Warn && STATUS_COLORS.pending) ||
					STATUS_COLORS.down
			)
			// Memory
			const memFormatted = formatBytes(memory, false, undefined, true)
			// Net
			const netFormatted = formatBytes(net, true, undefined, true)

			return (
				<div className="flex flex-col gap-1.5 w-full max-w-[200px] text-xs">
					{/* CPU */}
					<div className="flex items-center gap-2">
						<span className="font-semibold text-muted-foreground/70 w-8">CPU</span>
						<span className="tabular-nums font-medium w-12 text-foreground/90">
							{decimalString(cpu, cpu >= 10 ? 1 : 2)}%
						</span>
						<div className="h-1.5 flex-1 bg-muted/30 rounded-full overflow-hidden">
							<div className={meterClass} style={{ width: `${cpu}%` }} />
						</div>
					</div>
					{/* Mem & Net */}
					<div className="grid grid-cols-2 gap-4">
						<div className="flex items-center gap-2 overflow-hidden">
							<span className="font-semibold text-muted-foreground/70 w-8">MEM</span>
							<span className="tabular-nums truncate text-foreground/90">
								{decimalString(memFormatted.value, memFormatted.value >= 10 ? 1 : 2)}
								<span className="text-muted-foreground/60 text-[10px] ms-0.5">{memFormatted.unit}</span>
							</span>
						</div>
						<div className="flex items-center gap-2 overflow-hidden">
							<span className="font-semibold text-muted-foreground/70 w-8">NET</span>
							<span className="tabular-nums truncate text-foreground/90">
								{decimalString(netFormatted.value, netFormatted.value >= 10 ? 1 : 2)}
								<span className="text-muted-foreground/60 text-[10px] ms-0.5">{netFormatted.unit}</span>
							</span>
						</div>
					</div>
				</div>
			)
		},
	},
	{
		id: "image",
		sortingFn: (a, b) => a.original.image.localeCompare(b.original.image),
		accessorFn: (record) => record.image,
		header: ({ column }) => (
			<HeaderButton column={column} name={t({ message: "Image", context: "Docker image" })} Icon={LayersIcon} />
		),
		cell: ({ getValue }) => {
			return <span className="ms-1.5 xl:w-40 block truncate text-muted-foreground">{getValue() as string}</span>
		},
	},
	{
		id: "state",
		accessorFn: (record) => record.status,
		header: ({ column }) => <HeaderButton column={column} name={t`Status`} Icon={HourglassIcon} />,
		cell: ({ row }) => {
			const status = row.original.status ?? ""
			const ops = getAvailableOps(status)
			return (
				<DropdownMenu>
					<DropdownMenuTrigger asChild>
						<button className="flex items-center gap-2 ms-1.5 w-36 px-2 py-1 rounded-md hover:bg-muted/60 transition">
							<span className={cn("size-2.5 rounded-full shrink-0 shadow-sm", getStatusColor(status))} />
							<span className="truncate capitalize text-left flex-1">{getStatusLabel(status)}</span>
						</button>
					</DropdownMenuTrigger>
					<DropdownMenuContent align="start" className="min-w-40">
						<DropdownMenuLabel className="text-xs text-muted-foreground">容器操作</DropdownMenuLabel>
						<DropdownMenuSeparator />
						{ops.map((op) => {
							const Icon = opIcon(op)
							return (
								<DropdownMenuItem
									key={op}
									onSelect={async (e) => {
										e.preventDefault()
										try {
											await onOperate(row.original, op)
										} catch (err: any) {
											toast({
												title: t`Operation failed`,
												description: err?.message || String(err),
												variant: "destructive",
											})
										}
									}}
								>
									<Icon className="size-4 me-2" /> {op.toUpperCase()}
								</DropdownMenuItem>
							)
						})}
					</DropdownMenuContent>
				</DropdownMenu>
			)
		},
	},
	{
		id: "uptime",
		accessorFn: (record) => record.uptime,
		invertSorting: true,
		header: ({ column }) => <HeaderButton column={column} name={t`Uptime`} Icon={TimerIcon} />,
		cell: ({ row }) => {
			const uptime = row.original.uptime ?? 0
			const formatted = formatSecondsToHuman(uptime)
			return <span className="ms-1.5 tabular-nums">{formatted || "—"}</span>
		},
	},
	{
		id: "updated",
		invertSorting: true,
		accessorFn: (record) => record.updated,
		header: ({ column }) => <HeaderButton column={column} name={t`Updated`} Icon={ClockIcon} />,
		cell: ({ getValue }) => {
			const timestamp = getValue() as number
			return (
				<span className="ms-1.5 tabular-nums text-muted-foreground">
					{hourWithSeconds(new Date(timestamp).toISOString())}
				</span>
			)
		},
	},
]

function HeaderButton({
	column,
	name,
	Icon,
}: {
	column: Column<ContainerRecord>
	name: string
	Icon: React.ElementType
}) {
	const isSorted = column.getIsSorted()
	return (
		<Button
			className={cn("h-9 px-3 flex items-center gap-2 duration-50 justify-start", isSorted && "text-foreground")}
			variant="ghost"
			onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
		>
			{Icon && <Icon className="size-4" />}
			{name}
			<ArrowUpDownIcon className="size-4" />
		</Button>
	)
}

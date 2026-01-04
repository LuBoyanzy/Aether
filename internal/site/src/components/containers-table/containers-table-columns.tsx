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
} from "lucide-react"
import { HourglassIcon } from "../ui/icons"
import { t } from "@lingui/core/macro"
import { $allSystemsById } from "@/lib/stores"
import { useStore } from "@nanostores/react"
import { MeterState } from "@/lib/enums"

const STATUS_COLORS = {
	up: "bg-green-500",
	down: "bg-red-500",
	pending: "bg-yellow-500",
	paused: "bg-primary/40",
} as const

function getStatusColor(status: string) {
	status = status.toLowerCase()
	if (status.startsWith("up") || status === "running") return STATUS_COLORS.up
	if (status.startsWith("exit") || status === "dead") return STATUS_COLORS.down
	if (status === "created" || status === "restarting") return STATUS_COLORS.pending
	return STATUS_COLORS.paused
}

export const containerChartCols: ColumnDef<ContainerRecord>[] = [
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
				<span className="ms-1.5 xl:w-34 block truncate text-muted-foreground bg-muted/50 px-2 py-0.5 rounded-md text-xs border border-muted/50">
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
		id: "status",
		accessorFn: (record) => record.status,
		invertSorting: true,
		sortingFn: (a, b) => (a.original.uptime ?? 0) - (b.original.uptime ?? 0),
		header: ({ column }) => <HeaderButton column={column} name={t`Status`} Icon={HourglassIcon} />,
		cell: ({ getValue }) => {
			const status = getValue() as string
			return (
				<div className="flex items-center gap-2 ms-1.5 w-25">
					<span className={cn("size-2.5 rounded-full shrink-0 shadow-sm", getStatusColor(status))} />
					<span className="truncate capitalize">{status}</span>
				</div>
			)
		},
	},
	{
		id: "uptime",
		accessorFn: (record) => record.uptime,
		invertSorting: true,
		header: ({ column }) => <HeaderButton column={column} name={t`运行时间`} Icon={TimerIcon} />,
		cell: ({ getValue }) => {
			const uptimeSeconds = getValue() as number
			const formatted = formatSecondsToHuman(uptimeSeconds)
			return <span className="ms-1.5 tabular-nums text-muted-foreground">{formatted || "—"}</span>
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
			className={cn(
				"h-9 px-3 flex items-center gap-2 duration-50",
				isSorted && "bg-accent/70 light:bg-accent text-accent-foreground/90"
			)}
			variant="ghost"
			onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
		>
			{Icon && <Icon className="size-4" />}
			{name}
			<ArrowUpDownIcon className="size-4" />
		</Button>
	)
}

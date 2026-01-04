import type { Column, ColumnDef } from "@tanstack/react-table"
import { Button } from "@/components/ui/button"
import { cn, decimalString, formatBytes, hourWithSeconds, getMeterState } from "@/lib/utils"
import type { SystemdRecord } from "@/types"
import { ServiceStatus, ServiceStatusLabels, ServiceSubState, ServiceSubStateLabels, MeterState } from "@/lib/enums"
import { ActivityIcon, ArrowUpDownIcon, ClockIcon, CpuIcon, MemoryStickIcon, TerminalSquareIcon } from "lucide-react"
import { t } from "@lingui/core/macro"
// import { $allSystemsById } from "@/lib/stores"
// import { useStore } from "@nanostores/react"

const STATUS_COLORS = {
	up: "bg-green-500",
	down: "bg-red-500",
	pending: "bg-yellow-500",
	paused: "bg-primary/40",
} as const

export const systemdTableCols: ColumnDef<SystemdRecord>[] = [
	{
		id: "name",
		sortingFn: (a, b) => a.original.name.localeCompare(b.original.name),
		accessorFn: (record) => record.name,
		header: ({ column }) => <HeaderButton column={column} name={t`Name`} Icon={TerminalSquareIcon} />,
		cell: ({ getValue }) => {
			return <span className="ms-1.5 xl:w-50 block truncate">{getValue() as string}</span>
		},
	},
	{
		id: "state",
		accessorFn: (record) => record.state,
		header: ({ column }) => <HeaderButton column={column} name={t`Status`} Icon={ActivityIcon} />,
		cell: ({ row }) => {
			const { state, sub } = row.original
			const label = ServiceSubStateLabels[sub] || ServiceStatusLabels[state] || "Unknown"
			const color = getStatusColor(state)
			return (
				<div className="flex items-center gap-2 ms-1.5 w-32">
					<span className={cn("size-2.5 rounded-full shrink-0 shadow-sm", color)} />
					<span className="truncate capitalize text-sm">{label}</span>
				</div>
			)
		},
	},
	{
		id: "cpu",
		accessorFn: (record) => {
			if (record.sub !== ServiceSubState.Running) {
				return -1
			}
			return record.cpu
		},
		invertSorting: true,
		header: ({ column }) => <HeaderButton column={column} name={`${t`CPU`} (10m)`} Icon={CpuIcon} />,
		cell: ({ getValue }) => {
			const val = getValue() as number
			if (val < 0) {
				return <span className="ms-1.5 text-muted-foreground">N/A</span>
			}
			const threshold = getMeterState(val)
			const meterClass = cn(
				"h-full rounded-full",
				(threshold === MeterState.Good && STATUS_COLORS.up) ||
					(threshold === MeterState.Warn && STATUS_COLORS.pending) ||
					STATUS_COLORS.down
			)
			return (
				<div className="flex items-center gap-2 ms-1.5 w-full max-w-[120px]">
					<span className="tabular-nums w-10 text-right">{`${decimalString(val, val >= 10 ? 1 : 2)}%`}</span>
					<div className="h-1.5 flex-1 bg-muted/30 rounded-full overflow-hidden">
						<div className={meterClass} style={{ width: `${Math.min(val, 100)}%` }} />
					</div>
				</div>
			)
		},
	},
	{
		id: "cpuPeak",
		accessorFn: (record) => {
			if (record.sub !== ServiceSubState.Running) {
				return -1
			}
			return record.cpuPeak ?? 0
		},
		invertSorting: true,
		header: ({ column }) => <HeaderButton column={column} name={t`CPU Peak`} Icon={CpuIcon} />,
		cell: ({ getValue }) => {
			const val = getValue() as number
			if (val < 0) {
				return <span className="ms-1.5 text-muted-foreground">N/A</span>
			}
			return <span className="ms-1.5 tabular-nums">{`${decimalString(val, val >= 10 ? 1 : 2)}%`}</span>
		},
	},
	{
		id: "memory",
		accessorFn: (record) => record.memory,
		invertSorting: true,
		header: ({ column }) => <HeaderButton column={column} name={t`Memory`} Icon={MemoryStickIcon} />,
		cell: ({ getValue }) => {
			const val = getValue() as number
			if (!val) {
				return <span className="ms-1.5 text-muted-foreground">N/A</span>
			}
			const formatted = formatBytes(val, false, undefined, false)
			return (
				<span className="ms-1.5 tabular-nums">{`${decimalString(formatted.value, formatted.value >= 10 ? 1 : 2)} ${
					formatted.unit
				}`}</span>
			)
		},
	},
	{
		id: "memPeak",
		accessorFn: (record) => record.memPeak,
		invertSorting: true,
		header: ({ column }) => <HeaderButton column={column} name={t`Memory Peak`} Icon={MemoryStickIcon} />,
		cell: ({ getValue }) => {
			const val = getValue() as number
			if (!val) {
				return <span className="ms-1.5 text-muted-foreground">N/A</span>
			}
			const formatted = formatBytes(val, false, undefined, false)
			return (
				<span className="ms-1.5 tabular-nums">{`${decimalString(formatted.value, formatted.value >= 10 ? 1 : 2)} ${
					formatted.unit
				}`}</span>
			)
		},
	},
	{
		id: "updated",
		invertSorting: true,
		accessorFn: (record) => record.updated,
		header: ({ column }) => <HeaderButton column={column} name={t`Updated`} Icon={ClockIcon} />,
		cell: ({ row }) => {
			const timestamp = row.original.updated
			return (
				<span className="ms-1.5 tabular-nums truncate text-muted-foreground">
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
	column: Column<SystemdRecord>
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

export function getStatusColor(status: ServiceStatus) {
	switch (status) {
		case ServiceStatus.Active:
			return "bg-green-500"
		case ServiceStatus.Failed:
			return "bg-red-500"
		case ServiceStatus.Reloading:
		case ServiceStatus.Activating:
		case ServiceStatus.Deactivating:
			return "bg-yellow-500"
		default:
			return "bg-zinc-500"
	}
}

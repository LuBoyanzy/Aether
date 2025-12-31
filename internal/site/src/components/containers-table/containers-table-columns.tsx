import type { Column, ColumnDef } from "@tanstack/react-table"
import { Button } from "@/components/ui/button"
import { cn, decimalString, formatBytes, formatSecondsToHuman, hourWithSeconds } from "@/lib/utils"
import type { ContainerRecord } from "@/types"
import {
	ArrowUpDownIcon,
	ClockIcon,
	ContainerIcon,
	CpuIcon,
	LayersIcon,
	MemoryStickIcon,
	ServerIcon,
	TimerIcon,
} from "lucide-react"
import { EthernetIcon, HourglassIcon } from "../ui/icons"
import { t } from "@lingui/core/macro"
import { $allSystemsById } from "@/lib/stores"
import { useStore } from "@nanostores/react"

export const containerChartCols: ColumnDef<ContainerRecord>[] = [
	{
		id: "name",
		sortingFn: (a, b) => a.original.name.localeCompare(b.original.name),
		accessorFn: (record) => record.name,
		header: ({ column }) => <HeaderButton column={column} name={t`Name`} Icon={ContainerIcon} />,
		cell: ({ getValue }) => {
			return <span className="ms-1.5 xl:w-48 block truncate">{getValue() as string}</span>
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
			return <span className="ms-1.5 xl:w-34 block truncate">{allSystems[getValue() as string]?.name ?? ""}</span>
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
		id: "cpu",
		accessorFn: (record) => record.cpu,
		invertSorting: true,
		header: ({ column }) => <HeaderButton column={column} name={t`CPU`} Icon={CpuIcon} />,
		cell: ({ getValue }) => {
			const val = getValue() as number
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
			const formatted = formatBytes(val, false, undefined, true)
			return (
				<span className="ms-1.5 tabular-nums">{`${decimalString(formatted.value, formatted.value >= 10 ? 1 : 2)} ${formatted.unit}`}</span>
			)
		},
	},
	{
		id: "net",
		accessorFn: (record) => record.net,
		invertSorting: true,
		header: ({ column }) => <HeaderButton column={column} name={t`Net`} Icon={EthernetIcon} />,
		cell: ({ getValue }) => {
			const val = getValue() as number
			const formatted = formatBytes(val, true, undefined, true)
			return (
				<span className="ms-1.5 tabular-nums">{`${decimalString(formatted.value, formatted.value >= 10 ? 1 : 2)} ${formatted.unit}`}</span>
			)
		},
	},
	{
		id: "image",
		sortingFn: (a, b) => a.original.image.localeCompare(b.original.image),
		accessorFn: (record) => record.image,
		header: ({ column }) => <HeaderButton column={column} name={t({ message: "Image", context: "Docker image" })} Icon={LayersIcon} />,
		cell: ({ getValue }) => {
			return <span className="ms-1.5 xl:w-40 block truncate">{getValue() as string}</span>
		},
	},
	{
		id: "status",
		accessorFn: (record) => record.status,
		invertSorting: true,
		sortingFn: (a, b) => (a.original.uptime ?? 0) - (b.original.uptime ?? 0),
		header: ({ column }) => <HeaderButton column={column} name={t`Status`} Icon={HourglassIcon} />,
		cell: ({ getValue }) => {
			return <span className="ms-1.5 w-25 block truncate">{getValue() as string}</span>
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
				<span className="ms-1.5 tabular-nums">
					{hourWithSeconds(new Date(timestamp).toISOString())}
				</span>
			)
		},
	},
]

function HeaderButton({ column, name, Icon }: { column: Column<ContainerRecord>; name: string; Icon: React.ElementType }) {
	const isSorted = column.getIsSorted()
	return (
		<Button
			className={cn("h-9 px-3 flex items-center gap-2 duration-50", isSorted && "bg-accent/70 light:bg-accent text-accent-foreground/90")}
			variant="ghost"
			onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
		>
			{Icon && <Icon className="size-4" />}
			{name}
			<ArrowUpDownIcon className="size-4" />
		</Button>
	)
}

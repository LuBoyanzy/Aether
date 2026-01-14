/**
 * 审查日志时间范围选择组件。
 * 将开始/结束时间合并为单一输入块，保持筛选区简洁。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { CalendarIcon } from "lucide-react"
import { Button } from "@/components/ui/button"
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { cn, formatShortDate } from "@/lib/utils"

type AuditDateRangeProps = {
	start: string
	end: string
	onStartChange: (value: string) => void
	onEndChange: (value: string) => void
	startId?: string
	endId?: string
	disabled?: boolean
	className?: string
}

export function AuditDateRange({
	start,
	end,
	onStartChange,
	onEndChange,
	startId,
	endId,
	disabled,
	className,
}: AuditDateRangeProps) {
	const startInputId = startId ?? "audit-range-start"
	const endInputId = endId ?? "audit-range-end"

	return (
		<div className={cn("grid gap-2", className)}>
			<Label className="text-xs font-medium text-muted-foreground">
				<Trans>Time Range</Trans>
			</Label>
			<DropdownMenu>
				<DropdownMenuTrigger asChild>
					<Button
						id="date"
						variant={"outline"}
						className={cn(
							"w-[260px] justify-start text-left font-normal h-9",
							!start && !end && "text-muted-foreground"
						)}
						disabled={disabled}
					>
						<CalendarIcon className="mr-2 h-4 w-4" />
						{start || end ? (
							<>
								{start ? formatShortDate(start) : t`Start Time`} - {end ? formatShortDate(end) : t`End Time`}
							</>
						) : (
							<span>
								<Trans>Pick a date range</Trans>
							</span>
						)}
					</Button>
				</DropdownMenuTrigger>
				<DropdownMenuContent className="w-auto p-4" align="start">
					<div className="flex flex-col gap-4">
						<div className="grid gap-2">
							<Label htmlFor={startInputId}>
								<Trans>Start Time</Trans>
							</Label>
							<Input
								id={startInputId}
								type="datetime-local"
								value={start}
								className="h-9 w-[200px]"
								aria-label={t`Start Time`}
								disabled={disabled}
								onChange={(event) => onStartChange(event.target.value)}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor={endInputId}>
								<Trans>End Time</Trans>
							</Label>
							<Input
								id={endInputId}
								type="datetime-local"
								value={end}
								className="h-9 w-[200px]"
								aria-label={t`End Time`}
								disabled={disabled}
								onChange={(event) => onEndChange(event.target.value)}
							/>
						</div>
					</div>
				</DropdownMenuContent>
			</DropdownMenu>
		</div>
	)
}

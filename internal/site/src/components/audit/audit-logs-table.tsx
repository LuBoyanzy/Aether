/**
 * 审查日志列表与筛选面板。
 * 负责拉取 Docker 审计记录并提供系统/时间筛选与搜索。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { useStore } from "@nanostores/react"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { AuditDateRange } from "@/components/audit/audit-date-range"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { toast } from "@/components/ui/use-toast"
import { listDockerAudits } from "@/lib/docker"
import { $allSystemsById, $systems } from "@/lib/stores"
import { formatShortDate } from "@/lib/utils"
import type { DockerAuditItem } from "@/types"
import { ChevronLeftIcon, ChevronRightIcon, LoaderCircleIcon, RefreshCwIcon } from "lucide-react"

const perPageOptions = [20, 50, 100]

type DateRange = {
	start?: Date
	end?: Date
}

const parseDateInput = (value: string, label: string): Date | undefined => {
	const trimmed = value.trim()
	if (!trimmed) return undefined
	const parsed = new Date(trimmed)
	if (Number.isNaN(parsed.getTime())) {
		throw new Error(`${label} must be a valid datetime-local value: ${value}`)
	}
	return parsed
}

const validateDateRange = (startRaw: string, endRaw: string): DateRange => {
	const start = parseDateInput(startRaw, "start")
	const end = parseDateInput(endRaw, "end")
	if (start && end && start > end) {
		throw new Error(`start must be before end: start=${start.toISOString()} end=${end.toISOString()}`)
	}
	return { start, end }
}

const renderStatusBadge = (status: string) => {
	const normalized = status.trim().toLowerCase()
	if (normalized === "success") {
		return <Badge className="bg-emerald-500/15 text-emerald-600 hover:bg-emerald-500/25">{t`Success`}</Badge>
	}
	if (normalized === "failed") {
		return <Badge variant="destructive">{t`Failed`}</Badge>
	}
	return <Badge variant="secondary">{status || t`Unknown`}</Badge>
}

export default memo(function AuditLogsTable() {
	const systems = useStore($systems)
	const systemsById = useStore($allSystemsById)
	const [loading, setLoading] = useState(false)
	const [items, setItems] = useState<DockerAuditItem[]>([])
	const [filter, setFilter] = useState("")
	const [page, setPage] = useState(1)
	const [perPage, setPerPage] = useState(perPageOptions[0])
	const [draftSystemId, setDraftSystemId] = useState("all")
	const [draftStart, setDraftStart] = useState("")
	const [draftEnd, setDraftEnd] = useState("")
	const [systemId, setSystemId] = useState("all")
	const [start, setStart] = useState("")
	const [end, setEnd] = useState("")

	const loadAudits = useCallback(async () => {
		setLoading(true)
		try {
			const { start: startDate, end: endDate } = validateDateRange(start, end)
			const params: { system?: string; start?: string; end?: string; page?: number; perPage?: number } = {
				page,
				perPage,
			}
			if (systemId !== "all") {
				params.system = systemId
			}
			if (startDate) {
				params.start = startDate.toISOString()
			}
			if (endDate) {
				params.end = endDate.toISOString()
			}
			const res = await listDockerAudits(params)
			setItems(res.items ?? [])
		} catch (err) {
			console.error("load audit logs failed", {
				err,
				systemId,
				start,
				end,
				page,
				perPage,
			})
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load audit logs` })
			throw err
		} finally {
			setLoading(false)
		}
	}, [systemId, start, end, page, perPage])

	useEffect(() => {
		void loadAudits()
	}, [loadAudits])

	const applyFilters = useCallback(() => {
		try {
			validateDateRange(draftStart, draftEnd)
			setPage(1)
			setSystemId(draftSystemId)
			setStart(draftStart)
			setEnd(draftEnd)
		} catch (err) {
			console.error("apply audit log filters failed", {
				err,
				draftSystemId,
				draftStart,
				draftEnd,
			})
			toast({ variant: "destructive", title: t`Error`, description: t`Invalid time range` })
			throw err
		}
	}, [draftSystemId, draftStart, draftEnd])

	const resetFilters = useCallback(() => {
		setDraftSystemId("all")
		setDraftStart("")
		setDraftEnd("")
		setPage(1)
		setSystemId("all")
		setStart("")
		setEnd("")
	}, [])

	const filteredItems = useMemo(() => {
		const term = filter.trim().toLowerCase()
		if (!term) return items
		return items.filter((item) => {
			const systemName = systemsById[item.system]?.name ?? ""
			const haystack = [
				item.system,
				systemName,
				item.user_email,
				item.action,
				item.resource_type,
				item.resource_id,
				item.status,
				item.detail,
			].join(" ")
			return haystack.toLowerCase().includes(term)
		})
	}, [items, filter, systemsById])

	const hasNextPage = items.length === perPage

	return (
		<Card className="p-6 @container w-full">
			<CardHeader className="p-0 mb-4">
				<div className="flex flex-wrap items-end justify-between gap-3">
					<div>
						<CardTitle className="mb-2">
							<Trans>Audit Logs</Trans>
						</CardTitle>
						<CardDescription>
							<Trans>Review Docker audit records and data cleanup runs.</Trans>
						</CardDescription>
					</div>
					<div className="flex flex-wrap items-center gap-2">
						<Input
							className="w-56"
							placeholder={t`Search logs...`}
							value={filter}
							onChange={(event) => setFilter(event.target.value)}
						/>
						<Button variant="outline" size="sm" onClick={() => void loadAudits()} disabled={loading}>
							{loading ? (
								<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
							) : (
								<RefreshCwIcon className="me-2 h-4 w-4" />
							)}
							<Trans>Refresh</Trans>
						</Button>
					</div>
				</div>
			</CardHeader>
			<CardContent className="p-0 space-y-4">
				<div className="flex flex-col gap-4 py-2 lg:flex-row lg:items-end">
					<div className="flex flex-wrap items-end gap-4 flex-1">
						<div className="flex flex-col gap-2 min-w-[200px]">
							<Label htmlFor="audit-system" className="text-xs font-medium text-muted-foreground">
								<Trans>System</Trans>
							</Label>
							<Select value={draftSystemId} onValueChange={setDraftSystemId} disabled={!systems.length}>
								<SelectTrigger id="audit-system" className="h-9">
									<SelectValue placeholder={t`All Systems`} />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="all">
										<Trans>All Systems</Trans>
									</SelectItem>
									{systems.map((system) => (
										<SelectItem key={system.id} value={system.id}>
											{system.name}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</div>
						<AuditDateRange
							start={draftStart}
							end={draftEnd}
							onStartChange={setDraftStart}
							onEndChange={setDraftEnd}
							startId="audit-start"
							endId="audit-end"
						/>
					</div>
					<div className="flex items-center gap-2 pb-0.5">
						<Button variant="outline" size="sm" onClick={resetFilters} className="h-9">
							<Trans>Reset Filters</Trans>
						</Button>
						<Button size="sm" onClick={applyFilters} className="h-9">
							<Trans>Apply Filters</Trans>
						</Button>
					</div>
				</div>

				<div className="h-min max-h-[calc(100dvh-26rem)] max-w-full relative overflow-auto border rounded-md bg-card">
					<Table className="min-w-[900px]">
						<TableHeader className="sticky top-0 z-10 bg-card">
							<TableRow>
								<TableHead className="w-[180px]">
									<Trans>System</Trans>
								</TableHead>
								<TableHead className="w-[140px] ps-0">
									<Trans>Action</Trans>
								</TableHead>
								<TableHead className="w-[220px]">
									<Trans>Resource</Trans>
								</TableHead>
								<TableHead className="w-[120px]">
									<Trans>Status</Trans>
								</TableHead>
								<TableHead className="w-[200px]">
									<Trans>User</Trans>
								</TableHead>
								<TableHead className="w-[150px]">
									<Trans>Detail</Trans>
								</TableHead>
								<TableHead className="w-[150px]">
									<Trans>Created</Trans>
								</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{filteredItems.length === 0 ? (
								<TableRow>
									<TableCell colSpan={7} className="py-10 text-center text-sm text-muted-foreground">
										<Trans>No audit logs found.</Trans>
									</TableCell>
								</TableRow>
							) : (
								filteredItems.map((item) => {
									const systemName = systemsById[item.system]?.name
									const resourceLabel = [item.resource_type, item.resource_id].filter(Boolean).join(":")
									return (
										<TableRow key={item.id}>
											<TableCell className="max-w-[180px]">
												<div className="truncate" title={systemName || item.system || "-"}>
													{systemName || item.system || "-"}
												</div>
											</TableCell>
											<TableCell className="ps-0">
												<Badge variant="outline" className="font-mono text-[10px] -ms-2">
													{item.action}
												</Badge>
											</TableCell>
											<TableCell className="max-w-[220px]">
												<div className="truncate font-mono text-xs" title={resourceLabel || "-"}>
													{resourceLabel || "-"}
												</div>
											</TableCell>
											<TableCell>{renderStatusBadge(item.status)}</TableCell>
											<TableCell className="max-w-[160px]">
												<div className="truncate" title={item.user_email || "-"}>
													{item.user_email || "-"}
												</div>
											</TableCell>
											<TableCell>
												<div className="max-w-[320px] truncate text-muted-foreground" title={item.detail || "-"}>
													{item.detail || "-"}
												</div>
											</TableCell>
											<TableCell className="tabular-nums text-xs" title={`${item.created} UTC`}>
												{formatShortDate(item.created)}
											</TableCell>
										</TableRow>
									)
								})
							)}
						</TableBody>
					</Table>
				</div>
				<div className="flex items-center justify-between pt-1">
					<div className="flex items-center gap-2 text-xs text-muted-foreground">
						<Trans>Rows per page</Trans>
						<Select
							value={String(perPage)}
							onValueChange={(value) => {
								const parsed = Number(value)
								setPage(1)
								setPerPage(Number.isFinite(parsed) ? parsed : perPageOptions[0])
							}}
						>
							<SelectTrigger className="h-8 w-[70px]">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								{perPageOptions.map((option) => (
									<SelectItem key={option} value={String(option)}>
										{option}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
					<div className="flex items-center gap-2">
						<Button
							variant="outline"
							size="icon"
							className="h-8 w-8"
							onClick={() => setPage((prev) => Math.max(1, prev - 1))}
							disabled={page <= 1}
						>
							<ChevronLeftIcon className="h-4 w-4" />
							<span className="sr-only">
								<Trans>Prev</Trans>
							</span>
						</Button>
						<div className="text-xs text-muted-foreground min-w-[3.5rem] text-center font-medium">
							{t`Page ${page}`}
						</div>
						<Button
							variant="outline"
							size="icon"
							className="h-8 w-8"
							onClick={() => setPage((prev) => prev + 1)}
							disabled={!hasNextPage}
						>
							<span className="sr-only">
								<Trans>Next</Trans>
							</span>
							<ChevronRightIcon className="h-4 w-4" />
						</Button>
					</div>
				</div>
			</CardContent>
		</Card>
	)
})

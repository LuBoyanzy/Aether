import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { useCallback, useEffect, useMemo, useState } from "react"
import Spinner from "@/components/spinner"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { toast } from "@/components/ui/use-toast"
import { MeterState } from "@/lib/enums"
import { pb } from "@/lib/api"
import { cn, decimalString, formatBytes, getMeterState } from "@/lib/utils"
import type { SystemNetworkMountRecord } from "@/types"

const mountFields =
	"id,system,source,source_host,source_path,mount_point,fstype,total_bytes,used_bytes,used_pct,updated"

function getUsageClass(pct: number) {
	const threshold = getMeterState(pct)
	return (
		(threshold === MeterState.Good && "bg-green-500") ||
		(threshold === MeterState.Warn && "bg-yellow-500") ||
		"bg-red-500"
	)
}

function formatSource(mount: SystemNetworkMountRecord) {
	if (mount.source_host) {
		if (mount.source_path) {
			return `${mount.source_host}:${mount.source_path}`
		}
		return mount.source_host
	}
	return mount.source || "-"
}

function formatBytesLabel(value: number) {
	const formatted = formatBytes(value, false, undefined, false)
	return `${decimalString(formatted.value, formatted.value >= 10 ? 1 : 2)} ${formatted.unit}`
}

function sortMounts(next: SystemNetworkMountRecord[]) {
	return [...next].sort((a, b) => a.mount_point.localeCompare(b.mount_point))
}

export default function NetworkMountsCard({ systemId }: { systemId: string }) {
	const [mounts, setMounts] = useState<SystemNetworkMountRecord[]>([])
	const [loading, setLoading] = useState(false)

	const loadMounts = useCallback(async () => {
		if (!systemId) return
		setLoading(true)
		try {
			const items = await pb.collection<SystemNetworkMountRecord>("system_network_mounts").getFullList({
				filter: pb.filter("system = {:system}", { system: systemId }),
				fields: mountFields,
				sort: "mount_point",
			})
			setMounts(items)
		} catch (error) {
			console.error("load network mounts failed", error)
			const message = error instanceof Error ? error.message : String(error)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: `${t`Failed to load network mounts`}: ${message}`,
			})
			throw error
		} finally {
			setLoading(false)
		}
	}, [systemId])

	useEffect(() => {
		if (systemId) {
			void loadMounts()
		} else {
			setMounts([])
		}
	}, [systemId, loadMounts])

	useEffect(() => {
		let unsubscribe: (() => void) | undefined
		if (!systemId) return
		const options = {
			fields: mountFields,
			filter: pb.filter("system = {:system}", { system: systemId }),
		}
		; (async () => {
			try {
				unsubscribe = await pb.collection("system_network_mounts").subscribe(
					"*",
					(event) => {
						const record = event.record as SystemNetworkMountRecord
						setMounts((current) => {
							const list = current ?? []
							const matchesSystem = record.system === systemId
							if (event.action === "delete") {
								return list.filter((item) => item.id !== record.id)
							}
							if (!matchesSystem) {
								return list.filter((item) => item.id !== record.id)
							}
							const existingIndex = list.findIndex((item) => item.id === record.id)
							if (existingIndex === -1) {
								return sortMounts([record, ...list])
							}
							const next = [...list]
							next[existingIndex] = record
							return sortMounts(next)
						})
					},
					options
				)
			} catch (error) {
				console.error("Failed to subscribe to network mounts:", error)
			}
		})()

		return () => {
			unsubscribe?.()
		}
	}, [systemId])

	const empty = !loading && mounts.length === 0
	const rows = useMemo(() => mounts.map((mount) => {
		const hasUsage = mount.total_bytes > 0
		const pct = hasUsage ? mount.used_pct : 0
		const usageLabel = hasUsage
			? `${formatBytesLabel(mount.used_bytes)} / ${formatBytesLabel(mount.total_bytes)}`
			: "-"
		return {
			...mount,
			pct,
			usageLabel,
			sourceLabel: formatSource(mount),
		}
	}), [mounts])

	return (
		<Card>
			<CardHeader className="pb-4">
				<CardTitle className="text-xl sm:text-2xl">
					<Trans>Network Drives</Trans>
				</CardTitle>
				<CardDescription>
					<Trans>Mounted network filesystems detected on the server</Trans>
				</CardDescription>
			</CardHeader>
			<CardContent className="pt-0">
				{loading && <Spinner className="py-6" />}
				{empty && (
					<p className="text-sm text-muted-foreground py-4">
						<Trans>No network mounts detected</Trans>
					</p>
				)}
				{!loading && mounts.length > 0 && (
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead className="h-9 min-w-28">
									<Trans>Category</Trans>
								</TableHead>
								<TableHead className="h-9 min-w-20">
									<Trans>Type</Trans>
								</TableHead>
								<TableHead className="h-9 min-w-44">
									<Trans>Source</Trans>
								</TableHead>
								<TableHead className="h-9 min-w-40">
									<Trans>Mount Point</Trans>
								</TableHead>
								<TableHead className="h-9 min-w-44">
									<Trans>Usage</Trans>
								</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{rows.map((mount) => (
								<TableRow key={mount.id}>
									<TableCell className="py-2 text-sm text-muted-foreground">
										<Trans>Network Drive</Trans>
									</TableCell>
									<TableCell className="py-2 font-medium uppercase">{mount.fstype ? mount.fstype.toUpperCase() : "-"}</TableCell>
									<TableCell className="py-2 truncate" title={mount.sourceLabel}>
										{mount.sourceLabel}
									</TableCell>
									<TableCell className="py-2 truncate" title={mount.mount_point}>
										{mount.mount_point}
									</TableCell>
									<TableCell className="py-2">
										<div className="flex flex-col gap-1">
											<div className="flex gap-2 items-center tabular-nums">
												<span className="min-w-10">
													{mount.total_bytes > 0 ? `${decimalString(mount.pct, mount.pct >= 10 ? 1 : 2)}%` : "-"}
												</span>
												<span className="flex-1 min-w-16 grid bg-muted/50 h-2.5 rounded-full overflow-hidden">
													<span
														className={cn(getUsageClass(mount.pct), "rounded-full")}
														style={{ width: `${mount.pct}%` }}
													></span>
												</span>
											</div>
											<span className="text-xs text-muted-foreground tabular-nums">{mount.usageLabel}</span>
										</div>
									</TableCell>
								</TableRow>
							))}
						</TableBody>
					</Table>
				)}
			</CardContent>
		</Card>
	)
}

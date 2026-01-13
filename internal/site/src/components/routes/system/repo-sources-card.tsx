import { Trans, useLingui } from "@lingui/react/macro"
import { RefreshCwIcon } from "lucide-react"
import { useCallback, useEffect, useMemo, useState } from "react"
import Spinner from "@/components/spinner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { toast } from "@/components/ui/use-toast"
import { pb } from "@/lib/api"
import { Os } from "@/lib/enums"
import { cn } from "@/lib/utils"
import type { SystemRepoSourceRecord } from "@/types"

const repoFields = "id,system,manager,repo_id,name,url,enabled,status,error,checked_at,updated"

const statusVariants: Record<string, "success" | "danger" | "warning" | "secondary"> = {
	ok: "success",
	error: "danger",
	unknown: "secondary",
	unsupported: "warning",
}

function normalizeStatus(status?: string) {
	if (!status) return "unknown"
	return status.toLowerCase()
}

function sortSources(next: SystemRepoSourceRecord[]) {
	return [...next].sort((a, b) => (a.manager || "").localeCompare(b.manager || ""))
}

export default function RepoSourcesCard({ systemId, os }: { systemId: string; os?: Os }) {
	const { t } = useLingui()
	const [sources, setSources] = useState<SystemRepoSourceRecord[]>([])
	const [loading, setLoading] = useState(false)
	const [checking, setChecking] = useState(false)

	const isSupported = os === undefined || os === Os.Linux

	const loadSources = useCallback(async () => {
		if (!systemId) return
		setLoading(true)
		try {
			const items = await pb.collection<SystemRepoSourceRecord>("system_repo_sources").getFullList({
				filter: pb.filter("system = {:system}", { system: systemId }),
				fields: repoFields,
				sort: "manager",
			})
			setSources(items)
		} catch (error) {
			console.error("load repo sources failed", error)
			const message = error instanceof Error ? error.message : String(error)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: `${t`Failed to load repository sources`}: ${message}`,
			})
			throw error
		} finally {
			setLoading(false)
		}
	}, [systemId])

	useEffect(() => {
		if (systemId) {
			void loadSources()
		} else {
			setSources([])
		}
	}, [systemId, loadSources])

	useEffect(() => {
		let unsubscribe: (() => void) | undefined
		if (!systemId) return
		const options = {
			fields: repoFields,
			filter: pb.filter("system = {:system}", { system: systemId }),
		}
		; (async () => {
			try {
				unsubscribe = await pb.collection("system_repo_sources").subscribe(
					"*",
					(event) => {
						const record = event.record as SystemRepoSourceRecord
						setSources((current) => {
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
								return sortSources([record, ...list])
							}
							const next = [...list]
							next[existingIndex] = record
							return sortSources(next)
						})
					},
					options
				)
			} catch (error) {
				console.error("Failed to subscribe to repo source updates:", error)
			}
		})()

		return () => {
			unsubscribe?.()
		}
	}, [systemId])

	const handleCheck = useCallback(async () => {
		if (!systemId) return
		setChecking(true)
		try {
			await pb.send("/api/aether/repo-sources/refresh", {
				method: "POST",
				query: { system: systemId },
			})
			await loadSources()
		} catch (error) {
			console.error("repo source check failed", error)
			const message = error instanceof Error ? error.message : String(error)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: `${t`Failed to check repository sources`}: ${message}`,
			})
			throw error
		} finally {
			setChecking(false)
		}
	}, [systemId, loadSources])

	const empty = !loading && sources.length === 0
	const statusLabels = useMemo(() => ({
		ok: t`Available`,
		error: t`Unavailable`,
		unknown: t`Not checked`,
		unsupported: t`Unsupported`,
	}), [t])
	const rows = useMemo(() => sources.map((source) => {
		const statusKey = normalizeStatus(source.status)
		return {
			...source,
			statusKey,
			statusLabel: statusLabels[statusKey] ?? statusLabels.unknown,
			statusVariant: statusVariants[statusKey] ?? statusVariants.unknown,
			displayName: source.name || source.repo_id || "-",
		}
	}), [sources, statusLabels])

	return (
		<Card>
			<CardHeader className="gap-2 sm:flex sm:flex-row sm:items-start sm:justify-between">
				<div className="grid gap-1.5">
					<CardTitle className="text-xl sm:text-2xl">
						<Trans>Repository Sources</Trans>
					</CardTitle>
					<CardDescription>
						<Trans>Package manager sources configured on the server</Trans>
					</CardDescription>
				</div>
				<Button
					variant="outline"
					size="sm"
					disabled={!isSupported || checking}
					onClick={handleCheck}
					className="gap-2"
				>
					<RefreshCwIcon className={cn("size-4", checking && "animate-spin")} />
					<Trans>Check</Trans>
				</Button>
			</CardHeader>
			<CardContent className="pt-0">
				{loading && <Spinner className="py-6" />}
				{empty && (
					<p className="text-sm text-muted-foreground py-4">
						{isSupported ? <Trans>No repository sources detected</Trans> : <Trans>Repository checks are not supported on this OS</Trans>}
					</p>
				)}
				{!loading && sources.length > 0 && (
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead className="h-9 min-w-24">
									<Trans>Manager</Trans>
								</TableHead>
								<TableHead className="h-9 min-w-32">
									<Trans>Source</Trans>
								</TableHead>
								<TableHead className="h-9 min-w-40">
									<Trans>URL</Trans>
								</TableHead>
								<TableHead className="h-9 min-w-24">
									<Trans>Status</Trans>
								</TableHead>
								<TableHead className="h-9 min-w-28">
									<Trans>Error</Trans>
								</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{rows.map((source) => (
								<TableRow key={source.id}>
									<TableCell className="py-2 uppercase font-medium">
										{source.manager || "-"}
									</TableCell>
									<TableCell className="py-2">
										<div className="flex flex-col gap-1">
											<span className="truncate" title={source.displayName}>
												{source.displayName}
											</span>
											{source.enabled === false && (
												<Badge variant="secondary" className="w-fit">
													<Trans>Disabled</Trans>
												</Badge>
											)}
										</div>
									</TableCell>
									<TableCell className="py-2 truncate" title={source.url}>
										{source.url}
									</TableCell>
									<TableCell className="py-2">
										<Badge variant={source.statusVariant}>
											{source.statusLabel}
										</Badge>
									</TableCell>
									<TableCell className="py-2 truncate text-muted-foreground" title={source.error || ""}>
										{source.error || "-"}
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

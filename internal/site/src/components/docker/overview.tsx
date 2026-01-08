/**
 * Docker 概览页组件。
 * 展示引擎基础信息与容器/镜像统计。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { toast } from "@/components/ui/use-toast"
import { fetchDockerOverview } from "@/lib/docker"
import type { DockerOverview } from "@/types"
import { formatBytesLabel } from "@/components/docker/utils"
import DockerEmptyState from "@/components/docker/empty-state"
import { LoaderCircleIcon, RefreshCwIcon } from "lucide-react"

export default memo(function DockerOverviewPanel({ systemId }: { systemId?: string }) {
	const [loading, setLoading] = useState(false)
	const [data, setData] = useState<DockerOverview | null>(null)

	const loadOverview = useCallback(async () => {
		if (!systemId) return
		setLoading(true)
		try {
			const overview = await fetchDockerOverview(systemId)
			setData(overview)
		} catch (err) {
			console.error("load docker overview failed", err)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to load Docker overview`,
			})
			throw err
		} finally {
			setLoading(false)
		}
	}, [systemId])

	useEffect(() => {
		if (systemId) {
			void loadOverview()
		}
	}, [systemId, loadOverview])

	const statCards = useMemo(() => {
		if (!data) return []
		return [
			{ label: t`Containers`, value: data.containers },
			{ label: t`Running`, value: data.containersRunning },
			{ label: t`Paused`, value: data.containersPaused },
			{ label: t`Stopped`, value: data.containersStopped },
			{ label: t`Images`, value: data.images },
		]
	}, [data])

	if (!systemId) {
		return <DockerEmptyState />
	}

	return (
		<div className="space-y-4">
			<div className="flex flex-wrap items-end justify-between gap-3">
				<div>
					<h2 className="text-lg font-semibold">
						<Trans>Overview</Trans>
					</h2>
					<p className="text-sm text-muted-foreground">
						<Trans>Docker engine summary for the selected system.</Trans>
					</p>
				</div>
				<Button variant="outline" size="sm" onClick={() => void loadOverview()} disabled={loading}>
					{loading ? (
						<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
					) : (
						<RefreshCwIcon className="me-2 h-4 w-4" />
					)}
					<Trans>Refresh</Trans>
				</Button>
			</div>
			<div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
				{statCards.map((item) => (
					<Card key={item.label}>
						<CardHeader className="pb-2">
							<CardTitle className="text-sm text-muted-foreground">{item.label}</CardTitle>
						</CardHeader>
						<CardContent className="text-2xl font-semibold">{item.value ?? 0}</CardContent>
					</Card>
				))}
			</div>
			<Card>
				<CardHeader className="pb-2">
					<CardTitle className="text-base">
						<Trans>Engine Details</Trans>
					</CardTitle>
				</CardHeader>
				<CardContent>
					<div className="grid gap-3 text-sm text-muted-foreground md:grid-cols-2">
						<div>
							<span className="font-medium text-foreground">
								<Trans>Server Version</Trans>
							</span>
							: {data?.serverVersion || "-"}
						</div>
						<div>
							<span className="font-medium text-foreground">
								<Trans>API Version</Trans>
							</span>
							: {data?.apiVersion || "-"}
						</div>
						<div>
							<span className="font-medium text-foreground">
								<Trans>Compose Version</Trans>
							</span>
							: {data?.composeVersion || "-"}
						</div>
						<div>
							<span className="font-medium text-foreground">
								<Trans>Operating System</Trans>
							</span>
							: {data?.operatingSystem || "-"}
						</div>
						<div>
							<span className="font-medium text-foreground">
								<Trans>Kernel Version</Trans>
							</span>
							: {data?.kernelVersion || "-"}
						</div>
						<div>
							<span className="font-medium text-foreground">
								<Trans>Architecture</Trans>
							</span>
							: {data?.architecture || "-"}
						</div>
						<div>
							<span className="font-medium text-foreground">
								<Trans>CPUs</Trans>
							</span>
							: {data?.cpus ?? "-"}
						</div>
						<div>
							<span className="font-medium text-foreground">
								<Trans>Total Memory</Trans>
							</span>
							: {data ? formatBytesLabel(data.memTotal) : "-"}
						</div>
						<div>
							<span className="font-medium text-foreground">
								<Trans>Storage Driver</Trans>
							</span>
							: {data?.storageDriver || "-"}
						</div>
						<div>
							<span className="font-medium text-foreground">
								<Trans>Logging Driver</Trans>
							</span>
							: {data?.loggingDriver || "-"}
						</div>
						<div>
							<span className="font-medium text-foreground">
								<Trans>Cgroup Driver</Trans>
							</span>
							: {data?.cgroupDriver || "-"}
						</div>
						<div>
							<span className="font-medium text-foreground">
								<Trans>Docker Root Dir</Trans>
							</span>
							: {data?.dockerRootDir || "-"}
						</div>
					</div>
				</CardContent>
			</Card>
		</div>
	)
})

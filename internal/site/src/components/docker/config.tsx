/**
 * Docker daemon 配置管理面板。
 * 支持读取与更新 daemon.json。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { toast } from "@/components/ui/use-toast"
import { fetchDockerConfig, updateDockerConfig } from "@/lib/docker"
import { isReadOnlyUser } from "@/lib/api"
import DockerEmptyState from "@/components/docker/empty-state"
import { LoaderCircleIcon, RefreshCwIcon } from "lucide-react"

export default memo(function DockerConfigPanel({ systemId }: { systemId?: string }) {
	const [loading, setLoading] = useState(false)
	const [saving, setSaving] = useState(false)
	const [path, setPath] = useState("")
	const [content, setContent] = useState("")
	const [exists, setExists] = useState(false)
	const [restart, setRestart] = useState(false)

	const loadConfig = useCallback(async () => {
		if (!systemId) return
		setLoading(true)
		try {
			const config = await fetchDockerConfig(systemId)
			setPath(config.path || "")
			setContent(config.content || "")
			setExists(config.exists)
		} catch (err) {
			console.error("load docker config failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load Docker config` })
			throw err
		} finally {
			setLoading(false)
		}
	}, [systemId])

	useEffect(() => {
		if (systemId) {
			void loadConfig()
		}
	}, [systemId, loadConfig])

	const handleSave = useCallback(async () => {
		if (!systemId) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		if (!content.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`Config content is required` })
			return
		}
		setSaving(true)
		try {
			await updateDockerConfig({
				system: systemId,
				content,
				path: path.trim() || undefined,
				restart,
			})
			toast({ title: t`Operation success`, description: t`Docker config updated` })
			await loadConfig()
		} catch (err) {
			console.error("update docker config failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to update Docker config` })
			throw err
		} finally {
			setSaving(false)
		}
	}, [systemId, content, path, restart, loadConfig])

	if (!systemId) {
		return <DockerEmptyState />
	}

	return (
		<div className="space-y-4">
			<div className="flex flex-wrap items-end justify-between gap-3">
				<div>
					<h2 className="text-lg font-semibold">
						<Trans>Config</Trans>
					</h2>
					<p className="text-sm text-muted-foreground">
						<Trans>View and update Docker daemon configuration.</Trans>
					</p>
				</div>
				<Button variant="outline" size="sm" onClick={() => void loadConfig()} disabled={loading}>
					{loading ? (
						<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
					) : (
						<RefreshCwIcon className="me-2 h-4 w-4" />
					)}
					<Trans>Refresh</Trans>
				</Button>
			</div>
			<Card>
				<CardHeader>
					<CardTitle className="text-base">
						<Trans>Daemon Configuration</Trans>
					</CardTitle>
				</CardHeader>
				<CardContent className="space-y-4">
					{!exists && (
						<div className="text-sm text-muted-foreground">
							<Trans>daemon.json not found; a new file will be created on save.</Trans>
						</div>
					)}
					<div className="grid gap-2">
						<Label htmlFor="docker-config-path">
							<Trans>Config Path</Trans>
						</Label>
						<Input
							id="docker-config-path"
							value={path}
							onChange={(event) => setPath(event.target.value)}
							placeholder={t`Default daemon.json path`}
						/>
					</div>
					<div className="grid gap-2">
						<Label htmlFor="docker-config-content">
							<Trans>Config Content</Trans>
						</Label>
						<Textarea
							id="docker-config-content"
							rows={12}
							value={content}
							onChange={(event) => setContent(event.target.value)}
							placeholder={t`Paste daemon.json content`}
						/>
					</div>
					<div className="flex flex-wrap items-center justify-between gap-3">
						<div className="flex items-center gap-2">
							<Switch id="docker-config-restart" checked={restart} onCheckedChange={setRestart} />
							<Label htmlFor="docker-config-restart">
								<Trans>Restart Docker after update</Trans>
							</Label>
						</div>
						<Button onClick={() => void handleSave()} disabled={saving}>
							{saving ? (
								<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
							) : null}
							<Trans>Save</Trans>
						</Button>
					</div>
				</CardContent>
			</Card>
		</div>
	)
})

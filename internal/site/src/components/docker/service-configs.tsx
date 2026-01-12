/**
 * Docker 服务配置面板。
 * 管理服务配置入口并在页签内查看与保存 YAML 原文。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { useStore } from "@nanostores/react"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import {
	AlertDialog,
	AlertDialogAction,
	AlertDialogCancel,
	AlertDialogContent,
	AlertDialogDescription,
	AlertDialogFooter,
	AlertDialogHeader,
	AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { toast } from "@/components/ui/use-toast"
import { isReadOnlyUser } from "@/lib/api"
import { $allSystemsById } from "@/lib/stores"
import { formatShortDate } from "@/lib/utils"
import {
	createDockerServiceConfig,
	deleteDockerServiceConfig,
	fetchDockerServiceConfigContent,
	listDockerServiceConfigs,
	updateDockerServiceConfig,
	updateDockerServiceConfigContent,
} from "@/lib/docker"
import type { DockerServiceConfigItem } from "@/types"
import DockerEmptyState from "@/components/docker/empty-state"
import { ChevronLeftIcon, LoaderCircleIcon, MoreHorizontalIcon, RefreshCwIcon } from "lucide-react"

// 数据接口实现见 internal/hub/docker.go
const parseServiceUrl = (value: string) => {
	const trimmed = value.trim()
	if (!trimmed) {
		return { port: "", path: "" }
	}
	if (trimmed.startsWith("/")) {
		return { port: "", path: trimmed }
	}
	const portOnlyMatch = trimmed.match(/^(\d+)(\/.*)?$/)
	if (portOnlyMatch) {
		return { port: portOnlyMatch[1], path: portOnlyMatch[2] ?? "" }
	}
	const withProtocol = /^https?:\/\//i.test(trimmed) ? trimmed : `http://${trimmed}`
	try {
		const parsed = new URL(withProtocol)
		const path = `${parsed.pathname}${parsed.search}${parsed.hash}`
		return { port: parsed.port, path: path === "/" ? "" : path }
	} catch {
		const match = trimmed.match(/^(?:https?:\/\/)?([^/]+)(\/.*)?$/i)
		if (!match) {
			return { port: "", path: trimmed }
		}
		const portMatch = match[1].match(/:(\d+)$/)
		return { port: portMatch?.[1] ?? "", path: match[2] ?? "" }
	}
}

export default memo(function DockerServiceConfigsPanel({ systemId }: { systemId?: string }) {
	const [loading, setLoading] = useState(true)
	const [data, setData] = useState<DockerServiceConfigItem[]>([])
	const [filter, setFilter] = useState("")
	const [formOpen, setFormOpen] = useState(false)
	const [formMode, setFormMode] = useState<"create" | "update">("create")
	const [formId, setFormId] = useState("")
	const [name, setName] = useState("")
	const [url, setUrl] = useState("")
	const [port, setPort] = useState("")
	const [token, setToken] = useState("")
	const [submitLoading, setSubmitLoading] = useState(false)
	const [deleteTarget, setDeleteTarget] = useState<DockerServiceConfigItem | null>(null)
	const [activeConfig, setActiveConfig] = useState<DockerServiceConfigItem | null>(null)
	const [content, setContent] = useState("")
	const [contentLoading, setContentLoading] = useState(false)
	const [contentSaving, setContentSaving] = useState(false)
	const systems = useStore($allSystemsById)
	const systemHost = systemId ? systems[systemId]?.host?.trim() : ""
	const ipPrefix = systemHost ? `http://${systemHost}` : "http://"

	const loadConfigs = useCallback(async () => {
		if (!systemId) return
		setLoading(true)
		try {
			const res = await listDockerServiceConfigs(systemId)
			setData(res.items || [])
		} catch (err) {
			console.error("load service configs failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load service configs` })
			throw err
		} finally {
			setLoading(false)
		}
	}, [systemId])

	const loadContent = useCallback(
		async (configId: string) => {
			if (!systemId) return
			setContentLoading(true)
			try {
				const res = await fetchDockerServiceConfigContent({ system: systemId, id: configId })
				setContent(res.content ?? "")
			} catch (err) {
				console.error("load service config content failed", err)
				toast({ variant: "destructive", title: t`Error`, description: t`Failed to load config content` })
				throw err
			} finally {
				setContentLoading(false)
			}
		},
		[systemId]
	)

	useEffect(() => {
		if (!systemId) return
		setActiveConfig(null)
		setContent("")
		void loadConfigs()
	}, [systemId, loadConfigs])

	useEffect(() => {
		if (!activeConfig) return
		const updated = data.find((item) => item.id === activeConfig.id)
		if (!updated) {
			setActiveConfig(null)
			setContent("")
			return
		}
		if (updated.name !== activeConfig.name || updated.url !== activeConfig.url) {
			setActiveConfig(updated)
		}
	}, [data, activeConfig])

	const filtered = useMemo(() => {
		const term = filter.trim().toLowerCase()
		if (!term) return data
		return data.filter((item) => `${item.name} ${item.url}`.toLowerCase().includes(term))
	}, [data, filter])

	const openCreate = useCallback(() => {
		setFormMode("create")
		setFormId("")
		setName("")
		setUrl("")
		setPort("")
		setToken("")
		setFormOpen(true)
	}, [])

	const openUpdate = useCallback((item: DockerServiceConfigItem) => {
		const parsed = parseServiceUrl(item.url)
		setFormMode("update")
		setFormId(item.id)
		setName(item.name)
		setUrl(parsed.path)
		setPort(parsed.port)
		setToken("")
		setFormOpen(true)
	}, [])

	const handleSubmit = useCallback(async () => {
		if (!systemId) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		if (!systemHost) {
			toast({ variant: "destructive", title: t`Error`, description: t`No system found.` })
			return
		}
		if (!name.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`Name and URL are required` })
			return
		}
		if (!port.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`Port is required` })
			return
		}
		if (!url.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`Path is required` })
			return
		}
		if (formMode === "create" && !token.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`Token is required` })
			return
		}
		const trimmedPath = url.trim()
		const cleanPath = trimmedPath.startsWith("/") ? trimmedPath : `/${trimmedPath}`
		const resolvedUrl = `${ipPrefix}:${port.trim()}${cleanPath}`
		setSubmitLoading(true)
		try {
			if (formMode === "create") {
				await createDockerServiceConfig({
					system: systemId,
					name: name.trim(),
					url: resolvedUrl,
					token: token.trim(),
				})
			} else {
				await updateDockerServiceConfig({
					id: formId,
					name: name.trim(),
					url: resolvedUrl,
				})
			}
			setFormOpen(false)
			await loadConfigs()
		} catch (err) {
			console.error("save service config failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to save service config` })
			throw err
		} finally {
			setSubmitLoading(false)
		}
	}, [formMode, formId, name, url, port, token, systemId, systemHost, ipPrefix, loadConfigs])

	const handleDelete = useCallback(async () => {
		if (!deleteTarget) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		try {
			await deleteDockerServiceConfig(deleteTarget.id)
			setDeleteTarget(null)
			if (activeConfig?.id === deleteTarget.id) {
				setActiveConfig(null)
				setContent("")
			}
			await loadConfigs()
		} catch (err) {
			console.error("delete service config failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to delete service config` })
			throw err
		}
	}, [deleteTarget, activeConfig, loadConfigs])

	const openDetails = useCallback(
		(item: DockerServiceConfigItem) => {
			setActiveConfig(item)
			setContent("")
			void loadContent(item.id)
		},
		[loadContent]
	)

	const handleSaveContent = useCallback(async () => {
		if (!systemId || !activeConfig) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		if (!content.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`Config content is required` })
			return
		}
		setContentSaving(true)
		try {
			await updateDockerServiceConfigContent({ system: systemId, id: activeConfig.id, content })
			toast({
				title: t`Operation success`,
				description: t`Config saved. Restart the service to apply changes.`,
			})
			await loadContent(activeConfig.id)
		} catch (err) {
			console.error("update service config content failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to update config content` })
			throw err
		} finally {
			setContentSaving(false)
		}
	}, [systemId, activeConfig, content, loadContent])

	if (!systemId) {
		return <DockerEmptyState />
	}

	if (activeConfig) {
		return (
			<div className="space-y-4">
				<div className="flex flex-wrap items-end justify-between gap-3">
					<div>
						<h2 className="text-lg font-semibold">
							<Trans>Service Config</Trans>
						</h2>
						<p className="text-sm text-muted-foreground">
							<Trans>Review and update config content for the selected service.</Trans>
						</p>
					</div>
					<div className="flex flex-wrap items-center gap-2">
						<Button variant="outline" size="sm" onClick={() => setActiveConfig(null)}>
							<ChevronLeftIcon className="me-2 h-4 w-4" />
							<Trans>Back</Trans>
						</Button>
						<Button
							variant="outline"
							size="sm"
							onClick={() => void loadContent(activeConfig.id)}
							disabled={contentLoading}
						>
							{contentLoading ? (
								<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
							) : (
								<RefreshCwIcon className="me-2 h-4 w-4" />
							)}
							<Trans>Refresh</Trans>
						</Button>
					</div>
				</div>
				<Card>
					<CardHeader className="space-y-1">
						<CardTitle className="text-base">{activeConfig.name}</CardTitle>
						<CardDescription className="break-all">{activeConfig.url}</CardDescription>
					</CardHeader>
					<CardContent className="space-y-4">
						<div className="grid gap-2">
							<Label htmlFor="service-config-content">
								<Trans>Config Content</Trans>
							</Label>
							<Textarea
								id="service-config-content"
								rows={14}
								value={content}
								onChange={(event) => setContent(event.target.value)}
								placeholder={t`Paste YAML content`}
								className="font-mono whitespace-pre"
							/>
						</div>
						<div className="flex flex-wrap items-center justify-between gap-3 text-sm text-muted-foreground">
							<Trans>Changes take effect after restarting the service.</Trans>
							<Button onClick={() => void handleSaveContent()} disabled={contentSaving}>
								{contentSaving ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : null}
								<Trans>Save</Trans>
							</Button>
						</div>
					</CardContent>
				</Card>
			</div>
		)
	}

	return (
		<div className="space-y-4">
			<div className="flex flex-wrap items-end justify-between gap-3">
				<div>
					<h2 className="text-lg font-semibold">
						<Trans>Service Configs</Trans>
					</h2>
					<p className="text-sm text-muted-foreground">
						<Trans>Maintain service configuration endpoints for the selected system.</Trans>
					</p>
				</div>
				<div className="flex flex-wrap items-center gap-3">
					<Button onClick={openCreate}>
						<Trans>Create Service</Trans>
					</Button>
					<Input
						className="w-56"
						placeholder={t`Filter services...`}
						value={filter}
						onChange={(event) => setFilter(event.target.value)}
					/>
					<Button variant="outline" size="sm" onClick={() => void loadConfigs()} disabled={loading}>
						{loading ? (
							<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
						) : (
							<RefreshCwIcon className="me-2 h-4 w-4" />
						)}
						<Trans>Refresh</Trans>
					</Button>
				</div>
			</div>
			{filtered.length === 0 ? (
				<Card>
					<CardContent className="py-10 text-center text-sm text-muted-foreground">
						<Trans>No service configs yet.</Trans>
					</CardContent>
				</Card>
			) : (
				<div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
					{filtered.map((item) => (
						<Card
							key={item.id}
							role="button"
							tabIndex={0}
							onClick={() => openDetails(item)}
							onKeyDown={(event) => {
								if (event.key === "Enter" || event.key === " ") {
									event.preventDefault()
									openDetails(item)
								}
							}}
							className="cursor-pointer transition hover:border-primary/60"
						>
							<CardHeader className="space-y-2">
								<div className="flex items-start justify-between gap-2">
									<CardTitle className="text-base">{item.name}</CardTitle>
									<DropdownMenu>
										<DropdownMenuTrigger asChild>
											<Button
												variant="ghost"
												size="icon"
												className="h-8 w-8"
												onClick={(event) => event.stopPropagation()}
											>
												<MoreHorizontalIcon className="h-4 w-4" />
											</Button>
										</DropdownMenuTrigger>
										<DropdownMenuContent align="end" onClick={(event) => event.stopPropagation()}>
											<DropdownMenuItem onClick={() => openUpdate(item)}>
												<Trans>Edit</Trans>
											</DropdownMenuItem>
											<DropdownMenuItem onClick={() => setDeleteTarget(item)}>
												<Trans>Delete</Trans>
											</DropdownMenuItem>
										</DropdownMenuContent>
									</DropdownMenu>
								</div>
								<CardDescription className="break-all">{item.url}</CardDescription>
							</CardHeader>
							<CardContent className="text-xs text-muted-foreground">
								<Trans>Updated</Trans>: {item.updated ? formatShortDate(item.updated) : "-"}
							</CardContent>
						</Card>
					))}
				</div>
			)}

			<Dialog open={formOpen} onOpenChange={setFormOpen}>
				<DialogContent className="w-[95vw] max-w-lg">
					<DialogHeader>
						<DialogTitle>
							{formMode === "create" ? <Trans>Create Service</Trans> : <Trans>Edit Service</Trans>}
						</DialogTitle>
						<DialogDescription>
							{formMode === "create" ? (
								<Trans>Register a configuration endpoint for this system.</Trans>
							) : (
								<Trans>Update service name or endpoint URL.</Trans>
							)}
						</DialogDescription>
					</DialogHeader>
					<div className="grid gap-4">
						<div className="grid gap-2">
							<Label htmlFor="service-config-name">
								<Trans>Service Name</Trans>
							</Label>
							<Input
								id="service-config-name"
								value={name}
								onChange={(event) => setName(event.target.value)}
								placeholder={t`Service name`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="service-config-url">
								<Trans>Endpoint URL</Trans>
							</Label>
							<div className="grid gap-2">
								<div className="flex gap-2">
									<div className="flex h-10 items-center rounded-md border border-input bg-muted px-3 text-sm text-muted-foreground whitespace-nowrap">
										{ipPrefix}
									</div>
									<Input
										placeholder={t`Port`}
										value={port}
										onChange={(event) => setPort(event.target.value)}
										type="number"
										className="w-24 flex-1"
									/>
								</div>
								<Input
									id="service-config-url"
									value={url}
									onChange={(event) => setUrl(event.target.value)}
									placeholder={t`Path (e.g. /api/v1/config)`}
								/>
							</div>
						</div>
						{formMode === "create" ? (
							<div className="grid gap-2">
								<Label htmlFor="service-config-token">
									<Trans>X-Config-Token</Trans>
								</Label>
								<Input
									id="service-config-token"
									type="password"
									value={token}
									onChange={(event) => setToken(event.target.value)}
									placeholder={t`Token for config API`}
								/>
								<p className="text-xs text-muted-foreground">
									<Trans>Token is stored securely and cannot be edited later.</Trans>
								</p>
							</div>
						) : (
							<p className="text-xs text-muted-foreground">
								<Trans>Token cannot be changed after creation.</Trans>
							</p>
						)}
						<Button onClick={() => void handleSubmit()} disabled={submitLoading}>
							{submitLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : null}
							{formMode === "create" ? <Trans>Create</Trans> : <Trans>Save</Trans>}
						</Button>
					</div>
				</DialogContent>
			</Dialog>

			<AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>
							<Trans>Delete service config?</Trans>
						</AlertDialogTitle>
						<AlertDialogDescription>
							<Trans>This action cannot be undone.</Trans>
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>
							<Trans>Cancel</Trans>
						</AlertDialogCancel>
						<AlertDialogAction onClick={() => void handleDelete()}>
							<Trans>Delete</Trans>
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	)
})

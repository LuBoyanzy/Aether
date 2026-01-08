/**
 * Docker 编排项目管理面板。
 * 支持查看、创建、更新与执行编排操作。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
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
import { Switch } from "@/components/ui/switch"
import { toast } from "@/components/ui/use-toast"
import {
	createDockerComposeProject,
	deleteDockerComposeProject,
	listDockerComposeProjects,
	operateDockerComposeProject,
	updateDockerComposeProject,
} from "@/lib/docker"
import { isReadOnlyUser } from "@/lib/api"
import type { DockerComposeProject } from "@/types"
import { formatTagList } from "@/components/docker/utils"
import DockerEmptyState from "@/components/docker/empty-state"
import { Badge } from "@/components/ui/badge"
import { LoaderCircleIcon, MoreHorizontalIcon, RefreshCwIcon } from "lucide-react"

const statusBadgeMap: Record<string, "success" | "warning" | "secondary"> = {
	running: "success",
	partial: "warning",
	stopped: "secondary",
}

export default memo(function DockerComposePanel({ systemId }: { systemId?: string }) {
	const [loading, setLoading] = useState(true)
	const [data, setData] = useState<DockerComposeProject[]>([])
	const [filter, setFilter] = useState("")
	const [formOpen, setFormOpen] = useState(false)
	const [formMode, setFormMode] = useState<"create" | "update">("create")
	const [formName, setFormName] = useState("")
	const [formContent, setFormContent] = useState("")
	const [formEnv, setFormEnv] = useState("")
	const [submitLoading, setSubmitLoading] = useState(false)
	const [logOpen, setLogOpen] = useState(false)
	const [logContent, setLogContent] = useState("")
	const [deleteTarget, setDeleteTarget] = useState<DockerComposeProject | null>(null)
	const [removeFiles, setRemoveFiles] = useState(false)

	const loadProjects = useCallback(async () => {
		if (!systemId) return
		setLoading(true)
		try {
			const items = await listDockerComposeProjects(systemId)
			setData(items)
		} catch (err) {
			console.error("load compose projects failed", err)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to load compose projects`,
			})
			throw err
		} finally {
			setLoading(false)
		}
	}, [systemId])

	useEffect(() => {
		if (systemId) {
			void loadProjects()
		}
	}, [systemId, loadProjects])

	const filtered = useMemo(() => {
		const term = filter.trim().toLowerCase()
		if (!term) return data
		return data.filter((item) => {
			return [item.name, item.status, formatTagList(item.services), item.workdir]
				.filter(Boolean)
				.join(" ")
				.toLowerCase()
				.includes(term)
		})
	}, [data, filter])

	const openCreate = useCallback(() => {
		setFormMode("create")
		setFormName("")
		setFormContent("")
		setFormEnv("")
		setFormOpen(true)
	}, [])

	const openUpdate = useCallback((item: DockerComposeProject) => {
		setFormMode("update")
		setFormName(item.name)
		setFormContent("")
		setFormEnv("")
		setFormOpen(true)
	}, [])

	const handleSubmit = useCallback(async () => {
		if (!systemId) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		if (!formName.trim() || !formContent.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`Name and content are required` })
			return
		}
		setSubmitLoading(true)
		try {
			const payload = {
				system: systemId,
				name: formName.trim(),
				content: formContent,
				env: formEnv || undefined,
			}
			const res =
				formMode === "create" ? await createDockerComposeProject(payload) : await updateDockerComposeProject(payload)
			setLogContent(res.logs || "")
			setLogOpen(true)
			setFormOpen(false)
			await loadProjects()
		} catch (err) {
			console.error("save compose project failed", err)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to save compose project`,
			})
			throw err
		} finally {
			setSubmitLoading(false)
		}
	}, [systemId, formMode, formName, formContent, formEnv, loadProjects])

	const handleOperate = useCallback(
		async (item: DockerComposeProject, operation: string) => {
			if (!systemId) return
			if (isReadOnlyUser()) {
				toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
				return
			}
			try {
				const res = await operateDockerComposeProject({
					system: systemId,
					name: item.name,
					operation,
				})
				setLogContent(res.logs || "")
				setLogOpen(true)
				await loadProjects()
			} catch (err) {
				console.error("operate compose project failed", err)
				toast({
					variant: "destructive",
					title: t`Error`,
					description: t`Failed to operate compose project`,
				})
				throw err
			}
		},
		[systemId, loadProjects]
	)

	const handleDelete = useCallback(async () => {
		if (!systemId || !deleteTarget) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		try {
			const res = await deleteDockerComposeProject({
				system: systemId,
				name: deleteTarget.name,
				removeFile: removeFiles,
			})
			setLogContent(res.logs || "")
			setLogOpen(true)
			setDeleteTarget(null)
			setRemoveFiles(false)
			await loadProjects()
		} catch (err) {
			console.error("delete compose project failed", err)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to delete compose project`,
			})
			throw err
		}
	}, [systemId, deleteTarget, removeFiles, loadProjects])

	if (!systemId) {
		return <DockerEmptyState />
	}

	return (
		<div className="space-y-4">
			<div className="flex flex-wrap items-end justify-between gap-3">
				<div>
					<h2 className="text-lg font-semibold">
						<Trans>Compose</Trans>
					</h2>
					<p className="text-sm text-muted-foreground">
						<Trans>Manage Docker Compose projects for the selected system.</Trans>
					</p>
				</div>
				<div className="flex flex-wrap items-center gap-3">
					<Button onClick={openCreate}>
						<Trans>Create Compose</Trans>
					</Button>
					<Input
						className="w-56"
						placeholder={t`Filter projects...`}
						value={filter}
						onChange={(event) => setFilter(event.target.value)}
					/>
					<Button variant="outline" size="sm" onClick={() => void loadProjects()} disabled={loading}>
						{loading ? (
							<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
						) : (
							<RefreshCwIcon className="me-2 h-4 w-4" />
						)}
						<Trans>Refresh</Trans>
					</Button>
				</div>
			</div>
			<div className="h-min max-h-[calc(100dvh-24rem)] max-w-full relative overflow-auto border rounded-md bg-card">
				<table className="w-full caption-bottom text-sm">
					<TableHeader className="sticky top-0 z-10 bg-card">
						<TableRow>
							<TableHead className="max-w-[100px]">
								<Trans>Name</Trans>
							</TableHead>
							<TableHead>
								<Trans>Status</Trans>
							</TableHead>
							<TableHead>
								<Trans>Containers</Trans>
							</TableHead>
							<TableHead>
								<Trans>Services</Trans>
							</TableHead>
							<TableHead>
								<Trans>Workdir</Trans>
							</TableHead>
							<TableHead className="text-center">
								<Trans>Actions</Trans>
							</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody
						className={
							loading
								? "opacity-50 pointer-events-none transition-opacity duration-300"
								: "opacity-100 transition-opacity duration-500"
						}
					>
						{loading && data.length === 0 ? (
							<TableRow>
								<TableCell colSpan={6} className="h-24 text-center">
									<div className="flex items-center justify-center gap-2 text-muted-foreground">
										<LoaderCircleIcon className="h-4 w-4 animate-spin" />
										<Trans>Loading...</Trans>
									</div>
								</TableCell>
							</TableRow>
						) : filtered.length === 0 ? (
							<TableRow>
								<TableCell colSpan={6} className="text-center text-sm text-muted-foreground">
									<Trans>No compose projects found.</Trans>
								</TableCell>
							</TableRow>
						) : (
							filtered.map((item) => (
								<TableRow key={item.name}>
									<TableCell className="max-w-[100px] py-3">
										<div className="font-medium text-foreground truncate" title={item.name}>
											{item.name}
										</div>
										{item.workdir && (
											<div className="text-xs text-muted-foreground truncate" title={item.workdir}>
												{item.workdir}
											</div>
										)}
										{item.configFiles?.length ? (
											<div className="text-xs text-muted-foreground truncate" title={formatTagList(item.configFiles)}>
												{formatTagList(item.configFiles)}
											</div>
										) : null}
									</TableCell>
									<TableCell className="p-2">
										<Badge variant={statusBadgeMap[item.status] ?? "secondary"} className="shadow-none">
											{item.status || "-"}
										</Badge>
									</TableCell>
									<TableCell className="text-xs text-muted-foreground py-3">
										{item.runningCount}/{item.containerCount}
									</TableCell>
									<TableCell className="max-w-[200px] text-xs text-muted-foreground py-3">
										<div className="truncate" title={formatTagList(item.services)}>
											{formatTagList(item.services)}
										</div>
									</TableCell>
									<TableCell className="max-w-[150px] text-xs text-muted-foreground py-3">
										<div className="truncate" title={item.workdir}>
											{item.workdir || "-"}
										</div>
									</TableCell>
									<TableCell className="text-right py-3">
										<DropdownMenu>
											<DropdownMenuTrigger asChild>
												<Button size="icon" variant="ghost">
													<MoreHorizontalIcon className="h-4 w-4" />
												</Button>
											</DropdownMenuTrigger>
											<DropdownMenuContent align="end">
												<DropdownMenuItem onSelect={() => void handleOperate(item, "up")}>
													<Trans>Up</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => void handleOperate(item, "down")}>
													<Trans>Down</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => void handleOperate(item, "start")}>
													<Trans>Start</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => void handleOperate(item, "stop")}>
													<Trans>Stop</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => void handleOperate(item, "restart")}>
													<Trans>Restart</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => void handleOperate(item, "pull")}>
													<Trans>Pull</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => openUpdate(item)}>
													<Trans>Update</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem
													onSelect={() => {
														setDeleteTarget(item)
													}}
												>
													<Trans>Delete</Trans>
												</DropdownMenuItem>
											</DropdownMenuContent>
										</DropdownMenu>
									</TableCell>
								</TableRow>
							))
						)}
					</TableBody>
				</table>
			</div>
			<Dialog open={formOpen} onOpenChange={setFormOpen}>
				<DialogContent className="max-w-3xl">
					<DialogHeader>
						<DialogTitle>
							{formMode === "create" ? <Trans>Create Compose</Trans> : <Trans>Update Compose</Trans>}
						</DialogTitle>
						<DialogDescription>
							<Trans>Provide a full docker-compose.yml content for deployment.</Trans>
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-4">
						<div className="grid gap-2">
							<Label htmlFor="compose-name">
								<Trans>Name</Trans>
							</Label>
							<Input
								id="compose-name"
								value={formName}
								onChange={(event) => setFormName(event.target.value)}
								disabled={formMode === "update"}
								placeholder={t`Compose project name`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="compose-content">
								<Trans>Compose Content</Trans>
							</Label>
							<Textarea
								id="compose-content"
								rows={10}
								value={formContent}
								onChange={(event) => setFormContent(event.target.value)}
								placeholder={t`Paste docker-compose.yml content`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="compose-env">
								<Trans>Environment</Trans>
							</Label>
							<Textarea
								id="compose-env"
								rows={4}
								value={formEnv}
								onChange={(event) => setFormEnv(event.target.value)}
								placeholder={t`Optional .env content`}
							/>
						</div>
						<div className="flex justify-end">
							<Button onClick={() => void handleSubmit()} disabled={submitLoading}>
								{submitLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : null}
								{formMode === "create" ? <Trans>Create</Trans> : <Trans>Save</Trans>}
							</Button>
						</div>
					</div>
				</DialogContent>
			</Dialog>
			<Dialog open={logOpen} onOpenChange={setLogOpen}>
				<DialogContent className="max-w-3xl">
					<DialogHeader>
						<DialogTitle>
							<Trans>Compose Logs</Trans>
						</DialogTitle>
						<DialogDescription>{logContent ? <Trans>Latest output from compose.</Trans> : "-"}</DialogDescription>
					</DialogHeader>
					<div className="max-h-[60vh] overflow-auto rounded-md border bg-muted/30 p-3 text-xs font-mono">
						<pre className="whitespace-pre-wrap break-words">{logContent || "-"}</pre>
					</div>
				</DialogContent>
			</Dialog>
			<AlertDialog
				open={!!deleteTarget}
				onOpenChange={(open) => {
					if (!open) {
						setDeleteTarget(null)
						setRemoveFiles(false)
					}
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>
							<Trans>Delete Compose Project</Trans>
						</AlertDialogTitle>
						<AlertDialogDescription>
							<Trans>This will stop and remove the selected compose project.</Trans>
						</AlertDialogDescription>
					</AlertDialogHeader>
					<div className="flex items-center gap-2">
						<Switch id="compose-remove-files" checked={removeFiles} onCheckedChange={setRemoveFiles} />
						<Label htmlFor="compose-remove-files">
							<Trans>Remove project files</Trans>
						</Label>
					</div>
					<AlertDialogFooter>
						<AlertDialogCancel>
							<Trans>Cancel</Trans>
						</AlertDialogCancel>
						<AlertDialogAction onClick={() => void handleDelete()}>
							<Trans>Continue</Trans>
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>
		</div>
	)
})

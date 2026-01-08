/**
 * Docker 仓库凭据管理面板。
 * 支持仓库列表展示与新增/更新/删除。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
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
import { toast } from "@/components/ui/use-toast"
import { createDockerRegistry, deleteDockerRegistry, listDockerRegistries, updateDockerRegistry } from "@/lib/docker"
import { isReadOnlyUser } from "@/lib/api"
import type { DockerRegistryItem } from "@/types"
import { LoaderCircleIcon, MoreHorizontalIcon, RefreshCwIcon } from "lucide-react"

export default memo(function DockerRegistriesPanel() {
	const [loading, setLoading] = useState(true)
	const [data, setData] = useState<DockerRegistryItem[]>([])
	const [filter, setFilter] = useState("")
	const [formOpen, setFormOpen] = useState(false)
	const [formMode, setFormMode] = useState<"create" | "update">("create")
	const [formId, setFormId] = useState("")
	const [name, setName] = useState("")
	const [server, setServer] = useState("")
	const [username, setUsername] = useState("")
	const [password, setPassword] = useState("")
	const [submitLoading, setSubmitLoading] = useState(false)
	const [deleteTarget, setDeleteTarget] = useState<DockerRegistryItem | null>(null)

	const loadRegistries = useCallback(async () => {
		setLoading(true)
		try {
			const res = await listDockerRegistries()
			setData(res.items || [])
		} catch (err) {
			console.error("load registries failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load registries` })
			throw err
		} finally {
			setLoading(false)
		}
	}, [])

	useEffect(() => {
		void loadRegistries()
	}, [loadRegistries])

	const filtered = useMemo(() => {
		const term = filter.trim().toLowerCase()
		if (!term) return data
		return data.filter((item) => {
			return [item.name, item.server, item.username].join(" ").toLowerCase().includes(term)
		})
	}, [data, filter])

	const openCreate = useCallback(() => {
		setFormMode("create")
		setFormId("")
		setName("")
		setServer("")
		setUsername("")
		setPassword("")
		setFormOpen(true)
	}, [])

	const openUpdate = useCallback((item: DockerRegistryItem) => {
		setFormMode("update")
		setFormId(item.id)
		setName(item.name)
		setServer(item.server)
		setUsername(item.username)
		setPassword("")
		setFormOpen(true)
	}, [])

	const handleSubmit = useCallback(async () => {
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		if (!name.trim() || !server.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`Name and server are required` })
			return
		}
		setSubmitLoading(true)
		try {
			if (formMode === "create") {
				await createDockerRegistry({
					name: name.trim(),
					server: server.trim(),
					username: username.trim() || undefined,
					password: password || undefined,
				})
			} else {
				const payload: { id: string; name?: string; server?: string; username?: string; password?: string } = {
					id: formId,
					name: name.trim(),
					server: server.trim(),
					username: username.trim() || undefined,
				}
				if (password) {
					payload.password = password
				}
				await updateDockerRegistry(payload)
			}
			setFormOpen(false)
			await loadRegistries()
		} catch (err) {
			console.error("save registry failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to save registry` })
			throw err
		} finally {
			setSubmitLoading(false)
		}
	}, [formMode, formId, name, server, username, password, loadRegistries])

	const handleDelete = useCallback(async () => {
		if (!deleteTarget) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		try {
			await deleteDockerRegistry(deleteTarget.id)
			setDeleteTarget(null)
			await loadRegistries()
		} catch (err) {
			console.error("delete registry failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to delete registry` })
			throw err
		}
	}, [deleteTarget, loadRegistries])

	return (
		<div className="space-y-4">
			<div className="flex flex-wrap items-end justify-between gap-3">
				<div>
					<h2 className="text-lg font-semibold">
						<Trans>Registries</Trans>
					</h2>
					<p className="text-sm text-muted-foreground">
						<Trans>Manage registry credentials for image pull and push.</Trans>
					</p>
				</div>
				<div className="flex flex-wrap items-center gap-3">
					<Button onClick={openCreate}>
						<Trans>Create Registry</Trans>
					</Button>
					<Input
						className="w-56"
						placeholder={t`Filter registries...`}
						value={filter}
						onChange={(event) => setFilter(event.target.value)}
					/>
					<Button variant="outline" size="sm" onClick={() => void loadRegistries()} disabled={loading}>
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
								<Trans>Server</Trans>
							</TableHead>
							<TableHead>
								<Trans>Username</Trans>
							</TableHead>
							<TableHead>
								<Trans>Created</Trans>
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
								<TableCell colSpan={5} className="h-24 text-center">
									<div className="flex items-center justify-center gap-2 text-muted-foreground">
										<LoaderCircleIcon className="h-4 w-4 animate-spin" />
										<Trans>Loading...</Trans>
									</div>
								</TableCell>
							</TableRow>
						) : filtered.length === 0 ? (
							<TableRow>
								<TableCell colSpan={5} className="text-center text-sm text-muted-foreground">
									<Trans>No registries found.</Trans>
								</TableCell>
							</TableRow>
						) : (
							filtered.map((item) => (
								<TableRow key={item.id} className="group">
									<TableCell className="max-w-[100px] py-3">
										<div className="font-medium text-foreground truncate" title={item.name}>
											{item.name}
										</div>
									</TableCell>
									<TableCell className="text-xs text-muted-foreground">{item.server}</TableCell>
									<TableCell className="text-xs text-muted-foreground">{item.username || "-"}</TableCell>
									<TableCell className="text-xs text-muted-foreground">{item.created}</TableCell>
									<TableCell className="text-center">
										<DropdownMenu>
											<DropdownMenuTrigger asChild>
												<Button size="icon" variant="ghost">
													<MoreHorizontalIcon className="h-4 w-4" />
												</Button>
											</DropdownMenuTrigger>
											<DropdownMenuContent align="end">
												<DropdownMenuItem onSelect={() => openUpdate(item)}>
													<Trans>Edit</Trans>
												</DropdownMenuItem>
												<DropdownMenuItem onSelect={() => setDeleteTarget(item)}>
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
				<DialogContent className="max-w-xl">
					<DialogHeader>
						<DialogTitle>
							{formMode === "create" ? <Trans>Create Registry</Trans> : <Trans>Edit Registry</Trans>}
						</DialogTitle>
						<DialogDescription>
							<Trans>Store credentials for private registries.</Trans>
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-4">
						<div className="grid gap-2">
							<Label htmlFor="registry-name">
								<Trans>Name</Trans>
							</Label>
							<Input
								id="registry-name"
								value={name}
								onChange={(event) => setName(event.target.value)}
								placeholder={t`Registry name`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="registry-server">
								<Trans>Server</Trans>
							</Label>
							<Input
								id="registry-server"
								value={server}
								onChange={(event) => setServer(event.target.value)}
								placeholder={t`Registry server`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="registry-username">
								<Trans>Username</Trans>
							</Label>
							<Input
								id="registry-username"
								value={username}
								onChange={(event) => setUsername(event.target.value)}
								placeholder={t`Registry username`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="registry-password">
								<Trans>Password</Trans>
							</Label>
							<Input
								id="registry-password"
								type="password"
								value={password}
								onChange={(event) => setPassword(event.target.value)}
								placeholder={t`Leave blank to keep unchanged`}
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
			<AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>
							<Trans>Delete Registry</Trans>
						</AlertDialogTitle>
						<AlertDialogDescription>
							<Trans>This will remove the selected registry credentials.</Trans>
						</AlertDialogDescription>
					</AlertDialogHeader>
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

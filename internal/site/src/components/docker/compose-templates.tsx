/**
 * Docker 编排模板管理面板。
 * 支持模板列表展示与新增/更新/删除。
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
import { toast } from "@/components/ui/use-toast"
import {
	createDockerComposeTemplate,
	deleteDockerComposeTemplate,
	listDockerComposeTemplates,
	updateDockerComposeTemplate,
} from "@/lib/docker"
import { isReadOnlyUser } from "@/lib/api"
import type { DockerComposeTemplateItem } from "@/types"
import { LoaderCircleIcon, MoreHorizontalIcon, RefreshCwIcon } from "lucide-react"

export default memo(function DockerComposeTemplatesPanel() {
	const [loading, setLoading] = useState(true)
	const [data, setData] = useState<DockerComposeTemplateItem[]>([])
	const [filter, setFilter] = useState("")
	const [formOpen, setFormOpen] = useState(false)
	const [formMode, setFormMode] = useState<"create" | "update">("create")
	const [formId, setFormId] = useState("")
	const [name, setName] = useState("")
	const [description, setDescription] = useState("")
	const [content, setContent] = useState("")
	const [env, setEnv] = useState("")
	const [submitLoading, setSubmitLoading] = useState(false)
	const [deleteTarget, setDeleteTarget] = useState<DockerComposeTemplateItem | null>(null)

	const loadTemplates = useCallback(async () => {
		setLoading(true)
		try {
			const res = await listDockerComposeTemplates()
			setData(res.items || [])
		} catch (err) {
			console.error("load compose templates failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load compose templates` })
			throw err
		} finally {
			setLoading(false)
		}
	}, [])

	useEffect(() => {
		void loadTemplates()
	}, [loadTemplates])

	const filtered = useMemo(() => {
		const term = filter.trim().toLowerCase()
		if (!term) return data
		return data.filter((item) => {
			return [item.name, item.description].join(" ").toLowerCase().includes(term)
		})
	}, [data, filter])

	const openCreate = useCallback(() => {
		setFormMode("create")
		setFormId("")
		setName("")
		setDescription("")
		setContent("")
		setEnv("")
		setFormOpen(true)
	}, [])

	const openUpdate = useCallback((item: DockerComposeTemplateItem) => {
		setFormMode("update")
		setFormId(item.id)
		setName(item.name)
		setDescription(item.description || "")
		setContent(item.content || "")
		setEnv(item.env || "")
		setFormOpen(true)
	}, [])

	const handleSubmit = useCallback(async () => {
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		if (!name.trim() || !content.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`Name and content are required` })
			return
		}
		setSubmitLoading(true)
		try {
			if (formMode === "create") {
				await createDockerComposeTemplate({
					name: name.trim(),
					description: description.trim() || undefined,
					content,
					env: env || undefined,
				})
			} else {
				await updateDockerComposeTemplate({
					id: formId,
					name: name.trim(),
					description: description.trim() || undefined,
					content,
					env: env || undefined,
				})
			}
			setFormOpen(false)
			await loadTemplates()
		} catch (err) {
			console.error("save compose template failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to save compose template` })
			throw err
		} finally {
			setSubmitLoading(false)
		}
	}, [formMode, formId, name, description, content, env, loadTemplates])

	const handleDelete = useCallback(async () => {
		if (!deleteTarget) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		try {
			await deleteDockerComposeTemplate(deleteTarget.id)
			setDeleteTarget(null)
			await loadTemplates()
		} catch (err) {
			console.error("delete compose template failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to delete compose template` })
			throw err
		}
	}, [deleteTarget, loadTemplates])

	return (
		<div className="space-y-4">
			<div className="flex flex-wrap items-end justify-between gap-3">
				<div>
					<h2 className="text-lg font-semibold">
						<Trans>Compose Templates</Trans>
					</h2>
					<p className="text-sm text-muted-foreground">
						<Trans>Maintain reusable Docker Compose templates.</Trans>
					</p>
				</div>
				<div className="flex flex-wrap items-center gap-3">
					<Button onClick={openCreate}>
						<Trans>Create Template</Trans>
					</Button>
					<Input
						className="w-56"
						placeholder={t`Filter templates...`}
						value={filter}
						onChange={(event) => setFilter(event.target.value)}
					/>
					<Button variant="outline" size="sm" onClick={() => void loadTemplates()} disabled={loading}>
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
								<Trans>Description</Trans>
							</TableHead>
							<TableHead>
								<Trans>Updated</Trans>
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
								<TableCell colSpan={4} className="h-24 text-center">
									<div className="flex items-center justify-center gap-2 text-muted-foreground">
										<LoaderCircleIcon className="h-4 w-4 animate-spin" />
										<Trans>Loading...</Trans>
									</div>
								</TableCell>
							</TableRow>
						) : filtered.length === 0 ? (
							<TableRow>
								<TableCell colSpan={4} className="text-center text-sm text-muted-foreground">
									<Trans>No templates found.</Trans>
								</TableCell>
							</TableRow>
						) : (
							filtered.map((item) => (
								<TableRow key={item.id}>
									<TableCell className="max-w-[100px] py-3">
										<div className="truncate font-medium text-foreground" title={item.name}>
											{item.name}
										</div>
									</TableCell>
									<TableCell className="max-w-[300px] text-xs text-muted-foreground py-3">
										<div className="truncate" title={item.description}>
											{item.description || "-"}
										</div>
									</TableCell>
									<TableCell className="text-xs text-muted-foreground whitespace-nowrap py-3">{item.updated}</TableCell>
									<TableCell className="text-center py-3">
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
				<DialogContent className="max-w-3xl">
					<DialogHeader>
						<DialogTitle>
							{formMode === "create" ? <Trans>Create Template</Trans> : <Trans>Edit Template</Trans>}
						</DialogTitle>
						<DialogDescription>
							<Trans>Compose templates can be reused to deploy stacks quickly.</Trans>
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-4">
						<div className="grid gap-2">
							<Label htmlFor="compose-template-name">
								<Trans>Name</Trans>
							</Label>
							<Input
								id="compose-template-name"
								value={name}
								onChange={(event) => setName(event.target.value)}
								placeholder={t`Template name`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="compose-template-description">
								<Trans>Description</Trans>
							</Label>
							<Input
								id="compose-template-description"
								value={description}
								onChange={(event) => setDescription(event.target.value)}
								placeholder={t`Optional description`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="compose-template-content">
								<Trans>Compose Content</Trans>
							</Label>
							<Textarea
								id="compose-template-content"
								rows={10}
								value={content}
								onChange={(event) => setContent(event.target.value)}
								placeholder={t`Paste docker-compose.yml content`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="compose-template-env">
								<Trans>Environment</Trans>
							</Label>
							<Textarea
								id="compose-template-env"
								rows={4}
								value={env}
								onChange={(event) => setEnv(event.target.value)}
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
			<AlertDialog open={!!deleteTarget} onOpenChange={(open) => !open && setDeleteTarget(null)}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>
							<Trans>Delete Template</Trans>
						</AlertDialogTitle>
						<AlertDialogDescription>
							<Trans>This will remove the selected compose template.</Trans>
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

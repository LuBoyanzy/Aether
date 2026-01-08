/**
 * Docker 镜像管理面板。
 * 提供镜像列表展示与拉取/推送/删除操作。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
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
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { toast } from "@/components/ui/use-toast"
import {
	listDockerImages,
	listDockerRegistries,
	pullDockerImage,
	pushDockerImage,
	removeDockerImage,
} from "@/lib/docker"
import { isReadOnlyUser } from "@/lib/api"
import type { DockerImage, DockerRegistryItem } from "@/types"
import { formatBytesLabel, formatShortId, formatTagList, formatUnixSeconds } from "@/components/docker/utils"
import DockerEmptyState from "@/components/docker/empty-state"
import { LoaderCircleIcon, MoreHorizontalIcon, RefreshCwIcon } from "lucide-react"

export default memo(function DockerImagesPanel({ systemId }: { systemId?: string }) {
	const [loading, setLoading] = useState(true)
	const [images, setImages] = useState<DockerImage[]>([])
	const [filter, setFilter] = useState("")
	const [showAll, setShowAll] = useState(true)
	const [registries, setRegistries] = useState<DockerRegistryItem[]>([])
	const [actionOpen, setActionOpen] = useState(false)
	const [actionMode, setActionMode] = useState<"pull" | "push">("pull")
	const [imageName, setImageName] = useState("")
	const [registryId, setRegistryId] = useState("")
	const [actionLoading, setActionLoading] = useState(false)
	const [logOpen, setLogOpen] = useState(false)
	const [logContent, setLogContent] = useState("")
	const [deleteTarget, setDeleteTarget] = useState<DockerImage | null>(null)
	const [forceDelete, setForceDelete] = useState(false)

	const loadImages = useCallback(async () => {
		if (!systemId) return
		setLoading(true)
		try {
			const items = await listDockerImages(systemId, showAll)
			setImages(items)
		} catch (err) {
			console.error("load docker images failed", err)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to load images`,
			})
			throw err
		} finally {
			setLoading(false)
		}
	}, [systemId, showAll])

	const loadRegistries = useCallback(async () => {
		try {
			const res = await listDockerRegistries()
			setRegistries(res.items || [])
		} catch (err) {
			console.error("load registries failed", err)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to load registries`,
			})
			throw err
		}
	}, [])

	useEffect(() => {
		if (systemId) {
			void loadImages()
		}
	}, [systemId, showAll, loadImages])

	useEffect(() => {
		void loadRegistries()
	}, [loadRegistries])

	const filtered = useMemo(() => {
		const term = filter.trim().toLowerCase()
		if (!term) return images
		return images.filter((item) => {
			return [formatTagList(item.repoTags), item.id, formatTagList(item.repoDigests)]
				.filter(Boolean)
				.join(" ")
				.toLowerCase()
				.includes(term)
		})
	}, [images, filter])

	const openAction = useCallback((mode: "pull" | "push") => {
		setActionMode(mode)
		setImageName("")
		setRegistryId("")
		setActionOpen(true)
	}, [])

	const handleAction = useCallback(async () => {
		if (!systemId) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		if (!imageName.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`Image name is required` })
			return
		}
		setActionLoading(true)
		try {
			const payload = {
				system: systemId,
				image: imageName.trim(),
				registryId: registryId || undefined,
			}
			const res = actionMode === "pull" ? await pullDockerImage(payload) : await pushDockerImage(payload)
			setLogContent(res.logs || "")
			setLogOpen(true)
			setActionOpen(false)
			await loadImages()
		} catch (err) {
			console.error("image action failed", err)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to process image`,
			})
			throw err
		} finally {
			setActionLoading(false)
		}
	}, [systemId, actionMode, imageName, registryId, loadImages])

	const handleDelete = useCallback(async () => {
		if (!systemId || !deleteTarget) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		try {
			await removeDockerImage({ system: systemId, image: deleteTarget.id, force: forceDelete })
			toast({ title: t`Operation success`, description: `${t`Delete`} ${formatShortId(deleteTarget.id)}` })
			setDeleteTarget(null)
			setForceDelete(false)
			await loadImages()
		} catch (err) {
			console.error("remove image failed", err)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to remove image`,
			})
			throw err
		}
	}, [systemId, deleteTarget, forceDelete, loadImages])

	if (!systemId) {
		return <DockerEmptyState />
	}

	return (
		<div className="space-y-4">
			<div className="flex flex-wrap items-end justify-between gap-3">
				<div>
					<h2 className="text-lg font-semibold">
						<Trans>Images</Trans>
					</h2>
					<p className="text-sm text-muted-foreground">
						<Trans>Manage local images and registries.</Trans>
					</p>
				</div>
				<div className="flex flex-wrap items-center gap-3">
					<Button onClick={() => openAction("pull")}>
						<Trans>Pull Image</Trans>
					</Button>
					<Button variant="outline" onClick={() => openAction("push")}>
						<Trans>Push Image</Trans>
					</Button>
					<div className="flex items-center gap-2">
						<Switch id="docker-images-all" checked={showAll} onCheckedChange={setShowAll} />
						<Label htmlFor="docker-images-all">
							<Trans>Show all</Trans>
						</Label>
					</div>
					<Input
						className="w-56"
						placeholder={t`Filter images...`}
						value={filter}
						onChange={(event) => setFilter(event.target.value)}
					/>
					<Button variant="outline" size="sm" onClick={() => void loadImages()} disabled={loading}>
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
								<Trans>Tags</Trans>
							</TableHead>
							<TableHead>
								<Trans>Image ID</Trans>
							</TableHead>
							<TableHead>
								<Trans>Size</Trans>
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
						{loading && images.length === 0 ? (
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
									<Trans>No images found.</Trans>
								</TableCell>
							</TableRow>
						) : (
							filtered.map((item) => (
								<TableRow key={item.id} className="group">
									<TableCell className="max-w-[100px] py-3">
										<div className="font-medium text-foreground truncate" title={formatTagList(item.repoTags)}>
											{formatTagList(item.repoTags)}
										</div>
									</TableCell>
									<TableCell className="text-xs text-muted-foreground py-3">
										<span className="font-mono opacity-80">{formatShortId(item.id)}</span>
									</TableCell>
									<TableCell className="text-xs text-muted-foreground p-2">
										<Badge variant="secondary" className="font-mono text-[10px] shadow-none">
											{formatBytesLabel(item.size)}
										</Badge>
									</TableCell>
									<TableCell className="text-xs text-muted-foreground whitespace-nowrap py-3">
										{formatUnixSeconds(item.created)}
									</TableCell>
									<TableCell className="text-center py-3">
										<DropdownMenu>
											<DropdownMenuTrigger asChild>
												<Button size="icon" variant="ghost">
													<MoreHorizontalIcon className="h-4 w-4" />
												</Button>
											</DropdownMenuTrigger>
											<DropdownMenuContent align="end">
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
			<Dialog open={actionOpen} onOpenChange={setActionOpen}>
				<DialogContent className="max-w-xl">
					<DialogHeader>
						<DialogTitle>{actionMode === "pull" ? <Trans>Pull Image</Trans> : <Trans>Push Image</Trans>}</DialogTitle>
						<DialogDescription>
							<Trans>Select a registry if authentication is required.</Trans>
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-4">
						<div className="grid gap-2">
							<Label htmlFor="docker-image-name">
								<Trans>Image</Trans>
							</Label>
							<Input
								id="docker-image-name"
								value={imageName}
								onChange={(event) => setImageName(event.target.value)}
								placeholder={t`Image name or tag`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="docker-image-registry">
								<Trans>Registry</Trans>
							</Label>
							<Select
								value={registryId || "__none__"}
								onValueChange={(value) => setRegistryId(value === "__none__" ? "" : value)}
							>
								<SelectTrigger id="docker-image-registry">
									<SelectValue placeholder={t`Optional registry`} />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="__none__">
										<Trans>None</Trans>
									</SelectItem>
									{registries.map((registry) => (
										<SelectItem key={registry.id} value={registry.id}>
											{registry.name} ({registry.server})
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</div>
						<div className="flex justify-end">
							<Button onClick={() => void handleAction()} disabled={actionLoading}>
								{actionLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : null}
								{actionMode === "pull" ? <Trans>Pull</Trans> : <Trans>Push</Trans>}
							</Button>
						</div>
					</div>
				</DialogContent>
			</Dialog>
			<Dialog open={logOpen} onOpenChange={setLogOpen}>
				<DialogContent className="max-w-3xl">
					<DialogHeader>
						<DialogTitle>
							<Trans>Image Logs</Trans>
						</DialogTitle>
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
						setForceDelete(false)
					}
				}}
			>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>
							<Trans>Delete Image</Trans>
						</AlertDialogTitle>
						<AlertDialogDescription>
							<Trans>This will remove the selected image from the host.</Trans>
						</AlertDialogDescription>
					</AlertDialogHeader>
					<div className="flex items-center gap-2">
						<Switch id="docker-image-force" checked={forceDelete} onCheckedChange={setForceDelete} />
						<Label htmlFor="docker-image-force">
							<Trans>Force removal</Trans>
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

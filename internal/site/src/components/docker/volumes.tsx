/**
 * Docker 存储卷管理面板。
 * 提供卷列表展示与创建/删除操作。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { Button } from "@/components/ui/button"
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
import { Textarea } from "@/components/ui/textarea"
import { toast } from "@/components/ui/use-toast"
import { createDockerVolume, listDockerVolumes, removeDockerVolume } from "@/lib/docker"
import { isReadOnlyUser } from "@/lib/api"
import type { DockerVolume } from "@/types"
import DockerEmptyState from "@/components/docker/empty-state"
import { LoaderCircleIcon, MoreHorizontalIcon, RefreshCwIcon } from "lucide-react"

function parseMapInput(raw: string, label: string) {
	if (!raw.trim()) return undefined
	try {
		const parsed = JSON.parse(raw)
		if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
			throw new Error("invalid")
		}
		return parsed as Record<string, string>
	} catch (err) {
		throw new Error(`${label} must be a JSON object`)
	}
}

export default memo(function DockerVolumesPanel({ systemId }: { systemId?: string }) {
	const [loading, setLoading] = useState(true)
	const [data, setData] = useState<DockerVolume[]>([])
	const [filter, setFilter] = useState("")
	const [createOpen, setCreateOpen] = useState(false)
	const [name, setName] = useState("")
	const [driver, setDriver] = useState("")
	const [labelsInput, setLabelsInput] = useState("")
	const [optionsInput, setOptionsInput] = useState("")
	const [submitLoading, setSubmitLoading] = useState(false)
	const [deleteTarget, setDeleteTarget] = useState<DockerVolume | null>(null)
	const [forceDelete, setForceDelete] = useState(false)

	const loadVolumes = useCallback(async () => {
		if (!systemId) return
		setLoading(true)
		try {
			const items = await listDockerVolumes(systemId)
			setData(items)
		} catch (err) {
			console.error("load docker volumes failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load volumes` })
			throw err
		} finally {
			setLoading(false)
		}
	}, [systemId])

	useEffect(() => {
		if (systemId) {
			void loadVolumes()
		}
	}, [systemId, loadVolumes])

	const filtered = useMemo(() => {
		const term = filter.trim().toLowerCase()
		if (!term) return data
		return data.filter((item) => {
			return [item.name, item.driver, item.mountpoint].join(" ").toLowerCase().includes(term)
		})
	}, [data, filter])

	const resetForm = () => {
		setName("")
		setDriver("")
		setLabelsInput("")
		setOptionsInput("")
	}

	const handleCreate = useCallback(async () => {
		if (!systemId) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		if (!name.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`Volume name is required` })
			return
		}
		setSubmitLoading(true)
		try {
			const payload = {
				system: systemId,
				name: name.trim(),
				driver: driver.trim() || undefined,
				labels: parseMapInput(labelsInput, "labels"),
				options: parseMapInput(optionsInput, "options"),
			}
			await createDockerVolume(payload)
			setCreateOpen(false)
			resetForm()
			await loadVolumes()
		} catch (err) {
			console.error("create volume failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to create volume` })
			throw err
		} finally {
			setSubmitLoading(false)
		}
	}, [systemId, name, driver, labelsInput, optionsInput, loadVolumes])

	const handleDelete = useCallback(async () => {
		if (!systemId || !deleteTarget) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		try {
			await removeDockerVolume({ system: systemId, name: deleteTarget.name, force: forceDelete })
			setDeleteTarget(null)
			setForceDelete(false)
			await loadVolumes()
		} catch (err) {
			console.error("delete volume failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to delete volume` })
			throw err
		}
	}, [systemId, deleteTarget, forceDelete, loadVolumes])

	if (!systemId) {
		return <DockerEmptyState />
	}

	return (
		<div className="space-y-4">
			<div className="flex flex-wrap items-end justify-between gap-3">
				<div>
					<h2 className="text-lg font-semibold">
						<Trans>Volumes</Trans>
					</h2>
					<p className="text-sm text-muted-foreground">
						<Trans>Manage Docker volumes for persistent storage.</Trans>
					</p>
				</div>
				<div className="flex flex-wrap items-center gap-3">
					<Button onClick={() => setCreateOpen(true)}>
						<Trans>Create Volume</Trans>
					</Button>
					<Input
						className="w-56"
						placeholder={t`Filter volumes...`}
						value={filter}
						onChange={(event) => setFilter(event.target.value)}
					/>
					<Button variant="outline" size="sm" onClick={() => void loadVolumes()} disabled={loading}>
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
								<Trans>Driver</Trans>
							</TableHead>
							<TableHead>
								<Trans>Mountpoint</Trans>
							</TableHead>
							<TableHead>
								<Trans>Created</Trans>
							</TableHead>
							<TableHead>
								<Trans>Mode</Trans>
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
									<Trans>No volumes found.</Trans>
								</TableCell>
							</TableRow>
						) : (
							filtered.map((item) => (
								<TableRow key={item.name} className="group">
									<TableCell className="max-w-[100px] py-3">
										<div className="font-medium text-foreground truncate" title={item.name}>
											{item.name}
										</div>
									</TableCell>
									<TableCell className="text-xs text-muted-foreground">{item.driver}</TableCell>
									<TableCell className="text-xs text-muted-foreground max-w-[260px] truncate">
										{item.mountpoint}
									</TableCell>
									<TableCell className="text-xs text-muted-foreground">{item.createdAt || "-"}</TableCell>
									<TableCell className="text-xs text-muted-foreground">{item.scope}</TableCell>
									<TableCell className="text-center">
										<DropdownMenu>
											<DropdownMenuTrigger asChild>
												<Button size="icon" variant="ghost">
													<MoreHorizontalIcon className="h-4 w-4" />
												</Button>
											</DropdownMenuTrigger>
											<DropdownMenuContent align="end">
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
			<Dialog open={createOpen} onOpenChange={setCreateOpen}>
				<DialogContent className="max-w-xl">
					<DialogHeader>
						<DialogTitle>
							<Trans>Create Volume</Trans>
						</DialogTitle>
						<DialogDescription>
							<Trans>Configure a new Docker volume.</Trans>
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-4">
						<div className="grid gap-2">
							<Label htmlFor="docker-volume-name">
								<Trans>Name</Trans>
							</Label>
							<Input
								id="docker-volume-name"
								value={name}
								onChange={(event) => setName(event.target.value)}
								placeholder={t`Volume name`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="docker-volume-driver">
								<Trans>Driver</Trans>
							</Label>
							<Input
								id="docker-volume-driver"
								value={driver}
								onChange={(event) => setDriver(event.target.value)}
								placeholder={t`Optional driver name`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="docker-volume-labels">
								<Trans>Labels (JSON)</Trans>
							</Label>
							<Textarea
								id="docker-volume-labels"
								rows={3}
								value={labelsInput}
								onChange={(event) => setLabelsInput(event.target.value)}
								placeholder={t`Optional labels as JSON`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="docker-volume-options">
								<Trans>Options (JSON)</Trans>
							</Label>
							<Textarea
								id="docker-volume-options"
								rows={3}
								value={optionsInput}
								onChange={(event) => setOptionsInput(event.target.value)}
								placeholder={t`Optional driver options as JSON`}
							/>
						</div>
						<div className="flex justify-end">
							<Button onClick={() => void handleCreate()} disabled={submitLoading}>
								{submitLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : null}
								<Trans>Create</Trans>
							</Button>
						</div>
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
							<Trans>Delete Volume</Trans>
						</AlertDialogTitle>
						<AlertDialogDescription>
							<Trans>This will remove the selected volume.</Trans>
						</AlertDialogDescription>
					</AlertDialogHeader>
					<div className="flex items-center gap-2">
						<Switch id="docker-volume-force" checked={forceDelete} onCheckedChange={setForceDelete} />
						<Label htmlFor="docker-volume-force">
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

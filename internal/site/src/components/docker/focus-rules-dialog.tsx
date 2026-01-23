/**
 * Docker 关注服务规则管理对话框。
 * 提供系统级规则的新增与删除入口。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useEffect, useRef, useState, type ChangeEvent } from "react"
import { Button } from "@/components/ui/button"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
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
import { createDockerFocusService, deleteDockerFocusService, updateDockerFocusService } from "@/lib/docker-focus"
import type { DockerFocusMatchType, DockerFocusServiceRecord } from "@/types"
import { AlertCircleIcon, LoaderCircleIcon, MoreHorizontalIcon } from "lucide-react"

type FocusRuleImportItem = {
	match_type: DockerFocusMatchType
	value: string
	value2?: string
	description?: string
}

function formatRuleValue(rule: DockerFocusServiceRecord) {
	switch (rule.match_type) {
		case "compose_service":
			return rule.value2 ? `${rule.value} / ${rule.value2}` : rule.value
		case "label":
			return rule.value2 ? `${rule.value}=${rule.value2}` : rule.value
		default:
			return rule.value
	}
}

function buildFocusRuleKey(rule: FocusRuleImportItem): string {
	return `${rule.match_type}::${rule.value}::${rule.value2 ?? ""}`
}

type DockerFocusRulesDialogProps = {
	systemId?: string
	open: boolean
	onOpenChange: (open: boolean) => void
	rules: DockerFocusServiceRecord[]
	loading: boolean
	onReload: () => Promise<void>
}

export default memo(function DockerFocusRulesDialog({
	systemId,
	open,
	onOpenChange,
	rules,
	loading,
	onReload,
}: DockerFocusRulesDialogProps) {
	const [matchType, setMatchType] = useState<DockerFocusMatchType | "">("")
	const [value, setValue] = useState("")
	const [value2, setValue2] = useState("")
	const [description, setDescription] = useState("")
	const [editingRule, setEditingRule] = useState<DockerFocusServiceRecord | null>(null)
	const [saving, setSaving] = useState(false)
	const [importing, setImporting] = useState(false)
	const [deleteTarget, setDeleteTarget] = useState<DockerFocusServiceRecord | null>(null)
	const focusTypeRef = useRef<HTMLButtonElement | null>(null)
	const fileInputRef = useRef<HTMLInputElement | null>(null)
	const readOnly = isReadOnlyUser()
	const matchTypeLabels: Record<DockerFocusMatchType, string> = {
		container_name: t`Container name`,
		image: t`Image`,
		compose_project: t`Compose project`,
		compose_service: t`Compose service`,
		label: t`Label`,
	}
	const isValidMatchType = (value: string): value is DockerFocusMatchType =>
		Object.prototype.hasOwnProperty.call(matchTypeLabels, value)

	const resetForm = useCallback(() => {
		setMatchType("")
		setValue("")
		setValue2("")
		setDescription("")
		setEditingRule(null)
	}, [])

	useEffect(() => {
		if (open) {
			resetForm()
		}
	}, [open, resetForm])

	const isEditing = Boolean(editingRule)
	const showValue2 = matchType === "compose_service" || matchType === "label"
	const valueLabel = (() => {
		switch (matchType) {
			case "container_name":
				return t`Container name`
			case "image":
				return t`Image`
			case "compose_project":
				return t`Compose project`
			case "compose_service":
				return t`Compose project`
			case "label":
				return t`Label key`
			default:
				return t`Value`
		}
	})()
	const value2Label = (() => {
		if (matchType === "compose_service") return t`Service`
		if (matchType === "label") return t`Label value`
		return t`Value`
	})()

	const handleSubmit = useCallback(async () => {
		if (!systemId) return
		if (readOnly) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		if (!matchType) {
			toast({ variant: "destructive", title: t`Error`, description: t`Rule type is required` })
			return
		}
		const trimmedValue = value.trim()
		const trimmedValue2 = value2.trim()
		const trimmedDescription = description.trim()
		if (!trimmedValue) {
			toast({ variant: "destructive", title: t`Error`, description: t`Value is required` })
			return
		}
		if (showValue2 && !trimmedValue2) {
			toast({ variant: "destructive", title: t`Error`, description: t`Secondary value is required` })
			return
		}
		setSaving(true)
		try {
			if (editingRule) {
				await updateDockerFocusService(editingRule.id, {
					match_type: editingRule.match_type,
					value: trimmedValue,
					value2: showValue2 ? trimmedValue2 : undefined,
					description: trimmedDescription ? trimmedDescription : undefined,
				})
			} else {
				await createDockerFocusService({
					system: systemId,
					match_type: matchType,
					value: trimmedValue,
					value2: showValue2 ? trimmedValue2 : undefined,
					description: trimmedDescription ? trimmedDescription : undefined,
				})
			}
			resetForm()
			await onReload()
		} catch (err) {
			console.error("save docker focus rule failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to create focus rule` })
			throw err
		} finally {
			setSaving(false)
		}
	}, [systemId, readOnly, matchType, value, value2, description, showValue2, editingRule, resetForm, onReload])

	const handleEdit = useCallback((rule: DockerFocusServiceRecord) => {
		setEditingRule(rule)
		setMatchType(rule.match_type)
		setValue(rule.value)
		setValue2(rule.value2 || "")
		setDescription(rule.description || "")
	}, [])

	const handleDelete = useCallback(async () => {
		if (!deleteTarget) return
		if (readOnly) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return
		}
		try {
			await deleteDockerFocusService(deleteTarget.id)
			if (editingRule?.id === deleteTarget.id) {
				resetForm()
			}
			setDeleteTarget(null)
			await onReload()
		} catch (err) {
			console.error("delete docker focus rule failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to delete focus rule` })
			throw err
		}
	}, [deleteTarget, editingRule, readOnly, resetForm, onReload])

	const handleExport = useCallback(() => {
		const payload = rules.map((rule) => ({
			match_type: rule.match_type,
			value: rule.value,
			value2: rule.value2 || undefined,
			description: rule.description || undefined,
		}))
		const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" })
		const url = URL.createObjectURL(blob)
		const a = document.createElement("a")
		a.href = url
		a.download = "docker_focus_rules.json"
		a.click()
		URL.revokeObjectURL(url)
	}, [rules])

	const handleImport = useCallback(
		async (event: ChangeEvent<HTMLInputElement>) => {
			const file = event.target.files?.[0]
			if (!file) return
			if (!systemId) {
				toast({ variant: "destructive", title: t`Error`, description: t`Please select a system to continue.` })
				return
			}
			if (readOnly) {
				toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
				return
			}
			setImporting(true)
			try {
				const text = await file.text()
				let payload: unknown
				try {
					payload = JSON.parse(text)
				} catch (err) {
					console.error("parse focus rules import file failed", err)
					const error = new Error("Invalid focus rules JSON file.", { cause: err })
					throw error
				}
				if (!Array.isArray(payload)) {
					throw new Error("Focus rules file must be a JSON array.")
				}
				const existingKeys = new Set(
					rules.map((rule) =>
						buildFocusRuleKey({
							match_type: rule.match_type,
							value: rule.value,
							value2: rule.value2 || undefined,
						})
					)
				)
				const toCreate: FocusRuleImportItem[] = []
				let skipped = 0
				for (let index = 0; index < payload.length; index += 1) {
					const raw = payload[index]
					if (!raw || typeof raw !== "object" || Array.isArray(raw)) {
						console.error("invalid focus rule import item", { index, item: raw })
						throw new Error(`Invalid focus rule at index ${index}: expected an object.`)
					}
					const record = raw as { match_type?: unknown; value?: unknown; value2?: unknown; description?: unknown }
					if (typeof record.match_type !== "string" || !isValidMatchType(record.match_type)) {
						console.error("invalid focus rule import match_type", { index, item: raw })
						throw new Error(`Invalid focus rule at index ${index}: match_type is invalid.`)
					}
					if (typeof record.value !== "string") {
						console.error("invalid focus rule import value", { index, item: raw })
						throw new Error(`Invalid focus rule at index ${index}: value must be a string.`)
					}
					const trimmedValue = record.value.trim()
					if (!trimmedValue) {
						console.error("invalid focus rule import value empty", { index, item: raw })
						throw new Error(`Invalid focus rule at index ${index}: value is required.`)
					}
					const requiresValue2 = record.match_type === "compose_service" || record.match_type === "label"
					if (record.value2 !== undefined && typeof record.value2 !== "string") {
						console.error("invalid focus rule import value2", { index, item: raw })
						throw new Error(`Invalid focus rule at index ${index}: value2 must be a string.`)
					}
					if (record.description !== undefined && typeof record.description !== "string") {
						console.error("invalid focus rule import description", { index, item: raw })
						throw new Error(`Invalid focus rule at index ${index}: description must be a string.`)
					}
					const trimmedValue2 = typeof record.value2 === "string" ? record.value2.trim() : undefined
					const trimmedDescription = typeof record.description === "string" ? record.description.trim() : undefined
					if (requiresValue2 && !trimmedValue2) {
						console.error("invalid focus rule import value2 empty", { index, item: raw })
						throw new Error(`Invalid focus rule at index ${index}: value2 is required.`)
					}
					if (!requiresValue2 && trimmedValue2) {
						console.error("unexpected focus rule import value2", { index, item: raw })
						throw new Error(`Invalid focus rule at index ${index}: value2 is not applicable.`)
					}
					const normalized: FocusRuleImportItem = {
						match_type: record.match_type,
						value: trimmedValue,
						value2: requiresValue2 ? trimmedValue2 : undefined,
						description: trimmedDescription || undefined,
					}
					const key = buildFocusRuleKey(normalized)
					if (existingKeys.has(key)) {
						skipped += 1
						continue
					}
					existingKeys.add(key)
					toCreate.push(normalized)
				}
				for (const rule of toCreate) {
					await createDockerFocusService({ system: systemId, ...rule })
				}
				await onReload()
				const importedCount = toCreate.length
				const skippedCount = skipped
				if (importedCount > 0 || skippedCount > 0) {
					toast({
						title: t`Import completed`,
						description: t`Imported ${importedCount} rules, skipped ${skippedCount}.`,
					})
				}
			} catch (err) {
				console.error("import focus rules failed", err)
				toast({
					variant: "destructive",
					title: t`Error`,
					description: t`Failed to import focus rules`,
				})
				throw err
			} finally {
				setImporting(false)
				event.target.value = ""
			}
		},
		[systemId, readOnly, rules, onReload, isValidMatchType]
	)

	return (
		<Sheet open={open} onOpenChange={onOpenChange}>
			<SheetContent
				className="w-full sm:max-w-3xl overflow-y-auto"
				onOpenAutoFocus={(event) => {
					event.preventDefault()
				}}
			>
				<SheetHeader>
					<SheetTitle className="flex items-center gap-2">
						<Trans>Focus rules</Trans>
						<Tooltip>
							<TooltipTrigger asChild>
								<button
									type="button"
									aria-label={t`Focus rules help`}
									className="inline-flex h-6 w-6 items-center justify-center rounded-full text-muted-foreground/60 hover:text-muted-foreground/80"
								>
									<AlertCircleIcon className="h-4 w-4" />
								</button>
							</TooltipTrigger>
							<TooltipContent
								side="right"
								align="center"
								sideOffset={8}
								avoidCollisions={false}
								className="max-w-sm text-xs leading-relaxed"
							>
								<div className="space-y-1">
									<p>
										<Trans>Rules are exact matches. Use the values shown in the container list or details.</Trans>
									</p>
									<ul className="list-disc space-y-1 ps-4">
										<li>
											<Trans>Container name: use the Name column value.</Trans>
										</li>
										<li>
											<Trans>Image: use the Image column value (including tag).</Trans>
										</li>
										<li>
											<Trans>
												Compose project: use the Compose value shown under the name, or the label
												com.docker.compose.project.
											</Trans>
										</li>
										<li>
											<Trans>
												Compose service: enter both the compose project and service name. Service name comes from
												docker-compose.yml (services.*) or label com.docker.compose.service.
											</Trans>
										</li>
										<li>
											<Trans>Label: enter the label key and value (see Inspect for labels).</Trans>
										</li>
									</ul>
								</div>
							</TooltipContent>
						</Tooltip>
					</SheetTitle>
					<SheetDescription>
						<Trans>Define system-level rules to filter the container list.</Trans>
					</SheetDescription>
				</SheetHeader>
				<div className="space-y-4 mt-6 px-4 pb-4">
					<div className="rounded-md border p-4 space-y-4">
						<div className="text-sm font-medium">{isEditing ? <Trans>Edit</Trans> : <Trans>Add rule</Trans>}</div>
						<div className="grid gap-3 md:grid-cols-2">
							<div className="grid gap-2">
								<Label htmlFor="docker-focus-type">
									<Trans>Rule type</Trans>
								</Label>
								<Select
									value={matchType}
									onValueChange={(next) => setMatchType(next as DockerFocusMatchType)}
									disabled={isEditing}
								>
									<SelectTrigger id="docker-focus-type" ref={focusTypeRef}>
										<SelectValue placeholder={t`Select rule type`} />
									</SelectTrigger>
									<SelectContent>
										{Object.entries(matchTypeLabels).map(([key, label]) => (
											<SelectItem key={key} value={key}>
												{label}
											</SelectItem>
										))}
									</SelectContent>
								</Select>
							</div>
							<div className="grid gap-2">
								<Label htmlFor="docker-focus-value">{valueLabel}</Label>
								<Input
									id="docker-focus-value"
									value={value}
									onChange={(event) => setValue(event.target.value)}
									placeholder={t`Enter value`}
								/>
							</div>
							{showValue2 ? (
								<div className="grid gap-2">
									<Label htmlFor="docker-focus-value2">{value2Label}</Label>
									<Input
										id="docker-focus-value2"
										value={value2}
										onChange={(event) => setValue2(event.target.value)}
										placeholder={t`Enter value`}
									/>
								</div>
							) : null}
							<div className="grid gap-2 md:col-span-2">
								<Label htmlFor="docker-focus-description">
									<Trans>Description</Trans>
								</Label>
								<Input
									id="docker-focus-description"
									value={description}
									onChange={(event) => setDescription(event.target.value)}
									placeholder={t`Optional description`}
								/>
							</div>
						</div>
						<div className="flex flex-wrap items-center justify-end gap-2">
							<input
								ref={fileInputRef}
								type="file"
								accept="application/json"
								onChange={handleImport}
								className="hidden"
							/>
							<Button variant="outline" size="sm" onClick={() => void handleExport()}>
								<Trans>Export rules</Trans>
							</Button>
							<Button
								variant="outline"
								size="sm"
								onClick={() => fileInputRef.current?.click()}
								disabled={readOnly || importing || !systemId}
								className="relative"
							>
								{importing && (
									<div className="absolute inset-0 flex items-center justify-center">
										<LoaderCircleIcon className="h-4 w-4 animate-spin" />
									</div>
								)}
								<span className={importing ? "invisible" : ""}>
									<Trans>Import rules</Trans>
								</span>
							</Button>
							{isEditing ? (
								<Button variant="outline" size="sm" onClick={resetForm}>
									<Trans>Cancel</Trans>
								</Button>
							) : null}
							<Button
								onClick={() => void handleSubmit()}
								disabled={saving || readOnly || !systemId}
								className="relative min-w-[6rem]"
							>
								{saving && (
									<div className="absolute inset-0 flex items-center justify-center">
										<LoaderCircleIcon className="h-4 w-4 animate-spin" />
									</div>
								)}
								<span className={saving ? "invisible" : ""}>
									{isEditing ? <Trans>Save</Trans> : <Trans>Add rule</Trans>}
								</span>
							</Button>
						</div>
						{readOnly ? (
							<div className="text-xs text-muted-foreground">
								<Trans>Read-only users cannot modify focus rules.</Trans>
							</div>
						) : null}
					</div>
					<div className="h-min max-h-[calc(100dvh-32rem)] max-w-full relative overflow-auto border rounded-md bg-card">
						<table className="w-full caption-bottom text-sm">
							<TableHeader className="sticky top-0 z-10 bg-card">
								<TableRow>
									<TableHead className="max-w-[160px]">
										<Trans>Type</Trans>
									</TableHead>
									<TableHead>
										<Trans>Value</Trans>
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
										: "opacity-100 transition-opacity duration-300"
								}
							>
								{rules.length === 0 && loading ? (
									<TableRow>
										<TableCell colSpan={4} className="h-24 text-center">
											<div className="flex items-center justify-center gap-2 text-muted-foreground">
												<LoaderCircleIcon className="h-4 w-4 animate-spin" />
												<Trans>Loading...</Trans>
											</div>
										</TableCell>
									</TableRow>
								) : rules.length === 0 ? (
									<TableRow>
										<TableCell colSpan={4} className="h-24 text-center text-sm text-muted-foreground">
											<Trans>No focus rules found.</Trans>
										</TableCell>
									</TableRow>
								) : (
									rules.map((rule) => (
										<TableRow key={rule.id}>
											<TableCell className="text-xs text-muted-foreground">
												{matchTypeLabels[rule.match_type] ?? rule.match_type}
											</TableCell>
											<TableCell className="text-xs text-muted-foreground">{formatRuleValue(rule)}</TableCell>
											<TableCell className="text-xs text-muted-foreground">{rule.updated}</TableCell>
											<TableCell className="text-center">
												<DropdownMenu>
													<DropdownMenuTrigger asChild>
														<Button size="icon" variant="ghost">
															<MoreHorizontalIcon className="h-4 w-4" />
														</Button>
													</DropdownMenuTrigger>
													<DropdownMenuContent align="end">
														<DropdownMenuItem onSelect={() => handleEdit(rule)}>
															<Trans>Edit</Trans>
														</DropdownMenuItem>
														<DropdownMenuItem onSelect={() => setDeleteTarget(rule)}>
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
				</div>
				<AlertDialog
					open={!!deleteTarget}
					onOpenChange={(nextOpen) => {
						if (!nextOpen) {
							setDeleteTarget(null)
						}
					}}
				>
					<AlertDialogContent>
						<AlertDialogHeader>
							<AlertDialogTitle>
								<Trans>Delete focus rule</Trans>
							</AlertDialogTitle>
							<AlertDialogDescription>
								<Trans>This will remove the selected focus rule.</Trans>
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
			</SheetContent>
		</Sheet>
	)
})

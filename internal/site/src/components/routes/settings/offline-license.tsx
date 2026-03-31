import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { redirectPage } from "@nanostores/router"
import { DownloadIcon, FileSearchIcon, FileUpIcon, KeyIcon, RefreshCwIcon } from "lucide-react"
import { type ChangeEvent, useEffect, useMemo, useState } from "react"
import { $router } from "@/components/router"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import { toast } from "@/components/ui/use-toast"
import { isReadOnlyUser } from "@/lib/api"
import {
	downloadOfflineLicenseArtifact,
	downloadOfflineLicenseCollector,
	fetchOfflineLicenseOverview,
	importOfflineActivationRequest,
	issueOfflineLicense,
	previewOfflineActivationRequest,
} from "@/lib/offline-license"
import { formatShortDate } from "@/lib/utils"
import type {
	OfflineLicenseActivationPreview,
	OfflineLicenseActivationRecord,
	OfflineLicenseSigningState,
} from "@/types"

function shorten(value: string, head = 10, tail = 8) {
	if (!value) {
		return "-"
	}
	if (value.length <= head + tail + 3) {
		return value
	}
	return `${value.slice(0, head)}...${value.slice(-tail)}`
}

function todayString(offsetDays = 0) {
	const date = new Date()
	date.setDate(date.getDate() + offsetDays)
	return date.toISOString().slice(0, 10)
}

function activationStatusVariant(status: string): "secondary" | "success" | "warning" | "danger" {
	switch (status) {
		case "active":
			return "success"
		case "disabled":
		case "revoked":
			return "danger"
		case "issued":
			return "success"
		default:
			return "warning"
	}
}

function initialSigningState(): OfflineLicenseSigningState {
	return {
		ready: false,
		errors: [],
		model_names: [],
	}
}

function initialImportForm() {
	return {
		customer: "",
		tenant: "",
		project_name: "",
		site_name: "",
		remarks: "",
	}
}

function initialIssueForm(modelNames: string[]) {
	return {
		customer: "",
		tenant: "",
		notBefore: todayString(0),
		notAfter: "",
		modelNames: [...modelNames],
	}
}

function hasCurrentLicense(activation: OfflineLicenseActivationRecord | null | undefined) {
	return Boolean(activation?.current_license_id && activation.current_export_name)
}

export default function OfflineLicenseSettingsPage() {
	if (isReadOnlyUser()) {
		redirectPage($router, "settings", { name: "general" })
	}

	const [isLoading, setIsLoading] = useState(true)
	const [collectionsReady, setCollectionsReady] = useState(true)
	const [signing, setSigning] = useState<OfflineLicenseSigningState>(initialSigningState())
	const [activations, setActivations] = useState<OfflineLicenseActivationRecord[]>([])

	const [isImportDialogOpen, setIsImportDialogOpen] = useState(false)
	const [activationContent, setActivationContent] = useState("")
	const [activationPreview, setActivationPreview] = useState<OfflineLicenseActivationPreview | null>(null)
	const [importForm, setImportForm] = useState(initialImportForm())
	const [isPreviewing, setIsPreviewing] = useState(false)
	const [isImporting, setIsImporting] = useState(false)

	const [isIssueDialogOpen, setIsIssueDialogOpen] = useState(false)
	const [issueActivationId, setIssueActivationId] = useState("")
	const [issueForm, setIssueForm] = useState(initialIssueForm([]))
	const [isIssuing, setIsIssuing] = useState(false)

	const [isDetailOpen, setIsDetailOpen] = useState(false)
	const [detailActivationId, setDetailActivationId] = useState("")

	const refreshData = async () => {
		setIsLoading(true)
		try {
			const overview = await fetchOfflineLicenseOverview()
			setCollectionsReady(overview.ready)
			setSigning(overview.signing ?? initialSigningState())
			setActivations(overview.activations ?? [])
		} catch (error) {
			console.error(error)
			toast({
				title: t`Failed to load offline license data`,
				description: error instanceof Error ? error.message : String(error),
				variant: "destructive",
			})
		} finally {
			setIsLoading(false)
		}
	}

	useEffect(() => {
		refreshData()
	}, [])

	const selectedIssueActivation = useMemo(
		() => activations.find((item) => item.id === issueActivationId) ?? null,
		[activations, issueActivationId]
	)

	const detailActivation = useMemo(
		() => activations.find((item) => item.id === detailActivationId) ?? null,
		[activations, detailActivationId]
	)

	const resetImportDialog = () => {
		setActivationContent("")
		setActivationPreview(null)
		setImportForm(initialImportForm())
	}

	const openImportDialog = () => {
		resetImportDialog()
		setIsImportDialogOpen(true)
	}

	const handleCollectorDownload = async () => {
		try {
			await downloadOfflineLicenseCollector()
			toast({
				title: t`Collector downloaded`,
				description: t`Run the script on the customer host to generate activation.req and device keys.`,
			})
		} catch (error) {
			toast({
				title: t`Failed to download collector`,
				description: error instanceof Error ? error.message : String(error),
				variant: "destructive",
			})
		}
	}

	const handleActivationFileChange = async (event: ChangeEvent<HTMLInputElement>) => {
		const file = event.target.files?.[0]
		if (!file) {
			return
		}
		const text = await file.text()
		setActivationContent(text)
	}

	const handlePreviewActivation = async () => {
		if (!activationContent.trim()) {
			toast({
				title: t`Activation content is required`,
				description: t`Paste activation.req content or choose a file first.`,
				variant: "destructive",
			})
			return
		}

		setIsPreviewing(true)
		try {
			const preview = await previewOfflineActivationRequest(activationContent)
			setActivationPreview(preview)
			setImportForm({
				customer: preview.existing_activation?.customer ?? "",
				tenant: preview.existing_activation?.tenant ?? "",
				project_name: preview.existing_activation?.project_name ?? "",
				site_name: preview.existing_activation?.site_name ?? "",
				remarks: preview.existing_activation?.remarks ?? "",
			})
		} catch (error) {
			toast({
				title: t`Failed to parse activation`,
				description: error instanceof Error ? error.message : String(error),
				variant: "destructive",
			})
		} finally {
			setIsPreviewing(false)
		}
	}

	const handleImportActivation = async () => {
		if (!activationPreview) {
			return
		}
		if (!importForm.customer.trim()) {
			toast({
				title: t`Customer is required`,
				description: t`Fill in the customer before saving this activation.`,
				variant: "destructive",
			})
			return
		}

		setIsImporting(true)
		try {
			await importOfflineActivationRequest({
				content: activationContent,
				customer: importForm.customer.trim(),
				tenant: importForm.tenant.trim(),
				project_name: importForm.project_name.trim(),
				site_name: importForm.site_name.trim(),
				remarks: importForm.remarks.trim(),
			})
			toast({
				title: t`Activation saved`,
				description: t`The device record has been added to the offline license registry.`,
			})
			setIsImportDialogOpen(false)
			resetImportDialog()
			await refreshData()
		} catch (error) {
			toast({
				title: t`Failed to save activation`,
				description: error instanceof Error ? error.message : String(error),
				variant: "destructive",
			})
		} finally {
			setIsImporting(false)
		}
	}

	const openIssueDialog = (activation: OfflineLicenseActivationRecord) => {
		setIssueActivationId(activation.id)
		setIssueForm({
			...initialIssueForm(signing.model_names ?? []),
			customer: activation.customer ?? "",
			tenant: activation.tenant ?? "",
		})
		setIsIssueDialogOpen(true)
	}

	const toggleIssueModel = (modelName: string, checked: boolean) => {
		setIssueForm((current) => {
			const next = new Set(current.modelNames)
			if (checked) {
				next.add(modelName)
			} else {
				next.delete(modelName)
			}
			return {
				...current,
				modelNames: [...next],
			}
		})
	}

	const handleIssueLicense = async () => {
		if (!selectedIssueActivation) {
			return
		}
		if (!issueForm.customer.trim()) {
			toast({
				title: t`Customer is required`,
				description: t`Fill in the customer before issuing the license.`,
				variant: "destructive",
			})
			return
		}
		if ((signing.model_names ?? []).length > 0 && issueForm.modelNames.length === 0) {
			toast({
				title: t`Select at least one model`,
				description: t`Choose one or more models before issuing the license.`,
				variant: "destructive",
			})
			return
		}

		setIsIssuing(true)
		try {
			const manifestModelNames = signing.model_names ?? []
			const selectedModelNames =
				manifestModelNames.length > 0 && issueForm.modelNames.length === manifestModelNames.length
					? undefined
					: issueForm.modelNames

			const response = await issueOfflineLicense({
				activationId: selectedIssueActivation.id,
				customer: issueForm.customer.trim(),
				tenant: issueForm.tenant.trim(),
				notBefore: issueForm.notBefore || undefined,
				notAfter: issueForm.notAfter || undefined,
				modelNames: selectedModelNames,
			})
			toast({
				title: t`License issued`,
				description: t`Created ${response.fileName} with ${response.modelCount} licensed models.`,
			})
			setIsIssueDialogOpen(false)
			await refreshData()
		} catch (error) {
			toast({
				title: t`Failed to issue license`,
				description: error instanceof Error ? error.message : String(error),
				variant: "destructive",
			})
		} finally {
			setIsIssuing(false)
		}
	}

	const handleExportArtifact = async (licenseId: string) => {
		try {
			await downloadOfflineLicenseArtifact(licenseId)
			toast({
				title: t`License exported`,
				description: t`The signed license.dat has been downloaded.`,
			})
		} catch (error) {
			toast({
				title: t`Failed to export license`,
				description: error instanceof Error ? error.message : String(error),
				variant: "destructive",
			})
		}
	}

	return (
		<div className="space-y-5">
			<Card>
				<CardHeader>
					<CardTitle className="flex items-center gap-2">
						<KeyIcon className="size-5" />
						<Trans>Offline License</Trans>
					</CardTitle>
					<CardDescription>
						<Trans>
							Use the headquarters hub to download the collector, register device activations, issue a signed
							license.dat, and export it back to the customer host.
						</Trans>
					</CardDescription>
				</CardHeader>
				<CardContent className="flex flex-wrap gap-3">
					<Button onClick={handleCollectorDownload}>
						<DownloadIcon className="size-4" />
						<Trans>Download collector</Trans>
					</Button>
					<Button variant="outline" onClick={openImportDialog} disabled={!collectionsReady}>
						<FileUpIcon className="size-4" />
						<Trans>Import activation.req</Trans>
					</Button>
					<Button variant="outline" onClick={refreshData} disabled={isLoading}>
						<RefreshCwIcon className="size-4" />
						<Trans>Refresh</Trans>
					</Button>
				</CardContent>
			</Card>

			{!collectionsReady && (
				<Card>
					<CardContent className="pt-6 text-sm text-amber-600">
						<Trans>Offline license collections are not initialized yet. Restart the Hub to run migrations.</Trans>
					</CardContent>
				</Card>
			)}

			<Card>
				<CardHeader>
					<CardTitle>
						<Trans>Signing readiness</Trans>
					</CardTitle>
					<CardDescription>
						<Trans>Issuing is enabled only after the headquarters signing key and model manifest are loaded.</Trans>
					</CardDescription>
				</CardHeader>
				<CardContent className="space-y-3">
					<div className="flex flex-wrap items-center gap-2">
						<Badge variant={signing.ready ? "success" : "warning"}>{signing.ready ? t`Ready` : t`Not ready`}</Badge>
						{(signing.model_names ?? []).map((name) => (
							<Badge key={name} variant="outline">
								{name}
							</Badge>
						))}
					</div>
					{!signing.ready && (signing.errors?.length ?? 0) > 0 && (
						<div className="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-700">
							{signing.errors.map((item) => (
								<div key={item}>{item}</div>
							))}
						</div>
					)}
				</CardContent>
			</Card>

			<Card>
				<CardHeader>
					<CardTitle>
						<Trans>Activation registry</Trans>
					</CardTitle>
					<CardDescription>
						<Trans>Each row represents a customer device activation. Use row actions to issue or export the current license and review device details.</Trans>
					</CardDescription>
				</CardHeader>
				<CardContent>
					<Table>
						<TableHeader>
							<TableRow>
								<TableHead>
									<Trans>Customer</Trans>
								</TableHead>
								<TableHead>
									<Trans>Tenant</Trans>
								</TableHead>
								<TableHead>
									<Trans>Project / Site</Trans>
								</TableHead>
								<TableHead>
									<Trans>Hostname</Trans>
								</TableHead>
								<TableHead>
									<Trans>Request ID</Trans>
								</TableHead>
								<TableHead>
									<Trans>Status</Trans>
								</TableHead>
								<TableHead>
									<Trans>Last issued</Trans>
								</TableHead>
								<TableHead>
									<Trans>Updated</Trans>
								</TableHead>
								<TableHead>
									<Trans>Action</Trans>
								</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{activations.length === 0 && (
								<TableRow>
									<TableCell colSpan={9}>
										<Trans>No activation records yet.</Trans>
									</TableCell>
								</TableRow>
							)}
							{activations.map((item) => (
								<TableRow key={item.id}>
									<TableCell>{item.customer || "-"}</TableCell>
									<TableCell>{item.tenant || "-"}</TableCell>
									<TableCell>{[item.project_name, item.site_name].filter(Boolean).join(" / ") || "-"}</TableCell>
									<TableCell>{item.hostname || "-"}</TableCell>
									<TableCell className="font-mono text-xs">{item.request_id}</TableCell>
									<TableCell>
										<Badge variant={activationStatusVariant(item.status)}>{item.status}</Badge>
									</TableCell>
									<TableCell>{item.last_issued_at ? formatShortDate(item.last_issued_at) : "-"}</TableCell>
									<TableCell>{formatShortDate(item.updated)}</TableCell>
									<TableCell className="space-x-2">
										<Button
											variant="outline"
											size="sm"
											onClick={() => openIssueDialog(item)}
											disabled={!collectionsReady || !signing.ready || item.status === "disabled"}
										>
											<KeyIcon className="size-4" />
											<Trans>Issue</Trans>
										</Button>
										{hasCurrentLicense(item) && (
											<Button
												variant="outline"
												size="sm"
												onClick={() => handleExportArtifact(item.current_license_id!)}
											>
												<DownloadIcon className="size-4" />
												<Trans>Export</Trans>
											</Button>
										)}
										<Button
											variant="outline"
											size="sm"
											onClick={() => {
												setDetailActivationId(item.id)
												setIsDetailOpen(true)
											}}
										>
											<FileSearchIcon className="size-4" />
											<Trans>Details</Trans>
										</Button>
									</TableCell>
								</TableRow>
							))}
						</TableBody>
					</Table>
				</CardContent>
			</Card>

			<Dialog
				open={isImportDialogOpen}
				onOpenChange={(open) => {
					setIsImportDialogOpen(open)
					if (!open) {
						resetImportDialog()
					}
				}}
			>
				<DialogContent className="max-w-3xl">
					<DialogHeader>
						<DialogTitle>
							<Trans>Import activation.req</Trans>
						</DialogTitle>
						<DialogDescription>
							<Trans>Parse the activation request first, then fill in customer information before saving it as a device record.</Trans>
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-4">
						{!activationPreview && (
							<>
								<div className="space-y-2">
									<Label htmlFor="activation-file">
										<Trans>Activation file</Trans>
									</Label>
									<Input
										id="activation-file"
										type="file"
										accept=".req,.json,.dat"
										onChange={handleActivationFileChange}
									/>
								</div>
								<div className="space-y-2">
									<Label htmlFor="activation-content">
										<Trans>Activation content</Trans>
									</Label>
									<Textarea
										id="activation-content"
										value={activationContent}
										onChange={(event) => setActivationContent(event.target.value)}
										className="min-h-52 font-mono text-xs"
										placeholder={`{\n  "request_id": "activation-...",\n  "device_public_key_b64": "..."\n}`}
									/>
								</div>
							</>
						)}

						{activationPreview && (
							<>
								<div className="grid gap-4 md:grid-cols-2">
									<div className="rounded-md border border-border/70 p-3 text-sm">
										<div className="font-medium">
											<Trans>Parsed activation</Trans>
										</div>
										<div className="mt-2">
											<Trans>Request ID</Trans>: {activationPreview.request_id}
										</div>
										<div>
											<Trans>Hostname</Trans>: {activationPreview.hostname || "-"}
										</div>
										<div>
											<Trans>Fingerprint</Trans>: {shorten(activationPreview.fingerprint, 14, 12)}
										</div>
									</div>
									<div className="rounded-md border border-border/70 p-3 text-sm">
										<div className="font-medium">
											<Trans>Machine factors</Trans>
										</div>
										<div className="mt-2">machine_id: {activationPreview.factors.machine_id || "-"}</div>
										<div>product_uuid: {activationPreview.factors.product_uuid || "-"}</div>
										<div>board_serial: {activationPreview.factors.board_serial || "-"}</div>
									</div>
								</div>

								{activationPreview.existing_activation && (
									<div className="rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-700">
										<Trans>This request already exists. Saving will update the existing activation record.</Trans>
									</div>
								)}

								<div className="grid gap-4 md:grid-cols-2">
									<div className="space-y-2">
										<Label htmlFor="import-customer">
											<Trans>Customer</Trans>
										</Label>
										<Input
											id="import-customer"
											value={importForm.customer}
											onChange={(event) =>
												setImportForm((current) => ({ ...current, customer: event.target.value }))
											}
										/>
									</div>
									<div className="space-y-2">
										<Label htmlFor="import-tenant">
											<Trans>Tenant</Trans>
										</Label>
										<Input
											id="import-tenant"
											value={importForm.tenant}
											onChange={(event) =>
												setImportForm((current) => ({ ...current, tenant: event.target.value }))
											}
										/>
									</div>
									<div className="space-y-2">
										<Label htmlFor="import-project">
											<Trans>Project name</Trans>
										</Label>
										<Input
											id="import-project"
											value={importForm.project_name}
											onChange={(event) =>
												setImportForm((current) => ({ ...current, project_name: event.target.value }))
											}
										/>
									</div>
									<div className="space-y-2">
										<Label htmlFor="import-site">
											<Trans>Site name</Trans>
										</Label>
										<Input
											id="import-site"
											value={importForm.site_name}
											onChange={(event) =>
												setImportForm((current) => ({ ...current, site_name: event.target.value }))
											}
										/>
									</div>
									<div className="space-y-2 md:col-span-2">
										<Label htmlFor="import-remarks">
											<Trans>Remarks</Trans>
										</Label>
										<Textarea
											id="import-remarks"
											value={importForm.remarks}
											onChange={(event) =>
												setImportForm((current) => ({ ...current, remarks: event.target.value }))
											}
											className="min-h-24"
										/>
									</div>
								</div>
							</>
						)}
					</div>
					<DialogFooter>
						{activationPreview ? (
							<>
								<Button variant="outline" onClick={() => setActivationPreview(null)}>
									<Trans>Back</Trans>
								</Button>
								<Button onClick={handleImportActivation} disabled={isImporting}>
									<FileUpIcon className="size-4" />
									<Trans>Save activation</Trans>
								</Button>
							</>
						) : (
							<Button onClick={handlePreviewActivation} disabled={isPreviewing}>
								<FileSearchIcon className="size-4" />
								<Trans>Parse activation</Trans>
							</Button>
						)}
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<Dialog open={isIssueDialogOpen} onOpenChange={setIsIssueDialogOpen}>
				<DialogContent className="max-w-2xl">
					<DialogHeader>
						<DialogTitle>
							<Trans>Issue license.dat</Trans>
						</DialogTitle>
						<DialogDescription>
							<Trans>Issue a new offline license and replace the current license snapshot for the selected activation record.</Trans>
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-4">
						<div className="rounded-md border border-border/70 p-3 text-sm text-muted-foreground">
							<div>
								<Trans>Activation</Trans>: {selectedIssueActivation?.request_id || "-"}
							</div>
							<div>
								<Trans>Hostname</Trans>: {selectedIssueActivation?.hostname || "-"}
							</div>
							<div>
								<Trans>Fingerprint</Trans>:{" "}
								{selectedIssueActivation ? shorten(selectedIssueActivation.fingerprint, 14, 12) : "-"}
							</div>
						</div>
						<div className="grid gap-4 md:grid-cols-2">
							<div className="space-y-2">
								<Label htmlFor="issue-customer">
									<Trans>Customer</Trans>
								</Label>
								<Input
									id="issue-customer"
									value={issueForm.customer}
									onChange={(event) => setIssueForm((current) => ({ ...current, customer: event.target.value }))}
								/>
							</div>
							<div className="space-y-2">
								<Label htmlFor="issue-tenant">
									<Trans>Tenant</Trans>
								</Label>
								<Input
									id="issue-tenant"
									value={issueForm.tenant}
									onChange={(event) => setIssueForm((current) => ({ ...current, tenant: event.target.value }))}
								/>
							</div>
							<div className="space-y-2">
								<Label htmlFor="issue-not-before">
									<Trans>Not before</Trans>
								</Label>
								<Input
									id="issue-not-before"
									type="date"
									value={issueForm.notBefore}
									onChange={(event) => setIssueForm((current) => ({ ...current, notBefore: event.target.value }))}
								/>
							</div>
							<div className="space-y-2">
								<Label htmlFor="issue-not-after">
									<Trans>Not after</Trans>
								</Label>
								<Input
									id="issue-not-after"
									type="date"
									value={issueForm.notAfter}
									onChange={(event) => setIssueForm((current) => ({ ...current, notAfter: event.target.value }))}
								/>
							</div>
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Licensed models</Trans>
							</Label>
							<div className="rounded-md border border-border/70 p-3">
								{(signing.model_names ?? []).length === 0 ? (
									<div className="text-sm text-muted-foreground">
										<Trans>No manifest models available.</Trans>
									</div>
								) : (
									<div className="grid gap-2 md:grid-cols-2">
										{signing.model_names.map((modelName) => (
											<label key={modelName} className="flex items-center gap-2 text-sm">
												<Checkbox
													checked={issueForm.modelNames.includes(modelName)}
													onCheckedChange={(value) => toggleIssueModel(modelName, Boolean(value))}
												/>
												<span>{modelName}</span>
											</label>
										))}
									</div>
								)}
							</div>
							<p className="text-xs text-muted-foreground">
								<Trans>Keep all manifest models selected to issue the full license scope.</Trans>
							</p>
						</div>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={() => setIsIssueDialogOpen(false)}>
							<Trans>Cancel</Trans>
						</Button>
						<Button onClick={handleIssueLicense} disabled={isIssuing || !signing.ready}>
							<KeyIcon className="size-4" />
							<Trans>Generate license</Trans>
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<Sheet open={isDetailOpen} onOpenChange={setIsDetailOpen}>
				<SheetContent className="sm:max-w-2xl w-[92vw] overflow-y-auto">
					<SheetHeader>
						<SheetTitle>
							<Trans>Activation details</Trans>
						</SheetTitle>
						<SheetDescription>
							<Trans>Review device information and manage the current license snapshot for this activation.</Trans>
						</SheetDescription>
					</SheetHeader>
					{detailActivation && (
						<div className="space-y-5 px-4 pb-6">
							<div className="grid gap-4 md:grid-cols-2">
								<div className="rounded-md border border-border/70 p-3 text-sm">
									<div>
										<Trans>Customer</Trans>: {detailActivation.customer || "-"}
									</div>
									<div>
										<Trans>Tenant</Trans>: {detailActivation.tenant || "-"}
									</div>
									<div>
										<Trans>Project name</Trans>: {detailActivation.project_name || "-"}
									</div>
									<div>
										<Trans>Site name</Trans>: {detailActivation.site_name || "-"}
									</div>
									<div>
										<Trans>Status</Trans>: {detailActivation.status}
									</div>
								</div>
								<div className="rounded-md border border-border/70 p-3 text-sm">
									<div>
										<Trans>Hostname</Trans>: {detailActivation.hostname || "-"}
									</div>
									<div>
										<Trans>Request ID</Trans>: {detailActivation.request_id}
									</div>
									<div>
										<Trans>Fingerprint</Trans>: {detailActivation.fingerprint}
									</div>
									<div>
										<Trans>Last issued</Trans>:{" "}
										{detailActivation.last_issued_at ? formatShortDate(detailActivation.last_issued_at) : "-"}
									</div>
									<div>
										<Trans>Updated</Trans>: {formatShortDate(detailActivation.updated)}
									</div>
								</div>
							</div>

							<div className="rounded-md border border-border/70 p-3 text-sm">
								<div className="font-medium">
									<Trans>Machine factors</Trans>
								</div>
								<div className="mt-2">machine_id: {detailActivation.factors_json.machine_id || "-"}</div>
								<div>product_uuid: {detailActivation.factors_json.product_uuid || "-"}</div>
								<div>board_serial: {detailActivation.factors_json.board_serial || "-"}</div>
							</div>

							<div className="space-y-2">
								<Label>
									<Trans>Remarks</Trans>
								</Label>
								<Textarea value={detailActivation.remarks || ""} readOnly className="min-h-20" />
							</div>

							<div className="space-y-2">
								<Label>
									<Trans>Activation payload</Trans>
								</Label>
								<Textarea
									value={JSON.stringify(detailActivation.activation_payload ?? {}, null, 2)}
									readOnly
									className="min-h-52 font-mono text-xs"
								/>
							</div>

							<div className="space-y-2">
								<Label>
									<Trans>Current license</Trans>
								</Label>
								{hasCurrentLicense(detailActivation) ? (
									<div className="space-y-3 rounded-md border border-border/70 p-3 text-sm">
										<div className="grid gap-3 md:grid-cols-2">
											<div>
												<Trans>Issued at</Trans>:{" "}
												{detailActivation.last_issued_at ? formatShortDate(detailActivation.last_issued_at) : "-"}
											</div>
											<div>
												<Trans>Expires</Trans>:{" "}
												{detailActivation.current_not_after ? formatShortDate(detailActivation.current_not_after) : "-"}
											</div>
											<div>
												<Trans>Not before</Trans>:{" "}
												{detailActivation.current_not_before
													? formatShortDate(detailActivation.current_not_before)
													: "-"}
											</div>
											<div>
												<Trans>Models</Trans>: {detailActivation.current_models_json?.length ?? 0}
											</div>
										</div>
										<div className="flex flex-wrap gap-2">
											{(detailActivation.current_models_json ?? []).map((item) => (
												<Badge key={item.name} variant="outline">
													{item.name}
												</Badge>
											))}
										</div>
										<div>
											<Button
												variant="outline"
												size="sm"
												onClick={() => handleExportArtifact(detailActivation.current_license_id!)}
											>
												<DownloadIcon className="size-4" />
												<Trans>Export</Trans>
											</Button>
										</div>
									</div>
								) : (
									<div className="rounded-md border border-dashed border-border/70 p-4 text-sm text-muted-foreground">
										<Trans>No license has been issued for this activation yet.</Trans>
									</div>
								)}
							</div>

							{detailActivation.current_license_payload && (
								<div className="space-y-2">
									<Label>
										<Trans>Current license payload</Trans>
									</Label>
									<Textarea
										value={JSON.stringify(detailActivation.current_license_payload ?? {}, null, 2)}
										readOnly
										className="min-h-52 font-mono text-xs"
									/>
								</div>
							)}
						</div>
					)}
				</SheetContent>
			</Sheet>
		</div>
	)
}

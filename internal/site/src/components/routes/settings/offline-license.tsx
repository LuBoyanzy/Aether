import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { redirectPage } from "@nanostores/router"
import {
	CheckCircle2Icon,
	DownloadIcon,
	FileSearchIcon,
	FileUpIcon,
	KeyIcon,
	Loader2Icon,
	RefreshCwIcon,
	ShieldCheckIcon,
	ShieldIcon,
	AlertCircleIcon,
	CalendarIcon,
	Building2Icon,
	ServerIcon,
	FingerprintIcon,
	TagIcon,
	ClockIcon,
	MoreHorizontalIcon,
	ChevronRightIcon,
	XCircleIcon,
	InfoIcon,
} from "lucide-react"
import { type ChangeEvent, useEffect, useMemo, useState } from "react"
import { $router } from "@/components/router"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
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
import { Separator } from "@/components/ui/separator"
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

function activationStatusVariant(status: string): "default" | "success" | "warning" | "danger" {
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

function activationStatusLabel(status: string): string {
	switch (status) {
		case "active":
			return t`Active`
		case "disabled":
			return t`Disabled`
		case "revoked":
			return t`Revoked`
		case "issued":
			return t`Issued`
		default:
			return status
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

// Apple-inspired Status Card Component
function StatusCard({
	title,
	description,
	isReady,
	modelNames,
	errors,
}: {
	title: string
	description: string
	isReady: boolean
	modelNames: string[]
	errors: string[]
}) {
	return (
		<div className="relative overflow-hidden rounded-2xl border border-border/50 bg-gradient-to-br from-card to-card/95 p-6 shadow-sm transition-all duration-300 hover:shadow-md">
			<div className="absolute inset-x-0 top-0 h-1 bg-gradient-to-r from-transparent via-primary/20 to-transparent" />
			<div className="flex items-start justify-between">
				<div className="flex items-center gap-4">
					<div
						className={`flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl transition-all duration-500 ${
							isReady
								? "bg-gradient-to-br from-emerald-500/15 to-emerald-500/5 text-emerald-600 shadow-sm"
								: "bg-gradient-to-br from-amber-500/15 to-amber-500/5 text-amber-600 shadow-sm"
						}`}
					>
						{isReady ? (
							<ShieldCheckIcon className="h-7 w-7" strokeWidth={1.5} />
						) : (
							<ShieldIcon className="h-7 w-7" strokeWidth={1.5} />
						)}
					</div>
					<div>
						<h3 className="text-lg font-semibold tracking-tight">{title}</h3>
						<p className="mt-1 text-sm text-muted-foreground leading-relaxed">{description}</p>
					</div>
				</div>
				<div className="flex flex-col items-end gap-2">
					<Badge
						variant={isReady ? "success" : "warning"}
						className={`px-3 py-1.5 text-xs font-medium ${
							isReady
								? "bg-emerald-500/10 text-emerald-700 border-emerald-500/20 hover:bg-emerald-500/15"
								: "bg-amber-500/10 text-amber-700 border-amber-500/20 hover:bg-amber-500/15"
						}`}
					>
						{isReady ? (
							<span className="flex items-center gap-1.5">
								<CheckCircle2Icon className="h-3.5 w-3.5" />
								{t`Ready`}
							</span>
						) : (
							<span className="flex items-center gap-1.5">
								<AlertCircleIcon className="h-3.5 w-3.5" />
								{t`Not Ready`}
							</span>
						)}
					</Badge>
				</div>
			</div>

			{modelNames.length > 0 && (
				<div className="mt-6">
					<div className="mb-3 flex items-center gap-2 text-sm font-medium text-muted-foreground">
						<TagIcon className="h-4 w-4" />
						<span>
							<Trans>Licensed Models</Trans>
						</span>
					</div>
					<div className="flex flex-wrap gap-2">
						{modelNames.map((name) => (
							<span
								key={name}
								className="inline-flex items-center gap-1.5 rounded-full border border-border/60 bg-secondary/50 px-3 py-1.5 text-sm font-medium text-secondary-foreground transition-colors hover:bg-secondary"
							>
								<KeyIcon className="h-3 w-3 text-muted-foreground" />
								{name}
							</span>
						))}
					</div>
				</div>
			)}

			{!isReady && errors.length > 0 && (
				<div className="mt-6 rounded-xl border border-amber-200/60 bg-gradient-to-r from-amber-50/80 to-amber-50/40 p-4 dark:border-amber-900/40 dark:from-amber-950/30 dark:to-amber-950/10">
					<div className="flex items-start gap-3">
						<InfoIcon className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />
						<div className="space-y-1">
							<p className="text-sm font-medium text-amber-800 dark:text-amber-200">
								<Trans>Configuration Required</Trans>
							</p>
							{errors.map((item, idx) => (
								<p key={idx} className="text-sm text-amber-700/80 dark:text-amber-300/70">
									• {item}
								</p>
							))}
						</div>
					</div>
				</div>
			)}
		</div>
	)
}

// Hero Card Component
function HeroCard({ onCollectorDownload, onImportClick, onRefresh, isLoading, disabled }: {
	onCollectorDownload: () => void
	onImportClick: () => void
	onRefresh: () => void
	isLoading: boolean
	disabled: boolean
}) {
	return (
		<div className="relative overflow-hidden rounded-3xl bg-gradient-to-br from-primary/[0.03] via-primary/[0.01] to-transparent p-8 lg:p-10">
			<div className="absolute -right-20 -top-20 h-64 w-64 rounded-full bg-primary/5 blur-3xl" />
			<div className="absolute -bottom-20 -left-20 h-64 w-64 rounded-full bg-primary/5 blur-3xl" />
			
			<div className="relative">
				<div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
					<div className="space-y-4 max-w-xl">
						<div className="inline-flex items-center gap-2 rounded-full bg-primary/10 px-3 py-1 text-xs font-medium text-primary">
							<ShieldCheckIcon className="h-3.5 w-3.5" />
							<span><Trans>Enterprise License Management</Trans></span>
						</div>
						<h1 className="text-3xl font-semibold tracking-tight lg:text-4xl">
							<Trans>Offline License</Trans>
						</h1>
						<p className="text-base leading-relaxed text-muted-foreground">
							<Trans>
								Manage offline license activations for air-gapped environments. 
								Download the collector, register devices, and issue signed license files.
							</Trans>
						</p>
					</div>
					
					<div className="flex flex-wrap gap-3">
						<Button
							onClick={onCollectorDownload}
							className="h-11 gap-2 rounded-xl bg-primary px-5 text-sm font-medium shadow-sm transition-all hover:bg-primary/90 hover:shadow-md active:scale-[0.98]"
						>
							<DownloadIcon className="h-4 w-4" />
							<span><Trans>Download Collector</Trans></span>
						</Button>
						<Button
							variant="outline"
							onClick={onImportClick}
							disabled={disabled}
							className="h-11 gap-2 rounded-xl border-border/60 px-5 text-sm font-medium transition-all hover:bg-secondary hover:border-border"
						>
							<FileUpIcon className="h-4 w-4" />
							<span><Trans>Import Request</Trans></span>
						</Button>
						<Button
							variant="ghost"
							onClick={onRefresh}
							disabled={isLoading}
							className="h-11 w-11 rounded-xl p-0 transition-all hover:bg-secondary"
						>
							<RefreshCwIcon className={`h-4 w-4 ${isLoading ? "animate-spin" : ""}`} />
						</Button>
					</div>
				</div>
			</div>
		</div>
	)
}

// Empty State Component
function EmptyState() {
	return (
		<div className="flex flex-col items-center justify-center rounded-2xl border border-dashed border-border/60 bg-secondary/20 py-16 text-center">
			<div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-secondary/50">
				<ServerIcon className="h-8 w-8 text-muted-foreground/60" strokeWidth={1.5} />
			</div>
			<h3 className="mt-5 text-base font-medium text-foreground">
				<Trans>No activation records</Trans>
			</h3>
			<p className="mt-1.5 max-w-sm text-sm text-muted-foreground">
				<Trans>Import your first activation request to get started with offline license management.</Trans>
			</p>
		</div>
	)
}

// Info Row Component for Detail Sheet
function InfoRow({
	icon: Icon,
	label,
	value,
	mono = false,
}: {
	icon: React.ElementType
	label: string
	value: string
	mono?: boolean
}) {
	return (
		<div className="flex items-center gap-4 py-3">
			<div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-secondary/70">
				<Icon className="h-4 w-4 text-muted-foreground" />
			</div>
			<div className="min-w-0 flex-1">
				<p className="text-xs font-medium text-muted-foreground">{label}</p>
				<p className={`mt-0.5 text-sm font-medium text-foreground ${mono ? "font-mono" : ""}`}>
					{value || "-"}
				</p>
			</div>
		</div>
	)
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
		<div className="space-y-8">
			{/* Hero Section */}
			<HeroCard
				onCollectorDownload={handleCollectorDownload}
				onImportClick={openImportDialog}
				onRefresh={refreshData}
				isLoading={isLoading}
				disabled={!collectionsReady}
			/>

			{/* Not Ready Warning */}
			{!collectionsReady && (
				<div className="rounded-2xl border border-amber-200/60 bg-gradient-to-r from-amber-50/80 to-amber-50/40 p-5 dark:border-amber-900/40 dark:from-amber-950/30 dark:to-amber-950/10">
					<div className="flex items-start gap-4">
						<div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-amber-500/15">
							<AlertCircleIcon className="h-5 w-5 text-amber-600" />
						</div>
						<div>
							<h4 className="font-medium text-amber-800 dark:text-amber-200">
								<Trans>Collections Not Initialized</Trans>
							</h4>
							<p className="mt-1 text-sm text-amber-700/80 dark:text-amber-300/70">
								<Trans>Offline license collections are not initialized yet. Restart the Hub to run migrations.</Trans>
							</p>
						</div>
					</div>
				</div>
			)}

			{/* Status Section */}
			<StatusCard
				title={t`Signing Readiness`}
				description={t`Issuing is enabled only after the headquarters signing key and model manifest are loaded.`}
				isReady={signing.ready}
				modelNames={signing.model_names ?? []}
				errors={signing.errors ?? []}
			/>

			{/* Activation Registry Section */}
			<div className="space-y-5">
				<div className="flex items-center justify-between">
					<div>
						<h2 className="text-xl font-semibold tracking-tight">
							<Trans>Activation Registry</Trans>
						</h2>
						<p className="mt-1 text-sm text-muted-foreground">
							<Trans>Manage customer device activations and license snapshots</Trans>
						</p>
					</div>
					<div className="flex items-center gap-2 text-sm text-muted-foreground">
						<span className="flex h-6 items-center justify-center rounded-full bg-secondary px-2.5 text-xs font-medium">
							{activations.length}
						</span>
						<span><Trans>records</Trans></span>
					</div>
				</div>

				{activations.length === 0 ? (
					<EmptyState />
				) : (
					<div className="overflow-hidden rounded-2xl border border-border/50 bg-card shadow-sm">
						<Table>
							<TableHeader>
								<TableRow className="border-b border-border/60 bg-secondary/30 hover:bg-secondary/30">
									<TableHead className="h-12 px-4 font-medium text-muted-foreground">
										<Trans>Customer</Trans>
									</TableHead>
									<TableHead className="h-12 px-4 font-medium text-muted-foreground">
										<Trans>Tenant</Trans>
									</TableHead>
									<TableHead className="h-12 px-4 font-medium text-muted-foreground">
										<Trans>Project / Site</Trans>
									</TableHead>
									<TableHead className="h-12 px-4 font-medium text-muted-foreground">
										<Trans>Hostname</Trans>
									</TableHead>
									<TableHead className="h-12 px-4 font-medium text-muted-foreground">
										<Trans>Status</Trans>
									</TableHead>
									<TableHead className="h-12 px-4 font-medium text-muted-foreground">
										<Trans>Last Issued</Trans>
									</TableHead>
									<TableHead className="h-12 px-4 text-right font-medium text-muted-foreground">
										<Trans>Actions</Trans>
									</TableHead>
								</TableRow>
							</TableHeader>
							<TableBody>
								{activations.map((item, index) => (
									<TableRow
										key={item.id}
										className="group border-b border-border/40 transition-colors hover:bg-secondary/20"
									>
										<TableCell className="px-4 py-4">
											<div className="flex items-center gap-3">
												<div className="flex h-9 w-9 items-center justify-center rounded-xl bg-primary/5 transition-colors group-hover:bg-primary/10">
													<Building2Icon className="h-4 w-4 text-primary/70" />
												</div>
												<span className="font-medium">{item.customer || "-"}</span>
											</div>
										</TableCell>
										<TableCell className="px-4 py-4 text-muted-foreground">
											{item.tenant || "-"}
										</TableCell>
										<TableCell className="px-4 py-4 text-muted-foreground">
											{[item.project_name, item.site_name].filter(Boolean).join(" / ") || "-"}
										</TableCell>
										<TableCell className="px-4 py-4">
											<div className="flex items-center gap-2">
												<ServerIcon className="h-3.5 w-3.5 text-muted-foreground/60" />
												<span className="font-mono text-xs">{item.hostname || "-"}</span>
											</div>
										</TableCell>
										<TableCell className="px-4 py-4">
											<Badge
												variant={activationStatusVariant(item.status)}
												className={`rounded-full px-2.5 py-0.5 text-xs font-medium ${
													item.status === "active"
														? "bg-emerald-500/10 text-emerald-700 border-emerald-500/20"
														: item.status === "disabled" || item.status === "revoked"
															? "bg-red-500/10 text-red-700 border-red-500/20"
															: "bg-amber-500/10 text-amber-700 border-amber-500/20"
												}`}
											>
												{activationStatusLabel(item.status)}
											</Badge>
										</TableCell>
										<TableCell className="px-4 py-4 text-muted-foreground">
											{item.last_issued_at ? (
												<span className="flex items-center gap-1.5">
													<CalendarIcon className="h-3.5 w-3.5" />
													{formatShortDate(item.last_issued_at)}
												</span>
											) : (
												<span className="text-muted-foreground/50">-</span>
											)}
										</TableCell>
										<TableCell className="px-4 py-4">
											<div className="flex items-center justify-end gap-2">
												<Button
													variant="ghost"
													size="sm"
													onClick={() => openIssueDialog(item)}
													disabled={!collectionsReady || !signing.ready || item.status === "disabled"}
													className="h-8 gap-1.5 rounded-lg px-3 text-xs font-medium hover:bg-primary/10 hover:text-primary"
												>
													<KeyIcon className="h-3.5 w-3.5" />
													<Trans>Issue</Trans>
												</Button>
												{hasCurrentLicense(item) && (
													<Button
														variant="ghost"
														size="sm"
														onClick={() => handleExportArtifact(item.current_license_id!)}
														className="h-8 gap-1.5 rounded-lg px-3 text-xs font-medium hover:bg-emerald-500/10 hover:text-emerald-600"
													>
														<DownloadIcon className="h-3.5 w-3.5" />
														<Trans>Export</Trans>
													</Button>
												)}
												<Button
													variant="ghost"
													size="sm"
													onClick={() => {
														setDetailActivationId(item.id)
														setIsDetailOpen(true)
													}}
													className="h-8 w-8 rounded-lg p-0 hover:bg-secondary"
												>
													<MoreHorizontalIcon className="h-4 w-4" />
												</Button>
											</div>
										</TableCell>
									</TableRow>
								))}
							</TableBody>
						</Table>
					</div>
				)}
			</div>

			{/* Import Dialog */}
			<Dialog
				open={isImportDialogOpen}
				onOpenChange={(open) => {
					setIsImportDialogOpen(open)
					if (!open) {
						resetImportDialog()
					}
				}}
			>
				<DialogContent className="max-w-2xl rounded-2xl border-border/60 p-0">
					<div className="p-6 pb-4">
						<DialogHeader className="space-y-3">
							<div className="flex items-center gap-3">
								<div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10">
									<FileUpIcon className="h-5 w-5 text-primary" />
								</div>
								<DialogTitle className="text-xl font-semibold">
									<Trans>Import Activation Request</Trans>
								</DialogTitle>
							</div>
							<DialogDescription className="text-sm text-muted-foreground leading-relaxed">
								<Trans>
									Parse the activation request file and register the device for offline licensing.
								</Trans>
							</DialogDescription>
						</DialogHeader>
					</div>

					<Separator className="bg-border/60" />

					<div className="p-6">
						{!activationPreview ? (
							<div className="space-y-5">
								<div className="space-y-3">
									<Label htmlFor="activation-file" className="text-sm font-medium">
										<Trans>Upload Activation File</Trans>
									</Label>
									<div className="relative">
										<Input
											id="activation-file"
											type="file"
											accept=".req,.json,.dat"
											onChange={handleActivationFileChange}
											className="h-12 cursor-pointer rounded-xl border-border/60 transition-colors file:mr-4 file:rounded-lg file:border-0 file:bg-secondary file:px-3 file:py-1.5 file:text-xs file:font-medium hover:border-border"
										/>
									</div>
									<p className="text-xs text-muted-foreground">
										<Trans>Supported formats: .req, .json, .dat</Trans>
									</p>
								</div>

								<div className="space-y-3">
									<div className="flex items-center justify-between">
										<Label htmlFor="activation-content" className="text-sm font-medium">
											<Trans>Or Paste Content</Trans>
										</Label>
										{activationContent && (
											<button
												onClick={() => setActivationContent("")}
												className="text-xs text-muted-foreground hover:text-foreground transition-colors"
											>
												<Trans>Clear</Trans>
											</button>
										)}
									</div>
									<Textarea
										id="activation-content"
										value={activationContent}
										onChange={(event) => setActivationContent(event.target.value)}
										className="min-h-40 rounded-xl border-border/60 font-mono text-xs leading-relaxed transition-colors focus:border-primary/50"
										placeholder={`{\n  "request_id": "activation-...",\n  "device_public_key_b64": "..."\n}`}
									/>
								</div>
							</div>
						) : (
							<div className="space-y-6">
								<div className="grid gap-4 sm:grid-cols-2">
									<div className="rounded-xl border border-border/60 bg-secondary/30 p-4">
										<h4 className="mb-3 flex items-center gap-2 text-sm font-medium">
											<FingerprintIcon className="h-4 w-4 text-primary" />
											<Trans>Device Information</Trans>
										</h4>
										<div className="space-y-2 text-sm">
											<div className="flex justify-between">
												<span className="text-muted-foreground"><Trans>Request ID</Trans></span>
												<span className="font-mono text-xs">{activationPreview.request_id}</span>
											</div>
											<div className="flex justify-between">
												<span className="text-muted-foreground"><Trans>Hostname</Trans></span>
												<span>{activationPreview.hostname || "-"}</span>
											</div>
											<div className="flex justify-between">
												<span className="text-muted-foreground"><Trans>Fingerprint</Trans></span>
												<span className="font-mono text-xs">{shorten(activationPreview.fingerprint, 14, 12)}</span>
											</div>
										</div>
									</div>

									<div className="rounded-xl border border-border/60 bg-secondary/30 p-4">
										<h4 className="mb-3 flex items-center gap-2 text-sm font-medium">
											<ServerIcon className="h-4 w-4 text-primary" />
											<Trans>Machine Factors</Trans>
										</h4>
										<div className="space-y-2 text-sm">
											<div className="flex justify-between">
												<span className="text-muted-foreground">machine_id</span>
												<span className="font-mono text-xs">{activationPreview.factors.machine_id || "-"}</span>
											</div>
											<div className="flex justify-between">
												<span className="text-muted-foreground">product_uuid</span>
												<span className="font-mono text-xs">{activationPreview.factors.product_uuid || "-"}</span>
											</div>
											<div className="flex justify-between">
												<span className="text-muted-foreground">board_serial</span>
												<span className="font-mono text-xs">{activationPreview.factors.board_serial || "-"}</span>
											</div>
										</div>
									</div>
								</div>

								{activationPreview.existing_activation && (
									<div className="rounded-xl border border-amber-200/60 bg-amber-50/50 p-4 dark:border-amber-900/40 dark:bg-amber-950/20">
										<div className="flex items-start gap-3">
											<InfoIcon className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />
											<p className="text-sm text-amber-800 dark:text-amber-200">
												<Trans>This request already exists. Saving will update the existing activation record.</Trans>
											</p>
										</div>
									</div>
								)}

								<Separator className="bg-border/60" />

								<div className="space-y-4">
									<h4 className="text-sm font-medium">
										<Trans>Customer Information</Trans>
									</h4>
									<div className="grid gap-4 sm:grid-cols-2">
										<div className="space-y-2">
											<Label htmlFor="import-customer" className="text-xs">
												<Trans>Customer *</Trans>
											</Label>
											<Input
												id="import-customer"
												value={importForm.customer}
												onChange={(event) =>
													setImportForm((current) => ({ ...current, customer: event.target.value }))
												}
												className="h-10 rounded-lg border-border/60"
												placeholder={t`Company name`}
											/>
										</div>
										<div className="space-y-2">
											<Label htmlFor="import-tenant" className="text-xs">
												<Trans>Tenant</Trans>
											</Label>
											<Input
												id="import-tenant"
												value={importForm.tenant}
												onChange={(event) =>
													setImportForm((current) => ({ ...current, tenant: event.target.value }))
												}
												className="h-10 rounded-lg border-border/60"
											/>
										</div>
										<div className="space-y-2">
											<Label htmlFor="import-project" className="text-xs">
												<Trans>Project</Trans>
											</Label>
											<Input
												id="import-project"
												value={importForm.project_name}
												onChange={(event) =>
													setImportForm((current) => ({ ...current, project_name: event.target.value }))
												}
												className="h-10 rounded-lg border-border/60"
											/>
										</div>
										<div className="space-y-2">
											<Label htmlFor="import-site" className="text-xs">
												<Trans>Site</Trans>
											</Label>
											<Input
												id="import-site"
												value={importForm.site_name}
												onChange={(event) =>
													setImportForm((current) => ({ ...current, site_name: event.target.value }))
												}
												className="h-10 rounded-lg border-border/60"
											/>
										</div>
									</div>
									<div className="space-y-2">
										<Label htmlFor="import-remarks" className="text-xs">
											<Trans>Remarks</Trans>
										</Label>
										<Textarea
											id="import-remarks"
											value={importForm.remarks}
											onChange={(event) =>
												setImportForm((current) => ({ ...current, remarks: event.target.value }))
											}
											className="min-h-20 rounded-lg border-border/60"
										/>
									</div>
								</div>
							</div>
						)}
					</div>

					<Separator className="bg-border/60" />

					<DialogFooter className="gap-2 p-6 pt-4">
						{activationPreview ? (
							<>
								<Button
									variant="outline"
									onClick={() => setActivationPreview(null)}
									className="h-10 rounded-lg border-border/60 px-4"
								>
									<Trans>Back</Trans>
								</Button>
								<Button
									onClick={handleImportActivation}
									disabled={isImporting}
									className="h-10 gap-2 rounded-lg bg-primary px-5"
								>
									{isImporting ? (
										<Loader2Icon className="h-4 w-4 animate-spin" />
									) : (
										<FileUpIcon className="h-4 w-4" />
									)}
									<Trans>Save Activation</Trans>
								</Button>
							</>
						) : (
							<Button
								onClick={handlePreviewActivation}
								disabled={isPreviewing || !activationContent.trim()}
								className="h-10 gap-2 rounded-lg bg-primary px-5"
							>
								{isPreviewing ? (
									<Loader2Icon className="h-4 w-4 animate-spin" />
								) : (
									<FileSearchIcon className="h-4 w-4" />
								)}
								<Trans>Parse & Continue</Trans>
							</Button>
						)}
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{/* Issue Dialog */}
			<Dialog open={isIssueDialogOpen} onOpenChange={setIsIssueDialogOpen}>
				<DialogContent className="max-w-2xl rounded-2xl border-border/60 p-0">
					<div className="p-6 pb-4">
						<DialogHeader className="space-y-3">
							<div className="flex items-center gap-3">
								<div className="flex h-10 w-10 items-center justify-center rounded-xl bg-emerald-500/10">
									<KeyIcon className="h-5 w-5 text-emerald-600" />
								</div>
								<DialogTitle className="text-xl font-semibold">
									<Trans>Issue License</Trans>
								</DialogTitle>
							</div>
							<DialogDescription className="text-sm text-muted-foreground leading-relaxed">
								<Trans>
									Generate a new signed license for the selected device activation.
								</Trans>
							</DialogDescription>
						</DialogHeader>
					</div>

					<Separator className="bg-border/60" />

					<div className="p-6 space-y-6">
						{/* Device Info Card */}
						<div className="rounded-xl border border-border/60 bg-secondary/30 p-4">
							<div className="grid gap-3 text-sm">
								<div className="flex items-center gap-3">
									<FingerprintIcon className="h-4 w-4 text-muted-foreground" />
									<span className="text-muted-foreground"><Trans>Activation</Trans>:</span>
									<span className="font-mono text-xs">{selectedIssueActivation?.request_id || "-"}</span>
								</div>
								<div className="flex items-center gap-3">
									<ServerIcon className="h-4 w-4 text-muted-foreground" />
									<span className="text-muted-foreground"><Trans>Hostname</Trans>:</span>
									<span>{selectedIssueActivation?.hostname || "-"}</span>
								</div>
								<div className="flex items-center gap-3">
									<TagIcon className="h-4 w-4 text-muted-foreground" />
									<span className="text-muted-foreground"><Trans>Fingerprint</Trans>:</span>
									<span className="font-mono text-xs">
										{selectedIssueActivation ? shorten(selectedIssueActivation.fingerprint, 14, 12) : "-"}
									</span>
								</div>
							</div>
						</div>

						<div className="grid gap-5 sm:grid-cols-2">
							<div className="space-y-2">
								<Label htmlFor="issue-customer" className="text-sm font-medium">
									<Trans>Customer</Trans>
								</Label>
								<Input
									id="issue-customer"
									value={issueForm.customer}
									onChange={(event) => setIssueForm((current) => ({ ...current, customer: event.target.value }))}
									className="h-11 rounded-xl border-border/60"
								/>
							</div>
							<div className="space-y-2">
								<Label htmlFor="issue-tenant" className="text-sm font-medium">
									<Trans>Tenant</Trans>
								</Label>
								<Input
									id="issue-tenant"
									value={issueForm.tenant}
									onChange={(event) => setIssueForm((current) => ({ ...current, tenant: event.target.value }))}
									className="h-11 rounded-xl border-border/60"
								/>
							</div>
							<div className="space-y-2">
								<Label htmlFor="issue-not-before" className="text-sm font-medium">
									<Trans>Valid From</Trans>
								</Label>
								<div className="relative">
									<CalendarIcon className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
									<Input
										id="issue-not-before"
										type="date"
										value={issueForm.notBefore}
										onChange={(event) => setIssueForm((current) => ({ ...current, notBefore: event.target.value }))}
										className="h-11 rounded-xl border-border/60 pl-10"
									/>
								</div>
							</div>
							<div className="space-y-2">
								<Label htmlFor="issue-not-after" className="text-sm font-medium">
									<Trans>Valid Until</Trans>
								</Label>
								<div className="relative">
									<CalendarIcon className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
									<Input
										id="issue-not-after"
										type="date"
										value={issueForm.notAfter}
										onChange={(event) => setIssueForm((current) => ({ ...current, notAfter: event.target.value }))}
										className="h-11 rounded-xl border-border/60 pl-10"
									/>
								</div>
							</div>
						</div>

						<div className="space-y-3">
							<Label className="text-sm font-medium">
								<Trans>Licensed Models</Trans>
							</Label>
							<div className="rounded-xl border border-border/60 bg-secondary/20 p-4">
								{(signing.model_names ?? []).length === 0 ? (
									<div className="flex items-center gap-3 py-4 text-sm text-muted-foreground">
										<InfoIcon className="h-4 w-4" />
										<Trans>No manifest models available.</Trans>
									</div>
								) : (
									<div className="grid gap-3 sm:grid-cols-2">
										{signing.model_names.map((modelName) => (
											<label
												key={modelName}
												className="flex cursor-pointer items-center gap-3 rounded-lg border border-border/40 bg-card p-3 transition-colors hover:border-border hover:bg-secondary/50"
											>
												<Checkbox
													checked={issueForm.modelNames.includes(modelName)}
													onCheckedChange={(value) => toggleIssueModel(modelName, Boolean(value))}
													className="rounded-md"
												/>
												<span className="text-sm font-medium">{modelName}</span>
											</label>
										))}
									</div>
								)}
							</div>
							<p className="text-xs text-muted-foreground">
								<Trans>Keep all models selected to issue the full license scope.</Trans>
							</p>
						</div>
					</div>

					<Separator className="bg-border/60" />

					<DialogFooter className="gap-2 p-6 pt-4">
						<Button
							variant="outline"
							onClick={() => setIsIssueDialogOpen(false)}
							className="h-10 rounded-lg border-border/60 px-4"
						>
							<Trans>Cancel</Trans>
						</Button>
						<Button
							onClick={handleIssueLicense}
							disabled={isIssuing || !signing.ready}
							className="h-10 gap-2 rounded-lg bg-emerald-600 px-5 hover:bg-emerald-600/90"
						>
							{isIssuing ? (
								<Loader2Icon className="h-4 w-4 animate-spin" />
							) : (
								<KeyIcon className="h-4 w-4" />
							)}
							<Trans>Generate License</Trans>
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			{/* Detail Sheet */}
			<Sheet open={isDetailOpen} onOpenChange={setIsDetailOpen}>
				<SheetContent className="w-full max-w-lg overflow-y-auto border-l border-border/60 p-0 sm:max-w-xl">
					<div className="p-6">
						<SheetHeader className="space-y-3 text-left">
							<div className="flex items-center gap-3">
								<div className="flex h-10 w-10 items-center justify-center rounded-xl bg-primary/10">
									<FileSearchIcon className="h-5 w-5 text-primary" />
								</div>
								<SheetTitle className="text-xl font-semibold">
									<Trans>Activation Details</Trans>
								</SheetTitle>
							</div>
							<SheetDescription className="text-sm text-muted-foreground">
								<Trans>Review device information and manage the current license.</Trans>
							</SheetDescription>
						</SheetHeader>
					</div>

					<Separator className="bg-border/60" />

					{detailActivation && (
						<div className="space-y-6 p-6">
							{/* Status Banner */}
							<div className="rounded-xl border border-border/60 bg-secondary/30 p-4">
								<div className="flex items-center justify-between">
									<span className="text-sm font-medium text-muted-foreground">
										<Trans>Status</Trans>
									</span>
									<Badge
										variant={activationStatusVariant(detailActivation.status)}
										className={`rounded-full px-3 py-1 text-xs font-medium ${
											detailActivation.status === "active"
												? "bg-emerald-500/10 text-emerald-700 border-emerald-500/20"
												: detailActivation.status === "disabled" || detailActivation.status === "revoked"
													? "bg-red-500/10 text-red-700 border-red-500/20"
													: "bg-amber-500/10 text-amber-700 border-amber-500/20"
										}`}
									>
										{activationStatusLabel(detailActivation.status)}
									</Badge>
								</div>
							</div>

							{/* Info Sections */}
							<div className="rounded-xl border border-border/60 bg-card p-2">
								<InfoRow
									icon={Building2Icon}
									label={t`Customer`}
									value={detailActivation.customer}
								/>
								<Separator className="bg-border/40" />
								<InfoRow
									icon={TagIcon}
									label={t`Tenant`}
									value={detailActivation.tenant}
								/>
								<Separator className="bg-border/40" />
								<InfoRow
									icon={Building2Icon}
									label={t`Project / Site`}
									value={[detailActivation.project_name, detailActivation.site_name].filter(Boolean).join(" / ")}
								/>
							</div>

							<div className="rounded-xl border border-border/60 bg-card p-2">
								<InfoRow
									icon={ServerIcon}
									label={t`Hostname`}
									value={detailActivation.hostname}
								/>
								<Separator className="bg-border/40" />
								<InfoRow
									icon={FingerprintIcon}
									label={t`Request ID`}
									value={detailActivation.request_id}
									mono
								/>
								<Separator className="bg-border/40" />
								<InfoRow
									icon={TagIcon}
									label={t`Fingerprint`}
									value={detailActivation.fingerprint}
									mono
								/>
							</div>

							<div className="rounded-xl border border-border/60 bg-card p-2">
								<InfoRow
									icon={ServerIcon}
									label={t`Machine ID`}
									value={detailActivation.factors_json.machine_id}
								/>
								<Separator className="bg-border/40" />
								<InfoRow
									icon={FingerprintIcon}
									label={t`Product UUID`}
									value={detailActivation.factors_json.product_uuid}
									mono
								/>
								<Separator className="bg-border/40" />
								<InfoRow
									icon={TagIcon}
									label={t`Board Serial`}
									value={detailActivation.factors_json.board_serial}
								/>
							</div>

							<div className="rounded-xl border border-border/60 bg-card p-2">
								<InfoRow
									icon={ClockIcon}
									label={t`Last Issued`}
									value={detailActivation.last_issued_at ? formatShortDate(detailActivation.last_issued_at) : ""}
								/>
								<Separator className="bg-border/40" />
								<InfoRow
									icon={ClockIcon}
									label={t`Updated`}
									value={formatShortDate(detailActivation.updated)}
								/>
							</div>

							{/* Remarks */}
							<div className="space-y-3">
								<Label className="text-sm font-medium">
									<Trans>Remarks</Trans>
								</Label>
								<Textarea
									value={detailActivation.remarks || ""}
									readOnly
									className="min-h-20 rounded-xl border-border/60 bg-secondary/20"
								/>
							</div>

							{/* Current License */}
							<div className="space-y-3">
								<Label className="text-sm font-medium">
									<Trans>Current License</Trans>
								</Label>
								{hasCurrentLicense(detailActivation) ? (
									<div className="rounded-xl border border-emerald-200/60 bg-emerald-50/30 p-4 dark:border-emerald-900/40 dark:bg-emerald-950/20">
										<div className="space-y-3">
											<div className="grid gap-2 text-sm">
												<div className="flex justify-between">
													<span className="text-muted-foreground"><Trans>Issued</Trans></span>
													<span>{detailActivation.last_issued_at ? formatShortDate(detailActivation.last_issued_at) : "-"}</span>
												</div>
												<div className="flex justify-between">
													<span className="text-muted-foreground"><Trans>Expires</Trans></span>
													<span>{detailActivation.current_not_after ? formatShortDate(detailActivation.current_not_after) : "-"}</span>
												</div>
												<div className="flex justify-between">
													<span className="text-muted-foreground"><Trans>Models</Trans></span>
													<span>{detailActivation.current_models_json?.length ?? 0}</span>
												</div>
											</div>
											<div className="flex flex-wrap gap-2">
												{(detailActivation.current_models_json ?? []).map((item) => (
													<Badge key={item.name} variant="outline" className="rounded-full">
														{item.name}
													</Badge>
												))}
											</div>
											<Button
												variant="outline"
												size="sm"
												onClick={() => handleExportArtifact(detailActivation.current_license_id!)}
												className="gap-2 rounded-lg border-emerald-200/60 hover:bg-emerald-50/50"
											>
												<DownloadIcon className="h-4 w-4" />
												<Trans>Export License</Trans>
											</Button>
										</div>
									</div>
								) : (
									<div className="rounded-xl border border-dashed border-border/60 bg-secondary/20 p-6 text-center">
										<XCircleIcon className="mx-auto h-8 w-8 text-muted-foreground/40" />
										<p className="mt-2 text-sm text-muted-foreground">
											<Trans>No license has been issued for this activation yet.</Trans>
										</p>
									</div>
								)}
							</div>

							{/* Raw Payloads */}
							<div className="space-y-3">
								<Label className="text-sm font-medium">
									<Trans>Activation Payload</Trans>
								</Label>
								<Textarea
									value={JSON.stringify(detailActivation.activation_payload ?? {}, null, 2)}
									readOnly
									className="min-h-40 rounded-xl border-border/60 font-mono text-xs leading-relaxed"
								/>
							</div>

							{detailActivation.current_license_payload && (
								<div className="space-y-3">
									<Label className="text-sm font-medium">
										<Trans>License Payload</Trans>
									</Label>
									<Textarea
										value={JSON.stringify(detailActivation.current_license_payload ?? {}, null, 2)}
										readOnly
										className="min-h-40 rounded-xl border-border/60 font-mono text-xs leading-relaxed"
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

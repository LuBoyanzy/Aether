// api-tests.tsx 渲染接口管理页面，提供合集/用例管理、执行与历史查看。
// 页面数据来自 Hub api-tests 接口与 PocketBase 集合。
import { Trans } from "@lingui/react/macro"
import { t } from "@lingui/core/macro"
import {
	CalendarIcon,
	DownloadIcon,
	EditIcon,
	HourglassIcon,
	LoaderCircleIcon,
	MoreHorizontalIcon,
	PlusIcon,
	PlayIcon,
	RefreshCwIcon,
	TimerIcon,
	Trash2Icon,
	UploadIcon,
	FolderIcon,
} from "lucide-react"
import { memo, useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent } from "react"
import { ActiveAlerts } from "@/components/active-alerts"
import { FooterRepoLink } from "@/components/footer-repo-link"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { InputTags } from "@/components/ui/input-tags"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { toast } from "@/components/ui/use-toast"
import {
	createApiTestCase,
	createApiTestCollection,
	deleteApiTestCase,
	deleteApiTestCollection,
	fetchApiTestSchedule,
	exportApiTests,
	importApiTests,
	listApiTestCases,
	listApiTestCollections,
	listApiTestRuns,
	runAllApiTests,
	runApiTestCase,
	runApiTestCollection,
	updateApiTestCase,
	updateApiTestCollection,
	updateApiTestSchedule,
} from "@/lib/api-tests"
import { BRAND_NAME, cn, formatDurationMs, formatShortDate } from "@/lib/utils"
import type {
	ApiTestBodyType,
	ApiTestCaseRecord,
	ApiTestCollectionRecord,
	ApiTestExportPayload,
	ApiTestImportMode,
	ApiTestKeyValue,
	ApiTestMethod,
	ApiTestRunItem,
	ApiTestRunResult,
	ApiTestScheduleConfig,
} from "@/types"

type CollectionDraft = {
	id?: string
	name: string
	description: string
	base_url: string
	sort_order: number
	tags: string[]
}

type CaseDraft = {
	id?: string
	collection: string
	name: string
	method: ApiTestMethod
	url: string
	description: string
	headers: ApiTestKeyValue[]
	params: ApiTestKeyValue[]
	body_type: ApiTestBodyType
	body: string
	expected_status: number
	timeout_ms: number
	schedule_enabled: boolean
	schedule_minutes: number
	sort_order: number
	tags: string[]
	alert_threshold: number
}

const methodOptions: ApiTestMethod[] = ["GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"]
const bodyTypeOptions: ApiTestBodyType[] = ["json", "text", "form"]
const ALL_FILTER_VALUE = "__all__"

const toFilterSelectValue = (value: string) => (value ? value : ALL_FILTER_VALUE)
const fromFilterSelectValue = (value: string) => (value === ALL_FILTER_VALUE ? "" : value)

const emptyCollectionDraft: CollectionDraft = {
	name: "",
	description: "",
	base_url: "",
	sort_order: 0,
	tags: [],
}

const emptyCaseDraft: CaseDraft = {
	collection: "",
	name: "",
	method: "GET",
	url: "",
	description: "",
	headers: [],
	params: [],
	body_type: "json",
	body: "",
	expected_status: 200,
	timeout_ms: 15000,
	schedule_enabled: false,
	schedule_minutes: 5,
	sort_order: 0,
	tags: [],
	alert_threshold: 1,
}

const emptyKeyValue: ApiTestKeyValue = { key: "", value: "", enabled: true }

function normalizeKeyValues(value: unknown): ApiTestKeyValue[] {
	if (!Array.isArray(value)) {
		return []
	}
	return value.map((item) => ({
		key: typeof item?.key === "string" ? item.key : String(item?.key ?? ""),
		value: typeof item?.value === "string" ? item.value : String(item?.value ?? ""),
		enabled: item?.enabled !== false,
	}))
}

function normalizeTags(value: unknown): string[] {
	if (!Array.isArray(value)) {
		return []
	}
	return value.map((item) => String(item ?? "")).filter((item) => item.trim() !== "")
}

function parseFormBody(value: string): { items: ApiTestKeyValue[]; error?: string } {
	if (!value.trim()) {
		return { items: [] }
	}
	try {
		const parsed = JSON.parse(value)
		if (Array.isArray(parsed)) {
			const items = parsed.map((item) => ({
				key: typeof item?.key === "string" ? item.key : String(item?.key ?? ""),
				value: typeof item?.value === "string" ? item.value : String(item?.value ?? ""),
				enabled: item?.enabled !== false,
			}))
			return { items }
		}
		if (parsed && typeof parsed === "object") {
			const items = Object.entries(parsed).map(([key, value]) => ({
				key,
				value: String(value ?? ""),
				enabled: true,
			}))
			return { items }
		}
		return { items: [], error: "Form body must be JSON object or array." }
	} catch (error) {
		return { items: [], error: error instanceof Error ? error.message : String(error) }
	}
}

function formatDuration(value?: number | null) {
	if (value === undefined || value === null) {
		return "-"
	}
	if (value >= 1000) {
		return `${(value / 1000).toFixed(2)}s`
	}
	return `${value}ms`
}

function formatRunSource(source: ApiTestRunItem["source"]) {
	return source === "schedule" ? t`Schedule` : t`Run`
}

function CaseStatusBadge({ record }: { record: ApiTestCaseRecord }) {
	if (!record.last_run_at) {
		return (
			<Badge variant="secondary">
				<Trans>Unknown</Trans>
			</Badge>
		)
	}
	if (record.last_success === true) {
		return (
			<Badge variant="success">
				<Trans>Success</Trans>
			</Badge>
		)
	}
	if (record.last_success === false) {
		return (
			<Badge variant="danger">
				<Trans>Failed</Trans>
			</Badge>
		)
	}
	return (
		<Badge variant="secondary">
			<Trans>Unknown</Trans>
		</Badge>
	)
}

function MethodBadge({ method, className }: { method: string; className?: string }) {
	let variant: "default" | "secondary" | "destructive" | "outline" | "success" | "danger" = "outline"
	// Map methods to approximate intent colors
	switch (method) {
		case "GET":
			variant = "secondary" // Blue-ish in many themes or gray
			break
		case "POST":
			variant = "success" // Green usually implies creation/action
			break
		case "PUT":
		case "PATCH":
			variant = "secondary" // Often yellow/orange, but secondary is safe
			break
		case "DELETE":
			variant = "danger" // Red
			break
	}
	// Fallback/Custom logic can be added if we have more specific Badge variants
	// Using outline for less emphasis if needed, but here we want color.
	// Actually, let's use outline for GET and solid/colored for others to distinguish.
	// But standard Badge variants: default, secondary, destructive, outline.
	// Beszel codebase seems to have success/danger variants (see CaseStatusBadge).

	return (
		<Badge variant={variant} className={cn("font-mono text-xs", className)}>
			{method}
		</Badge>
	)
}

function KeyValueEditor({
	value,
	onChange,
	emptyLabel,
}: {
	value: ApiTestKeyValue[]
	onChange: (next: ApiTestKeyValue[]) => void
	emptyLabel: string
}) {
	const updateItem = (index: number, patch: Partial<ApiTestKeyValue>) => {
		onChange(value.map((item, idx) => (idx === index ? { ...item, ...patch } : item)))
	}
	const removeItem = (index: number) => {
		onChange(value.filter((_, idx) => idx !== index))
	}
	const addItem = () => {
		onChange([...value, { ...emptyKeyValue }])
	}
	return (
		<div className="space-y-2">
			{value.length === 0 && <div className="text-xs text-muted-foreground">{emptyLabel}</div>}
			{value.map((item, index) => (
				<div key={`kv-${index}`} className="flex items-center gap-2">
					<Checkbox checked={item.enabled} onCheckedChange={(checked) => updateItem(index, { enabled: !!checked })} />
					<Input
						value={item.key}
						onChange={(event) => updateItem(index, { key: event.target.value })}
						placeholder="Key"
					/>
					<Input
						value={item.value}
						onChange={(event) => updateItem(index, { value: event.target.value })}
						placeholder="Value"
					/>
					<Button variant="ghost" size="icon" onClick={() => removeItem(index)}>
						<Trash2Icon className="h-4 w-4" />
					</Button>
				</div>
			))}
			<Button variant="outline" size="sm" onClick={addItem}>
				<PlusIcon className="me-1 h-4 w-4" />
				<Trans>Add</Trans>
			</Button>
		</div>
	)
}

export default memo(function ApiTestsPage() {
	const [activeTab, setActiveTab] = useState("cases")
	const [collections, setCollections] = useState<ApiTestCollectionRecord[]>([])
	const [cases, setCases] = useState<ApiTestCaseRecord[]>([])
	const [runs, setRuns] = useState<ApiTestRunItem[]>([])
	const [schedule, setSchedule] = useState<ApiTestScheduleConfig | null>(null)
	const [selectedCollectionId, setSelectedCollectionId] = useState("")
	const [historyCollectionId, setHistoryCollectionId] = useState("")
	const [historyCaseId, setHistoryCaseId] = useState("")
	const [collectionDialogOpen, setCollectionDialogOpen] = useState(false)
	const [caseDialogOpen, setCaseDialogOpen] = useState(false)
	const [caseDetailOpen, setCaseDetailOpen] = useState(false)
	const [caseDetailId, setCaseDetailId] = useState<string | null>(null)
	const [runResultOpen, setRunResultOpen] = useState(false)
	const [runResult, setRunResult] = useState<ApiTestRunResult | null>(null)
	const [collectionDraft, setCollectionDraft] = useState<CollectionDraft>({ ...emptyCollectionDraft })
	const [caseDraft, setCaseDraft] = useState<CaseDraft>({ ...emptyCaseDraft })
	const [formItems, setFormItems] = useState<ApiTestKeyValue[]>([])
	const [formBodyError, setFormBodyError] = useState("")
	const [saving, setSaving] = useState(false)
	const [importDialogOpen, setImportDialogOpen] = useState(false)
	const [importMode, setImportMode] = useState<ApiTestImportMode>("skip")
	const [importFile, setImportFile] = useState<File | null>(null)
	const [importing, setImporting] = useState(false)
	const [exporting, setExporting] = useState(false)
	const importFileRef = useRef<HTMLInputElement | null>(null)

	useEffect(() => {
		document.title = BRAND_NAME
	}, [])

	const handleApiError = useCallback((title: string, error: unknown, context?: Record<string, unknown>) => {
		console.error(title, { error, ...context })
		toast({
			title,
			description: error instanceof Error ? error.message : String(error),
			variant: "destructive",
		})
		throw error instanceof Error ? error : new Error(String(error))
	}, [])

	const refreshCollections = useCallback(async () => {
		try {
			const data = await listApiTestCollections()
			setCollections(data)
			setSelectedCollectionId((current) => {
				if (current && data.some((item) => item.id === current)) {
					return current
				}
				return data[0]?.id ?? ""
			})
		} catch (error) {
			handleApiError(t`Failed to load collections`, error)
		}
	}, [handleApiError])

	const refreshCases = useCallback(async () => {
		try {
			const data = await listApiTestCases()
			setCases(data)
		} catch (error) {
			handleApiError(t`Failed to load cases`, error)
		}
	}, [handleApiError])

	const refreshSchedule = useCallback(async () => {
		try {
			const data = await fetchApiTestSchedule()
			setSchedule(data)
		} catch (error) {
			handleApiError(t`Failed to load schedule`, error)
		}
	}, [handleApiError])

	const refreshRuns = useCallback(
		async (collectionId?: string, caseId?: string) => {
			try {
				const data = await listApiTestRuns({
					collectionId,
					caseId,
					perPage: 50,
				})
				setRuns(data.items)
			} catch (error) {
				handleApiError(t`Failed to load run history`, error)
			}
		},
		[handleApiError]
	)

	useEffect(() => {
		refreshCollections()
		refreshCases()
		refreshSchedule()
	}, [refreshCollections, refreshCases, refreshSchedule])

	useEffect(() => {
		if (activeTab === "history") {
			refreshRuns(historyCollectionId || undefined, historyCaseId || undefined)
		}
	}, [activeTab, historyCollectionId, historyCaseId, refreshRuns])

	const filteredCases = useMemo(() => {
		if (!selectedCollectionId) {
			return cases
		}
		return cases.filter((item) => item.collection === selectedCollectionId)
	}, [cases, selectedCollectionId])

	const caseNameById = useMemo(() => new Map(cases.map((item) => [item.id, item.name])), [cases])
	const collectionById = useMemo(() => new Map(collections.map((item) => [item.id, item])), [collections])
	const caseSummary = useMemo(() => {
		let scheduled = 0
		let ok = 0
		let fail = 0
		for (const item of cases) {
			if (item.schedule_enabled) {
				scheduled += 1
			}
			if (item.last_success === true) {
				ok += 1
			}
			if (item.last_success === false) {
				fail += 1
			}
		}
		return { scheduled, ok, fail, total: cases.length }
	}, [cases])
	const collectionStats = useMemo(() => {
		const stats = new Map<string, { total: number; ok: number; fail: number }>()
		for (const item of cases) {
			const current = stats.get(item.collection) ?? { total: 0, ok: 0, fail: 0 }
			current.total += 1
			if (item.last_success === true) {
				current.ok += 1
			}
			if (item.last_success === false) {
				current.fail += 1
			}
			stats.set(item.collection, current)
		}
		return stats
	}, [cases])

	const historyCases = useMemo(() => {
		if (!historyCollectionId) {
			return cases
		}
		return cases.filter((item) => item.collection === historyCollectionId)
	}, [cases, historyCollectionId])

	const caseDetailRecord = useMemo(() => {
		if (!caseDetailId) {
			return null
		}
		return cases.find((item) => item.id === caseDetailId) ?? null
	}, [caseDetailId, cases])

	useEffect(() => {
		if (historyCaseId && !historyCases.some((item) => item.id === historyCaseId)) {
			setHistoryCaseId("")
		}
	}, [historyCaseId, historyCases])

	const openNewCollection = () => {
		setCollectionDraft({ ...emptyCollectionDraft })
		setCollectionDialogOpen(true)
	}

	const openEditCollection = (record: ApiTestCollectionRecord) => {
		setCollectionDraft({
			id: record.id,
			name: record.name,
			description: record.description ?? "",
			base_url: record.base_url ?? "",
			sort_order: record.sort_order ?? 0,
			tags: normalizeTags(record.tags),
		})
		setCollectionDialogOpen(true)
	}

	const saveCollection = async () => {
		if (!collectionDraft.name.trim()) {
			handleApiError(t`Collection name is required`, new Error("Collection name is required"))
		}
		setSaving(true)
		try {
			const payload = {
				name: collectionDraft.name.trim(),
				description: collectionDraft.description.trim(),
				base_url: collectionDraft.base_url.trim(),
				sort_order: collectionDraft.sort_order,
				tags: collectionDraft.tags,
			}
			if (collectionDraft.id) {
				await updateApiTestCollection(collectionDraft.id, payload)
			} else {
				await createApiTestCollection(payload)
			}
			toast({ title: t`Collection saved` })
			setCollectionDialogOpen(false)
			await refreshCollections()
		} catch (error) {
			handleApiError(t`Failed to save collection`, error)
		} finally {
			setSaving(false)
		}
	}

	const deleteCollection = async (record: ApiTestCollectionRecord) => {
		if (!window.confirm(t`Delete this collection?`)) {
			return
		}
		try {
			await deleteApiTestCollection(record.id)
			toast({ title: t`Collection deleted` })
			await refreshCollections()
			await refreshCases()
		} catch (error) {
			handleApiError(t`Failed to delete collection`, error, { id: record.id })
		}
	}

	const openNewCase = () => {
		setCaseDraft({ ...emptyCaseDraft, collection: selectedCollectionId || "" })
		setFormItems([])
		setFormBodyError("")
		setCaseDialogOpen(true)
	}

	const openEditCase = (record: ApiTestCaseRecord) => {
		const parsedForm = parseFormBody(record.body ?? "")
		setCaseDraft({
			id: record.id,
			collection: record.collection,
			name: record.name,
			method: record.method,
			url: record.url,
			description: record.description ?? "",
			headers: normalizeKeyValues(record.headers),
			params: normalizeKeyValues(record.params),
			body_type: record.body_type,
			body: record.body ?? "",
			expected_status: record.expected_status ?? 200,
			timeout_ms: record.timeout_ms ?? 15000,
			schedule_enabled: record.schedule_enabled ?? false,
			schedule_minutes: record.schedule_minutes ?? 5,
			sort_order: record.sort_order ?? 0,
			tags: normalizeTags(record.tags),
			alert_threshold: record.alert_threshold ?? 1,
		})
		setFormItems(parsedForm.items)
		setFormBodyError(parsedForm.error ?? "")
		setCaseDialogOpen(true)
	}

	const saveCase = async () => {
		if (!caseDraft.collection) {
			handleApiError(t`Collection is required`, new Error("Collection is required"))
		}
		if (!caseDraft.name.trim()) {
			handleApiError(t`Case name is required`, new Error("Case name is required"))
		}
		if (!caseDraft.url.trim()) {
			handleApiError(t`Request URL is required`, new Error("Request URL is required"))
		}
		if (caseDraft.expected_status <= 0) {
			handleApiError(t`Expected status must be greater than 0`, new Error("Invalid expected status"))
		}
		if (caseDraft.timeout_ms <= 0) {
			handleApiError(t`Timeout must be greater than 0`, new Error("Invalid timeout"))
		}
		if (caseDraft.schedule_enabled && caseDraft.schedule_minutes <= 0) {
			handleApiError(t`Schedule minutes must be greater than 0`, new Error("Invalid schedule minutes"))
		}
		if (caseDraft.alert_threshold <= 0) {
			handleApiError(t`Alert threshold must be greater than 0`, new Error("Invalid alert threshold"))
		}
		let body = caseDraft.body
		if (caseDraft.body_type === "json" && body.trim()) {
			try {
				JSON.parse(body)
			} catch (error) {
				handleApiError(t`JSON body is invalid`, error)
			}
		}
		if (caseDraft.body_type === "form") {
			if (formBodyError) {
				handleApiError(t`Form body is invalid`, new Error(formBodyError))
			}
			body = JSON.stringify(formItems)
		}
		setSaving(true)
		try {
			const payload = {
				collection: caseDraft.collection,
				name: caseDraft.name.trim(),
				method: caseDraft.method,
				url: caseDraft.url.trim(),
				description: caseDraft.description.trim(),
				headers: caseDraft.headers,
				params: caseDraft.params,
				body_type: caseDraft.body_type,
				body,
				expected_status: caseDraft.expected_status,
				timeout_ms: caseDraft.timeout_ms,
				schedule_enabled: caseDraft.schedule_enabled,
				schedule_minutes: caseDraft.schedule_minutes,
				sort_order: caseDraft.sort_order,
				tags: caseDraft.tags,
				alert_threshold: caseDraft.alert_threshold,
			}
			if (caseDraft.id) {
				await updateApiTestCase(caseDraft.id, payload)
			} else {
				await createApiTestCase(payload)
			}
			toast({ title: t`Case saved` })
			setCaseDialogOpen(false)
			await refreshCases()
		} catch (error) {
			handleApiError(t`Failed to save case`, error)
		} finally {
			setSaving(false)
		}
	}

	const deleteCase = async (record: ApiTestCaseRecord) => {
		if (!window.confirm(t`Delete this case?`)) {
			return
		}
		try {
			await deleteApiTestCase(record.id)
			toast({ title: t`Case deleted` })
			await refreshCases()
		} catch (error) {
			handleApiError(t`Failed to delete case`, error, { id: record.id })
		}
	}

	const updateCaseToggle = async (record: ApiTestCaseRecord, patch: Partial<ApiTestCaseRecord>) => {
		try {
			await updateApiTestCase(record.id, patch)
			await refreshCases()
		} catch (error) {
			handleApiError(t`Failed to update case`, error, { id: record.id })
		}
	}

	const handleRunCase = async (record: ApiTestCaseRecord) => {
		try {
			const result = await runApiTestCase(record.id)
			setRunResult(result)
			setRunResultOpen(true)
			toast({ title: t`Case executed` })
			await refreshCases()
			await refreshRuns(historyCollectionId || undefined, historyCaseId || undefined)
		} catch (error) {
			handleApiError(t`Failed to run case`, error, { id: record.id })
		}
	}

	const handleRunCollection = async () => {
		if (!selectedCollectionId) {
			handleApiError(t`Collection is required`, new Error("Collection is required"))
		}
		try {
			await runApiTestCollection(selectedCollectionId)
			toast({ title: t`Collection executed` })
			await refreshCases()
			await refreshRuns(historyCollectionId || undefined, historyCaseId || undefined)
		} catch (error) {
			handleApiError(t`Failed to run collection`, error, { id: selectedCollectionId })
		}
	}

	const handleRunAll = async () => {
		try {
			await runAllApiTests()
			toast({ title: t`All cases executed` })
			await refreshCases()
			await refreshRuns(historyCollectionId || undefined, historyCaseId || undefined)
		} catch (error) {
			handleApiError(t`Failed to run all cases`, error)
		}
	}

	const openCaseDetail = (record: ApiTestCaseRecord) => {
		setCaseDetailId(record.id)
		setCaseDetailOpen(true)
	}

	const saveSchedule = async () => {
		if (!schedule) {
			handleApiError(t`Schedule not loaded`, new Error("Schedule not loaded"))
			return
		}
		if (schedule.intervalMinutes <= 0) {
			handleApiError(t`Interval must be greater than 0`, new Error("Invalid interval"))
		}
		if (schedule.historyRetentionDays <= 0) {
			handleApiError(t`Retention must be greater than 0`, new Error("Invalid retention"))
		}
		setSaving(true)
		try {
			const updated = await updateApiTestSchedule({
				enabled: schedule.enabled,
				intervalMinutes: schedule.intervalMinutes,
				alertEnabled: schedule.alertEnabled,
				alertOnRecover: schedule.alertOnRecover,
				historyRetentionDays: schedule.historyRetentionDays,
			})
			setSchedule(updated)
			toast({ title: t`Schedule saved` })
		} catch (error) {
			handleApiError(t`Failed to save schedule`, error)
		} finally {
			setSaving(false)
		}
	}

	const handleRefreshAll = async () => {
		await refreshCollections()
		await refreshCases()
		await refreshSchedule()
		if (activeTab === "history") {
			await refreshRuns(historyCollectionId || undefined, historyCaseId || undefined)
		}
	}

	const openImportDialog = useCallback(() => {
		setImportDialogOpen(true)
		setImportMode("skip")
		setImportFile(null)
		if (importFileRef.current) {
			importFileRef.current.value = ""
		}
	}, [])

	const closeImportDialog = useCallback(() => {
		setImportDialogOpen(false)
		setImportFile(null)
		setImportMode("skip")
		if (importFileRef.current) {
			importFileRef.current.value = ""
		}
	}, [])

	const handleImportFileChange = useCallback((event: ChangeEvent<HTMLInputElement>) => {
		const file = event.target.files?.[0] ?? null
		setImportFile(file)
	}, [])

	const handleExport = useCallback(async () => {
		setExporting(true)
		try {
			const payload = await exportApiTests()
			const blob = new Blob([JSON.stringify(payload, null, 2)], { type: "application/json" })
			const url = URL.createObjectURL(blob)
			const anchor = document.createElement("a")
			anchor.href = url
			anchor.download = "api_tests_export.json"
			anchor.click()
			URL.revokeObjectURL(url)
		} catch (error) {
			handleApiError(t`Failed to export API tests`, error)
		} finally {
			setExporting(false)
		}
	}, [handleApiError])

	const handleImport = useCallback(async () => {
		if (!importFile) {
			handleApiError(t`Select JSON file`, new Error("Import file missing"))
			return
		}
		setImporting(true)
		const text = await importFile.text()
		let data: ApiTestExportPayload
		try {
			data = JSON.parse(text) as ApiTestExportPayload
		} catch (error) {
			setImporting(false)
			handleApiError(t`JSON body is invalid`, error)
			return
		}
		try {
			await importApiTests({ mode: importMode, data })
			toast({ title: t`Import completed` })
			closeImportDialog()
			await handleRefreshAll()
		} catch (error) {
			handleApiError(t`Failed to import API tests`, error)
		} finally {
			setImporting(false)
		}
	}, [importFile, importMode, handleApiError, handleRefreshAll, closeImportDialog])

	return (
		<>
			<div className="grid gap-4">
				<ActiveAlerts />
				<Card className="p-6 @container w-full">
					<CardHeader className="p-0 mb-4">
						<div className="flex flex-wrap items-end justify-between gap-3">
							<div>
								<CardTitle className="mb-2">
									<Trans>API Tests</Trans>
								</CardTitle>
								<CardDescription>
									<Trans>Manage and run API tests.</Trans>
								</CardDescription>
							</div>
							<div className="flex items-center gap-2">
								<Button variant="outline" size="sm" onClick={handleRefreshAll}>
									<RefreshCwIcon className="me-2 h-4 w-4" />
									<Trans>Refresh</Trans>
								</Button>
								<Button size="sm" onClick={handleRunAll}>
									<PlayIcon className="me-2 h-4 w-4" />
									<Trans>Run All</Trans>
								</Button>
							</div>
						</div>
					</CardHeader>
					<div className="grid gap-4">
						<div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
							<Card className="p-4">
								<div className="text-xs text-muted-foreground">
									<Trans>Collections</Trans>
								</div>
								<div className="mt-1 text-2xl font-semibold">{collections.length}</div>
								<div className="mt-1 text-xs text-muted-foreground">
									<Trans>Cases</Trans>: {cases.length}
								</div>
							</Card>
							<Card className="p-4">
								<div className="text-xs text-muted-foreground">
									<Trans>Schedule</Trans>
								</div>
								<div className="mt-1 text-2xl font-semibold">{caseSummary.scheduled}</div>
								<div className="mt-1 text-xs text-muted-foreground">
									{schedule ? (
										<>
											<Trans>Interval (minutes)</Trans>: {schedule.intervalMinutes}
										</>
									) : (
										<Trans>Loading schedule...</Trans>
									)}
								</div>
							</Card>
							<Card className="p-4">
								<div className="text-xs text-muted-foreground">
									<Trans>Success</Trans>
								</div>
								<div className="mt-1 text-2xl font-semibold">{caseSummary.ok}</div>
							</Card>
							<Card className="p-4">
								<div className="text-xs text-muted-foreground">
									<Trans>Failed</Trans>
								</div>
								<div className="mt-1 text-2xl font-semibold">{caseSummary.fail}</div>
							</Card>
						</div>
						<Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
							<TabsList className="flex h-auto w-full flex-wrap justify-start gap-2 bg-transparent p-0">
								<TabsTrigger
									value="cases"
									className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
								>
									<Trans>Cases</Trans>
								</TabsTrigger>
								<TabsTrigger
									value="schedule"
									className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
								>
									<Trans>Schedule</Trans>
								</TabsTrigger>
								<TabsTrigger
									value="history"
									className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
								>
									<Trans>History</Trans>
								</TabsTrigger>
							</TabsList>
							<TabsContent value="cases" className="mt-4 animate-fade-in duration-300">
								<div className="grid gap-4 lg:grid-cols-[320px_1fr]">
									<Card className="flex h-full flex-col p-4">
										<div className="mb-4 flex items-center justify-between gap-2">
											<div className="space-y-0.5">
												<div className="text-sm font-semibold">
													<Trans>Collections</Trans>
												</div>
												<div className="text-xs text-muted-foreground">
													<Trans>Manage and run API tests.</Trans>
												</div>
											</div>
											<div className="flex items-center gap-1.5">
												<DropdownMenu>
													<DropdownMenuTrigger asChild>
														<Button variant="outline" size="icon" className="h-8 w-8">
															<MoreHorizontalIcon className="h-4 w-4" />
														</Button>
													</DropdownMenuTrigger>
													<DropdownMenuContent align="end">
														<DropdownMenuItem onClick={handleExport} disabled={exporting}>
															{exporting ? (
																<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
															) : (
																<DownloadIcon className="me-2 h-4 w-4" />
															)}
															<Trans>Export</Trans>
														</DropdownMenuItem>
														<DropdownMenuItem onClick={openImportDialog} disabled={importing}>
															<UploadIcon className="me-2 h-4 w-4" />
															<Trans>Import</Trans>
														</DropdownMenuItem>
													</DropdownMenuContent>
												</DropdownMenu>
												<Button size="sm" onClick={openNewCollection} className="h-8 px-3">
													<PlusIcon className="me-1.5 h-3.5 w-3.5" />
													<Trans>New</Trans>
												</Button>
											</div>
										</div>
										<div className="space-y-2">
											<div
												onClick={() => setSelectedCollectionId("")}
												className={cn(
													"group flex cursor-pointer items-center justify-between rounded-lg border px-3 py-3 text-sm transition-all hover:bg-accent/50",
													!selectedCollectionId ? "border-primary/50 bg-accent shadow-sm" : "border-transparent bg-card"
												)}
											>
												<div className="flex flex-col gap-1">
													<span className="font-semibold">
														<Trans>All collections</Trans>
													</span>
													<span className="text-xs text-muted-foreground">
														<Trans>Total cases</Trans>: {cases.length}
													</span>
												</div>
												<Badge variant="secondary" className="h-5 px-1.5 text-[10px] font-normal">
													{cases.length}
												</Badge>
											</div>
											{collections.length === 0 ? (
												<div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
													<Trans>No collections yet</Trans>
												</div>
											) : (
												collections.map((record) => {
													const stats = collectionStats.get(record.id)
													const isActive = selectedCollectionId === record.id
													return (
														<div
															key={record.id}
															role="button"
															tabIndex={0}
															onClick={() => setSelectedCollectionId(record.id)}
															onKeyDown={(event) => {
																if (event.key === "Enter" || event.key === " ") {
																	event.preventDefault()
																	setSelectedCollectionId(record.id)
																}
															}}
															className={cn(
																"group relative flex flex-col gap-3 rounded-xl border p-4 text-left transition-all hover:bg-accent/50",
																isActive ? "border-primary/20 bg-card shadow-md" : "border-transparent bg-transparent"
															)}
														>
															<div className="flex items-start justify-between gap-2">
																<div className="font-bold text-base text-foreground truncate pr-6">{record.name}</div>
																<div className="shrink-0 text-xs text-muted-foreground">
																	{stats?.total ?? 0} <Trans>cases</Trans>
																</div>
															</div>

															<div className="text-xs text-muted-foreground line-clamp-2 min-h-[1.5em]">
																{record.description || (
																	<span className="opacity-50">
																		<Trans>No detailed description...</Trans>
																	</span>
																)}
															</div>

															<div className="flex items-center justify-between mt-1">
																<div className="rounded bg-muted/50 px-1.5 py-0.5 text-[10px] font-mono text-muted-foreground truncate max-w-[120px]">
																	<Trans>BASE</Trans> {record.base_url || "..."}
																</div>
																<div className="flex items-center gap-3 text-[10px] font-medium shrink-0">
																	<div className="flex items-center gap-1 text-emerald-600 dark:text-emerald-500">
																		<div className="h-1.5 w-1.5 rounded-full bg-current" />
																		<Trans>Pass</Trans> {stats?.ok ?? 0}
																	</div>
																	<div className="flex items-center gap-1 text-red-600 dark:text-red-500">
																		<div className="h-1.5 w-1.5 rounded-full bg-current" />
																		<Trans>Fail</Trans> {stats?.fail ?? 0}
																	</div>
																</div>
															</div>

															<div className="absolute top-2 right-2 opacity-0 transition-opacity group-hover:opacity-100 data-[state=open]:opacity-100">
																<DropdownMenu>
																	<DropdownMenuTrigger asChild>
																		<Button variant="ghost" size="icon" className="h-6 w-6 text-muted-foreground">
																			<MoreHorizontalIcon className="h-4 w-4" />
																		</Button>
																	</DropdownMenuTrigger>
																	<DropdownMenuContent align="end">
																		<DropdownMenuItem
																			onClick={(event) => {
																				event.stopPropagation()
																				openEditCollection(record)
																			}}
																		>
																			<EditIcon className="me-2 h-4 w-4" />
																			<Trans>Edit</Trans>
																		</DropdownMenuItem>
																		<DropdownMenuSeparator />
																		<DropdownMenuItem
																			className="text-destructive focus:text-destructive"
																			onClick={(event) => {
																				event.stopPropagation()
																				deleteCollection(record)
																			}}
																		>
																			<Trash2Icon className="me-2 h-4 w-4" />
																			<Trans>Delete</Trans>
																		</DropdownMenuItem>
																	</DropdownMenuContent>
																</DropdownMenu>
															</div>
														</div>
													)
												})
											)}
										</div>
									</Card>
									<div className="grid gap-4">
										<Card className="flex flex-col gap-4 p-4 md:flex-row md:items-center md:justify-between mb-4">
											<div className="space-y-0.5">
												<div className="text-lg font-semibold">
													<Trans>Cases</Trans>
												</div>
												<div className="text-xs text-muted-foreground">
													<Trans>Manage and run API cases</Trans>
												</div>
											</div>
											<div className="flex items-center gap-2">
												<Button
													variant="outline"
													size="sm"
													className="h-9 border-dashed"
													onClick={handleRunCollection}
													disabled={!selectedCollectionId}
												>
													<PlayIcon className="me-2 h-4 w-4" />
													<Trans>Run Collection</Trans>
												</Button>
												<Button size="sm" className="h-9 shadow-sm" onClick={openNewCase}>
													<PlusIcon className="me-2 h-4 w-4" />
													<Trans>New Case</Trans>
												</Button>
											</div>
										</Card>

										<div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
											{filteredCases.length === 0 ? (
												<div className="col-span-full flex flex-col items-center justify-center gap-2 rounded-lg border border-dashed py-12 text-center text-sm text-muted-foreground">
													<div className="flex h-12 w-12 items-center justify-center rounded-full bg-muted/50">
														<MoreHorizontalIcon className="h-6 w-6 opacity-50" />
													</div>
													<p>
														<Trans>No cases yet</Trans>
													</p>
												</div>
											) : (
												filteredCases.map((record) => {
													const collection = collectionById.get(record.collection)
													return (
														<Card
															key={record.id}
															className="group relative flex flex-col justify-between overflow-hidden rounded-xl border bg-card text-card-foreground shadow-sm transition-all hover:shadow-md hover:border-primary/50"
														>
															<div className="flex flex-col gap-4 p-5">
																<div className="flex items-start justify-between gap-3">
																	<div className="flex items-center gap-2 min-w-0">
																		<MethodBadge method={record.method} className="shrink-0 shadow-sm" />
																		<div className="truncate font-bold text-base leading-none tracking-tight text-foreground/90">
																			{record.name}
																		</div>
																	</div>
																	{record.last_status !== undefined && record.last_status !== null && (
																		<Badge
																			variant="outline"
																			className={cn(
																				"h-5 px-1.5 text-[10px] font-medium border-transparent",
																				record.last_status >= 200 && record.last_status < 300
																					? "bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400"
																					: "bg-red-50 text-red-600 dark:bg-red-500/10 dark:text-red-400"
																			)}
																		>
																			{record.last_status >= 200 && record.last_status < 300 ? (
																				<Trans>Pass</Trans>
																			) : (
																				<Trans>Fail</Trans>
																			)}
																		</Badge>
																	)}
																</div>

																<div className="text-xs text-muted-foreground line-clamp-2 min-h-[1.5em]">
																	{record.description || (
																		<span className="opacity-50">
																			<Trans>No detailed description...</Trans>
																		</span>
																	)}
																</div>

																<div className="rounded-lg bg-muted/30 p-3 flex flex-col gap-2 text-xs font-mono text-muted-foreground">
																	<div className="flex gap-2 items-center">
																		<span className="w-10 opacity-40 text-[10px] font-bold uppercase tracking-wider">
																			<Trans>ENV</Trans>
																		</span>
																		<span className="truncate opacity-80">{collection?.base_url || "http://..."}</span>
																	</div>
																	<div className="flex gap-2 items-center">
																		<span className="w-10 opacity-40 text-[10px] font-bold uppercase tracking-wider">
																			<Trans>URL</Trans>
																		</span>
																		<span className="truncate opacity-80">{record.url}</span>
																	</div>
																	<div className="flex gap-2 items-center">
																		<span className="w-10 opacity-40 text-[10px] font-bold uppercase tracking-wider">
																			<Trans>EXPECT</Trans>
																		</span>
																		<Badge
																			variant="outline"
																			className="h-4 px-1 text-[9px] text-muted-foreground bg-background/50 border-muted-foreground/20"
																		>
																			<Trans>STATUS</Trans> {record.expected_status}
																		</Badge>
																	</div>
																</div>

																<div className="flex items-center gap-4 text-xs text-muted-foreground/60 mt-1">
																	<div className="flex items-center gap-1.5">
																		<TimerIcon className="h-3 w-3" />
																		<span className="font-mono">{formatDurationMs(record.last_duration_ms ?? 0)}</span>
																	</div>
																	<div className="flex items-center gap-1.5">
																		<CalendarIcon className="h-3 w-3" />
																		<span>{record.last_run_at ? formatShortDate(record.last_run_at) : "-"}</span>
																	</div>
																</div>
															</div>

															<div className="absolute top-2 right-2 opacity-0 transition-opacity group-hover:opacity-100 data-[state=open]:opacity-100">
																<DropdownMenu>
																	<DropdownMenuTrigger asChild>
																		<Button variant="ghost" size="icon" className="h-6 w-6 text-muted-foreground">
																			<MoreHorizontalIcon className="h-4 w-4" />
																		</Button>
																	</DropdownMenuTrigger>
																	<DropdownMenuContent align="end">
																		<DropdownMenuItem
																			onClick={(event) => {
																				event.stopPropagation()
																				openEditCase(record)
																			}}
																		>
																			<EditIcon className="me-2 h-4 w-4" />
																			<Trans>Edit</Trans>
																		</DropdownMenuItem>
																		<DropdownMenuSeparator />
																		<DropdownMenuItem
																			className="text-destructive focus:text-destructive"
																			onClick={(event) => {
																				event.stopPropagation()
																				deleteCase(record)
																			}}
																		>
																			<Trash2Icon className="me-2 h-4 w-4" />
																			<Trans>Delete</Trans>
																		</DropdownMenuItem>
																	</DropdownMenuContent>
																</DropdownMenu>
															</div>

															<div className="flex items-center justify-between border-t bg-muted/5 px-4 py-3 text-xs transition-colors group-hover:bg-muted/10">
																<div className="flex items-center gap-2" title={t`Auto-run schedule`}>
																	<Switch
																		checked={record.schedule_enabled}
																		onCheckedChange={(checked) =>
																			updateCaseToggle(record, {
																				schedule_enabled: !!checked,
																			})
																		}
																		className="scale-75 data-[state=checked]:bg-primary"
																	/>
																	<span
																		className={cn(
																			"text-[10px] uppercase tracking-wider font-semibold transition-colors",
																			record.schedule_enabled ? "text-primary/80" : "text-muted-foreground/50"
																		)}
																	>
																		<Trans>Auto</Trans>
																	</span>
																</div>

																<div className="flex items-center gap-2">
																	<Button
																		variant="outline"
																		size="sm"
																		className="h-7 px-3 text-xs"
																		onClick={() => openCaseDetail(record)}
																	>
																		<Trans>Detail</Trans>
																	</Button>
																	<Button
																		size="sm"
																		className="h-7 px-3 text-xs font-medium shadow-sm transition-all hover:shadow-md hover:bg-primary hover:text-primary-foreground"
																		onClick={() => handleRunCase(record)}
																	>
																		<PlayIcon className="me-1.5 h-3 w-3" />
																		<Trans>Run</Trans>
																	</Button>
																</div>
															</div>
														</Card>
													)
												})
											)}
										</div>
									</div>
								</div>
							</TabsContent>
							<TabsContent value="schedule" className="mt-4 animate-fade-in duration-300">
								<Card className="p-4">
									<div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
										<div>
											<div className="text-sm font-semibold">
												<Trans>Schedule</Trans>
											</div>
											<div className="text-xs text-muted-foreground">
												<Trans>Only scheduled cases will run</Trans>
											</div>
										</div>
									</div>
									<div className="mt-4 space-y-4">
										{schedule ? (
											<>
												<div className="flex items-center justify-between">
													<div>
														<Label>
															<Trans>Enable schedule</Trans>
														</Label>
														<div className="text-xs text-muted-foreground">
															<Trans>Only scheduled cases will run</Trans>
														</div>
													</div>
													<Switch
														checked={schedule.enabled}
														onCheckedChange={(checked) => setSchedule({ ...schedule, enabled: !!checked })}
													/>
												</div>
												<div className="grid gap-4 md:grid-cols-2">
													<div className="space-y-2">
														<Label>
															<Trans>Interval (minutes)</Trans>
														</Label>
														<Input
															type="number"
															value={schedule.intervalMinutes}
															onChange={(event) =>
																setSchedule({ ...schedule, intervalMinutes: Number(event.target.value) })
															}
														/>
													</div>
													<div className="space-y-2">
														<Label>
															<Trans>History retention (days)</Trans>
														</Label>
														<Input
															type="number"
															value={schedule.historyRetentionDays}
															onChange={(event) =>
																setSchedule({ ...schedule, historyRetentionDays: Number(event.target.value) })
															}
														/>
													</div>
												</div>
												<div className="grid gap-4 md:grid-cols-2">
													<div className="flex items-center justify-between">
														<Label>
															<Trans>Enable alerts</Trans>
														</Label>
														<Switch
															checked={schedule.alertEnabled}
															onCheckedChange={(checked) => setSchedule({ ...schedule, alertEnabled: !!checked })}
														/>
													</div>
													<div className="flex items-center justify-between">
														<Label>
															<Trans>Alert on recovery</Trans>
														</Label>
														<Switch
															checked={schedule.alertOnRecover}
															onCheckedChange={(checked) => setSchedule({ ...schedule, alertOnRecover: !!checked })}
														/>
													</div>
												</div>
												<div className="grid gap-2 text-sm text-muted-foreground">
													<div>
														<Trans>Last run</Trans>: {schedule.lastRunAt ? formatShortDate(schedule.lastRunAt) : "-"}
													</div>
													<div>
														<Trans>Next run</Trans>: {schedule.nextRunAt ? formatShortDate(schedule.nextRunAt) : "-"}
													</div>
													{schedule.lastError && (
														<div className="text-destructive">
															<Trans>Last error</Trans>: {schedule.lastError}
														</div>
													)}
												</div>
												<div className="flex flex-wrap gap-2">
													<Button onClick={saveSchedule} disabled={saving}>
														<Trans>Save</Trans>
													</Button>
													<Button variant="outline" onClick={handleRunAll}>
														<PlayIcon className="me-2 h-4 w-4" />
														<Trans>Run Now</Trans>
													</Button>
												</div>
											</>
										) : (
											<div className="text-sm text-muted-foreground">
												<Trans>Loading schedule...</Trans>
											</div>
										)}
									</div>
								</Card>
							</TabsContent>
							<TabsContent value="history" className="mt-4 animate-fade-in duration-300">
								<Card className="p-4">
									<div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
										<div className="text-sm font-semibold">
											<Trans>History</Trans>
										</div>
										<div className="flex flex-wrap items-center gap-2">
											<Select
												value={toFilterSelectValue(historyCollectionId)}
												onValueChange={(value) => setHistoryCollectionId(fromFilterSelectValue(value))}
											>
												<SelectTrigger className="min-w-[160px]">
													<SelectValue placeholder={t`All collections`} />
												</SelectTrigger>
												<SelectContent>
													<SelectItem value={ALL_FILTER_VALUE}>
														<Trans>All collections</Trans>
													</SelectItem>
													{collections.map((record) => (
														<SelectItem key={record.id} value={record.id}>
															{record.name}
														</SelectItem>
													))}
												</SelectContent>
											</Select>
											<Select
												value={toFilterSelectValue(historyCaseId)}
												onValueChange={(value) => setHistoryCaseId(fromFilterSelectValue(value))}
												disabled={historyCases.length === 0}
											>
												<SelectTrigger className="min-w-[180px]">
													<SelectValue placeholder={t`All cases`} />
												</SelectTrigger>
												<SelectContent>
													<SelectItem value={ALL_FILTER_VALUE}>
														<Trans>All cases</Trans>
													</SelectItem>
													{historyCases.map((record) => (
														<SelectItem key={record.id} value={record.id}>
															{record.name}
														</SelectItem>
													))}
												</SelectContent>
											</Select>
										</div>
									</div>
									<div className="mt-3 max-h-[360px] overflow-auto rounded-md border">
										<Table>
											<TableHeader>
												<TableRow>
													<TableHead>
														<Trans>Status</Trans>
													</TableHead>
													<TableHead>
														<Trans>Case</Trans>
													</TableHead>
													<TableHead>
														<Trans>Duration</Trans>
													</TableHead>
													<TableHead>
														<Trans>Source</Trans>
													</TableHead>
													<TableHead>
														<Trans>Time</Trans>
													</TableHead>
													<TableHead>
														<Trans>Error</Trans>
													</TableHead>
												</TableRow>
											</TableHeader>
											<TableBody>
												{runs.length === 0 && (
													<TableRow>
														<TableCell colSpan={6} className="text-center text-muted-foreground">
															<Trans>No history yet</Trans>
														</TableCell>
													</TableRow>
												)}
												{runs.map((record) => (
													<TableRow key={record.id}>
														<TableCell>
															{record.success ? (
																<Badge variant="success">
																	<Trans>Success</Trans>
																</Badge>
															) : (
																<Badge variant="danger">
																	<Trans>Failed</Trans>
																</Badge>
															)}
														</TableCell>
														<TableCell>{caseNameById.get(record.caseId) ?? record.caseId}</TableCell>
														<TableCell>
															<Badge variant="outline" className="font-mono font-normal">
																{formatDuration(record.durationMs)}
															</Badge>
														</TableCell>
														<TableCell>{formatRunSource(record.source)}</TableCell>
														<TableCell>{record.created ? formatShortDate(record.created) : "-"}</TableCell>
														<TableCell className="max-w-[240px] truncate">{record.error || "-"}</TableCell>
													</TableRow>
												))}
											</TableBody>
										</Table>
									</div>
								</Card>
							</TabsContent>
						</Tabs>
					</div>
				</Card>
			</div>

			<Dialog
				open={importDialogOpen}
				onOpenChange={(open) => {
					if (!open) {
						closeImportDialog()
						return
					}
					setImportDialogOpen(true)
				}}
			>
				<DialogContent className="max-w-lg">
					<DialogHeader>
						<DialogTitle>
							<Trans>Import API tests</Trans>
						</DialogTitle>
					</DialogHeader>
					<div className="grid gap-4">
						<div className="grid gap-2">
							<Label>
								<Trans>Conflict strategy</Trans>
							</Label>
							<div className="flex flex-wrap gap-2 rounded-lg border bg-muted/30 p-1">
								<Button
									type="button"
									size="sm"
									variant={importMode === "skip" ? "default" : "ghost"}
									onClick={() => setImportMode("skip")}
								>
									<Trans>Skip existing</Trans>
								</Button>
								<Button
									type="button"
									size="sm"
									variant={importMode === "overwrite" ? "default" : "ghost"}
									onClick={() => setImportMode("overwrite")}
								>
									<Trans>Overwrite existing</Trans>
								</Button>
							</div>
						</div>
						<div className="grid gap-2">
							<Label>
								<Trans>Select JSON file</Trans>
							</Label>
							<div className="flex flex-wrap items-center gap-3">
								<input
									ref={importFileRef}
									type="file"
									accept="application/json"
									onChange={handleImportFileChange}
									className="hidden"
								/>
								<Button
									type="button"
									variant="outline"
									size="sm"
									onClick={() => importFileRef.current?.click()}
									disabled={importing}
								>
									<UploadIcon className="me-2 h-4 w-4" />
									<Trans>Upload</Trans>
								</Button>
								<span className="text-sm text-muted-foreground max-w-[240px] truncate">
									{importFile ? importFile.name : t`No file selected`}
								</span>
							</div>
						</div>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={closeImportDialog} disabled={importing}>
							<Trans>Cancel</Trans>
						</Button>
						<Button onClick={handleImport} disabled={importing || !importFile}>
							{importing ? (
								<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
							) : (
								<UploadIcon className="me-2 h-4 w-4" />
							)}
							<Trans>Import</Trans>
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<Dialog open={collectionDialogOpen} onOpenChange={setCollectionDialogOpen}>
				<DialogContent className="max-w-xl">
					<DialogHeader>
						<DialogTitle>{collectionDraft.id ? t`Edit collection` : t`New collection`}</DialogTitle>
					</DialogHeader>
					<div className="grid gap-4">
						<div className="space-y-2">
							<Label>
								<Trans>Name</Trans>
							</Label>
							<Input
								value={collectionDraft.name}
								onChange={(event) => setCollectionDraft({ ...collectionDraft, name: event.target.value })}
							/>
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Base URL</Trans>
							</Label>
							<Input
								value={collectionDraft.base_url}
								onChange={(event) => setCollectionDraft({ ...collectionDraft, base_url: event.target.value })}
							/>
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Description</Trans>
							</Label>
							<Textarea
								value={collectionDraft.description}
								onChange={(event) => setCollectionDraft({ ...collectionDraft, description: event.target.value })}
							/>
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Sort order</Trans>
							</Label>
							<Input
								type="number"
								value={collectionDraft.sort_order}
								onChange={(event) => setCollectionDraft({ ...collectionDraft, sort_order: Number(event.target.value) })}
							/>
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Tags</Trans>
							</Label>
							<InputTags
								value={collectionDraft.tags}
								onChange={(tags: string[]) => setCollectionDraft({ ...collectionDraft, tags })}
							/>
						</div>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={() => setCollectionDialogOpen(false)}>
							<Trans>Cancel</Trans>
						</Button>
						<Button onClick={saveCollection} disabled={saving}>
							<Trans>Save</Trans>
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<Dialog open={caseDialogOpen} onOpenChange={setCaseDialogOpen}>
				<DialogContent className="max-w-3xl max-h-[85vh] overflow-auto">
					<DialogHeader>
						<DialogTitle>{caseDraft.id ? t`Edit case` : t`New case`}</DialogTitle>
					</DialogHeader>
					<div className="grid gap-4 py-2">
						{/* Top Section: Basic Info */}
						<div className="grid gap-4">
							<div className="grid gap-4 md:grid-cols-12">
								<div className="md:col-span-4 space-y-2">
									<Label>
										<Trans>Collection</Trans>
									</Label>
									<Select
										value={caseDraft.collection}
										onValueChange={(value) => setCaseDraft({ ...caseDraft, collection: value })}
									>
										<SelectTrigger>
											<SelectValue placeholder={t`Select collection`} />
										</SelectTrigger>
										<SelectContent>
											{collections.map((record) => (
												<SelectItem key={record.id} value={record.id}>
													{record.name}
												</SelectItem>
											))}
										</SelectContent>
									</Select>
								</div>
								<div className="md:col-span-8 space-y-2">
									<Label>
										<Trans>Name</Trans>
									</Label>
									<Input
										value={caseDraft.name}
										onChange={(event) => setCaseDraft({ ...caseDraft, name: event.target.value })}
									/>
								</div>
							</div>
							<div className="grid gap-4 md:grid-cols-12">
								<div className="md:col-span-3 space-y-2">
									<Label>
										<Trans>Method</Trans>
									</Label>
									<Select
										value={caseDraft.method}
										onValueChange={(value) => setCaseDraft({ ...caseDraft, method: value as ApiTestMethod })}
									>
										<SelectTrigger>
											<SelectValue placeholder={t`Select method`} />
										</SelectTrigger>
										<SelectContent>
											{methodOptions.map((method) => (
												<SelectItem key={method} value={method}>
													{method}
												</SelectItem>
											))}
										</SelectContent>
									</Select>
								</div>
								<div className="md:col-span-9 space-y-2">
									<Label>
										<Trans>URL</Trans>
									</Label>
									<Input
										value={caseDraft.url}
										onChange={(event) => setCaseDraft({ ...caseDraft, url: event.target.value })}
									/>
								</div>
							</div>
						</div>

						{/* Tabs Section */}
						<Tabs defaultValue="body" className="w-full min-h-[400px]">
							<TabsList className="grid w-full grid-cols-4">
								<TabsTrigger value="body">
									<Trans>Body</Trans>
								</TabsTrigger>
								<TabsTrigger value="params">
									<Trans>Params</Trans>
								</TabsTrigger>
								<TabsTrigger value="headers">
									<Trans>Headers</Trans>
								</TabsTrigger>
								<TabsTrigger value="settings">
									<Trans>Settings</Trans>
								</TabsTrigger>
							</TabsList>

							{/* Tab: Body */}
							<TabsContent value="body" className="mt-4 space-y-4">
								<div className="flex items-center gap-4">
									<Label className="whitespace-nowrap">
										<Trans>Body type</Trans>
									</Label>
									<Select
										value={caseDraft.body_type}
										onValueChange={(value) => setCaseDraft({ ...caseDraft, body_type: value as ApiTestBodyType })}
									>
										<SelectTrigger className="w-[180px]">
											<SelectValue placeholder={t`Select body type`} />
										</SelectTrigger>
										<SelectContent>
											{bodyTypeOptions.map((bodyType) => (
												<SelectItem key={bodyType} value={bodyType}>
													{bodyType}
												</SelectItem>
											))}
										</SelectContent>
									</Select>
								</div>
								{caseDraft.body_type === "form" ? (
									<div className="space-y-2">
										{formBodyError && <div className="text-xs text-destructive">{formBodyError}</div>}
										<KeyValueEditor
											value={formItems}
											onChange={(next) => {
												setFormItems(next)
												if (formBodyError) {
													setFormBodyError("")
												}
											}}
											emptyLabel={t`No form fields`}
										/>
									</div>
								) : (
									<div className="space-y-2">
										<Textarea
											value={caseDraft.body}
											onChange={(event) => setCaseDraft({ ...caseDraft, body: event.target.value })}
											rows={12}
											className="font-mono text-sm"
										/>
									</div>
								)}
							</TabsContent>

							{/* Tab: Params */}
							<TabsContent value="params" className="mt-4">
								<KeyValueEditor
									value={caseDraft.params}
									onChange={(next) => setCaseDraft({ ...caseDraft, params: next })}
									emptyLabel={t`No params`}
								/>
							</TabsContent>

							{/* Tab: Headers */}
							<TabsContent value="headers" className="mt-4">
								<KeyValueEditor
									value={caseDraft.headers}
									onChange={(next) => setCaseDraft({ ...caseDraft, headers: next })}
									emptyLabel={t`No headers`}
								/>
							</TabsContent>

							{/* Tab: Settings */}
							<TabsContent value="settings" className="mt-4 space-y-4">
								<div className="grid gap-4 md:grid-cols-3">
									<div className="space-y-2">
										<Label>
											<Trans>Expected status</Trans>
										</Label>
										<Input
											type="number"
											value={caseDraft.expected_status}
											onChange={(event) => setCaseDraft({ ...caseDraft, expected_status: Number(event.target.value) })}
										/>
									</div>
									<div className="space-y-2">
										<Label>
											<Trans>Timeout (ms)</Trans>
										</Label>
										<Input
											type="number"
											value={caseDraft.timeout_ms}
											onChange={(event) => setCaseDraft({ ...caseDraft, timeout_ms: Number(event.target.value) })}
										/>
									</div>
									<div className="space-y-2">
										<Label>
											<Trans>Sort order</Trans>
										</Label>
										<Input
											type="number"
											value={caseDraft.sort_order}
											onChange={(event) => setCaseDraft({ ...caseDraft, sort_order: Number(event.target.value) })}
										/>
									</div>
								</div>
								<div className="space-y-2">
									<Label>
										<Trans>Tags</Trans>
									</Label>
									<InputTags
										value={caseDraft.tags}
										onChange={(tags: string[]) => setCaseDraft({ ...caseDraft, tags })}
									/>
								</div>
								<div className="space-y-2">
									<Label>
										<Trans>Description</Trans>
									</Label>
									<Textarea
										value={caseDraft.description}
										onChange={(event) => setCaseDraft({ ...caseDraft, description: event.target.value })}
										rows={4}
									/>
								</div>
							</TabsContent>
						</Tabs>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={() => setCaseDialogOpen(false)}>
							<Trans>Cancel</Trans>
						</Button>
						<Button onClick={saveCase} disabled={saving}>
							<Trans>Save</Trans>
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<Dialog
				open={runResultOpen}
				onOpenChange={(open) => {
					setRunResultOpen(open)
					if (!open) {
						setRunResult(null)
					}
				}}
			>
				<DialogContent className="max-w-2xl max-h-[85vh] overflow-auto">
					<DialogHeader className="space-y-2">
						<div className="flex items-center justify-between gap-2">
							<DialogTitle>
								<Trans>Run result</Trans>
							</DialogTitle>
							{runResult && (
								<Badge
									variant="outline"
									className={cn(
										"border-transparent",
										runResult.success
											? "bg-emerald-50 text-emerald-600 dark:bg-emerald-500/10 dark:text-emerald-400"
											: "bg-red-50 text-red-600 dark:bg-red-500/10 dark:text-red-400"
									)}
								>
									{runResult.success ? <Trans>Success</Trans> : <Trans>Failed</Trans>}
								</Badge>
							)}
						</div>
						<div className="text-sm text-muted-foreground truncate">{runResult?.name || "-"}</div>
					</DialogHeader>
					<div className="grid gap-4">
						<div className="grid gap-3 md:grid-cols-3">
							<div className="rounded-lg border bg-muted/30 p-3">
								<div className="text-xs text-muted-foreground">
									<Trans>Status</Trans>
								</div>
								<div className="mt-2 text-sm font-medium">
									{runResult?.status ? runResult.status : "-"}
								</div>
							</div>
							<div className="rounded-lg border bg-muted/30 p-3">
								<div className="text-xs text-muted-foreground">
									<Trans>Duration</Trans>
								</div>
								<div className="mt-2 text-sm font-medium">
									{runResult ? formatDurationMs(runResult.durationMs) : "-"}
								</div>
							</div>
							<div className="rounded-lg border bg-muted/30 p-3">
								<div className="text-xs text-muted-foreground">
									<Trans>Time</Trans>
								</div>
								<div className="mt-2 text-sm font-medium">
									{runResult?.runAt ? formatShortDate(runResult.runAt) : "-"}
								</div>
							</div>
						</div>

						{runResult?.error ? (
							<div className="rounded-lg border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive whitespace-pre-wrap">
								<div className="text-xs text-muted-foreground">
									<Trans>Error</Trans>
								</div>
								<div className="mt-2">{runResult.error}</div>
							</div>
						) : null}

						<div className="grid gap-2">
							<div className="text-xs text-muted-foreground">
								<Trans>Response snippet</Trans>
							</div>
							{runResult?.responseSnippet ? (
								<pre className="max-h-64 overflow-auto rounded-lg border bg-muted/30 p-3 text-xs whitespace-pre-wrap text-muted-foreground">
									{runResult.responseSnippet}
								</pre>
							) : (
								<div className="rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">
									<Trans>No details available.</Trans>
								</div>
							)}
						</div>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={() => setRunResultOpen(false)}>
							<Trans>Close</Trans>
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>

			<Dialog
				open={caseDetailOpen}
				onOpenChange={(open) => {
					setCaseDetailOpen(open)
					if (!open) {
						setCaseDetailId(null)
					}
				}}
			>
				<DialogContent className="max-w-3xl max-h-[85vh] overflow-auto">
					<DialogHeader className="space-y-2">
						<DialogTitle className="flex flex-col gap-1">
							<span className="text-xs uppercase tracking-wide text-muted-foreground">
								<Trans>Detail</Trans>
							</span>
							<span>{caseDetailRecord?.name || "-"}</span>
						</DialogTitle>
						<div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
							<Badge variant="outline">{caseDetailRecord?.method || "-"}</Badge>
							<span className="break-all">{caseDetailRecord?.url || "-"}</span>
						</div>
					</DialogHeader>
					<div className="grid gap-4">
						<div className="grid gap-3 md:grid-cols-4">
							<div className="rounded-lg border bg-muted/30 p-3">
								<div className="text-xs text-muted-foreground">
									<Trans>Status</Trans>
								</div>
								<div className="mt-2">
									{caseDetailRecord ? (
										<CaseStatusBadge record={caseDetailRecord} />
									) : (
										<Badge variant="secondary">
											<Trans>Unknown</Trans>
										</Badge>
									)}
								</div>
							</div>
							<div className="rounded-lg border bg-muted/30 p-3">
								<div className="text-xs text-muted-foreground">
									<Trans>Last Status</Trans>
								</div>
								<div className="mt-2 text-sm font-medium">
									{caseDetailRecord?.last_status === undefined || caseDetailRecord?.last_status === null
										? "-"
										: String(caseDetailRecord.last_status)}
								</div>
							</div>
							<div className="rounded-lg border bg-muted/30 p-3">
								<div className="text-xs text-muted-foreground">
									<Trans>Last Run</Trans>
								</div>
								<div className="mt-2 text-sm font-medium">
									{caseDetailRecord?.last_run_at ? formatShortDate(caseDetailRecord.last_run_at) : "-"}
								</div>
							</div>
							<div className="rounded-lg border bg-muted/30 p-3">
								<div className="text-xs text-muted-foreground">
									<Trans>Duration</Trans>
								</div>
								<div className="mt-2 text-sm font-medium">{formatDuration(caseDetailRecord?.last_duration_ms)}</div>
							</div>
						</div>
						<div className="grid gap-3 md:grid-cols-2">
							<div className="rounded-lg border p-4">
								<div className="mb-2 text-xs text-muted-foreground">
									<Trans>Error</Trans>
								</div>
								<div
									className={
										caseDetailRecord?.last_error
											? "text-sm text-destructive whitespace-pre-wrap"
											: "text-sm text-muted-foreground"
									}
								>
									{caseDetailRecord?.last_error || "-"}
								</div>
							</div>
							<div className="rounded-lg border p-4">
								<div className="mb-2 text-xs text-muted-foreground">
									<Trans>Detail</Trans>
								</div>
								{caseDetailRecord?.last_response_snippet ? (
									<pre className="max-h-48 overflow-auto whitespace-pre-wrap rounded-md bg-muted/40 p-3 text-xs font-mono text-foreground">
										{caseDetailRecord.last_response_snippet}
									</pre>
								) : (
									<div className="text-sm text-muted-foreground">
										<Trans>No details available.</Trans>
									</div>
								)}
							</div>
						</div>
					</div>
					<DialogFooter>
						<Button variant="outline" onClick={() => setCaseDetailOpen(false)}>
							<Trans>Close</Trans>
						</Button>
					</DialogFooter>
				</DialogContent>
			</Dialog>
			<FooterRepoLink />
		</>
	)
})

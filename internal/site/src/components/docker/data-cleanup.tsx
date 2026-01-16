/**
 * Docker 数据清理配置与执行面板。
 * 负责连接配置、资源拉取与清理进度展示。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { AlertDialog, AlertDialogAction, AlertDialogCancel, AlertDialogContent, AlertDialogDescription, AlertDialogFooter, AlertDialogHeader, AlertDialogTitle } from "@/components/ui/alert-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { toast } from "@/components/ui/use-toast"
import DockerEmptyState from "@/components/docker/empty-state"
import { isReadOnlyUser } from "@/lib/api"
import {
	fetchDockerDataCleanupConfig,
	fetchDockerDataCleanupRun,
	listDockerDataCleanupESIndices,
	listDockerDataCleanupMinioBuckets,
	listDockerDataCleanupMinioPrefixes,
	listDockerDataCleanupMySQLDatabases,
	listDockerDataCleanupMySQLTables,
	listDockerDataCleanupRedisDatabases,
	retryDockerDataCleanupRun,
	startDockerDataCleanupRun,
	upsertDockerDataCleanupConfig,
} from "@/lib/docker"
import type { DockerDataCleanupConfig, DockerDataCleanupRun, DockerDataCleanupRunResult } from "@/types"
import { LoaderCircleIcon, PlayCircleIcon, RefreshCwIcon, RotateCcwIcon, SaveIcon } from "lucide-react"

const parsePositiveNumber = (value: string) => {
	const trimmed = value.trim()
	if (!trimmed) return 0
	const parsed = Number(trimmed)
	if (!Number.isFinite(parsed) || parsed <= 0) return NaN
	return Math.trunc(parsed)
}

const parseOptionalNumber = (value: string) => {
	const trimmed = value.trim()
	if (!trimmed) return NaN
	const parsed = Number(trimmed)
	if (!Number.isFinite(parsed)) return NaN
	return Math.trunc(parsed)
}

const toggleString = (value: string, selected: string[], setSelected: (next: string[]) => void) => {
	if (selected.includes(value)) {
		setSelected(selected.filter((item) => item !== value))
		return
	}
	setSelected([...selected, value])
}

const renderStatusBadge = (status: string) => {
	if (status === "success") {
		return <Badge className="bg-emerald-500/15 text-emerald-600 hover:bg-emerald-500/25">{t`Success`}</Badge>
	}
	if (status === "failed") {
		return <Badge variant="destructive">{t`Failed`}</Badge>
	}
	if (status === "running") {
		return <Badge className="bg-blue-500/15 text-blue-600 hover:bg-blue-500/25">{t`Running`}</Badge>
	}
	return <Badge variant="secondary">{t`Pending`}</Badge>
}

export default memo(function DockerDataCleanupPanel({ systemId }: { systemId?: string }) {
	const [loading, setLoading] = useState(false)
	const [saving, setSaving] = useState(false)
	const [confirmOpen, setConfirmOpen] = useState(false)
	const [runOpen, setRunOpen] = useState(false)
	const [runLoading, setRunLoading] = useState(false)
	const [runId, setRunId] = useState("")
	const [runStatus, setRunStatus] = useState<DockerDataCleanupRun["status"]>("pending")
	const [runProgress, setRunProgress] = useState(0)
	const [runStep, setRunStep] = useState("")
	const [runLogs, setRunLogs] = useState<string[]>([])
	const [runResults, setRunResults] = useState<DockerDataCleanupRunResult[]>([])

	const [mysqlHost, setMysqlHost] = useState("")
	const [mysqlPort, setMysqlPort] = useState("")
	const [mysqlUsername, setMysqlUsername] = useState("")
	const [mysqlPassword, setMysqlPassword] = useState("")
	const [mysqlHasPassword, setMysqlHasPassword] = useState(false)
	const [mysqlUseStoredPassword, setMysqlUseStoredPassword] = useState(false)
	const [mysqlDatabase, setMysqlDatabase] = useState("")
	const [mysqlDatabases, setMysqlDatabases] = useState<string[]>([])
	const [mysqlTables, setMysqlTables] = useState<string[]>([])
	const [mysqlSelectedTables, setMysqlSelectedTables] = useState<string[]>([])
	const [mysqlDbLoading, setMysqlDbLoading] = useState(false)
	const [mysqlTablesLoading, setMysqlTablesLoading] = useState(false)

	const [redisHost, setRedisHost] = useState("")
	const [redisPort, setRedisPort] = useState("")
	const [redisUsername, setRedisUsername] = useState("")
	const [redisPassword, setRedisPassword] = useState("")
	const [redisHasPassword, setRedisHasPassword] = useState(false)
	const [redisUseStoredPassword, setRedisUseStoredPassword] = useState(false)
	const [redisDB, setRedisDB] = useState("")
	const [redisDBs, setRedisDBs] = useState<number[]>([])
	const [redisPatterns, setRedisPatterns] = useState<string[]>([])
	const [redisLoading, setRedisLoading] = useState(false)

	const [minioHost, setMinioHost] = useState("")
	const [minioPort, setMinioPort] = useState("")
	const [minioAccessKey, setMinioAccessKey] = useState("")
	const [minioSecretKey, setMinioSecretKey] = useState("")
	const [minioHasSecretKey, setMinioHasSecretKey] = useState(false)
	const [minioUseStoredSecret, setMinioUseStoredSecret] = useState(false)
	const [minioBucket, setMinioBucket] = useState("")
	const [minioBuckets, setMinioBuckets] = useState<string[]>([])
	const [minioPrefixes, setMinioPrefixes] = useState<string[]>([])
	const [minioSelectedPrefixes, setMinioSelectedPrefixes] = useState<string[]>([])
	const [minioBucketsLoading, setMinioBucketsLoading] = useState(false)
	const [minioPrefixesLoading, setMinioPrefixesLoading] = useState(false)

	const [esHost, setEsHost] = useState("")
	const [esPort, setEsPort] = useState("")
	const [esUsername, setEsUsername] = useState("")
	const [esPassword, setEsPassword] = useState("")
	const [esHasPassword, setEsHasPassword] = useState(false)
	const [esUseStoredPassword, setEsUseStoredPassword] = useState(false)
	const [esIndices, setEsIndices] = useState<string[]>([])
	const [esSelectedIndices, setEsSelectedIndices] = useState<string[]>([])
	const [esLoading, setEsLoading] = useState(false)

	const resetConfigState = useCallback(() => {
		setMysqlHost("")
		setMysqlPort("")
		setMysqlUsername("")
		setMysqlPassword("")
		setMysqlHasPassword(false)
		setMysqlUseStoredPassword(false)
		setMysqlDatabase("")
		setMysqlDatabases([])
		setMysqlTables([])
		setMysqlSelectedTables([])

		setRedisHost("")
		setRedisPort("")
		setRedisUsername("")
		setRedisPassword("")
		setRedisHasPassword(false)
		setRedisUseStoredPassword(false)
		setRedisDB("")
		setRedisDBs([])
		setRedisPatterns([])

		setMinioHost("")
		setMinioPort("")
		setMinioAccessKey("")
		setMinioSecretKey("")
		setMinioHasSecretKey(false)
		setMinioUseStoredSecret(false)
		setMinioBucket("")
		setMinioBuckets([])
		setMinioPrefixes([])
		setMinioSelectedPrefixes([])

		setEsHost("")
		setEsPort("")
		setEsUsername("")
		setEsPassword("")
		setEsHasPassword(false)
		setEsUseStoredPassword(false)
		setEsIndices([])
		setEsSelectedIndices([])
	}, [])

	const loadMySQLResources = useCallback(
		async (config: DockerDataCleanupConfig) => {
			if (!systemId) return
			const host = config.mysql?.host?.trim() ?? ""
			const port = config.mysql?.port ?? 0
			if (!host || !port) return
			setMysqlDbLoading(true)
			let selectedDatabase = ""
			try {
				const res = await listDockerDataCleanupMySQLDatabases({
					system: systemId,
					host,
					port,
					username: config.mysql?.username?.trim() ?? "",
					password: "",
					useStoredPassword: !!config.mysql?.hasPassword,
					database: "",
				})
				const items = res.items ?? []
				setMysqlDatabases(items)
				const database = config.mysql?.database?.trim() ?? ""
				if (database && items.includes(database)) {
					selectedDatabase = database
					setMysqlDatabase(database)
				} else {
					setMysqlDatabase("")
					setMysqlTables([])
					setMysqlSelectedTables([])
					return
				}
			} catch (err) {
				console.error("load mysql databases failed", err)
				toast({ variant: "destructive", title: t`Error`, description: t`Failed to load MySQL databases` })
				throw err
			} finally {
				setMysqlDbLoading(false)
			}

			if (!selectedDatabase) return
			setMysqlTablesLoading(true)
			try {
				const res = await listDockerDataCleanupMySQLTables({
					system: systemId,
					host,
					port,
					username: config.mysql?.username?.trim() ?? "",
					password: "",
					useStoredPassword: !!config.mysql?.hasPassword,
					database: selectedDatabase,
				})
				const items = res.items ?? []
				setMysqlTables(items)
				const selected = (config.mysql?.tables ?? []).filter((table) => items.includes(table))
				setMysqlSelectedTables(selected)
			} catch (err) {
				console.error("load mysql tables failed", err)
				toast({ variant: "destructive", title: t`Error`, description: t`Failed to load MySQL tables` })
				throw err
			} finally {
				setMysqlTablesLoading(false)
			}
		},
		[systemId],
	)

	const loadRedisResources = useCallback(
		async (config: DockerDataCleanupConfig) => {
			if (!systemId) return
			const host = config.redis?.host?.trim() ?? ""
			const port = config.redis?.port ?? 0
			if (!host || !port) return
			setRedisLoading(true)
			try {
				const res = await listDockerDataCleanupRedisDatabases({
					system: systemId,
					host,
					port,
					username: config.redis?.username?.trim() ?? "",
					password: "",
					useStoredPassword: !!config.redis?.hasPassword,
					database: "",
				})
				const items = res.items ?? []
				setRedisDBs(items)
				const configuredDB = config.redis?.db
				const dbStrings = items.map(String)
				if (configuredDB !== undefined && dbStrings.includes(String(configuredDB))) {
					setRedisDB(String(configuredDB))
				} else if (configuredDB !== undefined) {
					setRedisDB("")
				} else if (dbStrings.includes("0")) {
					// Legacy/backward-compat: backend may omit db when it's 0 (e.g. json omitempty).
					setRedisDB("0")
				} else {
					setRedisDB("")
				}
			} catch (err) {
				console.error("load redis dbs failed", err)
				toast({ variant: "destructive", title: t`Error`, description: t`Failed to load Redis databases` })
				throw err
			} finally {
				setRedisLoading(false)
			}
		},
		[systemId],
	)

	const loadMinioResources = useCallback(
		async (config: DockerDataCleanupConfig) => {
			if (!systemId) return
			const host = config.minio?.host?.trim() ?? ""
			const port = config.minio?.port ?? 0
			if (!host || !port) return
			setMinioBucketsLoading(true)
			let selectedBucket = ""
			try {
				const res = await listDockerDataCleanupMinioBuckets({
					system: systemId,
					host,
					port,
					accessKey: config.minio?.accessKey?.trim() ?? "",
					secretKey: "",
					useStoredSecret: !!config.minio?.hasSecretKey,
					bucket: "",
				})
				const items = res.items ?? []
				setMinioBuckets(items)
				const bucket = config.minio?.bucket?.trim() ?? ""
				if (bucket && items.includes(bucket)) {
					selectedBucket = bucket
					setMinioBucket(bucket)
				} else {
					setMinioBucket("")
					setMinioPrefixes([])
					setMinioSelectedPrefixes([])
					return
				}
			} catch (err) {
				console.error("load minio buckets failed", err)
				toast({ variant: "destructive", title: t`Error`, description: t`Failed to load MinIO buckets` })
				throw err
			} finally {
				setMinioBucketsLoading(false)
			}

			if (!selectedBucket) return
			setMinioPrefixesLoading(true)
			try {
				const res = await listDockerDataCleanupMinioPrefixes({
					system: systemId,
					host,
					port,
					accessKey: config.minio?.accessKey?.trim() ?? "",
					secretKey: "",
					useStoredSecret: !!config.minio?.hasSecretKey,
					bucket: selectedBucket,
				})
				const items = res.items ?? []
				setMinioPrefixes(items)
				const selected = (config.minio?.prefixes ?? []).filter((prefix) => items.includes(prefix))
				setMinioSelectedPrefixes(selected)
			} catch (err) {
				console.error("load minio prefixes failed", err)
				toast({ variant: "destructive", title: t`Error`, description: t`Failed to load MinIO folders` })
				throw err
			} finally {
				setMinioPrefixesLoading(false)
			}
		},
		[systemId],
	)

	const loadESResources = useCallback(
		async (config: DockerDataCleanupConfig) => {
			if (!systemId) return
			const host = config.es?.host?.trim() ?? ""
			const port = config.es?.port ?? 0
			if (!host || !port) return
			setEsLoading(true)
			try {
				const res = await listDockerDataCleanupESIndices({
					system: systemId,
					host,
					port,
					username: config.es?.username?.trim() ?? "",
					password: "",
					useStoredPassword: !!config.es?.hasPassword,
					database: "",
				})
				const items = res.items ?? []
				setEsIndices(items)
				const selected = (config.es?.indices ?? []).filter((index) => items.includes(index))
				setEsSelectedIndices(selected)
			} catch (err) {
				console.error("load es indices failed", err)
				toast({ variant: "destructive", title: t`Error`, description: t`Failed to load Elasticsearch indices` })
				throw err
			} finally {
				setEsLoading(false)
			}
		},
		[systemId],
	)

	const loadConfigResources = useCallback(
		async (config: DockerDataCleanupConfig) => {
			const results = await Promise.allSettled([
				loadMySQLResources(config),
				loadRedisResources(config),
				loadMinioResources(config),
				loadESResources(config),
			])
			const failures = results.filter((result): result is PromiseRejectedResult => result.status === "rejected")
			if (failures.length > 0) {
				const error = new Error(`auto load cleanup resources failed for system ${systemId ?? "unknown"}`)
				;(error as { causes?: unknown[]; context?: Record<string, unknown> }).causes = failures.map((failure) => failure.reason)
				;(error as { causes?: unknown[]; context?: Record<string, unknown> }).context = { systemId }
				console.error("auto load cleanup resources failed", error)
				throw error
			}
		},
		[loadMySQLResources, loadRedisResources, loadMinioResources, loadESResources, systemId],
	)

	const loadConfig = useCallback(async () => {
		if (!systemId) return false
		setLoading(true)
		let config: DockerDataCleanupConfig | null = null
		try {
			config = await fetchDockerDataCleanupConfig(systemId)
			setMysqlHost(config.mysql?.host ?? "")
			setMysqlPort(config.mysql?.port ? String(config.mysql.port) : "")
			setMysqlUsername(config.mysql?.username ?? "")
			setMysqlPassword("")
			setMysqlHasPassword(!!config.mysql?.hasPassword)
			setMysqlUseStoredPassword(!!config.mysql?.hasPassword)
			setMysqlDatabase(config.mysql?.database ?? "")
			setMysqlSelectedTables(config.mysql?.tables ?? [])
			setMysqlTables([])
			setMysqlDatabases([])

			setRedisHost(config.redis?.host ?? "")
			setRedisPort(config.redis?.port ? String(config.redis.port) : "")
			setRedisUsername(config.redis?.username ?? "")
			setRedisPassword("")
			setRedisHasPassword(!!config.redis?.hasPassword)
			setRedisUseStoredPassword(!!config.redis?.hasPassword)
			setRedisDB(config.redis?.db !== undefined ? String(config.redis.db) : "")
			setRedisPatterns(config.redis?.patterns ?? [])
			setRedisDBs([])

			setMinioHost(config.minio?.host ?? "")
			setMinioPort(config.minio?.port ? String(config.minio.port) : "")
			setMinioAccessKey(config.minio?.accessKey ?? "")
			setMinioSecretKey("")
			setMinioHasSecretKey(!!config.minio?.hasSecretKey)
			setMinioUseStoredSecret(!!config.minio?.hasSecretKey)
			setMinioBucket(config.minio?.bucket ?? "")
			setMinioSelectedPrefixes(config.minio?.prefixes ?? [])
			setMinioBuckets([])
			setMinioPrefixes([])

			setEsHost(config.es?.host ?? "")
			setEsPort(config.es?.port ? String(config.es.port) : "")
			setEsUsername(config.es?.username ?? "")
			setEsPassword("")
			setEsHasPassword(!!config.es?.hasPassword)
			setEsUseStoredPassword(!!config.es?.hasPassword)
			setEsSelectedIndices(config.es?.indices ?? [])
			setEsIndices([])
		} catch (err) {
			console.error("load data cleanup config failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load cleanup config` })
			throw err
		} finally {
			setLoading(false)
		}
		if (config) {
			await loadConfigResources(config)
		}
		return true
	}, [systemId, loadConfigResources])

	useEffect(() => {
		if (!systemId) {
			resetConfigState()
			return
		}
		void loadConfig()
	}, [systemId, loadConfig, resetConfigState])

	const mysqlPortValue = useMemo(() => parsePositiveNumber(mysqlPort), [mysqlPort])
	const redisPortValue = useMemo(() => parsePositiveNumber(redisPort), [redisPort])
	const minioPortValue = useMemo(() => parsePositiveNumber(minioPort), [minioPort])
	const esPortValue = useMemo(() => parsePositiveNumber(esPort), [esPort])
	const redisDBValue = useMemo(() => parseOptionalNumber(redisDB), [redisDB])

	const loadMySQLDatabases = useCallback(async () => {
		if (!systemId) return
		if (!mysqlHost.trim() || !mysqlPortValue) {
			toast({ variant: "destructive", title: t`Error`, description: t`MySQL host and port are required` })
			return
		}
		setMysqlDbLoading(true)
		try {
			const res = await listDockerDataCleanupMySQLDatabases({
				system: systemId,
				host: mysqlHost.trim(),
				port: mysqlPortValue,
				username: mysqlUsername.trim(),
				password: mysqlPassword,
				useStoredPassword: mysqlUseStoredPassword && !mysqlPassword.trim(),
				database: "",
			})
			const items = res.items ?? []
			setMysqlDatabases(items)
			if (mysqlDatabase && !items.includes(mysqlDatabase)) {
				setMysqlDatabase("")
				setMysqlTables([])
				setMysqlSelectedTables([])
			}
		} catch (err) {
			console.error("load mysql databases failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load MySQL databases` })
			throw err
		} finally {
			setMysqlDbLoading(false)
		}
	}, [systemId, mysqlHost, mysqlPortValue, mysqlUsername, mysqlPassword, mysqlUseStoredPassword, mysqlDatabase])

	const loadMySQLTables = useCallback(async () => {
		if (!systemId) return
		if (!mysqlHost.trim() || !mysqlPortValue || !mysqlDatabase.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`MySQL host, port and database are required` })
			return
		}
		setMysqlTablesLoading(true)
		try {
			const res = await listDockerDataCleanupMySQLTables({
				system: systemId,
				host: mysqlHost.trim(),
				port: mysqlPortValue,
				username: mysqlUsername.trim(),
				password: mysqlPassword,
				useStoredPassword: mysqlUseStoredPassword && !mysqlPassword.trim(),
				database: mysqlDatabase.trim(),
			})
			const items = res.items ?? []
			setMysqlTables(items)
			setMysqlSelectedTables((prev) => prev.filter((table) => items.includes(table)))
		} catch (err) {
			console.error("load mysql tables failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load MySQL tables` })
			throw err
		} finally {
			setMysqlTablesLoading(false)
		}
	}, [systemId, mysqlHost, mysqlPortValue, mysqlUsername, mysqlPassword, mysqlUseStoredPassword, mysqlDatabase])

	const loadRedisDBs = useCallback(async () => {
		if (!systemId) return
		if (!redisHost.trim() || !redisPortValue) {
			toast({ variant: "destructive", title: t`Error`, description: t`Redis host and port are required` })
			return
		}
		setRedisLoading(true)
		try {
			const res = await listDockerDataCleanupRedisDatabases({
				system: systemId,
				host: redisHost.trim(),
				port: redisPortValue,
				username: redisUsername.trim(),
				password: redisPassword,
				useStoredPassword: redisUseStoredPassword && !redisPassword.trim(),
				database: "",
			})
			const items = res.items ?? []
			setRedisDBs(items)
			if (redisDB && !items.map(String).includes(redisDB)) {
				setRedisDB("")
			}
		} catch (err) {
			console.error("load redis dbs failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load Redis databases` })
			throw err
		} finally {
			setRedisLoading(false)
		}
	}, [systemId, redisHost, redisPortValue, redisUsername, redisPassword, redisUseStoredPassword, redisDB])

	const loadMinioBuckets = useCallback(async () => {
		if (!systemId) return
		if (!minioHost.trim() || !minioPortValue) {
			toast({ variant: "destructive", title: t`Error`, description: t`MinIO host and port are required` })
			return
		}
		setMinioBucketsLoading(true)
		try {
			const res = await listDockerDataCleanupMinioBuckets({
				system: systemId,
				host: minioHost.trim(),
				port: minioPortValue,
				accessKey: minioAccessKey.trim(),
				secretKey: minioSecretKey,
				useStoredSecret: minioUseStoredSecret && !minioSecretKey.trim(),
				bucket: "",
			})
			const items = res.items ?? []
			setMinioBuckets(items)
			if (minioBucket && !items.includes(minioBucket)) {
				setMinioBucket("")
				setMinioPrefixes([])
				setMinioSelectedPrefixes([])
			}
		} catch (err) {
			console.error("load minio buckets failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load MinIO buckets` })
			throw err
		} finally {
			setMinioBucketsLoading(false)
		}
	}, [systemId, minioHost, minioPortValue, minioAccessKey, minioSecretKey, minioUseStoredSecret, minioBucket])

	const loadMinioPrefixes = useCallback(async () => {
		if (!systemId) return
		if (!minioHost.trim() || !minioPortValue || !minioBucket.trim()) {
			toast({ variant: "destructive", title: t`Error`, description: t`MinIO host, port and bucket are required` })
			return
		}
		setMinioPrefixesLoading(true)
		try {
			const res = await listDockerDataCleanupMinioPrefixes({
				system: systemId,
				host: minioHost.trim(),
				port: minioPortValue,
				accessKey: minioAccessKey.trim(),
				secretKey: minioSecretKey,
				useStoredSecret: minioUseStoredSecret && !minioSecretKey.trim(),
				bucket: minioBucket.trim(),
			})
			const items = res.items ?? []
			setMinioPrefixes(items)
			setMinioSelectedPrefixes((prev) => prev.filter((prefix) => items.includes(prefix)))
		} catch (err) {
			console.error("load minio prefixes failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load MinIO folders` })
			throw err
		} finally {
			setMinioPrefixesLoading(false)
		}
	}, [systemId, minioHost, minioPortValue, minioAccessKey, minioSecretKey, minioUseStoredSecret, minioBucket])

	const loadESIndices = useCallback(async () => {
		if (!systemId) return
		if (!esHost.trim() || !esPortValue) {
			toast({ variant: "destructive", title: t`Error`, description: t`Elasticsearch host and port are required` })
			return
		}
		setEsLoading(true)
		try {
			const res = await listDockerDataCleanupESIndices({
				system: systemId,
				host: esHost.trim(),
				port: esPortValue,
				username: esUsername.trim(),
				password: esPassword,
				useStoredPassword: esUseStoredPassword && !esPassword.trim(),
				database: "",
			})
			const items = res.items ?? []
			setEsIndices(items)
			setEsSelectedIndices((prev) => prev.filter((index) => items.includes(index)))
		} catch (err) {
			console.error("load es indices failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to load Elasticsearch indices` })
			throw err
		} finally {
			setEsLoading(false)
		}
	}, [systemId, esHost, esPortValue, esUsername, esPassword, esUseStoredPassword])

	const saveConfig = useCallback(async (): Promise<boolean> => {
		if (!systemId) return
		if (isReadOnlyUser()) {
			toast({ title: t`Forbidden`, description: t`You have read-only access`, variant: "destructive" })
			return false
		}

		if (mysqlPortValue !== 0 && Number.isNaN(mysqlPortValue)) {
			toast({ variant: "destructive", title: t`Error`, description: t`MySQL port is invalid` })
			return false
		}
		if (redisPortValue !== 0 && Number.isNaN(redisPortValue)) {
			toast({ variant: "destructive", title: t`Error`, description: t`Redis port is invalid` })
			return false
		}
		if (minioPortValue !== 0 && Number.isNaN(minioPortValue)) {
			toast({ variant: "destructive", title: t`Error`, description: t`MinIO port is invalid` })
			return false
		}
		if (esPortValue !== 0 && Number.isNaN(esPortValue)) {
			toast({ variant: "destructive", title: t`Error`, description: t`Elasticsearch port is invalid` })
			return false
		}
		const hasRedisConfig =
			!!redisHost.trim() || !!redisPort.trim() || !!redisUsername.trim() || !!redisPassword.trim()
		if (hasRedisConfig && Number.isNaN(redisDBValue)) {
			toast({ variant: "destructive", title: t`Error`, description: t`Redis DB is invalid` })
			return false
		}

		const payload: DockerDataCleanupConfig = {
			system: systemId,
			mysql: {
				host: mysqlHost.trim(),
				port: mysqlPortValue,
				username: mysqlUsername.trim(),
				password: mysqlPassword,
				database: mysqlDatabase.trim(),
				tables: mysqlSelectedTables,
			},
			redis: {
				host: redisHost.trim(),
				port: redisPortValue,
				username: redisUsername.trim(),
				password: redisPassword,
				db: Number.isNaN(redisDBValue) ? 0 : redisDBValue,
				patterns: redisPatterns,
			},
			minio: {
				host: minioHost.trim(),
				port: minioPortValue,
				accessKey: minioAccessKey.trim(),
				secretKey: minioSecretKey,
				bucket: minioBucket.trim(),
				prefixes: minioSelectedPrefixes,
			},
			es: {
				host: esHost.trim(),
				port: esPortValue,
				username: esUsername.trim(),
				password: esPassword,
				indices: esSelectedIndices,
			},
		}

		setSaving(true)
		try {
			await upsertDockerDataCleanupConfig(payload)
			toast({ title: t`Saved`, description: t`Cleanup configuration saved` })
			await loadConfig()
			return true
		} catch (err) {
			console.error("save cleanup config failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to save cleanup config` })
			throw err
		} finally {
			setSaving(false)
		}
	}, [
		systemId,
		mysqlHost,
		mysqlPortValue,
		mysqlUsername,
		mysqlPassword,
		mysqlDatabase,
		mysqlSelectedTables,
		redisHost,
		redisPortValue,
		redisUsername,
		redisPassword,
		redisDBValue,
		redisPatterns,
		minioHost,
		minioPortValue,
		minioAccessKey,
		minioSecretKey,
		minioBucket,
		minioSelectedPrefixes,
		esHost,
		esPortValue,
		esUsername,
		esPassword,
		esSelectedIndices,
		loadConfig,
	])

	const startRun = useCallback(async () => {
		if (!systemId) return
		setRunLoading(true)
		try {
			const saved = await saveConfig()
			if (!saved) {
				setRunLoading(false)
				setConfirmOpen(false)
				return
			}
			const res = await startDockerDataCleanupRun({ system: systemId })
			setRunId(res.runId)
			setRunStatus("pending")
			setRunProgress(0)
			setRunStep("")
			setRunLogs([])
			setRunResults([])
			setRunOpen(true)
		} catch (err) {
			console.error("start cleanup run failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to start cleanup` })
			throw err
		} finally {
			setRunLoading(false)
			setConfirmOpen(false)
		}
	}, [systemId, saveConfig])

	const retryRun = useCallback(async () => {
		if (!systemId) return
		setRunLoading(true)
		try {
			const res = await retryDockerDataCleanupRun({ system: systemId })
			setRunId(res.runId)
			setRunStatus("pending")
			setRunProgress(0)
			setRunStep("")
			setRunLogs([])
			setRunResults([])
			setRunOpen(true)
		} catch (err) {
			console.error("retry cleanup run failed", err)
			toast({ variant: "destructive", title: t`Error`, description: t`Failed to retry cleanup` })
			throw err
		} finally {
			setRunLoading(false)
		}
	}, [systemId])

	useEffect(() => {
		if (!runId || !runOpen) return
		let cancelled = false
		let timer: number | undefined

		const poll = async () => {
			try {
				const res = await fetchDockerDataCleanupRun(runId)
				if (cancelled) return
				setRunStatus(res.status)
				setRunProgress(res.progress ?? 0)
				setRunStep(res.step ?? "")
				setRunLogs(res.logs ?? [])
				setRunResults(res.results ?? [])
				if (res.status === "success" || res.status === "failed") {
					if (timer) {
						window.clearInterval(timer)
						timer = undefined
					}
				}
			} catch (err) {
				console.error("poll cleanup run failed", err)
			}
		}

		void poll()
		timer = window.setInterval(poll, 2000)
		return () => {
			cancelled = true
			if (timer) {
				window.clearInterval(timer)
			}
		}
	}, [runId, runOpen])

	const moduleSummary = useMemo(() => {
		const summary = new Map<string, DockerDataCleanupRunResult>()
		for (const result of runResults) {
			const current = summary.get(result.module)
			if (!current || result.status === "failed") {
				summary.set(result.module, result)
			}
		}
		return Array.from(summary.values())
	}, [runResults])

	if (!systemId) {
		return <DockerEmptyState />
	}

	return (
		<div className="space-y-6">
			<Card className="border-dashed">
				<CardHeader className="pb-4">
					<CardTitle>
						<Trans>Data Cleanup</Trans>
					</CardTitle>
					<CardDescription>
						<Trans>Configure MySQL, Redis, MinIO, and Elasticsearch cleanup targets for the selected system.</Trans>
					</CardDescription>
				</CardHeader>
				<CardContent className="flex flex-wrap items-center gap-3">
					<Button onClick={() => void loadConfig()} variant="outline" disabled={loading}>
						{loading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : <RefreshCwIcon className="me-2 h-4 w-4" />}
						<Trans>Refresh Config</Trans>
					</Button>
					<Button onClick={() => void saveConfig()} disabled={saving}>
						{saving ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : <SaveIcon className="me-2 h-4 w-4" />}
						<Trans>Save Config</Trans>
					</Button>
					<Button onClick={() => setConfirmOpen(true)} variant="destructive" disabled={runLoading}>
						{runLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : <PlayCircleIcon className="me-2 h-4 w-4" />}
						<Trans>Start Cleanup</Trans>
					</Button>
				</CardContent>
			</Card>

			<Card>
				<CardHeader>
					<CardTitle>MySQL</CardTitle>
					<CardDescription>
						<Trans>Select a database and tables to delete with DELETE statements.</Trans>
					</CardDescription>
				</CardHeader>
				<CardContent className="space-y-4">
					<div className="grid gap-4 md:grid-cols-2">
						<div className="space-y-2">
							<Label>
								<Trans>Host</Trans>
							</Label>
							<Input value={mysqlHost} onChange={(e) => setMysqlHost(e.target.value)} placeholder="127.0.0.1" />
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Port</Trans>
							</Label>
							<Input value={mysqlPort} onChange={(e) => setMysqlPort(e.target.value)} placeholder="3306" />
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Username</Trans>
							</Label>
							<Input value={mysqlUsername} onChange={(e) => setMysqlUsername(e.target.value)} placeholder="root" />
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Password</Trans>
							</Label>
							<Input
								type="password"
								value={mysqlPassword}
								onChange={(e) => {
									setMysqlPassword(e.target.value)
									if (e.target.value.trim()) {
										setMysqlUseStoredPassword(false)
									}
								}}
								placeholder={mysqlHasPassword ? t`Stored password available` : "********"}
							/>
							{mysqlHasPassword ? (
								<div className="flex items-center gap-2 text-xs text-muted-foreground">
									<Checkbox
										checked={mysqlUseStoredPassword}
										onCheckedChange={(checked) => setMysqlUseStoredPassword(checked === true)}
									/>
									<span>
										<Trans>Use stored password</Trans>
									</span>
								</div>
							) : null}
						</div>
					</div>

					<div className="flex flex-wrap items-center gap-3">
						<Button variant="outline" onClick={() => void loadMySQLDatabases()} disabled={mysqlDbLoading}>
							{mysqlDbLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : <RefreshCwIcon className="me-2 h-4 w-4" />}
							<Trans>Load Databases</Trans>
						</Button>
						<Select value={mysqlDatabase} onValueChange={setMysqlDatabase} disabled={mysqlDatabases.length === 0}>
							<SelectTrigger className="min-w-[200px]">
								<SelectValue placeholder={t`Select database`} />
							</SelectTrigger>
							<SelectContent>
								{mysqlDatabases.map((item) => (
									<SelectItem key={item} value={item}>
										{item}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
						<Button
							variant="outline"
							onClick={() => void loadMySQLTables()}
							disabled={mysqlTablesLoading || !mysqlDatabase}
						>
							{mysqlTablesLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : <RefreshCwIcon className="me-2 h-4 w-4" />}
							<Trans>Load Tables</Trans>
						</Button>
					</div>

					<div className="rounded-md border p-3">
						{mysqlTables.length === 0 ? (
							<p className="text-sm text-muted-foreground">
								<Trans>No tables loaded.</Trans>
							</p>
						) : (
							<div className="grid max-h-48 gap-2 overflow-auto">
								{mysqlTables.map((table) => (
									<label key={table} className="flex items-center gap-2 text-sm">
										<Checkbox
											checked={mysqlSelectedTables.includes(table)}
											onCheckedChange={() =>
												toggleString(table, mysqlSelectedTables, (next) => setMysqlSelectedTables(next))
											}
										/>
										<span className="truncate">{table}</span>
									</label>
								))}
							</div>
						)}
					</div>
				</CardContent>
			</Card>

			<Card>
				<CardHeader>
					<CardTitle>Redis</CardTitle>
					<CardDescription>
						<Trans>Choose a database and clean keys by predefined patterns.</Trans>
					</CardDescription>
				</CardHeader>
				<CardContent className="space-y-4">
					<div className="grid gap-4 md:grid-cols-2">
						<div className="space-y-2">
							<Label>
								<Trans>Host</Trans>
							</Label>
							<Input value={redisHost} onChange={(e) => setRedisHost(e.target.value)} placeholder="127.0.0.1" />
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Port</Trans>
							</Label>
							<Input value={redisPort} onChange={(e) => setRedisPort(e.target.value)} placeholder="6379" />
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Username</Trans>
							</Label>
							<Input value={redisUsername} onChange={(e) => setRedisUsername(e.target.value)} placeholder="default" />
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Password</Trans>
							</Label>
							<Input
								type="password"
								value={redisPassword}
								onChange={(e) => {
									setRedisPassword(e.target.value)
									if (e.target.value.trim()) {
										setRedisUseStoredPassword(false)
									}
								}}
								placeholder={redisHasPassword ? t`Stored password available` : "********"}
							/>
							{redisHasPassword ? (
								<div className="flex items-center gap-2 text-xs text-muted-foreground">
									<Checkbox
										checked={redisUseStoredPassword}
										onCheckedChange={(checked) => setRedisUseStoredPassword(checked === true)}
									/>
									<span>
										<Trans>Use stored password</Trans>
									</span>
								</div>
							) : null}
						</div>
					</div>

					<div className="flex flex-wrap items-center gap-3">
						<Button variant="outline" onClick={() => void loadRedisDBs()} disabled={redisLoading}>
							{redisLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : <RefreshCwIcon className="me-2 h-4 w-4" />}
							<Trans>Load Databases</Trans>
						</Button>
						<Select value={redisDB} onValueChange={setRedisDB} disabled={redisDBs.length === 0}>
							<SelectTrigger className="min-w-[160px]">
								<SelectValue placeholder={t`Select DB`} />
							</SelectTrigger>
							<SelectContent>
								{redisDBs.map((db) => (
									<SelectItem key={db} value={String(db)}>
										<Trans>DB {db}</Trans>
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>

					<div className="rounded-md border p-3">
						{redisPatterns.length === 0 ? (
							<p className="text-sm text-muted-foreground">
								<Trans>No patterns configured.</Trans>
							</p>
						) : (
							<div className="grid max-h-40 gap-2 overflow-auto text-sm text-muted-foreground">
								{redisPatterns.map((pattern) => (
									<div key={pattern} className="flex items-center gap-2">
										<Checkbox checked={true} disabled />
										<span className="truncate">{pattern}</span>
									</div>
								))}
							</div>
						)}
					</div>
				</CardContent>
			</Card>

			<Card>
				<CardHeader>
					<CardTitle>MinIO</CardTitle>
					<CardDescription>
						<Trans>Select a bucket and top-level folders to clean.</Trans>
					</CardDescription>
				</CardHeader>
				<CardContent className="space-y-4">
					<div className="grid gap-4 md:grid-cols-2">
						<div className="space-y-2">
							<Label>
								<Trans>Host</Trans>
							</Label>
							<Input value={minioHost} onChange={(e) => setMinioHost(e.target.value)} placeholder="127.0.0.1" />
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Port</Trans>
							</Label>
							<Input value={minioPort} onChange={(e) => setMinioPort(e.target.value)} placeholder="9000" />
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Access Key</Trans>
							</Label>
							<Input value={minioAccessKey} onChange={(e) => setMinioAccessKey(e.target.value)} placeholder="minioadmin" />
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Secret Key</Trans>
							</Label>
							<Input
								type="password"
								value={minioSecretKey}
								onChange={(e) => {
									setMinioSecretKey(e.target.value)
									if (e.target.value.trim()) {
										setMinioUseStoredSecret(false)
									}
								}}
								placeholder={minioHasSecretKey ? t`Stored secret available` : "********"}
							/>
							{minioHasSecretKey ? (
								<div className="flex items-center gap-2 text-xs text-muted-foreground">
									<Checkbox
										checked={minioUseStoredSecret}
										onCheckedChange={(checked) => setMinioUseStoredSecret(checked === true)}
									/>
									<span>
										<Trans>Use stored secret</Trans>
									</span>
								</div>
							) : null}
						</div>
					</div>

					<div className="flex flex-wrap items-center gap-3">
						<Button variant="outline" onClick={() => void loadMinioBuckets()} disabled={minioBucketsLoading}>
							{minioBucketsLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : <RefreshCwIcon className="me-2 h-4 w-4" />}
							<Trans>Load Buckets</Trans>
						</Button>
						<Select value={minioBucket} onValueChange={setMinioBucket} disabled={minioBuckets.length === 0}>
							<SelectTrigger className="min-w-[200px]">
								<SelectValue placeholder={t`Select bucket`} />
							</SelectTrigger>
							<SelectContent>
								{minioBuckets.map((bucket) => (
									<SelectItem key={bucket} value={bucket}>
										{bucket}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
						<Button
							variant="outline"
							onClick={() => void loadMinioPrefixes()}
							disabled={minioPrefixesLoading || !minioBucket}
						>
							{minioPrefixesLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : <RefreshCwIcon className="me-2 h-4 w-4" />}
							<Trans>Load Folders</Trans>
						</Button>
					</div>

					<div className="rounded-md border p-3">
						{minioPrefixes.length === 0 ? (
							<p className="text-sm text-muted-foreground">
								<Trans>No folders loaded.</Trans>
							</p>
						) : (
							<div className="grid max-h-48 gap-2 overflow-auto">
								{minioPrefixes.map((prefix) => (
									<label key={prefix} className="flex items-center gap-2 text-sm">
										<Checkbox
											checked={minioSelectedPrefixes.includes(prefix)}
											onCheckedChange={() =>
												toggleString(prefix, minioSelectedPrefixes, (next) => setMinioSelectedPrefixes(next))
											}
										/>
										<span className="truncate">{prefix}</span>
									</label>
								))}
							</div>
						)}
					</div>
				</CardContent>
			</Card>

			<Card>
				<CardHeader>
					<CardTitle>Elasticsearch</CardTitle>
					<CardDescription>
						<Trans>Load indices from _cat/indices and delete documents by query.</Trans>
					</CardDescription>
				</CardHeader>
				<CardContent className="space-y-4">
					<div className="grid gap-4 md:grid-cols-2">
						<div className="space-y-2">
							<Label>
								<Trans>Host</Trans>
							</Label>
							<Input value={esHost} onChange={(e) => setEsHost(e.target.value)} placeholder="127.0.0.1" />
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Port</Trans>
							</Label>
							<Input value={esPort} onChange={(e) => setEsPort(e.target.value)} placeholder="9200" />
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Username</Trans>
							</Label>
							<Input value={esUsername} onChange={(e) => setEsUsername(e.target.value)} placeholder="elastic" />
						</div>
						<div className="space-y-2">
							<Label>
								<Trans>Password</Trans>
							</Label>
							<Input
								type="password"
								value={esPassword}
								onChange={(e) => {
									setEsPassword(e.target.value)
									if (e.target.value.trim()) {
										setEsUseStoredPassword(false)
									}
								}}
								placeholder={esHasPassword ? t`Stored password available` : "********"}
							/>
							{esHasPassword ? (
								<div className="flex items-center gap-2 text-xs text-muted-foreground">
									<Checkbox checked={esUseStoredPassword} onCheckedChange={(checked) => setEsUseStoredPassword(checked === true)} />
									<span>
										<Trans>Use stored password</Trans>
									</span>
								</div>
							) : null}
						</div>
					</div>

					<div className="flex flex-wrap items-center gap-3">
						<Button variant="outline" onClick={() => void loadESIndices()} disabled={esLoading}>
							{esLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : <RefreshCwIcon className="me-2 h-4 w-4" />}
							<Trans>Load Indices</Trans>
						</Button>
					</div>

					<div className="rounded-md border p-3">
						{esIndices.length === 0 ? (
							<p className="text-sm text-muted-foreground">
								<Trans>No indices loaded.</Trans>
							</p>
						) : (
							<div className="grid max-h-48 gap-2 overflow-auto">
								{esIndices.map((index) => (
									<label key={index} className="flex items-center gap-2 text-sm">
										<Checkbox
											checked={esSelectedIndices.includes(index)}
											onCheckedChange={() =>
												toggleString(index, esSelectedIndices, (next) => setEsSelectedIndices(next))
											}
										/>
										<span className="truncate">{index}</span>
									</label>
								))}
							</div>
						)}
					</div>
				</CardContent>
			</Card>

			<AlertDialog open={confirmOpen} onOpenChange={setConfirmOpen}>
				<AlertDialogContent>
					<AlertDialogHeader>
						<AlertDialogTitle>
							<Trans>Confirm cleanup</Trans>
						</AlertDialogTitle>
						<AlertDialogDescription>
							<Trans>Cleanup will delete data immediately. Please confirm the targets and proceed.</Trans>
						</AlertDialogDescription>
					</AlertDialogHeader>
					<AlertDialogFooter>
						<AlertDialogCancel>
							<Trans>Cancel</Trans>
						</AlertDialogCancel>
						<AlertDialogAction onClick={() => void startRun()} disabled={runLoading}>
							<Trans>Start cleanup</Trans>
						</AlertDialogAction>
					</AlertDialogFooter>
				</AlertDialogContent>
			</AlertDialog>

			<Dialog open={runOpen} onOpenChange={setRunOpen}>
				<DialogContent className="max-w-3xl">
					<DialogHeader>
						<DialogTitle>
							<Trans>Cleanup Progress</Trans>
						</DialogTitle>
						<DialogDescription>
							<Trans>Track cleanup steps, logs, and results in real time.</Trans>
						</DialogDescription>
					</DialogHeader>
					<div className="space-y-4">
						<div className="space-y-2">
							<div className="flex items-center justify-between text-sm">
								<span>
									<Trans>Current step</Trans>: {runStep || "-"}
								</span>
								<span>{runProgress}%</span>
							</div>
							<div className="h-2 overflow-hidden rounded-full bg-muted">
								<div className="h-full bg-primary transition-all" style={{ width: `${runProgress}%` }} />
							</div>
							<div className="flex items-center gap-2 text-sm">
								<Trans>Status</Trans>: {renderStatusBadge(runStatus)}
							</div>
						</div>

						<Separator />

						<div className="space-y-2">
							<p className="text-sm font-medium">
								<Trans>Module results</Trans>
							</p>
							{moduleSummary.length === 0 ? (
								<p className="text-sm text-muted-foreground">-</p>
							) : (
								<div className="grid gap-2 text-sm">
									{moduleSummary.map((result) => (
										<div key={`${result.module}-${result.status}`} className="flex items-center justify-between gap-2">
											<span className="font-medium">{result.module}</span>
											<div className="flex items-center gap-2">
												{renderStatusBadge(result.status)}
												{result.detail ? (
													<span className="max-w-[260px] truncate text-xs text-muted-foreground">{result.detail}</span>
												) : null}
											</div>
										</div>
									))}
								</div>
							)}
						</div>

						<Separator />

						<div className="space-y-2">
							<p className="text-sm font-medium">
								<Trans>Logs</Trans>
							</p>
							<div className="max-h-56 overflow-auto rounded-md border bg-muted/30 p-3 text-xs">
								{runLogs.length === 0 ? (
									<p className="text-muted-foreground">-</p>
								) : (
									<pre className="whitespace-pre-wrap break-words">{runLogs.join("\n")}</pre>
								)}
							</div>
						</div>

						<div className="flex flex-wrap justify-end gap-2">
							<Button variant="outline" onClick={() => setRunOpen(false)}>
								<Trans>Close</Trans>
							</Button>
							<Button variant="outline" onClick={() => void retryRun()} disabled={runStatus !== "failed" || runLoading}>
								{runLoading ? <LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" /> : <RotateCcwIcon className="me-2 h-4 w-4" />}
								<Trans>Retry</Trans>
							</Button>
						</div>
					</div>
				</DialogContent>
			</Dialog>
		</div>
	)
})

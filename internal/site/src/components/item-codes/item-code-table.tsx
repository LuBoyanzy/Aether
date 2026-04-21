// Item Code table with batch selection, actions, and expandable detail rows.
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import React, { memo, useCallback, useEffect, useRef, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { toast } from "@/components/ui/use-toast"
import { isAdmin, isReadOnlyUser } from "@/lib/api"
import { batchDeleteItemCodesByCode, deleteItemCodeByCode, getItemCodeDetailFromDB, listItemCodesFromDB } from "@/lib/itemCodeApi"
import { formatShortDate } from "@/lib/utils"
import type { ItemCodeDBDetail, ItemCodeDBRecord } from "@/types"
import AdminVerifyDialog, { checkAdminVerifyWindow } from "@/components/item-codes/admin-verify-dialog"
import ItemCodeDetail from "@/components/item-codes/item-code-detail"
import { ChevronDownIcon, ChevronRightIcon, ClipboardListIcon, EditIcon, LoaderCircleIcon, RefreshCwIcon, Trash2Icon } from "lucide-react"

interface ItemCodeTableProps {
	onEdit?: (record: ItemCodeDBRecord) => void
	onQueryDelete?: () => void
	onAuditLogs?: () => void
}

export default memo(function ItemCodeTable({ onEdit, onQueryDelete, onAuditLogs }: ItemCodeTableProps) {
	const [items, setItems] = useState<ItemCodeDBRecord[]>([])
	const [total, setTotal] = useState(0)
	const [loading, setLoading] = useState(false)
	const [page, setPage] = useState(1)
	const [perPage] = useState(50)
	const [search, setSearch] = useState("")
	const [statusFilter, setStatusFilter] = useState("all")
	const [selectedCodes, setSelectedCodes] = useState<Set<string>>(new Set())
	const [deleting, setDeleting] = useState(false)
	const [adminVerifyOpen, setAdminVerifyOpen] = useState(false)
	const [pendingAction, setPendingAction] = useState<"singleDelete" | "batchDelete" | "queryDelete" | "auditLogs" | null>(null)
	const [pendingDeleteCode, setPendingDeleteCode] = useState<string | null>(null)
	const [expandedCode, setExpandedCode] = useState<string | null>(null)
	const [detailMap, setDetailMap] = useState<Record<string, ItemCodeDBDetail>>({})
	const [detailLoading, setDetailLoading] = useState<Set<string>>(new Set())
	const latestRequestRef = useRef<number>(0)

	const loadItems = useCallback(async () => {
		const requestId = Date.now()
		latestRequestRef.current = requestId
		setLoading(true)
		try {
			const res = await listItemCodesFromDB({
				page,
				perPage,
				search: search.trim(),
				status: statusFilter,
			})
			// 忽略过期请求的结果
			if (latestRequestRef.current !== requestId) return
			setItems(res.items)
			setTotal(res.total)
			setSelectedCodes(new Set())
		} catch (err: any) {
			// 忽略过期请求的错误
			if (latestRequestRef.current !== requestId) return
			toast({ variant: "destructive", title: t`错误`, description: t`加载失败` })
		} finally {
			// 只有最新请求才关闭 loading
			if (latestRequestRef.current === requestId) {
				setLoading(false)
			}
		}
	}, [page, perPage, search, statusFilter])

	useEffect(() => {
		const timer = setTimeout(() => {
			void loadItems()
		}, 300)
		return () => clearTimeout(timer)
	}, [loadItems])

	const toggleSelectAll = useCallback(() => {
		if (selectedCodes.size === items.length && items.length > 0) {
			setSelectedCodes(new Set())
		} else {
			setSelectedCodes(new Set(items.map((i) => i.code)))
		}
	}, [selectedCodes.size, items])

	const toggleSelect = useCallback((code: string) => {
		setSelectedCodes((prev) => {
			const next = new Set(prev)
			if (next.has(code)) {
				next.delete(code)
			} else {
				next.add(code)
			}
			return next
		})
	}, [])

	const toggleExpand = useCallback(async (code: string) => {
		if (expandedCode === code) {
			setExpandedCode(null)
			return
		}
		setExpandedCode(code)
		if (!detailMap[code] && !detailLoading.has(code)) {
			setDetailLoading((prev) => new Set(prev).add(code))
			try {
				const detail = await getItemCodeDetailFromDB(code)
				setDetailMap((prev) => ({ ...prev, [code]: detail }))
			} catch (err: any) {
				toast({ variant: "destructive", title: t`错误`, description: t`加载详情失败` })
			} finally {
				setDetailLoading((prev) => {
					const next = new Set(prev)
					next.delete(code)
					return next
				})
			}
		}
	}, [expandedCode, detailMap, detailLoading])

	const doDelete = useCallback(async (code: string, password: string) => {
		try {
			await deleteItemCodeByCode(code, password)
			toast({ title: t`已删除`, description: t`Item Code 已删除` })
			void loadItems()
		} catch (err: any) {
			toast({ variant: "destructive", title: t`错误`, description: err?.message || t`删除失败` })
		}
	}, [loadItems])

	const doBatchDelete = useCallback(async (password: string) => {
		if (selectedCodes.size === 0) return
		setDeleting(true)
		try {
			const res = await batchDeleteItemCodesByCode(Array.from(selectedCodes), password)
			toast({ title: t`已删除`, description: t`已删除 ${res.deleted} 项` })
			void loadItems()
		} catch (err: any) {
			toast({ variant: "destructive", title: t`错误`, description: err?.message || t`批量删除失败` })
		} finally {
			setDeleting(false)
		}
	}, [selectedCodes, loadItems])

	const handleDelete = useCallback(
		async (code: string) => {
			if (!isAdmin()) {
				toast({ variant: "destructive", title: t`无权限`, description: t`只有管理员可以删除` })
				return
			}
			if (!confirm(t`确定要删除此项吗？`)) return
			setPendingDeleteCode(code)
			setPendingAction("singleDelete")
			setAdminVerifyOpen(true)
		},
		[]
	)

	const handleBatchDelete = useCallback(async () => {
		if (selectedCodes.size === 0) return
		if (!isAdmin()) {
			toast({ variant: "destructive", title: t`无权限`, description: t`只有管理员可以删除` })
			return
		}
		if (!confirm(t`确定要删除 ${selectedCodes.size} 项吗？`)) return
		setPendingAction("batchDelete")
		setAdminVerifyOpen(true)
	}, [selectedCodes])

	const renderStatusBadge = (status: string) => {
		if (status === "active") {
			return <Badge className="bg-emerald-500/15 text-emerald-600 hover:bg-emerald-500/25">{t`启用`}</Badge>
		}
		if (status === "inactive") {
			return <Badge variant="secondary">{t`停用`}</Badge>
		}
		if (status === "obsolete") {
			return <Badge variant="destructive">{t`废弃`}</Badge>
		}
		return <Badge variant="outline">{status}</Badge>
	}

	const colCount = isAdmin() ? 8 : 7

	return (
		<Card className="p-6 @container w-full">
			<CardHeader className="p-0 mb-4">
				<div className="flex flex-wrap items-end justify-between gap-3">
					<div>
						<CardTitle className="mb-2">
							<Trans>Item Code 管理</Trans>
						</CardTitle>
						<CardDescription>
							<Trans>管理 Item Code 并追踪变更。</Trans>
						</CardDescription>
					</div>
					<div className="flex flex-wrap items-center gap-2">
						{isAdmin() && selectedCodes.size > 0 && (
							<Button variant="destructive" size="sm" onClick={() => void handleBatchDelete()} disabled={deleting}>
								<Trash2Icon className="me-2 h-4 w-4" />
								<Trans>批量删除</Trans>
								<span className="ms-1">({selectedCodes.size})</span>
							</Button>
						)}
						{onQueryDelete && (
							<Button variant="outline" size="sm" onClick={() => {
								if (isAdmin()) {
									onQueryDelete()
								} else {
									setPendingAction("queryDelete")
									setAdminVerifyOpen(true)
								}
							}}>
								<Trash2Icon className="me-2 h-4 w-4" />
								<Trans>查询删除</Trans>
							</Button>
						)}
						{onAuditLogs && (
							<Button variant="outline" size="sm" onClick={() => {
								if (isAdmin()) {
									onAuditLogs()
								} else {
									setPendingAction("auditLogs")
									setAdminVerifyOpen(true)
								}
							}}>
								<ClipboardListIcon className="me-2 h-4 w-4" />
								<Trans>审计日志</Trans>
							</Button>
						)}
						<Button variant="outline" size="sm" onClick={() => void loadItems()} disabled={loading}>
							{loading ? (
								<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
							) : (
								<RefreshCwIcon className="me-2 h-4 w-4" />
							)}
							<Trans>刷新</Trans>
						</Button>
					</div>
				</div>
			</CardHeader>
			<CardContent className="p-0 space-y-4">
				<div className="flex flex-wrap items-end gap-3">
					<Input
						className="w-56"
						placeholder={t`搜索编码或名称...`}
						value={search}
						onChange={(e) => { setSearch(e.target.value); setPage(1) }}
					/>
					<Select value={statusFilter} onValueChange={(v) => { setStatusFilter(v); setPage(1) }}>
						<SelectTrigger className="h-9 w-36">
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="all">{t`全部状态`}</SelectItem>
							<SelectItem value="active">{t`启用`}</SelectItem>
							<SelectItem value="inactive">{t`停用`}</SelectItem>
							<SelectItem value="obsolete">{t`废弃`}</SelectItem>
						</SelectContent>
					</Select>
				</div>
				<div className="h-min max-h-[calc(100dvh-26rem)] max-w-full relative overflow-auto border rounded-md bg-card">
					<Table>
						<TableHeader className="sticky top-0 z-10 bg-card">
							<TableRow>
								{isAdmin() && (
									<TableHead className="w-[40px]">
										<Checkbox
											checked={items.length > 0 && selectedCodes.size === items.length}
											data-state={selectedCodes.size > 0 && selectedCodes.size < items.length ? 'indeterminate' : undefined}
											onCheckedChange={toggleSelectAll}
										/>
									</TableHead>
								)}
								<TableHead className="w-[140px]">
									<Trans>编码</Trans>
								</TableHead>
								<TableHead>
									<Trans>名称</Trans>
								</TableHead>
								<TableHead className="w-[120px]">
									<Trans>分类</Trans>
								</TableHead>
								<TableHead className="w-[100px]">
									<Trans>状态</Trans>
								</TableHead>
								<TableHead className="w-[200px]">
									<Trans>描述</Trans>
								</TableHead>
								<TableHead className="w-[150px]">
									<Trans>更新于</Trans>
								</TableHead>
								<TableHead className="w-[100px] text-right">
									<Trans>操作</Trans>
								</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{items.length === 0 ? (
								<TableRow>
									<TableCell
										colSpan={colCount}
										className="py-10 text-center text-sm text-muted-foreground"
									>
										<Trans>暂无 Item Code。</Trans>
									</TableCell>
								</TableRow>
							) : (
								items.map((item) => (
									<React.Fragment key={item.code}>
										<TableRow
											className={expandedCode === item.code ? "bg-muted/30" : undefined}
										>
											{isAdmin() && (
												<TableCell>
													<Checkbox
														checked={selectedCodes.has(item.code)}
														onCheckedChange={() => toggleSelect(item.code)}
													/>
												</TableCell>
											)}
											<TableCell className="font-mono text-xs">
												<Button
													variant="ghost"
													size="sm"
													className="h-auto px-1 py-0 font-mono"
													onClick={() => toggleExpand(item.code)}
												>
													{expandedCode === item.code ? (
														<ChevronDownIcon className="h-3 w-3 mr-1" />
													) : (
														<ChevronRightIcon className="h-3 w-3 mr-1" />
													)}
													{item.code}
												</Button>
											</TableCell>
											<TableCell>{item.name}</TableCell>
											<TableCell>{item.category || "-"}</TableCell>
											<TableCell>{renderStatusBadge(item.status)}</TableCell>
											<TableCell>
												<div className="max-w-[200px] truncate text-muted-foreground" title={item.description || "-"}>
													{item.description || "-"}
												</div>
											</TableCell>
											<TableCell className="tabular-nums text-xs">
												{formatShortDate(item.updated)}
											</TableCell>
											<TableCell className="text-right">
												<div className="flex items-center justify-end gap-1">
													{!isReadOnlyUser() && (
														<Button
															variant="ghost"
															size="icon"
															className="h-8 w-8"
															onClick={() => onEdit?.(item)}
														>
															<EditIcon className="h-4 w-4" />
														</Button>
													)}
													{!isReadOnlyUser() && (
														<Button
															variant="ghost"
															size="icon"
															className="h-8 w-8 text-destructive"
															onClick={() => handleDelete(item.code)}
														>
															<Trash2Icon className="h-4 w-4" />
														</Button>
													)}
												</div>
											</TableCell>
										</TableRow>
										{expandedCode === item.code && (
											<TableRow key={`${item.code}-detail`}>
												<TableCell colSpan={colCount} className="p-0">
													{detailMap[item.code] ? (
														<ItemCodeDetail detail={detailMap[item.code]} />
													) : (
														<div className="p-8 text-center text-sm text-muted-foreground">
															{detailLoading.has(item.code) ? (
																<div className="flex items-center justify-center gap-2">
																	<LoaderCircleIcon className="h-4 w-4 animate-spin" />
																	<Trans>加载详情中...</Trans>
																</div>
															) : (
																<Trans>加载详情失败</Trans>
															)}
														</div>
													)}
												</TableCell>
											</TableRow>
										)}
									</React.Fragment>
								))
							)}
						</TableBody>
					</Table>
				</div>
				<div className="flex items-center justify-between pt-1">
					<div className="text-xs text-muted-foreground">
						<Trans>{total} 条记录</Trans>
					</div>
					<div className="flex items-center gap-2">
						<Button
							variant="outline"
							size="sm"
							onClick={() => setPage((prev) => Math.max(1, prev - 1))}
							disabled={page <= 1 || loading}
						>
							<Trans>上一页</Trans>
						</Button>
						<div className="text-xs text-muted-foreground min-w-[3rem] text-center font-medium">
							{t`第 ${page} 页`}
						</div>
						<Button
							variant="outline"
							size="sm"
							onClick={() => setPage((prev) => prev + 1)}
							disabled={page * perPage >= total || loading}
						>
							<Trans>下一页</Trans>
						</Button>
					</div>
				</div>
			</CardContent>
			<AdminVerifyDialog
				open={adminVerifyOpen}
				onOpenChange={setAdminVerifyOpen}
					onVerified={(password) => {
						if (pendingAction === "singleDelete" && pendingDeleteCode) {
							void doDelete(pendingDeleteCode, password)
						} else if (pendingAction === "batchDelete") {
							void doBatchDelete(password)
						} else if (pendingAction === "queryDelete" && onQueryDelete) {
							onQueryDelete()
						} else if (pendingAction === "auditLogs" && onAuditLogs) {
							onAuditLogs()
						}
						setPendingAction(null)
						setPendingDeleteCode(null)
					}}
			/>
		</Card>
	)
})

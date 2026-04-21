// Item Code table with batch selection and actions.
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { toast } from "@/components/ui/use-toast"
import { isAdmin, isReadOnlyUser } from "@/lib/api"
import { batchDeleteItemCodes, deleteItemCode, listItemCodes } from "@/lib/itemCodeApi"
import { formatShortDate } from "@/lib/utils"
import type { ItemCodeRecord } from "@/types"
import AdminVerifyDialog from "@/components/item-codes/admin-verify-dialog"
import { ClipboardListIcon, EditIcon, LoaderCircleIcon, RefreshCwIcon, Trash2Icon } from "lucide-react"

interface ItemCodeTableProps {
	onEdit?: (record: ItemCodeRecord) => void
	onQueryDelete?: () => void
	onAuditLogs?: () => void
}

export default memo(function ItemCodeTable({ onEdit, onQueryDelete, onAuditLogs }: ItemCodeTableProps) {
	const [items, setItems] = useState<ItemCodeRecord[]>([])
	const [loading, setLoading] = useState(false)
	const [page, setPage] = useState(1)
	const [perPage] = useState(50)
	const [filter, setFilter] = useState("")
	const [statusFilter, setStatusFilter] = useState("all")
	const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())
	const [deleting, setDeleting] = useState(false)
	const [adminVerifyOpen, setAdminVerifyOpen] = useState(false)
	const [pendingAction, setPendingAction] = useState<"queryDelete" | "auditLogs" | null>(null)

	const loadItems = useCallback(async () => {
		setLoading(true)
		try {
			const filters: string[] = []
			if (filter.trim()) {
				filters.push(`(code ~ "${filter.trim()}" || name ~ "${filter.trim()}")`)
			}
			if (statusFilter !== "all") {
				filters.push(`status = "${statusFilter}"`)
			}
			const res = await listItemCodes({
				page,
				perPage,
				filter: filters.join(" && "),
			})
			setItems(res.items)
			setSelectedIds(new Set())
		} catch (err: any) {
			toast({ variant: "destructive", title: t`错误`, description: t`加载失败` })
		} finally {
			setLoading(false)
		}
	}, [page, perPage, filter, statusFilter])

	useEffect(() => {
		void loadItems()
	}, [loadItems])

	const filteredItems = useMemo(() => {
		const term = filter.trim().toLowerCase()
		if (!term && statusFilter === "all") return items
		return items
	}, [items, filter, statusFilter])

	const toggleSelectAll = useCallback(() => {
		if (selectedIds.size === filteredItems.length && filteredItems.length > 0) {
			setSelectedIds(new Set())
		} else {
			setSelectedIds(new Set(filteredItems.map((i) => i.id)))
		}
	}, [selectedIds.size, filteredItems])

	const toggleSelect = useCallback((id: string) => {
		setSelectedIds((prev) => {
			const next = new Set(prev)
			if (next.has(id)) {
				next.delete(id)
			} else {
				next.add(id)
			}
			return next
		})
	}, [])

	const handleDelete = useCallback(
		async (id: string) => {
			if (!confirm(t`确定要删除此项吗？`)) return
			try {
				await deleteItemCode(id)
				toast({ title: t`已删除`, description: t`Item Code 已删除` })
				void loadItems()
			} catch (err: any) {
				toast({ variant: "destructive", title: t`错误`, description: err?.message || t`删除失败` })
			}
		},
		[loadItems]
	)

	const handleBatchDelete = useCallback(async () => {
		if (selectedIds.size === 0) return
		if (!confirm(t`确定要删除 ${selectedIds.size} 项吗？`)) return
		setDeleting(true)
		try {
			const res = await batchDeleteItemCodes(Array.from(selectedIds))
			toast({ title: t`已删除`, description: t`已删除 ${res.deleted} 项` })
			void loadItems()
		} catch (err: any) {
			toast({ variant: "destructive", title: t`错误`, description: err?.message || t`批量删除失败` })
		} finally {
			setDeleting(false)
		}
	}, [selectedIds, loadItems])

	const renderStatusBadge = (status: string) => {
		if (status === "active") {
			return <Badge className="bg-emerald-500/15 text-emerald-600 hover:bg-emerald-500/25">启用</Badge>
		}
		if (status === "inactive") {
			return <Badge variant="secondary">停用</Badge>
		}
		if (status === "obsolete") {
			return <Badge variant="destructive">废弃</Badge>
		}
		return <Badge variant="outline">{status}</Badge>
	}

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
						{isAdmin() && selectedIds.size > 0 && (
							<Button variant="destructive" size="sm" onClick={() => void handleBatchDelete()} disabled={deleting}>
								<Trash2Icon className="me-2 h-4 w-4" />
								<Trans>批量删除</Trans>
								<span className="ms-1">({selectedIds.size})</span>
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
						value={filter}
						onChange={(e) => { setFilter(e.target.value); setPage(1) }}
					/>
					<Select value={statusFilter} onValueChange={(v) => { setStatusFilter(v); setPage(1) }}>
						<SelectTrigger className="h-9 w-36">
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="all">{t`全部状态`}</SelectItem>
							<SelectItem value="active">启用</SelectItem>
							<SelectItem value="inactive">停用</SelectItem>
							<SelectItem value="obsolete">废弃</SelectItem>
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
											checked={filteredItems.length > 0 && selectedIds.size === filteredItems.length}
											data-state={selectedIds.size > 0 && selectedIds.size < filteredItems.length ? 'indeterminate' : undefined}
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
							{filteredItems.length === 0 ? (
								<TableRow>
									<TableCell
										colSpan={isAdmin() ? 8 : 7}
										className="py-10 text-center text-sm text-muted-foreground"
									>
										<Trans>暂无 Item Code。</Trans>
									</TableCell>
								</TableRow>
							) : (
								filteredItems.map((item) => (
									<TableRow key={item.id}>
										{isAdmin() && (
											<TableCell>
												<Checkbox
													checked={selectedIds.has(item.id)}
													onCheckedChange={() => toggleSelect(item.id)}
												/>
											</TableCell>
										)}
										<TableCell className="font-mono text-xs">{item.code}</TableCell>
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
														onClick={() => handleDelete(item.id)}
													>
														<Trash2Icon className="h-4 w-4" />
													</Button>
												)}
											</div>
										</TableCell>
									</TableRow>
								))
							)}
						</TableBody>
					</Table>
				</div>
				<div className="flex items-center justify-between pt-1">
					<div className="text-xs text-muted-foreground">
						<Trans>{filteredItems.length} 条记录</Trans>
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
							disabled={filteredItems.length < perPage || loading}
						>
							<Trans>下一页</Trans>
						</Button>
					</div>
				</div>
			</CardContent>
			<AdminVerifyDialog
				open={adminVerifyOpen}
				onOpenChange={setAdminVerifyOpen}
				onVerified={() => {
					if (pendingAction === "queryDelete" && onQueryDelete) {
						onQueryDelete()
					} else if (pendingAction === "auditLogs" && onAuditLogs) {
						onAuditLogs()
					}
					setPendingAction(null)
				}}
			/>
		</Card>
	)
})

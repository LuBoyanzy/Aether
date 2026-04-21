// Audit logs dialog for Item Code management.
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useEffect, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
// import { Input } from "@/components/ui/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { toast } from "@/components/ui/use-toast"
import { listItemCodeAuditLogs } from "@/lib/itemCodeApi"
import { formatShortDate } from "@/lib/utils"
import type { ItemCodeAuditItem } from "@/types"
import { ChevronLeftIcon, ChevronRightIcon, LoaderCircleIcon, RefreshCwIcon } from "lucide-react"

interface AuditLogsDialogProps {
	open: boolean
	onOpenChange: (open: boolean) => void
}

const perPageOptions = [20, 50, 100]

export default memo(function AuditLogsDialog({ open, onOpenChange }: AuditLogsDialogProps) {
	const [items, setItems] = useState<ItemCodeAuditItem[]>([])
	const [loading, setLoading] = useState(false)
	const [page, setPage] = useState(1)
	const [perPage, setPerPage] = useState(perPageOptions[0])
	const [actionFilter, setActionFilter] = useState("all")

	const loadLogs = useCallback(async () => {
		setLoading(true)
		try {
			const params: { action?: string; page: number; perPage: number } = { page, perPage }
			if (actionFilter !== "all") {
				params.action = actionFilter
			}
			const res = await listItemCodeAuditLogs(params)
			setItems(res.items ?? [])
		} catch (err: any) {
			toast({ variant: "destructive", title: t`错误`, description: t`加载审计日志失败` })
		} finally {
			setLoading(false)
		}
	}, [page, perPage, actionFilter])

	useEffect(() => {
		if (open) {
			void loadLogs()
		}
	}, [open, loadLogs])

	const renderStatusBadge = (status: string) => {
		const normalized = status.trim().toLowerCase()
		if (normalized === "success") {
			return <Badge className="bg-emerald-500/15 text-emerald-600 hover:bg-emerald-500/25">{t`成功`}</Badge>
		}
		if (normalized === "failed") {
			return <Badge variant="destructive">{t`失败`}</Badge>
		}
		return <Badge variant="secondary">{status || t`未知`}</Badge>
	}

	const hasNextPage = items.length === perPage

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-5xl max-h-[90dvh] overflow-auto">
				<DialogHeader>
					<DialogTitle>
						<Trans>Item Code 审计日志</Trans>
					</DialogTitle>
					<DialogDescription>
						<Trans>查看所有 Item Code 操作的历史记录。</Trans>
					</DialogDescription>
				</DialogHeader>
				<div className="flex flex-wrap items-center gap-2 mb-4">
					<Select value={actionFilter} onValueChange={(v) => { setActionFilter(v); setPage(1) }}>
						<SelectTrigger className="h-9 w-44">
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="all">{t`全部操作`}</SelectItem>
							<SelectItem value="single_delete">{t`单条删除`}</SelectItem>
							<SelectItem value="batch_delete">{t`批量删除`}</SelectItem>
							<SelectItem value="query_delete">{t`查询删除`}</SelectItem>
						</SelectContent>
					</Select>
					<Button variant="outline" size="sm" onClick={() => void loadLogs()} disabled={loading}>
						{loading ? (
							<LoaderCircleIcon className="me-2 h-4 w-4 animate-spin" />
						) : (
							<RefreshCwIcon className="me-2 h-4 w-4" />
						)}
						<Trans>刷新</Trans>
					</Button>
				</div>
				<div className="h-min max-h-[calc(100dvh-22rem)] overflow-auto border rounded-md bg-card">
					<Table>
						<TableHeader className="sticky top-0 z-10 bg-card">
							<TableRow>
								<TableHead className="w-[140px]">
									<Trans>操作</Trans>
								</TableHead>
								<TableHead className="w-[200px]">
									<Trans>目标 ID</Trans>
								</TableHead>
								<TableHead className="w-[120px]">
									<Trans>状态</Trans>
								</TableHead>
								<TableHead className="w-[200px]">
									<Trans>筛选条件</Trans>
								</TableHead>
								<TableHead className="w-[200px]">
									<Trans>详情</Trans>
								</TableHead>
								<TableHead className="w-[150px]">
									<Trans>创建时间</Trans>
								</TableHead>
							</TableRow>
						</TableHeader>
						<TableBody>
							{items.length === 0 ? (
								<TableRow>
									<TableCell colSpan={6} className="py-10 text-center text-sm text-muted-foreground">
										<Trans>暂无审计日志。</Trans>
									</TableCell>
								</TableRow>
							) : (
								items.map((item) => (
									<TableRow key={item.id}>
										<TableCell>
											<Badge variant="outline" className="font-mono text-[10px]">
												{item.action}
											</Badge>
										</TableCell>
										<TableCell className="max-w-[200px]">
											<div className="truncate font-mono text-xs" title={item.target_ids || "-"}>
												{item.target_ids || "-"}
											</div>
										</TableCell>
										<TableCell>{renderStatusBadge(item.status)}</TableCell>
										<TableCell className="max-w-[200px]">
											<div className="truncate text-muted-foreground" title={item.filter || "-"}>
												{item.filter || "-"}
											</div>
										</TableCell>
										<TableCell>
											<div className="max-w-[200px] truncate text-muted-foreground" title={item.detail || "-"}>
												{item.detail || "-"}
											</div>
										</TableCell>
										<TableCell className="tabular-nums text-xs" title={`${item.created} UTC`}>
											{formatShortDate(item.created)}
										</TableCell>
									</TableRow>
								))
							)}
						</TableBody>
					</Table>
				</div>
				<div className="flex items-center justify-between pt-3">
					<div className="flex items-center gap-2 text-xs text-muted-foreground">
						<Trans>每页行数</Trans>
						<Select
							value={String(perPage)}
							onValueChange={(value) => {
								const parsed = Number(value)
								setPage(1)
								setPerPage(Number.isFinite(parsed) ? parsed : perPageOptions[0])
							}}
						>
							<SelectTrigger className="h-8 w-[70px]">
								<SelectValue />
							</SelectTrigger>
							<SelectContent>
								{perPageOptions.map((option) => (
									<SelectItem key={option} value={String(option)}>
										{option}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
					<div className="flex items-center gap-2">
						<Button
							variant="outline"
							size="icon"
							className="h-8 w-8"
							onClick={() => setPage((prev) => Math.max(1, prev - 1))}
							disabled={page <= 1}
						>
							<ChevronLeftIcon className="h-4 w-4" />
							<span className="sr-only">
								<Trans>上一页</Trans>
							</span>
						</Button>
						<div className="text-xs text-muted-foreground min-w-[3.5rem] text-center font-medium">
							{t`第 ${page} 页`}
						</div>
						<Button
							variant="outline"
							size="icon"
							className="h-8 w-8"
							onClick={() => setPage((prev) => prev + 1)}
							disabled={!hasNextPage}
						>
							<span className="sr-only">
								<Trans>下一页</Trans>
							</span>
							<ChevronRightIcon className="h-4 w-4" />
						</Button>
					</div>
				</div>
			</DialogContent>
		</Dialog>
	)
})

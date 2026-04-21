// Query delete dialog for Item Code management.
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { toast } from "@/components/ui/use-toast"
import { previewQueryDeleteItemCodes, queryDeleteItemCodes } from "@/lib/itemCodeApi"

interface QueryDeleteDialogProps {
	open: boolean
	onOpenChange: (open: boolean) => void
	onSuccess?: () => void
}

export default memo(function QueryDeleteDialog({ open, onOpenChange, onSuccess }: QueryDeleteDialogProps) {
	const [filter, setFilter] = useState("")
	const [previewing, setPreviewing] = useState(false)
	const [deleting, setDeleting] = useState(false)
	const [previewItems, setPreviewItems] = useState<{ id: string; code: string; name: string; category: string; status: string }[]>([])
	const [showPreview, setShowPreview] = useState(false)

	const handlePreview = useCallback(async () => {
		if (!filter.trim()) {
			toast({ variant: "destructive", title: t`错误`, description: t`筛选条件为必填项` })
			return
		}
		setPreviewing(true)
		try {
			const res = await previewQueryDeleteItemCodes(filter.trim())
			setPreviewItems(res.items ?? [])
			setShowPreview(true)
		} catch (err: any) {
			toast({ variant: "destructive", title: t`错误`, description: err?.message || t`预览失败` })
		} finally {
			setPreviewing(false)
		}
	}, [filter])

	const handleDelete = useCallback(async () => {
		setDeleting(true)
		try {
			const res = await queryDeleteItemCodes(filter.trim())
			toast({ title: t`已删除`, description: t`已删除 ${res.deleted} 条记录` })
			onOpenChange(false)
			setShowPreview(false)
			setPreviewItems([])
			setFilter("")
			onSuccess?.()
		} catch (err: any) {
			toast({ variant: "destructive", title: t`错误`, description: err?.message || t`删除失败` })
		} finally {
			setDeleting(false)
		}
	}, [filter, onOpenChange, onSuccess])

	const handleClose = useCallback(() => {
		onOpenChange(false)
		setShowPreview(false)
		setPreviewItems([])
		setFilter("")
	}, [onOpenChange])

	return (
		<Dialog open={open} onOpenChange={handleClose}>
			<DialogContent className="sm:max-w-2xl max-h-[90dvh] overflow-auto">
				<DialogHeader>
					<DialogTitle>
						<Trans>查询删除</Trans>
					</DialogTitle>
				</DialogHeader>
				<div className="grid gap-4 py-2">
					<div className="grid gap-2">
						<Label htmlFor="query-filter">
							<Trans>筛选表达式</Trans>
						</Label>
						<Input
							id="query-filter"
							value={filter}
							onChange={(e) => setFilter(e.target.value)}
							placeholder={t`例如：category = 'obsolete' && status = 'inactive'`}
							disabled={showPreview}
						/>
					</div>
					{!showPreview && (
						<Button onClick={() => void handlePreview()} disabled={previewing}>
							{previewing ? t`预览中...` : <Trans>预览</Trans>}
						</Button>
					)}
					{showPreview && (
						<>
							<div className="text-sm text-muted-foreground">
								<Trans>匹配到 {previewItems.length} 条记录</Trans>
							</div>
							<div className="h-min max-h-[40dvh] overflow-auto border rounded-md bg-card">
								<Table>
									<TableHeader className="sticky top-0 z-10 bg-card">
										<TableRow>
											<TableHead>
												<Trans>编码</Trans>
											</TableHead>
											<TableHead>
												<Trans>名称</Trans>
											</TableHead>
											<TableHead>
												<Trans>分类</Trans>
											</TableHead>
											<TableHead>
												<Trans>状态</Trans>
											</TableHead>
										</TableRow>
									</TableHeader>
									<TableBody>
										{previewItems.length === 0 ? (
											<TableRow>
												<TableCell colSpan={4} className="py-8 text-center text-sm text-muted-foreground">
													<Trans>无匹配记录。</Trans>
												</TableCell>
											</TableRow>
										) : (
											previewItems.map((item) => (
												<TableRow key={item.id}>
													<TableCell className="font-mono text-xs">{item.code}</TableCell>
													<TableCell>{item.name}</TableCell>
													<TableCell>{item.category || "-"}</TableCell>
													<TableCell>{item.status}</TableCell>
												</TableRow>
											))
										)}
									</TableBody>
								</Table>
							</div>
						</>
					)}
				</div>
				<DialogFooter>
					<Button variant="outline" onClick={handleClose}>
						<Trans>取消</Trans>
					</Button>
					{showPreview && previewItems.length > 0 && (
						<Button variant="destructive" onClick={() => void handleDelete()} disabled={deleting}>
							{deleting ? t`删除中...` : <Trans>确认删除</Trans>}
						</Button>
					)}
				</DialogFooter>
			</DialogContent>
		</Dialog>
	)
})

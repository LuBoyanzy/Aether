// Query delete dialog for Item Code management (SQL-based).
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useState } from "react"
import Editor from "react-simple-code-editor"
import { highlight, languages } from "prismjs"
import "prismjs/components/prism-sql"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { toast } from "@/components/ui/use-toast"
import { previewQueryDeleteItemCodes, queryDeleteItemCodes } from "@/lib/itemCodeApi"
import AdminVerifyDialog from "@/components/item-codes/admin-verify-dialog"

interface QueryDeleteDialogProps {
	open: boolean
	onOpenChange: (open: boolean) => void
	onSuccess?: () => void
}

export default memo(function QueryDeleteDialog({ open, onOpenChange, onSuccess }: QueryDeleteDialogProps) {
	const [sql, setSql] = useState("")
	const [previewing, setPreviewing] = useState(false)
	const [deleting, setDeleting] = useState(false)
	const [previewItems, setPreviewItems] = useState<{ code: string; name: string; category: string; status: string }[]>([])
	const [executedSQL, setExecutedSQL] = useState("")
	const [showPreview, setShowPreview] = useState(false)
	const [confirmed, setConfirmed] = useState(false)
	const [adminVerifyOpen, setAdminVerifyOpen] = useState(false)

	const handlePreview = useCallback(async () => {
		if (!sql.trim()) {
			toast({ variant: "destructive", title: t`错误`, description: t`SQL 为必填项` })
			return
		}
		setPreviewing(true)
		try {
			const res = await previewQueryDeleteItemCodes(sql.trim())
			setPreviewItems(res.items ?? [])
			setExecutedSQL(res.executedSQL ?? "")
			setShowPreview(true)
			setConfirmed(false)
		} catch (err: any) {
			toast({ variant: "destructive", title: t`错误`, description: err?.message || t`预览失败` })
		} finally {
			setPreviewing(false)
		}
	}, [sql])

	const handleVerifiedDelete = useCallback(
		async (password: string) => {
			setDeleting(true)
			try {
				const res = await queryDeleteItemCodes(sql.trim(), password)
				toast({ title: t`已删除`, description: t`已删除 ${res.deleted} 条记录` })
				onOpenChange(false)
				setShowPreview(false)
				setPreviewItems([])
				setExecutedSQL("")
				setSql("")
				setConfirmed(false)
				onSuccess?.()
			} catch (err: any) {
				toast({ variant: "destructive", title: t`错误`, description: err?.message || t`删除失败` })
			} finally {
				setDeleting(false)
			}
		},
		[sql, onOpenChange, onSuccess]
	)

	const handleDeleteClick = useCallback(() => {
		setAdminVerifyOpen(true)
	}, [])

	const handleClose = useCallback(() => {
		onOpenChange(false)
		setShowPreview(false)
		setPreviewItems([])
		setExecutedSQL("")
		setSql("")
		setConfirmed(false)
	}, [onOpenChange])

	return (
		<Dialog open={open} onOpenChange={handleClose}>
			<DialogContent className="sm:max-w-3xl max-h-[90dvh] overflow-auto">
				<DialogHeader>
					<DialogTitle>
						<Trans>SQL 查询删除</Trans>
					</DialogTitle>
					<DialogDescription>
						<Trans>输入 SQL DELETE 语句以批量删除匹配的 Item Code。</Trans>
					</DialogDescription>
				</DialogHeader>
				<div className="grid gap-4 py-2">
					<div className="grid gap-2">
						<Label htmlFor="query-sql">
							<Trans>SQL 语句</Trans>
						</Label>
						<div className="border rounded-md font-mono text-sm bg-background">
							<Editor
								value={sql}
								onValueChange={setSql}
								highlight={(code) => highlight(code, languages.sql, "sql")}
								padding={12}
								style={{
									fontFamily: "ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace",
									fontSize: 14,
									minHeight: 80,
								}}
								textareaClassName="focus:outline-none"
								disabled={showPreview}
							/>
						</div>
						<p className="text-xs text-muted-foreground">
							<Trans>例如：DELETE FROM product_info WHERE category_name = 'obsolete' AND is_deleted = false</Trans>
						</p>
					</div>
					{!showPreview && (
						<Button onClick={() => void handlePreview()} disabled={previewing}>
							{previewing ? t`运行预览中...` : <Trans>运行预览</Trans>}
						</Button>
					)}
					{showPreview && (
						<>
							<div className="text-sm text-muted-foreground">
								<Trans>匹配到 {previewItems.length} 条记录</Trans>
							</div>
							{executedSQL && (
								<div className="bg-muted p-3 rounded-md text-xs font-mono overflow-auto whitespace-pre-wrap border">
									<div className="text-muted-foreground mb-1">
										<Trans>实际执行的预览语句：</Trans>
									</div>
									{executedSQL}
								</div>
							)}
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
												<TableRow key={item.code}>
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
							{previewItems.length > 0 && (
								<div className="flex items-center gap-2">
									<Checkbox
										id="confirm-delete"
										checked={confirmed}
										onCheckedChange={(checked) => setConfirmed(checked === true)}
									/>
									<Label htmlFor="confirm-delete" className="text-sm cursor-pointer">
										<Trans>我已确认上述记录即将被删除</Trans>
									</Label>
								</div>
							)}
						</>
					)}
				</div>
				<DialogFooter>
					<Button variant="outline" onClick={handleClose}>
						<Trans>取消</Trans>
					</Button>
					{showPreview && previewItems.length > 0 && confirmed && (
						<Button variant="destructive" onClick={() => void handleDeleteClick()} disabled={deleting}>
							{deleting ? t`删除中...` : <Trans>确认删除</Trans>}
						</Button>
					)}
				</DialogFooter>
			</DialogContent>
			<AdminVerifyDialog
				open={adminVerifyOpen}
				onOpenChange={setAdminVerifyOpen}
				onVerified={(password) => void handleVerifiedDelete(password)}
			/>
		</Dialog>
	)
})

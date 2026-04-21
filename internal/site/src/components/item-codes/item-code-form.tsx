// Item Code form dialog for create and edit.
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useCallback, useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { toast } from "@/components/ui/use-toast"
import { updateItemCodeInDB } from "@/lib/itemCodeApi"
import type { ItemCodeDBRecord } from "@/types"

interface ItemCodeFormProps {
	open: boolean
	onOpenChange: (open: boolean) => void
	record?: ItemCodeDBRecord
	onSuccess?: () => void
}

export default memo(function ItemCodeForm({ open, onOpenChange, record, onSuccess }: ItemCodeFormProps) {
	const [code, setCode] = useState("")
	const [name, setName] = useState("")
	const [category, setCategory] = useState("")
	const [status, setStatus] = useState<"active" | "inactive" | "obsolete">("active")
	const [description, setDescription] = useState("")
	const [submitting, setSubmitting] = useState(false)

	const isEdit = !!record

	useEffect(() => {
		if (open) {
			if (record) {
				setCode(record.code)
				setName(record.name)
				setCategory(record.category)
				setStatus(record.status)
				setDescription(record.description)
			} else {
				setCode("")
				setName("")
				setCategory("")
				setStatus("active")
				setDescription("")
			}
		}
	}, [open, record])

	const handleSubmit = useCallback(async () => {
		if (!code.trim() || !name.trim()) {
			toast({ variant: "destructive", title: t`错误`, description: t`编码和名称为必填项` })
			return
		}
		setSubmitting(true)
		try {
			const data = {
				code: code.trim(),
				name: name.trim(),
				category: category.trim(),
				status,
				description: description.trim(),
			}
			if (isEdit && record) {
				await updateItemCodeInDB({
					code: record.code,
					name: data.name,
					category: data.category,
					description: data.description,
				})
				toast({ title: t`已更新`, description: t`Item Code 更新成功` })
			}
			onOpenChange(false)
			onSuccess?.()
		} catch (err: any) {
			toast({ variant: "destructive", title: t`错误`, description: err?.message || t`保存失败` })
		} finally {
			setSubmitting(false)
		}
	}, [code, name, category, status, description, isEdit, record, onOpenChange, onSuccess])

	return (
		<Dialog open={open} onOpenChange={onOpenChange}>
			<DialogContent className="sm:max-w-lg">
				<DialogHeader>
					<DialogTitle><Trans>编辑 Item Code</Trans></DialogTitle>
					<DialogDescription>
						<Trans>修改 Item Code 的基本信息。</Trans>
					</DialogDescription>
				</DialogHeader>
				<div className="grid gap-4 py-2">
					<div className="grid gap-2">
						<Label htmlFor="item-code">
							<Trans>编码</Trans>
						</Label>
						<Input
							id="item-code"
							value={code}
							onChange={(e) => setCode(e.target.value)}
							placeholder={t`输入编码`}
							disabled={isEdit}
						/>
					</div>
					<div className="grid gap-2">
						<Label htmlFor="item-name">
							<Trans>名称</Trans>
						</Label>
						<Input
							id="item-name"
							value={name}
							onChange={(e) => setName(e.target.value)}
							placeholder={t`输入名称`}
						/>
					</div>
					<div className="grid grid-cols-2 gap-4">
						<div className="grid gap-2">
							<Label htmlFor="item-category">
								<Trans>分类</Trans>
							</Label>
							<Input
								id="item-category"
								value={category}
								onChange={(e) => setCategory(e.target.value)}
								placeholder={t`输入分类`}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="item-status">
								<Trans>状态</Trans>
							</Label>
							<Select value={status} onValueChange={(v) => setStatus(v as "active" | "inactive" | "obsolete")}>
								<SelectTrigger id="item-status">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									<SelectItem value="active">启用</SelectItem>
									<SelectItem value="inactive">停用</SelectItem>
									<SelectItem value="obsolete">废弃</SelectItem>
								</SelectContent>
							</Select>
						</div>
					</div>
					<div className="grid gap-2">
						<Label htmlFor="item-description">
							<Trans>描述</Trans>
						</Label>
						<Textarea
							id="item-description"
							value={description}
							onChange={(e) => setDescription(e.target.value)}
							placeholder={t`输入描述`}
							rows={3}
						/>
					</div>
				</div>
				<DialogFooter>
					<Button variant="outline" onClick={() => onOpenChange(false)}>
						<Trans>取消</Trans>
					</Button>
					<Button onClick={() => void handleSubmit()} disabled={submitting}>
						{submitting ? t`保存中...` : <Trans>保存</Trans>}
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	)
})

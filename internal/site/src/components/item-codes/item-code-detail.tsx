import { t } from "@lingui/core/macro"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import type { ItemCodeDBDetail } from "@/types"
import { FileIcon, FileImageIcon, BoxIcon, RulerIcon, SettingsIcon, ActivityIcon } from "lucide-react"

interface ItemCodeDetailProps {
	detail: ItemCodeDBDetail
}

function StatusBadge({ status }: { status: string }) {
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

function InfoRow({ label, value }: { label: string; value?: string | number | boolean | null }) {
	if (value === undefined || value === null || value === "") return null
	return (
		<div className="flex justify-between py-1 text-sm">
			<span className="text-muted-foreground">{label}</span>
			<span className="font-medium text-right max-w-[60%] break-words">{String(value)}</span>
		</div>
	)
}

function SectionCard({
	title,
	icon: Icon,
	children,
}: {
	title: string
	icon: React.ComponentType<{ className?: string }>
	children: React.ReactNode
}) {
	return (
		<Card className="overflow-hidden">
			<CardHeader className="bg-muted/30 py-3">
				<CardTitle className="text-sm font-semibold flex items-center gap-2">
					<Icon className="h-4 w-4 text-muted-foreground" />
					{title}
				</CardTitle>
			</CardHeader>
			<CardContent className="p-4 space-y-0">{children}</CardContent>
		</Card>
	)
}

export default function ItemCodeDetail({ detail }: ItemCodeDetailProps) {
	return (
		<div className="p-4 bg-muted/20 border-t">
			<div className="grid grid-cols-1 @lg:grid-cols-2 @xl:grid-cols-3 gap-4">
				{/* 基础信息 */}
				<SectionCard title={t`基础信息`} icon={BoxIcon}>
					<InfoRow label={t`编码`} value={detail.code} />
					<InfoRow label={t`名称`} value={detail.name} />
					<InfoRow label={t`分类`} value={detail.category} />
					<div className="flex justify-between py-1 text-sm">
						<span className="text-muted-foreground">{t`状态`}</span>
						<StatusBadge status={detail.status} />
					</div>
					<InfoRow label={t`描述`} value={detail.description} />
					<InfoRow label={t`更新时间`} value={detail.updated} />
					<InfoRow label={t`创建时间`} value={detail.createTime} />
				</SectionCard>

				{/* 文件信息 */}
				<SectionCard title={t`文件信息`} icon={FileIcon}>
					<div className="flex justify-between py-1 text-sm">
						<span className="text-muted-foreground">{t`3D 模型`}</span>
						<Badge variant={detail.has3dModel ? "default" : "outline"}>
							{detail.has3dModel ? t`有` : t`无`}
						</Badge>
					</div>
					<div className="flex justify-between py-1 text-sm">
						<span className="text-muted-foreground">{t`2D 图纸`}</span>
						<Badge variant={detail.has2dImage ? "default" : "outline"}>
							{detail.has2dImage ? t`有` : t`无`}
						</Badge>
					</div>
					<InfoRow label={t`文件路径`} value={detail.filePath} />
					<InfoRow label={t`GLB 地址`} value={detail.glbAddress} />
					<InfoRow label={t`源文件路径`} value={detail.sourceFilePath} />
					<InfoRow label={t`转换后路径`} value={detail.convertedFilePath} />
					<InfoRow label={t`模型 MD5`} value={detail.modelMd5} />
				</SectionCard>

				{/* 物理属性 */}
				<SectionCard title={t`物理属性`} icon={RulerIcon}>
					<InfoRow label={t`材料类型`} value={detail.materialType} />
					<InfoRow label={t`零件号`} value={detail.partNumber} />
					<div className="grid grid-cols-3 gap-2 py-1">
						<div className="text-center">
							<div className="text-xs text-muted-foreground">{t`X 长度`}</div>
							<div className="font-medium">{detail.xLength || "-"}</div>
						</div>
						<div className="text-center">
							<div className="text-xs text-muted-foreground">{t`Y 长度`}</div>
							<div className="font-medium">{detail.yLength || "-"}</div>
						</div>
						<div className="text-center">
							<div className="text-xs text-muted-foreground">{t`Z 长度`}</div>
							<div className="font-medium">{detail.zLength || "-"}</div>
						</div>
					</div>
				</SectionCard>

				{/* PLM 信息 */}
				<SectionCard title={t`PLM 信息`} icon={SettingsIcon}>
					<InfoRow label={t`CAD 编号`} value={detail.cadNumber} />
					<InfoRow label={t`图纸 URL`} value={detail.drawingUrl} />
					<InfoRow label={t`设计状态`} value={detail.designState} />
					<InfoRow label={t`生命周期`} value={detail.lifeCycle} />
					<InfoRow label={t`管径`} value={detail.pipeDiameter} />
					<Separator className="my-2" />
					<div className="text-xs text-muted-foreground mb-1">{t`预估包装尺寸`}</div>
					<div className="grid grid-cols-3 gap-2">
						<InfoRow label={t`长`} value={detail.packLength} />
						<InfoRow label={t`宽`} value={detail.packWidth} />
						<InfoRow label={t`高`} value={detail.packHeight} />
					</div>
					<div className="text-xs text-muted-foreground mb-1 mt-2">{t`物料尺寸`}</div>
					<div className="grid grid-cols-3 gap-2">
						<InfoRow label={t`长`} value={detail.itemLength} />
						<InfoRow label={t`宽`} value={detail.itemWidth} />
						<InfoRow label={t`高`} value={detail.itemHeight} />
					</div>
				</SectionCard>

				{/* 处理状态 */}
				<SectionCard title={t`处理状态`} icon={ActivityIcon}>
					<div className="flex justify-between py-1 text-sm">
						<span className="text-muted-foreground">{t`下载状态`}</span>
						<ProcessStatusBadge status={detail.downloadStatus} />
					</div>
					<div className="flex justify-between py-1 text-sm">
						<span className="text-muted-foreground">{t`上传状态`}</span>
						<ProcessStatusBadge status={detail.uploadStatus} />
					</div>
					<div className="flex justify-between py-1 text-sm">
						<span className="text-muted-foreground">{t`处理状态`}</span>
						<ProcessStatusBadge status={detail.processStatus} />
					</div>
				</SectionCard>
			</div>
		</div>
	)
}

function ProcessStatusBadge({ status }: { status?: string }) {
	if (!status) return <Badge variant="outline">-</Badge>
	const s = status.toLowerCase()
	if (s === "success") return <Badge className="bg-emerald-500/15 text-emerald-600">{t`成功`}</Badge>
	if (s === "failed" || s === "error") return <Badge variant="destructive">{t`失败`}</Badge>
	if (s === "processing" || s === "pending") return <Badge variant="secondary">{t`进行中`}</Badge>
	return <Badge variant="outline">{status}</Badge>
}

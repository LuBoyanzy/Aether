/**
 * Docker 模块空态提示组件。
 * 用于提示用户选择系统或暂无数据。
 */
import { Trans } from "@lingui/react/macro"
import type { ReactNode } from "react"
import { Card, CardContent } from "@/components/ui/card"

export default function DockerEmptyState({ message }: { message?: ReactNode }) {
	return (
		<Card className="border-dashed">
			<CardContent className="py-8 text-center text-sm text-muted-foreground">
				{message ?? <Trans>Please select a system to continue.</Trans>}
			</CardContent>
		</Card>
	)
}

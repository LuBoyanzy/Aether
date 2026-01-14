// audit-logs.tsx 渲染审查日志页面入口。
// 负责挂载审查日志列表与全局告警视图。
import { memo, useEffect, useMemo } from "react"
import { ActiveAlerts } from "@/components/active-alerts"
import AuditLogsTable from "@/components/audit/audit-logs-table"
import { FooterRepoLink } from "@/components/footer-repo-link"
import { BRAND_NAME } from "@/lib/utils"

export default memo(() => {
	useEffect(() => {
		document.title = BRAND_NAME
	}, [])

	return useMemo(
		() => (
			<>
				<div className="grid gap-4">
					<ActiveAlerts />
					<AuditLogsTable />
				</div>
				<FooterRepoLink />
			</>
		),
		[]
	)
})

// containers.tsx 渲染 Docker 模块入口页面。
// 负责挂载 Docker Tabs 与全局告警视图。
import { useLingui } from "@lingui/react/macro"
import { memo, useEffect, useMemo } from "react"
import { ActiveAlerts } from "@/components/active-alerts"
import DockerTabs from "@/components/docker/docker-tabs"
import { FooterRepoLink } from "@/components/footer-repo-link"
import { BRAND_NAME } from "@/lib/utils"

export default memo(() => {
	const { t } = useLingui()

	useEffect(() => {
		document.title = `${t`Docker`} / ${BRAND_NAME}`
	}, [t])

	return useMemo(
		() => (
			<>
				<div className="grid gap-4">
					<ActiveAlerts />
					<DockerTabs />
				</div>
				<FooterRepoLink />
			</>
		),
		[]
	)
})

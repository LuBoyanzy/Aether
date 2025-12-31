import { useLingui } from "@lingui/react/macro"
import { memo, useEffect, useMemo } from "react"
import ContainersTable from "@/components/containers-table/containers-table"
import { ActiveAlerts } from "@/components/active-alerts"
import { FooterRepoLink } from "@/components/footer-repo-link"
import { BRAND_NAME } from "@/lib/utils"

export default memo(() => {
	const { t } = useLingui()

	useEffect(() => {
		// document.title = `${t`Containers`} / ${BRAND_NAME}`
	}, [t])

	return useMemo(
		() => (
			<>
				<div className="grid gap-4">
					<ActiveAlerts />
					<ContainersTable />
				</div>
				<FooterRepoLink />
			</>
		),
		[]
	)
})

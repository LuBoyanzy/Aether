import { useEffect } from "react"
import SmartTable from "@/components/routes/system/smart-table"
import { ActiveAlerts } from "@/components/active-alerts"
import { FooterRepoLink } from "@/components/footer-repo-link"
import { BRAND_NAME } from "@/lib/utils"

export default function Smart() {
	useEffect(() => {
		document.title = `S.M.A.R.T. / ${BRAND_NAME}`
	}, [])

	return (
		<>
			<div className="grid gap-4">
				<ActiveAlerts />
				<SmartTable />
			</div>
			<FooterRepoLink />
		</>
	)
}

// item-codes.tsx renders the Item Code management page.
import { Trans } from "@lingui/react/macro"
import { t } from "@lingui/core/macro"
import { memo, useCallback, useEffect, useMemo, useState } from "react"
import { ActiveAlerts } from "@/components/active-alerts"
import AuditLogsDialog from "@/components/item-codes/audit-logs-dialog"
import ItemCodeForm from "@/components/item-codes/item-code-form"
import ItemCodeTable from "@/components/item-codes/item-code-table"
import QueryDeleteDialog from "@/components/item-codes/query-delete-dialog"
import { FooterRepoLink } from "@/components/footer-repo-link"
import { BRAND_NAME } from "@/lib/utils"
import type { ItemCodeRecord } from "@/types"

export default memo(function ItemCodesPage() {
	const [formOpen, setFormOpen] = useState(false)
	const [editRecord, setEditRecord] = useState<ItemCodeRecord | undefined>()
	const [queryDeleteOpen, setQueryDeleteOpen] = useState(false)
	const [auditLogsOpen, setAuditLogsOpen] = useState(false)
	const [refreshKey, setRefreshKey] = useState(0)

	useEffect(() => {
		document.title = `${t`Item Code 管理`} - ${BRAND_NAME}`
	}, [])

	const handleEdit = useCallback((record: ItemCodeRecord) => {
		setEditRecord(record)
		setFormOpen(true)
	}, [])

	const handleSuccess = useCallback(() => {
		setRefreshKey((prev) => prev + 1)
	}, [])

	return useMemo(
		() => (
			<>
				<div className="grid gap-4">
					<ActiveAlerts />
					<ItemCodeTable
						key={refreshKey}
						onEdit={handleEdit}
						onQueryDelete={() => setQueryDeleteOpen(true)}
						onAuditLogs={() => setAuditLogsOpen(true)}
					/>
				</div>
				<FooterRepoLink />
				<ItemCodeForm
					open={formOpen}
					onOpenChange={setFormOpen}
					record={editRecord}
					onSuccess={handleSuccess}
				/>
				<QueryDeleteDialog
					open={queryDeleteOpen}
					onOpenChange={setQueryDeleteOpen}
					onSuccess={handleSuccess}
				/>
				<AuditLogsDialog open={auditLogsOpen} onOpenChange={setAuditLogsOpen} />
			</>
		),
		[refreshKey, formOpen, editRecord, queryDeleteOpen, auditLogsOpen, handleEdit, handleSuccess]
	)
})

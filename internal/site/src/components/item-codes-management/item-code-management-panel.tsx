// item-code-management-panel.tsx renders the Item Code management panel.
import { memo, useCallback, useMemo, useState } from "react"
import AuditLogsDialog from "@/components/item-codes/audit-logs-dialog"
import ItemCodeForm from "@/components/item-codes/item-code-form"
import ItemCodeTable from "@/components/item-codes/item-code-table"
import QueryDeleteDialog from "@/components/item-codes/query-delete-dialog"
import type { ItemCodeDBRecord } from "@/types"

export default memo(function ItemCodeManagementPanel() {
	const [formOpen, setFormOpen] = useState(false)
	const [editRecord, setEditRecord] = useState<ItemCodeDBRecord | undefined>()
	const [queryDeleteOpen, setQueryDeleteOpen] = useState(false)
	const [auditLogsOpen, setAuditLogsOpen] = useState(false)
	const [refreshKey, setRefreshKey] = useState(0)

	const handleEdit = useCallback((record: ItemCodeDBRecord) => {
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
					<ItemCodeTable
						key={refreshKey}
						onEdit={handleEdit}
						onQueryDelete={() => setQueryDeleteOpen(true)}
						onAuditLogs={() => setAuditLogsOpen(true)}
					/>
				</div>
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

/**
 * Docker 关注告警规则抽屉。
 * 提供系统级开关与基础策略配置。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useEffect, useState } from "react"
import { Button } from "@/components/ui/button"
import { DialogDescription, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Sheet, SheetContent } from "@/components/ui/sheet"
import { Switch } from "@/components/ui/switch"
import { toast } from "@/components/ui/use-toast"
import { isAdmin } from "@/lib/api"
import {
	createDockerFocusAlertSettings,
	getDockerFocusAlertSettings,
	updateDockerFocusAlertSettings,
} from "@/lib/docker-focus-alert-settings"
import type { DockerFocusAlertSettingsRecord } from "@/types"
import { LoaderCircleIcon } from "lucide-react"

const defaultRecoverySeconds = 15

type FocusAlertSettingsSheetProps = {
	open: boolean
	onOpenChange: (open: boolean) => void
	systemId?: string
}

export default memo(function FocusAlertSettingsSheet({ open, onOpenChange, systemId }: FocusAlertSettingsSheetProps) {
	const [settingsId, setSettingsId] = useState<string | null>(null)
	const [enabled, setEnabled] = useState(true)
	const [recoverySeconds, setRecoverySeconds] = useState(String(defaultRecoverySeconds))
	const [alertOnNoMatch, setAlertOnNoMatch] = useState(true)
	const [loading, setLoading] = useState(false)
	const [saving, setSaving] = useState(false)

	const canEdit = isAdmin()
	const isBusy = loading || saving

	useEffect(() => {
		if (!open || !systemId) {
			return
		}
		const fetchSettings = async () => {
			setLoading(true)
			try {
				const record = await getDockerFocusAlertSettings(systemId)
				const next = record ?? {
					enabled: true,
					recovery_seconds: defaultRecoverySeconds,
					alert_on_no_match: true,
				}
				setSettingsId(record?.id ?? null)
				setEnabled(next.enabled)
				setRecoverySeconds(String(next.recovery_seconds))
				setAlertOnNoMatch(next.alert_on_no_match)
			} catch (err) {
				console.error("load docker focus alert settings failed", err)
				toast({
					variant: "destructive",
					title: t`Error`,
					description: t`Failed to load alert settings`,
				})
				throw err
			} finally {
				setLoading(false)
			}
		}
		void fetchSettings()
	}, [open, systemId])

	const handleSave = async () => {
		if (!systemId || !canEdit || isBusy) {
			return
		}
		const parsedSeconds = Number.parseInt(recoverySeconds, 10)
		if (!Number.isFinite(parsedSeconds) || parsedSeconds <= 0) {
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Recovery seconds must be greater than 0`,
			})
			return
		}
		setSaving(true)
		try {
			const payload = {
				enabled,
				recovery_seconds: parsedSeconds,
				alert_on_no_match: alertOnNoMatch,
			}
			let record: DockerFocusAlertSettingsRecord
			if (settingsId) {
				record = await updateDockerFocusAlertSettings(settingsId, payload)
			} else {
				record = await createDockerFocusAlertSettings({ system: systemId, ...payload })
			}
			setSettingsId(record.id)
			toast({ title: t`Settings saved` })
		} catch (err) {
			console.error("save docker focus alert settings failed", err)
			toast({
				variant: "destructive",
				title: t`Error`,
				description: t`Failed to save settings`,
			})
			throw err
		} finally {
			setSaving(false)
		}
	}

	return (
		<Sheet open={open} onOpenChange={onOpenChange}>
			<SheetContent
				className="max-h-full overflow-auto w-150 !max-w-full p-4 sm:p-6"
				onOpenAutoFocus={(event) => {
					event.preventDefault()
				}}
			>
				<DialogHeader>
					<DialogTitle className="text-xl">
						<Trans>Alert rules</Trans>
					</DialogTitle>
					<DialogDescription>
						<Trans>Configure Docker focus alert rules for this system.</Trans>
					</DialogDescription>
				</DialogHeader>
				{!canEdit && (
					<p className="text-sm text-muted-foreground">
						<Trans>Only administrators can edit these settings.</Trans>
					</p>
				)}
				{loading ? (
					<div className="flex items-center gap-2 text-sm text-muted-foreground">
						<LoaderCircleIcon className="h-4 w-4 animate-spin" />
						<Trans>Loading...</Trans>
					</div>
				) : (
					<div className="mt-4 space-y-5">
						<div className="space-y-4 rounded-md border p-4">
							<div className="flex items-center justify-between gap-4">
								<div>
									<Label htmlFor="docker-focus-alert-enabled">
										<Trans>Enable alerts</Trans>
									</Label>
									<p className="text-xs text-muted-foreground">
										<Trans>Toggle Docker focus alerts for this system.</Trans>
									</p>
								</div>
								<Switch
									id="docker-focus-alert-enabled"
									checked={enabled}
									onCheckedChange={setEnabled}
									disabled={!canEdit}
								/>
							</div>
							<div className="grid gap-2">
								<Label htmlFor="docker-focus-alert-recovery">
									<Trans>Recovery stability (seconds)</Trans>
								</Label>
								<Input
									id="docker-focus-alert-recovery"
									type="number"
									min={1}
									value={recoverySeconds}
									onChange={(event) => setRecoverySeconds(event.target.value)}
									disabled={!canEdit}
								/>
								<p className="text-xs text-muted-foreground">
									<Trans>Send recovery alerts after this many seconds of stability.</Trans>
								</p>
							</div>
							<div className="flex items-center justify-between gap-4">
								<div>
									<Label htmlFor="docker-focus-alert-no-match">
										<Trans>Alert when no containers match</Trans>
									</Label>
									<p className="text-xs text-muted-foreground">
										<Trans>Trigger alerts when a focus rule matches zero containers.</Trans>
									</p>
								</div>
								<Switch
									id="docker-focus-alert-no-match"
									checked={alertOnNoMatch}
									onCheckedChange={setAlertOnNoMatch}
									disabled={!canEdit}
								/>
							</div>
						</div>
						<div className="flex justify-end gap-2">
							<Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
								<Trans>Cancel</Trans>
							</Button>
							<Button type="button" onClick={handleSave} disabled={!canEdit || isBusy}>
								{saving ? <Trans>Loading...</Trans> : <Trans>Save Settings</Trans>}
							</Button>
						</div>
					</div>
				)}
			</SheetContent>
		</Sheet>
	)
})

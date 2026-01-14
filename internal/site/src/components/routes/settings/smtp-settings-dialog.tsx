// SMTP 设置弹窗组件，用于在通知页配置并测试邮件服务器。
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { LoaderCircleIcon, SaveIcon, SendHorizonalIcon } from "lucide-react"
import { type ReactNode, useEffect, useMemo, useState } from "react"
import * as v from "valibot"
import { Button } from "@/components/ui/button"
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogFooter,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { toast } from "@/components/ui/use-toast"
import { isAdmin, pb } from "@/lib/api"

type MailSettingsResponse = {
	meta: {
		senderName: string
		senderAddress: string
	}
	smtp: {
		enabled: boolean
		host: string
		port: number
		username: string
		authMethod: string
		tls: boolean
		localName: string
		passwordSet: boolean
	}
}

type MailSettingsPayload = {
	meta: {
		senderName: string
		senderAddress: string
	}
	smtp: {
		enabled: boolean
		host: string
		port: number
		username: string
		password: string
		authMethod: string
		tls: boolean
		localName: string
	}
}

const EmailSchema = v.pipe(v.string(), v.email())

const authMethodOptions = [
	{ value: "PLAIN", label: "PLAIN" },
	{ value: "LOGIN", label: "LOGIN" },
]

export const SmtpSettingsDialog = ({ children }: { children: ReactNode }) => {
	const [open, setOpen] = useState(false)
	const [isLoading, setIsLoading] = useState(false)
	const [isSaving, setIsSaving] = useState(false)
	const [isTesting, setIsTesting] = useState(false)
	const [passwordSet, setPasswordSet] = useState(false)
	const [senderName, setSenderName] = useState("")
	const [senderAddress, setSenderAddress] = useState("")
	const [smtpEnabled, setSmtpEnabled] = useState(false)
	const [smtpHost, setSmtpHost] = useState("")
	const [smtpPort, setSmtpPort] = useState("587")
	const [smtpUsername, setSmtpUsername] = useState("")
	const [smtpPassword, setSmtpPassword] = useState("")
	const [smtpAuthMethod, setSmtpAuthMethod] = useState("PLAIN")
	const [smtpTLS, setSmtpTLS] = useState(false)
	const [smtpLocalName, setSmtpLocalName] = useState("")
	const [testEmail, setTestEmail] = useState("")

	const canEdit = isAdmin()
	const isBusy = isLoading || isSaving || isTesting

	const authMethodValue = useMemo(() => smtpAuthMethod || "PLAIN", [smtpAuthMethod])

	useEffect(() => {
		if (!open) {
			setSmtpPassword("")
			return
		}

		const fetchSettings = async () => {
			setIsLoading(true)
			try {
				const data = await pb.send<MailSettingsResponse>("/api/aether/mail-settings", {})
				setSenderName(data.meta.senderName)
				setSenderAddress(data.meta.senderAddress)
				setSmtpEnabled(data.smtp.enabled)
				setSmtpHost(data.smtp.host)
				setSmtpPort(String(data.smtp.port))
				setSmtpUsername(data.smtp.username)
				setSmtpAuthMethod(data.smtp.authMethod || "PLAIN")
				setSmtpTLS(data.smtp.tls)
				setSmtpLocalName(data.smtp.localName)
				setPasswordSet(data.smtp.passwordSet)
				setSmtpPassword("")
			} catch (error: any) {
				toast({
					title: t`Error`,
					description: error.message,
					variant: "destructive",
				})
			} finally {
				setIsLoading(false)
			}
		}

		void fetchSettings()
	}, [open])

	const handleSave = async () => {
		if (!canEdit || isBusy) {
			return
		}

		if (!senderName.trim()) {
			toast({
				title: t`Missing sender name`,
				description: t`Please enter a sender name.`,
				variant: "destructive",
			})
			return
		}
		if (!senderAddress.trim()) {
			toast({
				title: t`Missing sender address`,
				description: t`Please enter a sender email address.`,
				variant: "destructive",
			})
			return
		}
		try {
			v.parse(EmailSchema, senderAddress)
		} catch (error: any) {
			toast({
				title: t`Invalid sender address`,
				description: error.message,
				variant: "destructive",
			})
			return
		}

		if (smtpEnabled && !smtpHost.trim()) {
			toast({
				title: t`Missing SMTP host`,
				description: t`Please enter an SMTP host.`,
				variant: "destructive",
			})
			return
		}

		const portValue = Number.parseInt(smtpPort, 10)
		if (!Number.isFinite(portValue) || portValue <= 0) {
			toast({
				title: t`Invalid SMTP port`,
				description: t`Please enter a valid port number.`,
				variant: "destructive",
			})
			return
		}

		setIsSaving(true)
		try {
			const payload: MailSettingsPayload = {
				meta: {
					senderName,
					senderAddress,
				},
				smtp: {
					enabled: smtpEnabled,
					host: smtpHost,
					port: portValue,
					username: smtpUsername,
					password: smtpPassword,
					authMethod: authMethodValue,
					tls: smtpTLS,
					localName: smtpLocalName,
				},
			}
			const data = await pb.send<MailSettingsResponse>("/api/aether/mail-settings", {
				method: "PUT",
				body: payload,
			})
			setPasswordSet(data.smtp.passwordSet)
			setSmtpPassword("")
			toast({
				title: t`SMTP settings saved`,
				description: t`Your changes have been saved.`,
			})
		} catch (error: any) {
			toast({
				title: t`Failed to save SMTP settings`,
				description: error.message,
				variant: "destructive",
			})
		} finally {
			setIsSaving(false)
		}
	}

	const handleTestEmail = async () => {
		if (!canEdit || isBusy) {
			return
		}
		if (!testEmail.trim()) {
			toast({
				title: t`Missing test email`,
				description: t`Please enter a test email address.`,
				variant: "destructive",
			})
			return
		}
		try {
			v.parse(EmailSchema, testEmail)
		} catch (error: any) {
			toast({
				title: t`Invalid test email`,
				description: error.message,
				variant: "destructive",
			})
			return
		}

		setIsTesting(true)
		try {
			await pb.send("/api/aether/mail-settings/test", {
				method: "POST",
				body: { email: testEmail },
			})
			toast({
				title: t`Test email sent`,
				description: t`Check the inbox for the test message.`,
			})
		} catch (error: any) {
			toast({
				title: t`Failed to send test email`,
				description: error.message,
				variant: "destructive",
			})
		} finally {
			setIsTesting(false)
		}
	}

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger asChild>
				<Button type="button" variant="link" className="h-auto p-0 text-sm">
					{children}
				</Button>
			</DialogTrigger>
			<DialogContent className="max-w-2xl max-h-[85vh] overflow-y-auto">
				<DialogHeader>
					<DialogTitle>
						<Trans>SMTP settings</Trans>
					</DialogTitle>
					<DialogDescription>
						<Trans>Configure the sender and SMTP server details for alert emails.</Trans>
					</DialogDescription>
				</DialogHeader>
				{!canEdit && (
					<p className="text-sm text-muted-foreground">
						<Trans>Only administrators can edit these settings.</Trans>
					</p>
				)}
				{isLoading ? (
					<div className="flex items-center gap-2 text-sm text-muted-foreground">
						<LoaderCircleIcon className="h-4 w-4 animate-spin" />
						<Trans>Loading SMTP settings...</Trans>
					</div>
				) : (
					<div className="space-y-5">
						<div className="grid gap-2">
							<Label htmlFor="smtp-sender-name">
								<Trans>Sender name</Trans>
							</Label>
							<Input
								id="smtp-sender-name"
								value={senderName}
								onChange={(event) => setSenderName(event.target.value)}
								disabled={!canEdit}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="smtp-sender-address">
								<Trans>Sender address</Trans>
							</Label>
							<Input
								id="smtp-sender-address"
								type="email"
								value={senderAddress}
								onChange={(event) => setSenderAddress(event.target.value)}
								disabled={!canEdit}
							/>
						</div>
						<div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2">
							<Label htmlFor="smtp-enabled">
								<Trans>SMTP enabled</Trans>
							</Label>
							<Switch id="smtp-enabled" checked={smtpEnabled} onCheckedChange={setSmtpEnabled} disabled={!canEdit} />
						</div>
						<div className="grid gap-2">
							<Label htmlFor="smtp-host">
								<Trans>SMTP host</Trans>
							</Label>
							<Input
								id="smtp-host"
								value={smtpHost}
								onChange={(event) => setSmtpHost(event.target.value)}
								disabled={!canEdit}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="smtp-port">
								<Trans>SMTP port</Trans>
							</Label>
							<Input
								id="smtp-port"
								type="number"
								value={smtpPort}
								onChange={(event) => setSmtpPort(event.target.value)}
								disabled={!canEdit}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="smtp-username">
								<Trans>SMTP username</Trans>
							</Label>
							<Input
								id="smtp-username"
								value={smtpUsername}
								onChange={(event) => setSmtpUsername(event.target.value)}
								disabled={!canEdit}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="smtp-password">
								<Trans>SMTP password</Trans>
							</Label>
							<Input
								id="smtp-password"
								type="password"
								value={smtpPassword}
								onChange={(event) => setSmtpPassword(event.target.value)}
								disabled={!canEdit}
								placeholder={t`Leave blank to keep existing password`}
							/>
							<p className="text-xs text-muted-foreground">
								{passwordSet ? <Trans>Password stored</Trans> : <Trans>No password stored</Trans>}
							</p>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="smtp-auth-method">
								<Trans>SMTP auth method</Trans>
							</Label>
							<Select value={authMethodValue} onValueChange={setSmtpAuthMethod} disabled={!canEdit}>
								<SelectTrigger id="smtp-auth-method">
									<SelectValue />
								</SelectTrigger>
								<SelectContent>
									{authMethodOptions.map((option) => (
										<SelectItem key={option.value} value={option.value}>
											{option.label}
										</SelectItem>
									))}
								</SelectContent>
							</Select>
						</div>
						<div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2">
							<Label htmlFor="smtp-tls">
								<Trans>Enforce TLS</Trans>
							</Label>
							<Switch id="smtp-tls" checked={smtpTLS} onCheckedChange={setSmtpTLS} disabled={!canEdit} />
						</div>
						<div className="grid gap-2">
							<Label htmlFor="smtp-local-name">
								<Trans>Local name (EHLO/HELO)</Trans>
							</Label>
							<Input
								id="smtp-local-name"
								value={smtpLocalName}
								onChange={(event) => setSmtpLocalName(event.target.value)}
								disabled={!canEdit}
							/>
						</div>
						<div className="grid gap-2">
							<Label htmlFor="smtp-test-email">
								<Trans>Test email address</Trans>
							</Label>
							<Input
								id="smtp-test-email"
								type="email"
								value={testEmail}
								onChange={(event) => setTestEmail(event.target.value)}
								disabled={!canEdit}
							/>
						</div>
					</div>
				)}
				<DialogFooter>
					<Button type="button" variant="outline" onClick={() => setOpen(false)}>
						<Trans>Cancel</Trans>
					</Button>
					<Button type="button" variant="outline" onClick={handleTestEmail} disabled={!canEdit || isBusy}>
						{isTesting ? <LoaderCircleIcon className="h-4 w-4 animate-spin" /> : <SendHorizonalIcon className="h-4 w-4" />}
						<span className="ms-1">
							<Trans>Send test email</Trans>
						</span>
					</Button>
					<Button type="button" onClick={handleSave} disabled={!canEdit || isBusy}>
						{isSaving ? <LoaderCircleIcon className="h-4 w-4 animate-spin" /> : <SaveIcon className="h-4 w-4" />}
						<span className="ms-1">
							<Trans>Save</Trans>
						</span>
					</Button>
				</DialogFooter>
			</DialogContent>
		</Dialog>
	)
}

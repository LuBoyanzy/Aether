// command-palette.tsx 提供全局命令面板快捷导航。
// 用于快速访问系统列表与设置入口。
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { getPagePath } from "@nanostores/router"
import { DialogDescription } from "@radix-ui/react-dialog"
import {
	AlertOctagonIcon,
	BookIcon,
	ClipboardListIcon,
	ContainerIcon,
	DatabaseBackupIcon,
	FingerprintIcon,
	HardDriveIcon,
	LogsIcon,
	MailIcon,
	Server,
	ServerIcon,
	SettingsIcon,
	TestTube2 as TestTube2Icon,
	UsersIcon,
} from "lucide-react"
import { memo, useEffect, useMemo } from "react"
import {
	CommandDialog,
	CommandEmpty,
	CommandGroup,
	CommandInput,
	CommandItem,
	CommandList,
	CommandSeparator,
	CommandShortcut,
} from "@/components/ui/command"
import { isAdmin } from "@/lib/api"
import { $systems } from "@/lib/stores"
import { getHostDisplayValue, listen, showDocsUnavailable } from "@/lib/utils"
import { $router, basePath, navigate, prependBasePath } from "./router"

export default memo(function CommandPalette({ open, setOpen }: { open: boolean; setOpen: (open: boolean) => void }) {
	useEffect(() => {
		const down = (e: KeyboardEvent) => {
			if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
				e.preventDefault()
				setOpen(!open)
			}
		}
		return listen(document, "keydown", down)
	}, [open, setOpen])

	return useMemo(() => {
		const systems = $systems.get()
		const SettingsShortcut = (
			<CommandShortcut>
				<Trans>Settings</Trans>
			</CommandShortcut>
		)
		const AdminShortcut = (
			<CommandShortcut>
				<Trans>Admin</Trans>
			</CommandShortcut>
		)
		return (
			<CommandDialog open={open} onOpenChange={setOpen}>
				<DialogDescription className="sr-only">Command palette</DialogDescription>
				<CommandInput placeholder={t`Search for systems or settings...`} />
				<CommandList>
					{systems.length > 0 && (
						<>
							<CommandGroup>
								{systems.map((system) => (
									<CommandItem
										key={system.id}
										onSelect={() => {
											navigate(getPagePath($router, "system", { id: system.id }))
											setOpen(false)
										}}
									>
										<Server className="me-2 size-4" />
										<span className="max-w-60 truncate">{system.name}</span>
										<CommandShortcut>{getHostDisplayValue(system)}</CommandShortcut>
									</CommandItem>
								))}
							</CommandGroup>
							<CommandSeparator className="mb-1.5" />
						</>
					)}
					<CommandGroup heading={t`Pages / Settings`}>
						<CommandItem
							keywords={["home"]}
							onSelect={() => {
								navigate(basePath)
								setOpen(false)
							}}
						>
							<ServerIcon className="me-2 size-4" />
							<span>
								<Trans>All Systems</Trans>
							</span>
							<CommandShortcut>
								<Trans>Page</Trans>
							</CommandShortcut>
						</CommandItem>
						<CommandItem
							onSelect={() => {
								navigate(getPagePath($router, "containers"))
								setOpen(false)
							}}
						>
							<ContainerIcon className="me-2 size-4" />
							<span>
								<Trans>Docker</Trans>
							</span>
							<CommandShortcut>
								<Trans>Page</Trans>
							</CommandShortcut>
						</CommandItem>
						<CommandItem
							onSelect={() => {
								navigate(getPagePath($router, "api_tests"))
								setOpen(false)
							}}
						>
							<TestTube2Icon className="me-2 size-4" />
							<span>
								<Trans>API Tests</Trans>
							</span>
							<CommandShortcut>
								<Trans>Page</Trans>
							</CommandShortcut>
						</CommandItem>
						<CommandItem
							onSelect={() => {
								navigate(getPagePath($router, "audit_logs"))
								setOpen(false)
							}}
						>
							<ClipboardListIcon className="me-2 size-4" />
							<span>
								<Trans>Audit Logs</Trans>
							</span>
							<CommandShortcut>
								<Trans>Page</Trans>
							</CommandShortcut>
						</CommandItem>
						<CommandItem
							onSelect={() => {
								navigate(getPagePath($router, "smart"))
								setOpen(false)
							}}
						>
							<HardDriveIcon className="me-2 size-4" />
							<span>S.M.A.R.T.</span>
							<CommandShortcut>
								<Trans>Page</Trans>
							</CommandShortcut>
						</CommandItem>
						<CommandItem
							onSelect={() => {
								navigate(getPagePath($router, "settings", { name: "general" }))
								setOpen(false)
							}}
						>
							<SettingsIcon className="me-2 size-4" />
							<span>
								<Trans>Settings</Trans>
							</span>
							{SettingsShortcut}
						</CommandItem>
						<CommandItem
							keywords={["alerts"]}
							onSelect={() => {
								navigate(getPagePath($router, "settings", { name: "notifications" }))
								setOpen(false)
							}}
						>
							<MailIcon className="me-2 size-4" />
							<span>
								<Trans>Notifications</Trans>
							</span>
							{SettingsShortcut}
						</CommandItem>
						<CommandItem
							keywords={[t`Universal token`]}
							onSelect={() => {
								navigate(getPagePath($router, "settings", { name: "tokens" }))
								setOpen(false)
							}}
						>
							<FingerprintIcon className="me-2 size-4" />
							<span>
								<Trans>Tokens & Fingerprints</Trans>
							</span>
							{SettingsShortcut}
						</CommandItem>
						<CommandItem
							onSelect={() => {
								navigate(getPagePath($router, "settings", { name: "alert-history" }))
								setOpen(false)
							}}
						>
							<AlertOctagonIcon className="me-2 size-4" />
							<span>
								<Trans>Alert History</Trans>
							</span>
							{SettingsShortcut}
						</CommandItem>
						<CommandItem
							keywords={["help", "oauth", "oidc"]}
							onSelect={() => {
								showDocsUnavailable()
								setOpen(false)
							}}
						>
							<BookIcon className="me-2 size-4" />
							<span>
								<Trans>Documentation</Trans>
							</span>
							<CommandShortcut>尚未开发</CommandShortcut>
						</CommandItem>
					</CommandGroup>
					{isAdmin() && (
						<>
							<CommandSeparator className="mb-1.5" />
							<CommandGroup heading={t`Admin`}>
								<CommandItem
									keywords={["pocketbase"]}
									onSelect={() => {
										setOpen(false)
										window.open(prependBasePath("/_/"), "_blank")
									}}
								>
									<UsersIcon className="me-2 size-4" />
									<span>
										<Trans>Users</Trans>
									</span>
									{AdminShortcut}
								</CommandItem>
								<CommandItem
									onSelect={() => {
										setOpen(false)
										window.open(prependBasePath("/_/#/logs"), "_blank")
									}}
								>
									<LogsIcon className="me-2 size-4" />
									<span>
										<Trans>Logs</Trans>
									</span>
									{AdminShortcut}
								</CommandItem>
								<CommandItem
									onSelect={() => {
										setOpen(false)
										window.open(prependBasePath("/_/#/settings/backups"), "_blank")
									}}
								>
									<DatabaseBackupIcon className="me-2 size-4" />
									<span>
										<Trans>Backups</Trans>
									</span>
									{AdminShortcut}
								</CommandItem>
								<CommandItem
									keywords={["email"]}
									onSelect={() => {
										setOpen(false)
										window.open(prependBasePath("/_/#/settings/mail"), "_blank")
									}}
								>
									<MailIcon className="me-2 size-4" />
									<span>
										<Trans>SMTP settings</Trans>
									</span>
									{AdminShortcut}
								</CommandItem>
							</CommandGroup>
						</>
					)}
					<CommandEmpty>
						<Trans>No results found.</Trans>
					</CommandEmpty>
				</CommandList>
			</CommandDialog>
		)
	}, [open])
})

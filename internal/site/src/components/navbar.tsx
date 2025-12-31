import { Trans } from "@lingui/react/macro"
import { getPagePath } from "@nanostores/router"
import {
	ContainerIcon,
	DatabaseBackupIcon,
	HardDriveIcon,
	LogOutIcon,
	LogsIcon,
	SearchIcon,
	ServerIcon,
	SettingsIcon,
	UserIcon,
	UsersIcon,
} from "lucide-react"
import { lazy, Suspense, useState } from "react"
import { Button, buttonVariants } from "@/components/ui/button"
import {
	DropdownMenu,
	DropdownMenuContent,
	DropdownMenuGroup,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { isAdmin, isReadOnlyUser, logOut, pb } from "@/lib/api"
import { cn, runOnce } from "@/lib/utils"
import { AddSystemButton } from "./add-system"
import { LangToggle } from "./lang-toggle"
import { Logo } from "./logo"
import { ModeToggle } from "./mode-toggle"
import { $router, basePath, Link, prependBasePath } from "./router"
import { t } from "@lingui/core/macro"

const CommandPalette = lazy(() => import("./command-palette"))

const isMac = navigator.platform.toUpperCase().indexOf("MAC") >= 0

export default function Navbar() {
	return (
		<div className="flex items-center h-14 md:h-16 bg-card px-4 pe-3 sm:px-6 border border-border/60 bt-0 rounded-md my-4">
			<Link
				href={basePath}
				aria-label="Home"
				className="py-0 pe-1 ps-0 me-1 group"
				onMouseEnter={runOnce(() => import("@/components/routes/home"))}
			>
				<Logo className="h-14 md:h-16 fill-foreground invert dark:invert-0" />
			</Link>
			<SearchButton />

			<div className="flex items-center ms-auto" onMouseEnter={() => import("@/components/routes/settings/general")}>
				<Tooltip>
					<TooltipTrigger asChild>
						<Link
							href={getPagePath($router, "containers")}
							className={cn(buttonVariants({ variant: "ghost", size: "icon" }))}
							aria-label={t`Containers`}
						>
							<ContainerIcon className="h-[1.2rem] w-[1.2rem]" strokeWidth={1.5} />
						</Link>
					</TooltipTrigger>
					<TooltipContent>
						<p><Trans>Containers</Trans></p>
					</TooltipContent>
				</Tooltip>
				<Tooltip>
					<TooltipTrigger asChild>
						<Link
							href={getPagePath($router, "smart")}
							className={cn("hidden md:grid", buttonVariants({ variant: "ghost", size: "icon" }))}
							aria-label="S.M.A.R.T."
						>
							<HardDriveIcon className="h-[1.2rem] w-[1.2rem]" strokeWidth={1.5} />
						</Link>
					</TooltipTrigger>
					<TooltipContent>
						<p>S.M.A.R.T.</p>
					</TooltipContent>
				</Tooltip>
				<Tooltip>
					<TooltipTrigger asChild>
						<div>
							<LangToggle />
						</div>
					</TooltipTrigger>
					<TooltipContent>
						<p><Trans>Language</Trans></p>
					</TooltipContent>
				</Tooltip>
				<Tooltip>
					<TooltipTrigger asChild>
						<div>
							<ModeToggle />
						</div>
					</TooltipTrigger>
					<TooltipContent>
						<p><Trans>Toggle theme</Trans></p>
					</TooltipContent>
				</Tooltip>
				<Tooltip>
					<TooltipTrigger asChild>
						<Link
							href={getPagePath($router, "settings", { name: "general" })}
							aria-label={t`Settings`}
							className={cn(buttonVariants({ variant: "ghost", size: "icon" }))}
						>
							<SettingsIcon className="h-[1.2rem] w-[1.2rem]" />
						</Link>
					</TooltipTrigger>
					<TooltipContent>
						<p><Trans>Settings</Trans></p>
					</TooltipContent>
				</Tooltip>
				<DropdownMenu>
					<Tooltip>
						<TooltipTrigger asChild>
							<DropdownMenuTrigger asChild>
								<button aria-label={t`User Actions`} className={cn(buttonVariants({ variant: "ghost", size: "icon" }))}>
									<UserIcon className="h-[1.2rem] w-[1.2rem]" />
								</button>
							</DropdownMenuTrigger>
						</TooltipTrigger>
						<TooltipContent>
							<p><Trans>User Actions</Trans></p>
						</TooltipContent>
					</Tooltip>
					<DropdownMenuContent align={isReadOnlyUser() ? "end" : "center"} className="min-w-44">
						<DropdownMenuLabel>{pb.authStore.record?.email}</DropdownMenuLabel>
						<DropdownMenuSeparator />
						<DropdownMenuGroup>
							{isAdmin() && (
								<>
									<DropdownMenuItem asChild>
										<a href={prependBasePath("/_/")} target="_blank">
											<UsersIcon className="me-2.5 h-4 w-4" />
											<span>
												<Trans>Users</Trans>
											</span>
										</a>
									</DropdownMenuItem>
									<DropdownMenuItem asChild>
										<a href={prependBasePath("/_/#/collections?collection=systems")} target="_blank">
											<ServerIcon className="me-2.5 h-4 w-4" />
											<span>
												<Trans>Systems</Trans>
											</span>
										</a>
									</DropdownMenuItem>
									<DropdownMenuItem asChild>
										<a href={prependBasePath("/_/#/logs")} target="_blank">
											<LogsIcon className="me-2.5 h-4 w-4" />
											<span>
												<Trans>Logs</Trans>
											</span>
										</a>
									</DropdownMenuItem>
									<DropdownMenuItem asChild>
										<a href={prependBasePath("/_/#/settings/backups")} target="_blank">
											<DatabaseBackupIcon className="me-2.5 h-4 w-4" />
											<span>
												<Trans>Backups</Trans>
											</span>
										</a>
									</DropdownMenuItem>
									<DropdownMenuSeparator />
								</>
							)}
						</DropdownMenuGroup>
						<DropdownMenuItem onSelect={logOut}>
							<LogOutIcon className="me-2.5 h-4 w-4" />
							<span>
								<Trans>Log Out</Trans>
							</span>
						</DropdownMenuItem>
					</DropdownMenuContent>
				</DropdownMenu>
				<AddSystemButton className="ms-2 hidden 450:flex" />
			</div>
		</div>
	)
}

function SearchButton() {
	const [open, setOpen] = useState(false)

	const Kbd = ({ children }: { children: React.ReactNode }) => (
		<kbd className="pointer-events-none inline-flex h-5 select-none items-center gap-1 rounded border bg-muted px-1.5 font-mono text-[10px] font-medium text-muted-foreground opacity-100">
			{children}
		</kbd>
	)

	return (
		<>
			<Button
				variant="outline"
				className="hidden md:block text-sm text-muted-foreground px-4"
				onClick={() => setOpen(true)}
			>
				<span className="flex items-center">
					<SearchIcon className="me-1.5 h-4 w-4" />
					<Trans>Search</Trans>
					<span className="flex items-center ms-3.5">
						<Kbd>{isMac ? "⌘" : "Ctrl"}</Kbd>
						<Kbd>K</Kbd>
					</span>
				</span>
			</Button>
			<Suspense>
				<CommandPalette open={open} setOpen={setOpen} />
			</Suspense>
		</>
	)
}

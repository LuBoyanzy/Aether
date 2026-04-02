import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { useStore } from "@nanostores/react"
import { getPagePath } from "@nanostores/router"
import { ChevronDownIcon, ExternalLinkIcon, PlusIcon } from "lucide-react"
import { memo, useEffect, useRef, useState } from "react"
import { Button } from "@/components/ui/button"
import {
	Dialog,
	DialogContent,
	DialogDescription,
	DialogHeader,
	DialogTitle,
	DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { toast } from "@/components/ui/use-toast"
import {
	getLocalAgentStatus,
	isReadOnlyUser,
	pb,
	setupLocalAgent,
} from "@/lib/api"
import { SystemStatus } from "@/lib/enums"
import { $publicKey } from "@/lib/stores"
import { cn, generateToken, showDocsUnavailable, tokenMap, useBrowserStorage } from "@/lib/utils"
import type { LocalAgentStatus, SystemRecord } from "@/types"
import {
	copyDockerCompose,
	copyDockerRun,
	copyLinuxCommand,
	copyWindowsCommand,
	type DropdownItem,
	InstallDropdown,
} from "./install-dropdowns"
import { $router, basePath, Link, navigate } from "./router"
import { Badge } from "./ui/badge"
import { DropdownMenu, DropdownMenuTrigger } from "./ui/dropdown-menu"
import { AppleIcon, DockerIcon, FreeBsdIcon, TuxIcon, WindowsIcon } from "./ui/icons"
import { InputCopy } from "./ui/input-copy"

export function AddSystemButton({ className }: { className?: string }) {
	if (isReadOnlyUser()) {
		return null
	}
	const [open, setOpen] = useState(false)
	const opened = useRef(false)
	if (open) {
		opened.current = true
	}

	return (
		<Dialog open={open} onOpenChange={setOpen}>
			<DialogTrigger asChild>
				<Button variant="outline" className={cn("flex gap-1 max-xs:h-[2.4rem]", className)}>
					<PlusIcon className="h-4 w-4 -ms-1" />
					添加系统
				</Button>
			</DialogTrigger>
			{opened.current && <SystemDialog setOpen={setOpen} />}
		</Dialog>
	)
}

/**
 * Token to be used for the next system.
 * Prevents token changing if user copies config, then closes dialog and opens again.
 */
let nextSystemToken: string | null = null

/**
 * SystemDialog component for adding or editing a system.
 * @param {Object} props - The component props.
 * @param {function} props.setOpen - Function to set the open state of the dialog.
 * @param {SystemRecord} [props.system] - Optional system record for editing an existing system.
 */
export const SystemDialog = ({ setOpen, system }: { setOpen: (open: boolean) => void; system?: SystemRecord }) => {
	const publicKey = useStore($publicKey)
	const port = useRef<HTMLInputElement>(null)
	const [hostValue, setHostValue] = useState(system?.host ?? "")
	const isUnixSocket = hostValue.startsWith("/")
	const [storedTab, setStoredTab] = useBrowserStorage("as-tab", "docker")
	const tab = system && storedTab === "local" ? "docker" : storedTab
	const [token, setToken] = useState(system?.token ?? "")

	useEffect(() => {
		;(async () => {
			// if no system, generate a new token
			if (!system) {
				nextSystemToken ||= generateToken()
				return setToken(nextSystemToken)
			}
			// if system exists,get the token from the fingerprint record
			if (tokenMap.has(system.id)) {
				return setToken(tokenMap.get(system.id)!)
			}
			const { token } = await pb.collection("fingerprints").getFirstListItem(`system = "${system.id}"`, {
				fields: "token",
			})
			tokenMap.set(system.id, token)
			setToken(token)
		})()
	}, [system?.id, nextSystemToken])

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault()
		const formData = new FormData(e.target as HTMLFormElement)
		const data = Object.fromEntries(formData) as Record<string, any>
		const name = String(data.name ?? "").trim()
		if (name === "本机") {
			toast({
				title: "操作失败",
				description: "名称“本机”仅供网页接入的本机系统使用，请换一个名称。",
				variant: "destructive",
			})
			return
		}
		data.name = name
		data.users = pb.authStore.record!.id
		try {
			if (system) {
				await pb.collection("systems").update(system.id, { ...data, status: SystemStatus.Pending })
			} else {
				const createdSystem = await pb.collection("systems").create(data)
				await pb.collection("fingerprints").create({
					system: createdSystem.id,
					token,
				})
				// Reset the current token after successful system
				// creation so next system gets a new token
				nextSystemToken = null
			}
			setOpen(false)
			navigate(basePath)
		} catch (e) {
			console.error(e)
			toast({
				title: "操作失败",
				description: getApiErrorMessage(e, "保存客户端失败"),
				variant: "destructive",
			})
		}
	}

	return (
		<DialogContent
			className="w-[90%] sm:w-auto sm:ns-dialog max-w-full rounded-lg"
			onCloseAutoFocus={() => {
				setHostValue(system?.host ?? "")
			}}
		>
			<Tabs value={tab} onValueChange={setStoredTab}>
				<DialogHeader>
					<DialogTitle className="mb-1 pb-1 max-w-100 truncate pr-8">
						{system ? "编辑系统" : "添加系统"}
					</DialogTitle>
					<TabsList className={cn("grid w-full", system ? "grid-cols-2" : "grid-cols-3")}>
						<TabsTrigger value="docker">Docker</TabsTrigger>
						<TabsTrigger value="binary">
							<Trans>Binary</Trans>
						</TabsTrigger>
						{!system && <TabsTrigger value="local">本机</TabsTrigger>}
					</TabsList>
				</DialogHeader>
				{/* Docker (set tab index to prevent auto focusing content in edit system dialog) */}
				<TabsContent value="docker" tabIndex={-1}>
					<DialogDescription className="mb-3 leading-relaxed w-0 min-w-full">
						<Trans>
							Copy the
							<code className="bg-muted px-1 rounded-sm leading-3">docker-compose.yml</code> content for the agent
							below, or register agents automatically with a{" "}
							<Link
								onClick={() => setOpen(false)}
								href={getPagePath($router, "settings", { name: "tokens" })}
								className="link"
							>
								universal token
							</Link>
							. For protected 3D deliveries, download the{" "}
							<Link
								onClick={() => setOpen(false)}
								href={getPagePath($router, "settings", { name: "offline-license" })}
								className="link"
							>
								offline license collector
							</Link>
							{" "}for the customer host.
						</Trans>
					</DialogDescription>
				</TabsContent>
				{/* Binary */}
				<TabsContent value="binary" tabIndex={-1}>
					<DialogDescription className="mb-3 leading-relaxed w-0 min-w-full">
						<Trans>
							Copy the installation command for the agent below, or register agents automatically with a{" "}
							<Link
								onClick={() => setOpen(false)}
								href={getPagePath($router, "settings", { name: "tokens" })}
								className="link"
							>
								universal token
							</Link>
							. For protected 3D deliveries, download the{" "}
							<Link
								onClick={() => setOpen(false)}
								href={getPagePath($router, "settings", { name: "offline-license" })}
								className="link"
							>
								offline license collector
							</Link>
							{" "}for the customer host.
						</Trans>
					</DialogDescription>
				</TabsContent>
				{tab === "local" && !system ? (
					<TabsContent value="local" tabIndex={-1} className="mt-0">
						<LocalAgentTabPanel onConnected={() => navigate(basePath)} />
					</TabsContent>
				) : (
					<form onSubmit={handleSubmit as any}>
						<div className="grid xs:grid-cols-[auto_1fr] gap-y-3 gap-x-4 items-center mt-1 mb-4">
							<Label htmlFor="name" className="xs:text-end">
								<Trans>Name</Trans>
							</Label>
							<Input id="name" name="name" defaultValue={system?.name} required />
							<Label htmlFor="host" className="xs:text-end">
								<Trans>Host / IP</Trans>
							</Label>
							<Input
								id="host"
								name="host"
								value={hostValue}
								required
								onChange={(e) => {
									setHostValue(e.target.value)
								}}
							/>
							<Label htmlFor="port" className={cn("xs:text-end", isUnixSocket && "hidden")}>
								<Trans>Port</Trans>
							</Label>
							<Input
								ref={port}
								name="port"
								id="port"
								defaultValue={system?.port || "45876"}
								required={!isUnixSocket}
								className={cn(isUnixSocket && "hidden")}
							/>
							<Label htmlFor="pkey" className="xs:text-end whitespace-pre">
								<Trans comment="Use 'Key' if your language requires many more characters">Public Key</Trans>
							</Label>
							<InputCopy value={publicKey} id="pkey" name="pkey" />
							<Label htmlFor="tkn" className="xs:text-end whitespace-pre">
								<Trans>Token</Trans>
							</Label>
							<InputCopy value={token} id="tkn" name="tkn" />
						</div>
						<div className="flex justify-end gap-x-2 gap-y-3 flex-col mt-5">
							{/* Docker */}
							<TabsContent value="docker" className="contents">
								<CopyButton
									text={t({ message: "Copy docker compose", context: "Button to copy docker compose file content" })}
									onClick={async () =>
										copyDockerCompose(isUnixSocket ? hostValue : port.current?.value, publicKey, token)
									}
									icon={<DockerIcon className="size-4 -me-0.5" />}
									dropdownItems={[
										{
											text: t({ message: "Copy docker run", context: "Button to copy docker run command" }),
											onClick: async () =>
												copyDockerRun(isUnixSocket ? hostValue : port.current?.value, publicKey, token),
											icons: [DockerIcon],
										},
									]}
								/>
							</TabsContent>
							{/* Binary */}
							<TabsContent value="binary" className="contents">
								<CopyButton
									text={t`Copy Linux command`}
									icon={<TuxIcon className="size-4" />}
									onClick={async () => copyLinuxCommand(isUnixSocket ? hostValue : port.current?.value, publicKey, token)}
									dropdownItems={[
										{
											text: t({ message: "Homebrew command", context: "Button to copy install command" }),
											onClick: async () =>
												copyLinuxCommand(isUnixSocket ? hostValue : port.current?.value, publicKey, token, true),
											icons: [AppleIcon, TuxIcon],
										},
										{
											text: t({ message: "Windows command", context: "Button to copy install command" }),
											onClick: async () => copyWindowsCommand(port.current?.value || "45876", publicKey, token),
											icons: [WindowsIcon],
										},
										{
											text: t({ message: "FreeBSD command", context: "Button to copy install command" }),
											onClick: async () =>
												copyLinuxCommand(isUnixSocket ? hostValue : port.current?.value, publicKey, token),
											icons: [FreeBsdIcon],
										},
										{
											text: t`Manual setup instructions`,
											onClick: showDocsUnavailable,
											icons: [ExternalLinkIcon],
										},
									]}
								/>
							</TabsContent>
							<Button>{system ? <Trans>Save system</Trans> : <Trans>Add system</Trans>}</Button>
						</div>
					</form>
				)}
			</Tabs>
		</DialogContent>
	)
}

function LocalAgentTabPanel({ onConnected }: { onConnected: () => void }) {
	const [status, setStatus] = useState<LocalAgentStatus | null>(null)
	const [submitting, setSubmitting] = useState(false)

	async function loadStatus(silent = false) {
		try {
			const nextStatus = await getLocalAgentStatus()
			setStatus(nextStatus)
		} catch (error) {
			if (!silent) {
				toast({
					title: "操作失败",
					description: getApiErrorMessage(error, "获取本机状态失败"),
					variant: "destructive",
				})
			}
		}
	}

	useEffect(() => {
		void loadStatus(true)
		const timer = window.setInterval(() => {
			void loadStatus(true)
		}, 5000)
		return () => {
			window.clearInterval(timer)
		}
	}, [])

	async function handleConnect() {
		try {
			setSubmitting(true)
			const nextStatus = await setupLocalAgent("本机")
			setStatus(nextStatus)
			toast({ title: "本机接入成功" })
			onConnected()
		} catch (error) {
			toast({
				title: "操作失败",
				description: getApiErrorMessage(error, "本机接入失败"),
				variant: "destructive",
			})
		} finally {
			setSubmitting(false)
		}
	}

	const unavailable = status ? !status.available : false
	const hasSystemRecord = Boolean(status?.systemId)
	const statusLabel = hasSystemRecord ? (status?.configured ? "已接入" : "接入记录异常") : "未接入"
	const runtimeLabel = status?.running ? "Agent 运行中" : "Agent 未运行"
	const systemStatusLabel = formatLocalSystemStatus(status?.systemStatus)
	const connectDisabled = submitting || unavailable || !!status?.configured
	const connectButtonLabel = status?.configured ? "本机已接入" : "一键接入本机"

	return (
		<div className="space-y-4">
			<div className="grid xs:grid-cols-[auto_1fr] gap-y-3 gap-x-4 items-center">
				<Label htmlFor="local-system-name" className="xs:text-end">
					名称
				</Label>
				<Input id="local-system-name" value="本机" readOnly disabled />
			</div>

			<div className="flex flex-wrap gap-2">
				<Badge variant={hasSystemRecord ? "success" : "secondary"}>{statusLabel}</Badge>
				<Badge variant={status?.running ? "success" : "secondary"}>{runtimeLabel}</Badge>
				{systemStatusLabel ? <Badge variant="outline">{systemStatusLabel}</Badge> : null}
			</div>

			<div className="rounded-lg border bg-muted/30 px-3 py-2 text-sm leading-6 text-muted-foreground">
				{hasSystemRecord && status?.configured
					? "本机已接入，请到首页“本机”行继续执行启动、停止、重启和日志查看。"
					: hasSystemRecord
						? "检测到本机记录，但本地配置不完整，请重新执行一键接入。"
						: "点击“一键接入本机”后，首页系统列表会自动生成本机条目。"}
			</div>

			{status?.error ? (
				<div className="rounded-lg border border-destructive/40 bg-destructive/5 px-3 py-2 text-sm text-destructive">
					{status.error}
				</div>
			) : null}

			<div className="grid gap-1 text-xs text-muted-foreground break-all">
				<div>
					<span className="font-medium text-foreground">Hub 地址:</span> {status?.hubUrl || "-"}
				</div>
				<div>
					<span className="font-medium text-foreground">Agent 地址:</span> {status?.host || "127.0.0.1"}:
					{status?.port || "45876"}
				</div>
			</div>

			<div className="flex flex-wrap gap-2">
				<Button type="button" onClick={() => void handleConnect()} disabled={connectDisabled}>
					{connectButtonLabel}
				</Button>
				<Button
					type="button"
					variant="outline"
					onClick={() => {
						void loadStatus()
					}}
					disabled={submitting}
				>
					刷新状态
				</Button>
			</div>
		</div>
	)
}

function formatLocalSystemStatus(status?: string) {
	switch (status) {
		case "up":
			return "系统在线"
		case "down":
			return "系统离线"
		case "paused":
			return "系统已暂停"
		case "pending":
			return "系统连接中"
		default:
			return ""
	}
}

function getApiErrorMessage(error: unknown, fallback: string) {
	if (error && typeof error === "object" && "message" in error && typeof error.message === "string" && error.message) {
		return error.message
	}
	return fallback
}

interface CopyButtonProps {
	text: string
	onClick: () => void
	dropdownItems: DropdownItem[]
	icon?: React.ReactElement<any>
}

const CopyButton = memo((props: CopyButtonProps) => {
	return (
		<div className="flex gap-0 rounded-lg">
			<Button
				type="button"
				variant="outline"
				onClick={props.onClick}
				className="rounded-e-none dark:border-e-0 grow flex items-center gap-2"
			>
				{props.text} {props.icon}
			</Button>
			<div className="w-px h-full bg-muted"></div>
			<DropdownMenu>
				<DropdownMenuTrigger asChild>
					<Button variant="outline" className={"px-2 rounded-s-none border-s-0"}>
						<ChevronDownIcon />
					</Button>
				</DropdownMenuTrigger>
				<InstallDropdown items={props.dropdownItems} />
			</DropdownMenu>
		</div>
	)
})

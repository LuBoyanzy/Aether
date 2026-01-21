import { i18n } from "@lingui/core"
import { memo } from "react"
import { copyToClipboard, getHubURL } from "@/lib/utils"
import { DropdownMenuContent, DropdownMenuItem } from "./ui/dropdown-menu"

// const isbeta = aether.hub_version.includes("beta")
// const imagetag = isbeta ? ":edge" : ""

/**
 * Get the URL of the script to install the agent.
 * @param path - The path to the script (e.g. "/brew").
 * @returns The URL for the script.
 */
const getShellScriptUrl = (path: string = "") => {
	const suffix = path === "/brew" ? "-brew" : ""
	return `https://raw.githubusercontent.com/LuBoyanzy/Aether/main/supplemental/scripts/install-agent${suffix}.sh`
}

const getPowerShellScriptUrl = () => {
	return `https://raw.githubusercontent.com/LuBoyanzy/Aether/main/supplemental/scripts/install-agent.ps1`
}

const getUnixSocketDir = (listen: string) => listen.split("/").slice(0, -1).join("/") || "/"

export function copyDockerCompose(listen = "45876", publicKey: string, token: string) {
	const unixSocketDir = listen.startsWith("/") ? getUnixSocketDir(listen) : ""
	const unixSocketVolume = unixSocketDir && unixSocketDir !== "/" ? `\n      # Required when LISTEN is a unix socket path\n      - ${unixSocketDir}:${unixSocketDir}` : ""

	copyToClipboard(`services:
  aether-agent:
    image: loboyanzy/aether-agent
    container_name: aether-agent
    restart: unless-stopped
    network_mode: host
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./aether_agent_data:/var/lib/aether-agent${unixSocketVolume}
      # monitor other disks / partitions by mounting a folder in /extra-filesystems
      # - /mnt/disk/.aether:/extra-filesystems/sda1:ro
    environment:
      LISTEN: '${listen}'
      KEY: '${publicKey}'
      TOKEN: '${token}'
      HUB_URL: '${getHubURL()}'`)
}

export function copyDockerRun(listen = "45876", publicKey: string, token: string) {
	const unixSocketDir = listen.startsWith("/") ? getUnixSocketDir(listen) : ""
	const unixSocketMount = unixSocketDir && unixSocketDir !== "/" ? ` -v ${unixSocketDir}:${unixSocketDir}` : ""

	copyToClipboard(
		`docker run -d --name aether-agent --network host --restart unless-stopped -v /var/run/docker.sock:/var/run/docker.sock:ro -v ./aether_agent_data:/var/lib/aether-agent${unixSocketMount} -e KEY="${publicKey}" -e LISTEN="${listen}" -e TOKEN="${token}" -e HUB_URL="${getHubURL()}" loboyanzy/aether-agent`
	)
}

export function copyLinuxCommand(port = "45876", publicKey: string, token: string, brew = false) {
	let cmd = `curl -sL ${getShellScriptUrl(
		brew ? "/brew" : ""
	)} -o /tmp/install-agent.sh && chmod +x /tmp/install-agent.sh && /tmp/install-agent.sh -p ${port} -k "${publicKey}" -t "${token}" -url "${getHubURL()}"`
	// Default to built-in mirror for zh-CN; the script decides how to apply it.
	if ((i18n.locale + navigator.language).includes("zh-CN")) {
		cmd += ` --china-mirrors`
	}
	copyToClipboard(cmd)
}

export function copyWindowsCommand(port = "45876", publicKey: string, token: string) {
	copyToClipboard(
		`& iwr -useb ${getPowerShellScriptUrl()} -OutFile "$env:TEMP\\install-agent.ps1"; & PowerShell -ExecutionPolicy Bypass -File "$env:TEMP\\install-agent.ps1" -Key "${publicKey}" -Port ${port} -Token "${token}" -Url "${getHubURL()}"`
	)
}

export interface DropdownItem {
	text: string
	onClick?: () => void
	url?: string
	icons?: React.ComponentType<React.SVGProps<SVGSVGElement>>[]
}

export const InstallDropdown = memo(({ items }: { items: DropdownItem[] }) => {
	return (
		<DropdownMenuContent align="end">
			{items.map((item, index) => {
				const className = "cursor-pointer flex items-center gap-1.5"
				return item.url ? (
					<DropdownMenuItem key={index} asChild>
						<a href={item.url} className={className} target="_blank" rel="noopener noreferrer">
							{item.text}{" "}
							{item.icons?.map((Icon, iconIndex) => (
								<Icon key={iconIndex} className="size-4" />
							))}
						</a>
					</DropdownMenuItem>
				) : (
					<DropdownMenuItem key={index} onClick={item.onClick} className={className}>
						{item.text}{" "}
						{item.icons?.map((Icon, iconIndex) => (
							<Icon key={iconIndex} className="size-4" />
						))}
					</DropdownMenuItem>
				)
			})}
		</DropdownMenuContent>
	)
})

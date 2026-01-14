/**
 * Docker 模块页签与系统选择面板。
 * 统一渲染 11 大功能入口并传递所选系统。
 */
import { t } from "@lingui/core/macro"
import { Trans } from "@lingui/react/macro"
import { memo, useEffect } from "react"
import { useStore } from "@nanostores/react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Card, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { $systems } from "@/lib/stores"
import { useBrowserStorage } from "@/lib/utils"
import DockerOverviewPanel from "@/components/docker/overview"
import DockerContainersPanel from "@/components/docker/containers"
import DockerComposePanel from "@/components/docker/compose"
import DockerImagesPanel from "@/components/docker/images"
import DockerNetworksPanel from "@/components/docker/networks"
import DockerVolumesPanel from "@/components/docker/volumes"
import DockerRegistriesPanel from "@/components/docker/registries"
import DockerComposeTemplatesPanel from "@/components/docker/compose-templates"
import DockerConfigPanel from "@/components/docker/config"
import DockerServiceConfigsPanel from "@/components/docker/service-configs"
import DockerDataCleanupPanel from "@/components/docker/data-cleanup"

const DockerTabs = memo(() => {
	const systems = useStore($systems)
	const [systemId, setSystemId] = useBrowserStorage<string>("docker-system", "", localStorage)
	const hasSystems = systems.length > 0

	useEffect(() => {
		if (!hasSystems) return
		const exists = systems.some((system) => system.id === systemId)
		if (!exists) {
			setSystemId(systems[0]?.id || "")
		}
	}, [hasSystems, systems, systemId, setSystemId])

	return (
		<Card className="p-6 @container w-full">
			<CardHeader className="p-0 mb-4">
				<div className="flex flex-wrap items-end justify-between gap-3">
					<div>
						<CardTitle className="mb-2">
							<Trans>Docker</Trans>
						</CardTitle>
						<CardDescription>
							<Trans>Select a system to manage Docker resources.</Trans>
						</CardDescription>
					</div>
					<div className="min-w-[220px]">
						<Select value={systemId} onValueChange={setSystemId} disabled={!hasSystems}>
							<SelectTrigger id="docker-system-select">
								<SelectValue placeholder={t`Select a system`} />
							</SelectTrigger>
							<SelectContent>
								{systems.map((system) => (
									<SelectItem key={system.id} value={system.id}>
										{system.name}
									</SelectItem>
								))}
							</SelectContent>
						</Select>
					</div>
				</div>
			</CardHeader>
			<Tabs defaultValue="overview" className="w-full">
				<TabsList className="flex h-auto w-full flex-wrap justify-start gap-2 bg-transparent p-0">
					<TabsTrigger
						value="overview"
						className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
					>
						<Trans>Overview</Trans>
					</TabsTrigger>
					<TabsTrigger
						value="containers"
						className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
					>
						<Trans>Containers</Trans>
					</TabsTrigger>
					<TabsTrigger
						value="compose"
						className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
					>
						<Trans>Compose</Trans>
					</TabsTrigger>
					<TabsTrigger
						value="images"
						className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
					>
						<Trans>Images</Trans>
					</TabsTrigger>
					<TabsTrigger
						value="networks"
						className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
					>
						<Trans>Networks</Trans>
					</TabsTrigger>
					<TabsTrigger
						value="volumes"
						className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
					>
						<Trans>Volumes</Trans>
					</TabsTrigger>
					<TabsTrigger
						value="registries"
						className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
					>
						<Trans>Registries</Trans>
					</TabsTrigger>
					<TabsTrigger
						value="compose-templates"
						className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
					>
						<Trans>Compose Templates</Trans>
					</TabsTrigger>
					<TabsTrigger
						value="config"
						className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
					>
						<Trans>Config</Trans>
					</TabsTrigger>
					<TabsTrigger
						value="service-configs"
						className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
					>
						<Trans>Service Configs</Trans>
					</TabsTrigger>
					<TabsTrigger
						value="data-cleanup"
						className="rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
					>
						<Trans>Data Cleanup</Trans>
					</TabsTrigger>
				</TabsList>
				<TabsContent value="overview" className="mt-4 animate-fade-in duration-300">
					<DockerOverviewPanel systemId={systemId} />
				</TabsContent>
				<TabsContent value="containers" className="mt-4 animate-fade-in duration-300">
					<DockerContainersPanel systemId={systemId} />
				</TabsContent>
				<TabsContent value="compose" className="mt-4 animate-fade-in duration-300">
					<DockerComposePanel systemId={systemId} />
				</TabsContent>
				<TabsContent value="images" className="mt-4 animate-fade-in duration-300">
					<DockerImagesPanel systemId={systemId} />
				</TabsContent>
				<TabsContent value="networks" className="mt-4 animate-fade-in duration-300">
					<DockerNetworksPanel systemId={systemId} />
				</TabsContent>
				<TabsContent value="volumes" className="mt-4 animate-fade-in duration-300">
					<DockerVolumesPanel systemId={systemId} />
				</TabsContent>
				<TabsContent value="registries" className="mt-4 animate-fade-in duration-300">
					<DockerRegistriesPanel />
				</TabsContent>
				<TabsContent value="compose-templates" className="mt-4 animate-fade-in duration-300">
					<DockerComposeTemplatesPanel />
				</TabsContent>
				<TabsContent value="config" className="mt-4 animate-fade-in duration-300">
					<DockerConfigPanel systemId={systemId} />
				</TabsContent>
				<TabsContent value="service-configs" className="mt-4 animate-fade-in duration-300">
					<DockerServiceConfigsPanel systemId={systemId} />
				</TabsContent>
				<TabsContent value="data-cleanup" className="mt-4 animate-fade-in duration-300">
					<DockerDataCleanupPanel systemId={systemId} />
				</TabsContent>
			</Tabs>
		</Card>
	)
})

export default DockerTabs

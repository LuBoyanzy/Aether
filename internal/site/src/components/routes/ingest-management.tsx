// ingest-management.tsx renders the unified ingest management entry.
import { BarcodeIcon, WorkflowIcon } from "lucide-react"
import { memo, useEffect } from "react"
import { ActiveAlerts } from "@/components/active-alerts"
import { FooterRepoLink } from "@/components/footer-repo-link"
import IngestVisualizationPanel from "@/components/ingest/ingest-visualization-panel"
import ItemCodeManagementPanel from "@/components/item-codes-management/item-code-management-panel"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { BRAND_NAME } from "@/lib/utils"

export default memo(function IngestManagementPage() {
	useEffect(() => {
		document.title = `入库管理 - ${BRAND_NAME}`
	}, [])

	return (
		<>
			<div className="grid gap-4">
				<ActiveAlerts />
				<Card className="border-border/60">
					<CardHeader className="gap-2">
						<div className="flex items-center gap-2">
							<WorkflowIcon className="h-5 w-5 text-primary" />
							<CardTitle>入库管理</CardTitle>
						</div>
						<CardDescription>集中查看入库过程、正式入库结果，并维护 Item Code。</CardDescription>
					</CardHeader>
					<CardContent>
						<Tabs defaultValue="visualization" className="w-full">
							<TabsList className="flex h-auto w-full flex-wrap justify-start gap-2 bg-transparent p-0">
								<TabsTrigger
									value="visualization"
									className="gap-2 rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
								>
									<WorkflowIcon className="h-4 w-4" />
									入库可视化
								</TabsTrigger>
								<TabsTrigger
									value="item-codes"
									className="gap-2 rounded-full border border-transparent bg-muted/40 px-4 transition-all duration-300 ease-in-out data-[state=active]:bg-primary data-[state=active]:text-primary-foreground data-[state=active]:shadow-sm hover:bg-muted/60"
								>
									<BarcodeIcon className="h-4 w-4" />
									Item Code 管理
								</TabsTrigger>
							</TabsList>
							<TabsContent value="visualization" className="mt-4">
								<IngestVisualizationPanel />
							</TabsContent>
							<TabsContent value="item-codes" className="mt-4">
								<ItemCodeManagementPanel />
							</TabsContent>
						</Tabs>
					</CardContent>
				</Card>
			</div>
			<FooterRepoLink />
		</>
	)
})

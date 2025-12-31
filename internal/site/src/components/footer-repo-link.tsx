import { BRAND_BASELINE, BRAND_NAME } from "@/lib/utils"
import { Separator } from "./ui/separator"

export function FooterRepoLink() {
	return (
		<div className="flex gap-1.5 justify-end items-center pe-3 sm:pe-6 mt-3.5 mb-4 text-xs opacity-80">
			<span className="text-muted-foreground">
				{BRAND_NAME} {globalThis.AETHER.HUB_VERSION}
			</span>
			<Separator orientation="vertical" className="h-2.5 bg-muted-foreground opacity-70" />
			<span className="text-muted-foreground">{BRAND_BASELINE}</span>
		</div>
	)
}

import { memo, useMemo } from "react"
import { cn } from "@/lib/utils"
import {
	ReactFlow,
	Background,
	Controls,
	Handle,
	MiniMap,
	MarkerType,
	Position,
	type Edge,
	type Node,
	type NodeProps,
} from "@xyflow/react"
import "@xyflow/react/dist/style.css"
import {
	AlertCircle,
	Brain,
	CheckCircle2,
	CircleDashed,
	Database,
	HardDriveUpload,
	HelpCircle,
	ListPlus,
	Lock,
	MessageSquare,
	PlayCircle,
	RotateCw,
	Search,
	ShieldCheck,
	SkipForward,
	type LucideIcon,
} from "lucide-react"
export type IngestPipelineStage =
	| "mq.preprocess_message.consume"
	| "mq.preprocess_message.validate"
	| "mq.preprocess_message.ack"
	| "mq.preprocess_message.nack"
	| "mq.preprocess_message.parse"
	| "minio.message.validate"
	| "minio.message.handle"
	| "minio.task.submit"
	| "minio.task.start"
	| "minio.task.end"
	| "minio.task.retry"
	| "minio.task.locked"
	| "minio.task.skip"
	| "minio.task.not_found"
	| "minio.ingest.skip"
	| "minio.upload_only.execute"
	| "minio.query.prepare"
	| "minio.query.execute"
	| "infer.request"
	| "other"

export type IngestPipelineToken = {
	key: string
	label: string
	stage: IngestPipelineStage
	status?: "running" | "success" | "failure"
}

type PipelineStatus = "running" | "success" | "failure"

type StageKind = "primary" | "branch" | "exception"

type StageNode = {
	key: IngestPipelineStage
	label: string
	kind?: StageKind
	icon: LucideIcon
}

type StageEdge = {
	from: IngestPipelineStage
	to: IngestPipelineStage
	kind?: StageKind
}

type PipelineNodeData = {
	label: string
	kind?: StageKind
	icon: LucideIcon
	count?: number
	status?: PipelineStatus
	active?: boolean
}

type PipelineNodeType = Node<PipelineNodeData, "pipeline">

const STAGE_NODES: StageNode[] = [
	{ key: "mq.preprocess_message.consume", label: "MQ 消费", kind: "primary", icon: MessageSquare },
	{ key: "mq.preprocess_message.validate", label: "MQ 校验", kind: "primary", icon: ShieldCheck },
	{ key: "mq.preprocess_message.ack", label: "MQ 确认", kind: "primary", icon: CheckCircle2 },
	{ key: "mq.preprocess_message.parse", label: "MQ 解析失败", kind: "exception", icon: AlertCircle },
	{ key: "mq.preprocess_message.nack", label: "MQ 拒收", kind: "exception", icon: CircleDashed },

	{ key: "minio.message.validate", label: "消息校验", kind: "primary", icon: ShieldCheck },
	{ key: "minio.message.handle", label: "消息异常", kind: "exception", icon: AlertCircle },

	{ key: "minio.ingest.skip", label: "智能跳过", kind: "branch", icon: SkipForward },
	{ key: "minio.upload_only.execute", label: "仅上传", kind: "branch", icon: HardDriveUpload },

	{ key: "minio.task.submit", label: "任务提交", kind: "primary", icon: ListPlus },
	{ key: "minio.task.start", label: "任务开始", kind: "primary", icon: PlayCircle },
	{ key: "minio.task.end", label: "任务结束", kind: "primary", icon: CheckCircle2 },
	{ key: "minio.task.retry", label: "任务重试", kind: "exception", icon: RotateCw },

	{ key: "minio.query.prepare", label: "查询准备", kind: "branch", icon: Search },
	{ key: "minio.query.execute", label: "查询执行", kind: "branch", icon: Database },

	{ key: "minio.task.locked", label: "任务锁定", kind: "exception", icon: Lock },
	{ key: "minio.task.skip", label: "状态跳过", kind: "exception", icon: SkipForward },
	{ key: "minio.task.not_found", label: "任务不存在", kind: "exception", icon: HelpCircle },

	{ key: "infer.request", label: "推理请求", kind: "primary", icon: Brain },
	{ key: "other", label: "未知事件", kind: "exception", icon: HelpCircle },
]

const STAGE_EDGES: StageEdge[] = [
	{ from: "mq.preprocess_message.consume", to: "mq.preprocess_message.validate", kind: "primary" },
	{ from: "mq.preprocess_message.consume", to: "mq.preprocess_message.parse", kind: "exception" },
	{ from: "mq.preprocess_message.parse", to: "mq.preprocess_message.nack", kind: "exception" },
	{ from: "mq.preprocess_message.validate", to: "mq.preprocess_message.ack", kind: "primary" },
	{ from: "mq.preprocess_message.validate", to: "mq.preprocess_message.nack", kind: "exception" },
	{ from: "mq.preprocess_message.ack", to: "minio.message.validate", kind: "primary" },
	{ from: "minio.message.validate", to: "mq.preprocess_message.nack", kind: "exception" },
	{ from: "minio.message.validate", to: "minio.task.submit", kind: "primary" },
	{ from: "minio.message.validate", to: "minio.ingest.skip", kind: "branch" },
	{ from: "minio.message.validate", to: "minio.upload_only.execute", kind: "branch" },
	{ from: "minio.message.validate", to: "minio.query.prepare", kind: "branch" },
	{ from: "minio.task.start", to: "minio.message.handle", kind: "exception" },
	{ from: "minio.message.handle", to: "mq.preprocess_message.nack", kind: "exception" },

	{ from: "minio.task.submit", to: "minio.task.start", kind: "primary" },
	{ from: "minio.task.start", to: "minio.task.end", kind: "primary" },
	{ from: "minio.task.end", to: "infer.request", kind: "primary" },
	{ from: "minio.task.end", to: "minio.task.retry", kind: "exception" },
	{ from: "minio.task.retry", to: "minio.task.start", kind: "exception" },
	{ from: "minio.task.end", to: "minio.query.prepare", kind: "branch" },

	{ from: "minio.task.submit", to: "minio.task.locked", kind: "exception" },
	{ from: "minio.task.submit", to: "minio.task.skip", kind: "exception" },
	{ from: "minio.task.submit", to: "minio.task.not_found", kind: "exception" },

	{ from: "minio.query.prepare", to: "infer.request", kind: "branch" },
	{ from: "infer.request", to: "minio.query.execute", kind: "branch" },
]

function getNodeColor(kind?: StageKind) {
	if (kind === "exception") return "text-destructive border-destructive/30 bg-destructive/10"
	if (kind === "branch") return "text-orange-500 border-orange-500/30 bg-orange-500/10"
	return "text-primary border-primary/30 bg-primary/10"
}

const LANE_GAP = 210
const COLUMN_GAP = 230
const ROW_GAP = 78

const LANE_OFFSET: Record<StageKind, number> = {
	primary: 0,
	branch: -LANE_GAP,
	exception: LANE_GAP,
}

function buildLayout() {
	const layoutMap: Record<IngestPipelineStage, { col: number; lane: StageKind; row?: number }> = {
		"mq.preprocess_message.consume": { col: 0, lane: "primary" },
		"mq.preprocess_message.validate": { col: 1, lane: "primary" },
		"mq.preprocess_message.parse": { col: 1, lane: "exception" },
		"mq.preprocess_message.ack": { col: 2, lane: "primary" },
		"mq.preprocess_message.nack": { col: 2, lane: "exception" },
		"minio.message.validate": { col: 3, lane: "primary" },
		"minio.ingest.skip": { col: 4, lane: "branch" },
		"minio.task.submit": { col: 4, lane: "primary" },
		"minio.task.locked": { col: 4, lane: "exception", row: 0 },
		"minio.task.skip": { col: 4, lane: "exception", row: 1 },
		"minio.task.not_found": { col: 4, lane: "exception", row: 2 },
		"minio.upload_only.execute": { col: 5, lane: "branch" },
		"minio.task.start": { col: 5, lane: "primary" },
		"minio.message.handle": { col: 5, lane: "exception", row: 3 },
		"minio.task.end": { col: 6, lane: "primary" },
		"minio.task.retry": { col: 6, lane: "exception", row: 4 },
		"minio.query.prepare": { col: 7, lane: "branch" },
		"infer.request": { col: 8, lane: "primary" },
		"minio.query.execute": { col: 9, lane: "branch" },
		other: { col: 10, lane: "exception", row: 0 },
	}

	const baseNodes: Node<PipelineNodeData>[] = STAGE_NODES.map((node) => {
		const layout = layoutMap[node.key] ?? { col: 0, lane: node.kind ?? "primary" }
		const laneOffset = LANE_OFFSET[layout.lane]
		return {
			id: node.key,
			type: "pipeline",
			position: {
				x: layout.col * COLUMN_GAP,
				y: laneOffset + (layout.row ?? 0) * ROW_GAP,
			},
			data: {
				label: node.label,
				kind: layout.lane,
				icon: node.icon,
			},
		}
	})

	const nodeLookup = new Map(
		baseNodes.map((node) => [node.id as IngestPipelineStage, { x: node.position.x, y: node.position.y }])
	)

	const baseEdges: Edge[] = STAGE_EDGES.map((edge, idx) => {
		const kind = edge.kind ?? "primary"
		const isException = kind === "exception"
		const isBranch = kind === "branch"
		const stroke = isException ? "var(--destructive)" : isBranch ? "#f59e0b" : "var(--primary)"
		const strokeWidth = isException ? 2 : isBranch ? 2.2 : 2.8
		const dash = isException ? "6 4" : isBranch ? "4 3" : undefined
		const sourcePos = nodeLookup.get(edge.from)
		const targetPos = nodeLookup.get(edge.to)
		const dx = sourcePos && targetPos ? targetPos.x - sourcePos.x : 0
		const dy = sourcePos && targetPos ? targetPos.y - sourcePos.y : 0
		let sourceHandle = "source-right"
		let targetHandle = "target-left"
		if (dx < -10) {
			sourceHandle = "source-left"
			targetHandle = "target-right"
		} else if (Math.abs(dy) > 30 && Math.abs(dx) < 80) {
			if (dy < 0) {
				sourceHandle = "source-top"
				targetHandle = "target-bottom"
			} else {
				sourceHandle = "source-bottom"
				targetHandle = "target-top"
			}
		} else if (Math.abs(dy) > 80) {
			if (dy < 0) {
				sourceHandle = "source-top"
				targetHandle = "target-bottom"
			} else {
				sourceHandle = "source-bottom"
				targetHandle = "target-top"
			}
		}
		return {
			id: `${edge.from}-${edge.to}-${idx}`,
			source: edge.from,
			target: edge.to,
			type: "smoothstep",
			animated: !isException,
			sourceHandle,
			targetHandle,
			style: {
				stroke,
				strokeWidth,
				strokeDasharray: dash,
				opacity: isException ? 0.7 : 0.85,
			},
			pathOptions: { offset: 16 },
			markerEnd: {
				type: MarkerType.ArrowClosed,
				color: stroke,
				width: 18,
				height: 18,
			},
		}
	})

	return { baseNodes, baseEdges }
}

function PipelineNode({ data }: NodeProps<PipelineNodeType>) {
	const Icon = data.icon
	const isActive = data.active
	const isFailure = data.status === "failure"
	const isSuccess = data.status === "success"
	const ring = isFailure
		? "ring-2 ring-destructive/70"
		: isSuccess
			? "ring-2 ring-emerald-500/60"
			: isActive
				? "ring-2 ring-primary/50"
				: "ring-1 ring-border/50"

	return (
		<div className="relative">
			<Handle id="target-left" type="target" position={Position.Left} className="opacity-0" />
			<Handle id="target-right" type="target" position={Position.Right} className="opacity-0" />
			<Handle id="target-top" type="target" position={Position.Top} className="opacity-0" />
			<Handle id="target-bottom" type="target" position={Position.Bottom} className="opacity-0" />
			<Handle id="source-left" type="source" position={Position.Left} className="opacity-0" />
			<Handle id="source-right" type="source" position={Position.Right} className="opacity-0" />
			<Handle id="source-top" type="source" position={Position.Top} className="opacity-0" />
			<Handle id="source-bottom" type="source" position={Position.Bottom} className="opacity-0" />
			<div
				className={cn(
					"rounded-xl border bg-card/90 px-3 py-2 shadow-sm backdrop-blur-sm transition",
					getNodeColor(data.kind),
					ring
				)}
			>
				<div className="flex items-center gap-2">
					<div
						className={cn(
							"grid h-8 w-8 place-items-center rounded-lg border bg-background/70",
							isFailure ? "border-destructive/40 text-destructive" : "border-border text-foreground"
						)}
					>
						<Icon className="h-4 w-4" />
					</div>
					<div className="flex-1 text-xs font-semibold text-foreground/90">{data.label}</div>
					{data.count ? (
						<span className="min-w-6 rounded-full bg-foreground px-2 py-0.5 text-[10px] font-bold text-background">
							{data.count}
						</span>
					) : null}
				</div>
			</div>
		</div>
	)
}

export const IngestPipeline = memo(({ tokens, className }: { tokens: IngestPipelineToken[]; className?: string }) => {
	const layout = useMemo(() => buildLayout(), [])

	const stageStats = useMemo(() => {
		const map = new Map<IngestPipelineStage, { count: number; status: PipelineStatus }>()
		for (const t of tokens) {
			const prev = map.get(t.stage)
			const status: PipelineStatus = t.status ?? "running"
			if (!prev) {
				map.set(t.stage, { count: 1, status })
				continue
			}
			prev.count += 1
			if (prev.status !== "failure") {
				if (status === "failure") prev.status = "failure"
				else if (status === "running") prev.status = "running"
			}
		}
		return map
	}, [tokens])

	const nodes = useMemo(() => {
		return layout.baseNodes.map((node) => {
			const stats = stageStats.get(node.id as IngestPipelineStage)
			return {
				...node,
				data: {
					...node.data,
					count: stats?.count,
					status: stats?.status,
					active: !!stats,
				},
			}
		})
	}, [layout.baseNodes, stageStats])

	return (
		<div className={cn("h-[360px] w-full overflow-hidden rounded-xl border bg-card/50", className)}>
			<ReactFlow
				nodes={nodes}
				edges={layout.baseEdges}
				nodeTypes={{ pipeline: PipelineNode }}
				fitView
				fitViewOptions={{ padding: 0.2 }}
				nodesDraggable={false}
				nodesConnectable={false}
				elementsSelectable={false}
				zoomOnScroll
				panOnScroll
				attributionPosition="bottom-left"
			>
				<Background gap={24} color="var(--border)" />
				<MiniMap pannable zoomable className="bg-background/80" />
				<Controls showInteractive={false} />
			</ReactFlow>
		</div>
	)
})
IngestPipeline.displayName = "IngestPipeline"

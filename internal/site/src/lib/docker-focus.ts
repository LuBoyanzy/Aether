/**
 * Docker 容器关注规则数据访问层。
 * 提供系统级规则的查询与维护接口。
 */
import { pb } from "@/lib/api"
import type { DockerFocusMatchType, DockerFocusServiceRecord } from "@/types"

const COLLECTION = pb.collection<DockerFocusServiceRecord>("docker_focus_services")
const FIELDS = "id,system,match_type,value,value2,created,updated"

export const listDockerFocusServices = async (systemId: string) =>
	await COLLECTION.getFullList({
		sort: "+match_type,+value",
		fields: FIELDS,
		filter: pb.filter("system={:system}", { system: systemId }),
	})

export const createDockerFocusService = async (payload: {
	system: string
	match_type: DockerFocusMatchType
	value: string
	value2?: string
}) => await COLLECTION.create(payload)

export const deleteDockerFocusService = async (id: string) => await COLLECTION.delete(id)

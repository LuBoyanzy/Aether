/**
 * Docker 关注告警设置数据访问层。
 * 提供系统级配置的读取与保存接口。
 */
import { pb } from "@/lib/api"
import type { DockerFocusAlertSettingsRecord } from "@/types"

// 后端集合定义: internal/migrations/202601231551_docker_focus_alert_settings.go
const COLLECTION = pb.collection<DockerFocusAlertSettingsRecord>("docker_focus_alert_settings")
const FIELDS = "id,system,enabled,recovery_seconds,alert_on_no_match,created,updated"

export const getDockerFocusAlertSettings = async (systemId: string) => {
	const records = await COLLECTION.getFullList({
		fields: FIELDS,
		filter: pb.filter("system={:system}", { system: systemId }),
	})
	return records[0] ?? null
}

export const createDockerFocusAlertSettings = async (payload: {
	system: string
	enabled: boolean
	recovery_seconds: number
	alert_on_no_match: boolean
}) => await COLLECTION.create(payload)

export const updateDockerFocusAlertSettings = async (
	id: string,
	payload: {
		enabled: boolean
		recovery_seconds: number
		alert_on_no_match: boolean
	}
) => await COLLECTION.update(id, payload)

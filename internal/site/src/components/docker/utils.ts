/**
 * Docker 前端模块通用格式化工具。
 * 集中处理时间、大小与标识符展示。
 */
import { decimalString, formatBytes, formatShortDate } from "@/lib/utils"

export const formatUnixSeconds = (value?: number) => {
	if (!value) return "-"
	return formatShortDate(new Date(value * 1000).toISOString())
}

export const formatBytesLabel = (value?: number, digits = 2) => {
	if (value === undefined || value === null) return "-"
	const { value: size, unit } = formatBytes(value)
	return `${decimalString(size, digits)} ${unit}`
}

export const formatTagList = (values?: string[]) => {
	if (!values || values.length === 0) return "-"
	return values.join(", ")
}

export const formatShortId = (value?: string) => {
	if (!value) return "-"
	return value.slice(0, 12)
}

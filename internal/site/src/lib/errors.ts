import { t } from "@lingui/core/macro"

const DEFAULT_MAX_ERROR_LENGTH = 200

type ApiErrorLike = {
	response?: unknown
	data?: unknown
	message?: unknown
}

const normalizeErrorText = (value: unknown) => {
	if (typeof value !== "string") return ""
	const trimmed = value.trim()
	if (!trimmed) return ""
	return trimmed.replace(/\s+/g, " ")
}

const extractErrorFromResponse = (response: unknown) => {
	if (!response || typeof response !== "object") return ""
	const data = response as { error?: unknown; message?: unknown; data?: unknown }
	const direct = normalizeErrorText(data.error) || normalizeErrorText(data.message)
	if (direct) return direct
	if (!data.data || typeof data.data !== "object") return ""
	const nested = data.data as { error?: unknown; message?: unknown }
	return normalizeErrorText(nested.error) || normalizeErrorText(nested.message)
}

const extractApiErrorMessage = (error: unknown) => {
	if (!error || typeof error !== "object") return ""
	const apiError = error as ApiErrorLike
	const response = apiError.response ?? apiError.data
	const responseMessage = extractErrorFromResponse(response)
	if (responseMessage) return responseMessage
	const message = normalizeErrorText(apiError.message)
	if (message && message !== "Something went wrong.") return message
	return ""
}

const truncateErrorMessage = (message: string, maxLength: number) => {
	if (!message) return ""
	const chars = Array.from(message)
	if (chars.length <= maxLength) return message
	return `${chars.slice(0, maxLength).join("")}...`
}

export const formatContainerOperationError = (error: unknown, maxLength = DEFAULT_MAX_ERROR_LENGTH) => {
	const raw = extractApiErrorMessage(error)
	if (!raw) return t`Failed to operate container`
	const short = truncateErrorMessage(raw, maxLength)
	const normalized = raw.toLowerCase()
	let friendly = ""

	if (normalized.includes("context deadline exceeded") || normalized.includes("deadline exceeded")) {
		friendly = t`Operation timed out`
	} else if (normalized.includes("operation succeeded but refresh failed")) {
		friendly = t`Operation completed, but refresh failed`
	} else if (normalized.includes("forbidden")) {
		friendly = t`You don't have permission to perform this operation`
	} else if (normalized.includes("system not found")) {
		friendly = t`System not found`
	}

	return friendly ? `${friendly} (${short})` : short
}

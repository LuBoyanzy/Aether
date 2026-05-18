const decimalFormatters = new Map<number, Intl.NumberFormat>()

function decimalString(value: number, digits: number) {
	let formatter = decimalFormatters.get(digits)
	if (!formatter) {
		formatter = new Intl.NumberFormat(undefined, {
			minimumFractionDigits: digits,
			maximumFractionDigits: digits,
		})
		decimalFormatters.set(digits, formatter)
	}
	return formatter.format(value)
}

function formatBytes(value: number, perSecond = false) {
	const safeValue = Number.isFinite(value) && value > 0 ? value : 0
	const suffix = perSecond ? "/s" : ""
	if (safeValue < 100) return { value: safeValue, unit: `B${suffix}` }
	if (safeValue < 1000 * 1024) return { value: safeValue / 1024, unit: `KB${suffix}` }
	if (safeValue < 1000 * 1024 ** 2) return { value: safeValue / 1024 ** 2, unit: `MB${suffix}` }
	if (safeValue < 1000 * 1024 ** 3) return { value: safeValue / 1024 ** 3, unit: `GB${suffix}` }
	return { value: safeValue / 1024 ** 4, unit: `TB${suffix}` }
}

export function formatI3DBytesPerSecond(value: number) {
	const formatted = formatBytes(value, true)
	return `${decimalString(formatted.value, formatted.value >= 10 ? 1 : 2)} ${formatted.unit}`
}

export function formatI3DBytesValue(value: number) {
	const formatted = formatBytes(value)
	return `${decimalString(formatted.value, formatted.value >= 10 ? 1 : 2)} ${formatted.unit}`
}

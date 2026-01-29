import type { Messages } from "@lingui/core"
import { i18n } from "@lingui/core"
import { t } from "@lingui/core/macro"
import { detect, fromNavigator, fromStorage } from "@lingui/detect-locale"
import languages from "@/lib/languages"
import * as enModule from "@/locales/en/en"
import { BatteryState } from "./enums"
import { $direction } from "./stores"

type LocaleModule = {
	messages?: Messages
	default?: Messages
}

const rtlLanguages = new Set(["ar", "fa", "he"])

function resolveMessages(module: LocaleModule, locale: string): Messages {
	const resolved = module.messages ?? module.default
	if (!resolved) {
		const error = new Error(`Missing messages export for locale: ${locale}`)
		console.error("Missing Lingui messages export", {
			locale,
			moduleKeys: Object.keys(module ?? {}),
			error,
		})
		throw error
	}
	return resolved
}

const enMessages = resolveMessages(enModule, "en")

// activates locale
function activateLocale(locale: string, messages: Messages = enMessages) {
	i18n.load(locale, messages)
	i18n.activate(locale)
	document.documentElement.lang = locale
	localStorage.setItem("lang", locale)
	$direction.set(rtlLanguages.has(locale) ? "rtl" : "ltr")
}

// dynamically loads translations for the given locale
export async function dynamicActivate(locale: string) {
	if (locale === "en") {
		activateLocale(locale)
	} else {
		try {
			const module = (await import(`../locales/${locale}/${locale}.ts`)) as LocaleModule
			activateLocale(locale, resolveMessages(module, locale))
		} catch (error) {
			console.error(`Error loading ${locale}`, error)
			activateLocale("en")
		}
	}
}

export function getLocale() {
	// let locale = detect(fromUrl("lang"), fromStorage("lang"), fromNavigator(), "en")
	let locale = detect(fromStorage("lang"), fromNavigator(), "en")
	// log if dev
	if (import.meta.env.DEV) {
		console.log("detected locale", locale)
	}
	// handle zh variants
	if (locale?.startsWith("zh-")) {
		// map zh variants to zh-CN
		const zhVariantMap: Record<string, string> = {
			"zh-HK": "zh-HK",
			"zh-TW": "zh",
			"zh-MO": "zh",
			"zh-Hant": "zh",
		}
		return zhVariantMap[locale] || "zh-CN"
	}
	locale = (locale || "en").split("-")[0]
	// use en if locale is not in languages
	if (!languages.some((l) => l.lang === locale)) {
		locale = "en"
	}
	return locale
}

////////////////////////////////////////////////////////

export const batteryStateTranslations = {
	[BatteryState.Unknown]: () => t({ message: "Unknown", comment: "Context: Battery state" }),
	[BatteryState.Empty]: () => t({ message: "Empty", comment: "Context: Battery state" }),
	[BatteryState.Full]: () => t({ message: "Full", comment: "Context: Battery state" }),
	[BatteryState.Charging]: () => t({ message: "Charging", comment: "Context: Battery state" }),
	[BatteryState.Discharging]: () => t({ message: "Discharging", comment: "Context: Battery state" }),
	[BatteryState.Idle]: () => t({ message: "Idle", comment: "Context: Battery state" }),
} as const

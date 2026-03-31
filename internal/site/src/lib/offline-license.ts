import { prependBasePath } from "@/components/router"
import { pb } from "./api"
import type {
	OfflineLicenseActivationRecord,
	OfflineLicenseActivationPreview,
	OfflineLicenseExportResponse,
	OfflineLicenseIssueResponse,
	OfflineLicenseOverviewResponse,
} from "@/types"

function createAuthHeaders() {
	const headers = new Headers()
	headers.set("Accept", "application/json")
	if (pb.authStore.token) {
		headers.set("Authorization", pb.authStore.token)
	}
	return headers
}

function downloadTextFile(fileName: string, content: string, mimeType: string) {
	const blob = new Blob([content], { type: mimeType })
	const url = URL.createObjectURL(blob)
	const anchor = document.createElement("a")
	anchor.href = url
	anchor.download = fileName
	anchor.click()
	URL.revokeObjectURL(url)
}

export async function downloadOfflineLicenseCollector() {
	const response = await fetch(prependBasePath("/api/aether/activation/collector"), {
		method: "GET",
		headers: createAuthHeaders(),
	})
	if (!response.ok) {
		throw new Error(await response.text())
	}
	const content = await response.text()
	downloadTextFile("i3d-license-collector.sh", content, "text/x-shellscript;charset=utf-8")
}

export async function previewOfflineActivationRequest(content: string, systemId = "") {
	return pb.send<OfflineLicenseActivationPreview>("/api/aether/activation/requests/preview", {
		method: "POST",
		body: {
			content,
			systemId,
		},
	})
}

export async function importOfflineActivationRequest(payload: {
	content: string
	systemId?: string
	customer: string
	tenant?: string
	project_name?: string
	site_name?: string
	remarks?: string
}) {
	return pb.send<{ id: string; action: string; requestId: string; activation: OfflineLicenseActivationRecord }>(
		"/api/aether/activation/requests/import",
		{
			method: "POST",
			body: payload,
		}
	)
}

export async function issueOfflineLicense(payload: {
	activationId: string
	customer: string
	tenant: string
	notBefore?: string
	notAfter?: string
	modelNames?: string[]
}) {
	return pb.send<OfflineLicenseIssueResponse>("/api/aether/licenses/issue", {
		method: "POST",
		body: payload,
	})
}

export function fetchOfflineLicenseOverview() {
	return pb.send<OfflineLicenseOverviewResponse>("/api/aether/licenses/overview", {
		method: "GET",
	})
}

export async function exportOfflineLicense(licenseId: string) {
	return pb.send<OfflineLicenseExportResponse>("/api/aether/licenses/export", {
		method: "GET",
		query: { licenseId },
	})
}

export async function downloadOfflineLicenseArtifact(licenseId: string) {
	const data = await exportOfflineLicense(licenseId)
	downloadTextFile(data.fileName || "license.dat", data.content, "application/json;charset=utf-8")
	return data
}

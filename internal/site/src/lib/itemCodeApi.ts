import { pb } from "@/lib/api"
import type { ItemCodeAuditItem, ItemCodeDBDetail, ItemCodeDBRecord, ItemCodeRecord } from "@/types"

export const listItemCodes = (params?: { page?: number; perPage?: number; filter?: string; sort?: string }) =>
	pb.collection("item_codes").getList<ItemCodeRecord>(params?.page ?? 1, params?.perPage ?? 50, {
		filter: params?.filter,
		sort: params?.sort ?? "-created",
	})

export const getItemCode = (id: string) => pb.collection("item_codes").getOne<ItemCodeRecord>(id)

export const updateItemCode = (id: string, data: Partial<ItemCodeRecord>) =>
	pb.collection("item_codes").update<ItemCodeRecord>(id, data)

export const deleteItemCode = (id: string) =>
	pb.send<{ status: string }>("/api/aether/item-codes", { method: "DELETE", query: { id } })

export const batchDeleteItemCodes = (ids: string[]) =>
	pb.send<{ deleted: number; failed: number }>("/api/aether/item-codes/batch-delete", {
		method: "POST",
		body: { ids },
	})

export const previewQueryDeleteItemCodes = (filter: string) =>
	pb.send<{ count: number; items: { id: string; code: string; name: string; category: string; status: string }[] }>(
		"/api/aether/item-codes/query-delete/preview",
		{ method: "POST", body: { filter } }
	)

export const queryDeleteItemCodes = (filter: string) =>
	pb.send<{ deleted: number }>("/api/aether/item-codes/query-delete", {
		method: "POST",
		body: { filter },
	})

export const listItemCodeAuditLogs = (params?: {
	action?: string
	userId?: string
	start?: string
	end?: string
	page?: number
	perPage?: number
}) =>
	pb.send<{ items: ItemCodeAuditItem[] }>("/api/aether/item-codes/audit-logs", {
		query: params ?? {},
	})

// --- PostgreSQL DB APIs ---

export const deleteItemCodeByCode = async (code: string, password: string) => {
	return pb.send<{ status: string }>("/api/aether/item-codes/db-delete", {
		method: "DELETE",
		query: { code, password },
	})
}

export const batchDeleteItemCodesByCode = async (codes: string[], password: string) => {
	return pb.send<{ deleted: number; failed: number }>("/api/aether/item-codes/db-batch-delete", {
		method: "POST",
		body: { codes, password },
	})
}

export const listItemCodesFromDB = (params?: {
	page?: number
	perPage?: number
	search?: string
	category?: string
	status?: string
}) =>
	pb.send<{ items: ItemCodeDBRecord[]; total: number }>("/api/aether/item-codes/db-list", {
		query: params ?? {},
	})

export const getItemCodeDetailFromDB = (code: string) =>
	pb.send<ItemCodeDBDetail>("/api/aether/item-codes/db-detail", {
		query: { code },
	})

export const updateItemCodeInDB = (payload: {
	code: string
	name: string
	category: string
	description: string
}) =>
	pb.send<{ status: string }>("/api/aether/item-codes/db-update", {
		method: "POST",
		body: payload,
	})

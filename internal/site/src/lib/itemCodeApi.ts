import { pb } from "@/lib/api"
import type { ItemCodeAuditItem, ItemCodeRecord } from "@/types"

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

// Package hub provides Item Code management API routes.
package hub

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"

	"aether/internal/hub/sqlguard"
)

const (
	itemCodesCollection         = "item_codes"
	itemCodeAuditLogsCollection = "item_code_audit_logs"
)

func (h *Hub) deleteItemCodeWithAudit(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}

	id := strings.TrimSpace(e.Request.URL.Query().Get("id"))
	if id == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}

	record, err := h.FindRecordById(itemCodesCollection, id)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "item code not found"})
	}
	code := record.GetString("code")

	if err := h.App.Delete(record); err != nil {
		if auditErr := h.recordItemCodeAudit(itemCodeAuditEntry{
			UserID:    e.Auth.Id,
			Action:    "single_delete",
			TargetIDs: id,
			Status:    itemCodeAuditStatusFailed,
			Detail:    err.Error(),
			IPAddress: e.RealIP(),
		}); auditErr != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
		}
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// sync to PostgreSQL
	if h.ingestMonitor != nil && code != "" {
		if dbErr := h.ingestMonitor.MarkItemCodeDeletedInDB(e.Request.Context(), code); dbErr != nil {
			h.Logger().Error("mark item code deleted in db failed", "logger", "hub", "code", code, "err", dbErr)
		}
	}

	if auditErr := h.recordItemCodeAudit(itemCodeAuditEntry{
		UserID:    e.Auth.Id,
		Action:    "single_delete",
		TargetIDs: id,
		Status:    itemCodeAuditStatusSuccess,
		IPAddress: e.RealIP(),
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}

	return e.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Hub) batchDeleteItemCodes(e *core.RequestEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	var payload struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if len(payload.IDs) == 0 {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "ids is required"})
	}

	deleted := 0
	failed := 0
	var failedIDs []string

	for _, id := range payload.IDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		record, err := h.FindRecordById(itemCodesCollection, id)
		if err != nil {
			failed++
			failedIDs = append(failedIDs, id)
			continue
		}
		code := record.GetString("code")
		if err := h.App.Delete(record); err != nil {
			failed++
			failedIDs = append(failedIDs, id)
			continue
		}
		if h.ingestMonitor != nil && code != "" {
			if dbErr := h.ingestMonitor.MarkItemCodeDeletedInDB(e.Request.Context(), code); dbErr != nil {
				h.Logger().Error("mark item code deleted in db failed", "logger", "hub", "code", code, "err", dbErr)
			}
		}
		deleted++
	}

	status := itemCodeAuditStatusSuccess
	detail := ""
	if failed > 0 {
		status = itemCodeAuditStatusFailed
		detail = "failed: " + strings.Join(failedIDs, ", ")
	}
	if auditErr := h.recordItemCodeAudit(itemCodeAuditEntry{
		UserID:    e.Auth.Id,
		Action:    "batch_delete",
		TargetIDs: strings.Join(payload.IDs, ","),
		Status:    status,
		Detail:    detail,
		IPAddress: e.RealIP(),
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}

	return e.JSON(http.StatusOK, map[string]any{
		"deleted": deleted,
		"failed":  failed,
	})
}

func (h *Hub) previewQueryDeleteItemCodes(e *core.RequestEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}
	if h.ingestMonitor == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-monitor not initialized"})
	}

	var payload struct {
		SQL string `json:"sql"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}

	// L1 + L2 validation
	whereClause, _, err := sqlguard.ValidateSQL(payload.SQL)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	previewSQL := sqlguard.BuildPreviewSelect(whereClause)

	// L3: EXPLAIN validation
	if err := sqlguard.ValidateViaExplain(e.Request.Context(), h.ingestMonitor.DB(), previewSQL); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var items []map[string]any
	err = h.ingestMonitor.withTenantTx(e.Request.Context(), func(tx *sql.Tx, cfg ingestMonitorConfig, queryCtx context.Context) error {
		rows, err := tx.QueryContext(queryCtx, previewSQL)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var code, name, category, description, status string
			var updated sql.NullTime
			if err := rows.Scan(&code, &name, &category, &description, &updated, &status); err != nil {
				return err
			}
			items = append(items, map[string]any{
				"code":     code,
				"name":     name,
				"category": category,
				"status":   status,
			})
		}
		return rows.Err()
	})
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]any{
		"count":       len(items),
		"items":       items,
		"executedSQL": previewSQL,
	})
}

func (h *Hub) queryDeleteItemCodes(e *core.RequestEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}
	if h.ingestMonitor == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-monitor not initialized"})
	}

	var payload struct {
		SQL      string `json:"sql"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}

	// Admin password re-verification (same as single/batch delete)
	password := strings.TrimSpace(payload.Password)
	if password == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "password is required"})
	}
	if !e.Auth.ValidatePassword(password) {
		return e.JSON(http.StatusForbidden, map[string]string{"error": "invalid password"})
	}

	// L1 + L2 validation
	whereClause, _, err := sqlguard.ValidateSQL(payload.SQL)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	execSQL := sqlguard.BuildDelete(whereClause)

	// L3: EXPLAIN validation
	if err := sqlguard.ValidateViaExplain(e.Request.Context(), h.ingestMonitor.DB(), execSQL); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var deletedCodes []string
	err = h.ingestMonitor.withTenantTxWrite(e.Request.Context(), func(tx *sql.Tx, cfg ingestMonitorConfig, queryCtx context.Context) error {
		rows, err := tx.QueryContext(queryCtx, execSQL)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var code string
			if err := rows.Scan(&code); err != nil {
				return err
			}
			deletedCodes = append(deletedCodes, code)
		}
		return rows.Err()
	})
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	deleted := len(deletedCodes)
	status := itemCodeAuditStatusSuccess
	if deleted == 0 {
		status = itemCodeAuditStatusFailed
	}

	if auditErr := h.recordItemCodeAudit(itemCodeAuditEntry{
		UserID:    e.Auth.Id,
		Action:    "query_delete",
		TargetIDs: strings.Join(deletedCodes, ","),
		Filter:    payload.SQL,
		Status:    status,
		Detail:    "",
		IPAddress: e.RealIP(),
	}); auditErr != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": auditErr.Error()})
	}

	return e.JSON(http.StatusOK, map[string]any{
		"deleted": deleted,
	})
}

func (h *Hub) listItemCodeAuditLogs(e *core.RequestEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}

	query := e.Request.URL.Query()
	action := strings.TrimSpace(query.Get("action"))
	userID := strings.TrimSpace(query.Get("userId"))
	startRaw := strings.TrimSpace(query.Get("start"))
	endRaw := strings.TrimSpace(query.Get("end"))
	pageRaw := strings.TrimSpace(query.Get("page"))
	perPageRaw := strings.TrimSpace(query.Get("perPage"))

	filters := make([]string, 0, 4)
	params := map[string]any{}

	if action != "" {
		switch action {
		case "single_delete":
			filters = append(filters, "(action = {:action} || action = {:action_db})")
			params["action"] = action
			params["action_db"] = "single_delete_db"
		case "batch_delete":
			filters = append(filters, "(action = {:action} || action = {:action_db})")
			params["action"] = action
			params["action_db"] = "batch_delete_db"
		default:
			filters = append(filters, "action = {:action}")
			params["action"] = action
		}
	}
	if userID != "" {
		filters = append(filters, "user = {:user}")
		params["user"] = userID
	}
	if startRaw != "" {
		parsed, err := types.ParseDateTime(startRaw)
		if err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid start time"})
		}
		filters = append(filters, "created >= {:start}")
		params["start"] = parsed
	}
	if endRaw != "" {
		parsed, err := types.ParseDateTime(endRaw)
		if err != nil {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid end time"})
		}
		filters = append(filters, "created <= {:end}")
		params["end"] = parsed
	}

	limit := -1
	offset := 0
	if pageRaw != "" || perPageRaw != "" {
		if pageRaw == "" || perPageRaw == "" {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "page and perPage are required"})
		}
		page, err := strconv.Atoi(pageRaw)
		if err != nil || page <= 0 {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "page must be a positive integer"})
		}
		perPage, err := strconv.Atoi(perPageRaw)
		if err != nil || perPage <= 0 {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "perPage must be a positive integer"})
		}
		limit = perPage
		offset = (page - 1) * perPage
	}

	filter := strings.Join(filters, " && ")
	records, err := h.FindRecordsByFilter(itemCodeAuditLogsCollection, filter, "-created", limit, offset, params)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	items := make([]map[string]any, 0, len(records))
	for _, record := range records {
		items = append(items, map[string]any{
			"id":         record.Id,
			"user":       record.GetString("user"),
			"action":     record.GetString("action"),
			"target_ids": record.GetString("target_ids"),
			"filter":     record.GetString("filter"),
			"status":     record.GetString("status"),
			"detail":     record.GetString("detail"),
			"ip_address": record.GetString("ip_address"),
			"created":    record.Get("created"),
		})
	}

	return e.JSON(http.StatusOK, map[string]any{"items": items})
}

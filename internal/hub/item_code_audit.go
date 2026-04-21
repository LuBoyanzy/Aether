// Package hub records Item Code operation audit information.
package hub

import (
	"errors"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

const (
	itemCodeAuditStatusSuccess = "success"
	itemCodeAuditStatusFailed  = "failed"
)

type itemCodeAuditEntry struct {
	UserID     string
	Action     string
	TargetIDs  string
	Filter     string
	Status     string
	Detail     string
	IPAddress  string
}

func (h *Hub) recordItemCodeAudit(entry itemCodeAuditEntry) error {
	if h == nil {
		return errors.New("hub is nil")
	}
	if strings.TrimSpace(entry.UserID) == "" {
		return errors.New("audit requires user id")
	}
	collection, err := h.FindCollectionByNameOrId("item_code_audit_logs")
	if err != nil {
		return err
	}
	record := core.NewRecord(collection)
	record.Set("user", entry.UserID)
	record.Set("action", entry.Action)
	record.Set("target_ids", entry.TargetIDs)
	record.Set("filter", entry.Filter)
	record.Set("status", entry.Status)
	record.Set("detail", entry.Detail)
	record.Set("ip_address", entry.IPAddress)
	return h.Save(record)
}

func requireAdmin(e *core.RequestEvent) error {
	if e.Auth == nil {
		return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}
	// PocketBase superuser has all permissions
	if e.Auth.IsSuperuser() {
		return nil
	}
	if e.Auth.GetString("role") == "admin" {
		return nil
	}
	return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
}

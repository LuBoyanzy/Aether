// Package hub 记录 Docker 操作审计信息。
// 审计日志用于追踪写操作与关键状态变更。
package hub

import (
	"errors"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

const (
	dockerAuditStatusSuccess = "success"
	dockerAuditStatusFailed  = "failed"
)

type dockerAuditEntry struct {
	SystemID     string
	UserID       string
	Action       string
	ResourceType string
	ResourceID   string
	Status       string
	Detail       string
}

func (h *Hub) recordDockerAudit(entry dockerAuditEntry) error {
	if h == nil {
		return errors.New("hub is nil")
	}
	if strings.TrimSpace(entry.UserID) == "" {
		return errors.New("audit requires user id")
	}
	systemID := strings.TrimSpace(entry.SystemID)
	if systemID == "" && entry.ResourceType != "registry" && entry.ResourceType != "compose_template" {
		return errors.New("audit requires system id")
	}
	collection, err := h.FindCollectionByNameOrId("docker_audits")
	if err != nil {
		return err
	}
	record := core.NewRecord(collection)
	if systemID != "" {
		record.Set("system", systemID)
	}
	record.Set("user", entry.UserID)
	record.Set("action", entry.Action)
	record.Set("resource_type", entry.ResourceType)
	record.Set("resource_id", entry.ResourceID)
	record.Set("status", entry.Status)
	record.Set("detail", entry.Detail)
	return h.Save(record)
}

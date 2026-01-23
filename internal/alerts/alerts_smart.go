package alerts

import (
	"fmt"
	"runtime/debug"

	"github.com/pocketbase/pocketbase/core"
)

// handleSmartDeviceAlert sends alerts when a SMART device state changes from PASSED to FAILED.
// This is automatic and does not require user opt-in.
func (am *AlertManager) handleSmartDeviceAlert(e *core.RecordEvent) error {
	oldState := e.Record.Original().GetString("state")
	newState := e.Record.GetString("state")

	// Only alert when transitioning from PASSED to FAILED
	if oldState != "PASSED" || newState != "FAILED" {
		return e.Next()
	}

	systemID := e.Record.GetString("system")
	if systemID == "" {
		return e.Next()
	}

	// Fetch the system record to get the name and users
	systemRecord, err := e.App.FindRecordById("systems", systemID)
	if err != nil {
		e.App.Logger().Error("Failed to find system for SMART alert", "logger", "alerts", "err", err, "systemID", systemID)
		return e.Next()
	}

	systemName := systemRecord.GetString("name")
	deviceName := e.Record.GetString("name")
	model := e.Record.GetString("model")

	lang, err := am.NotificationLanguage()
	if err != nil {
		e.App.Logger().Error("读取通知语言失败", "logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack()), "system", systemName, "device", deviceName)
		return e.Next()
	}
	descriptor := deviceName
	if model != "" {
		descriptor = fmt.Sprintf("%s %s", deviceName, model)
	}
	content := NotificationContent{
		SystemName:   systemName,
		AlertType:    "SMART",
		Descriptor:   descriptor,
		State:        NotificationStateTriggered,
		CurrentValue: "FAILED",
		Threshold:    "PASSED",
		Duration:     FormatImmediateDuration(lang),
	}
	text, err := FormatNotification(lang, content)
	if err != nil {
		e.App.Logger().Error("SMART 告警格式化失败", "logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack()), "system", systemName, "device", deviceName)
		return e.Next()
	}

	// Get users associated with the system
	userIDs := systemRecord.GetStringSlice("users")
	if len(userIDs) == 0 {
		return e.Next()
	}

	// Send alert to each user
	for _, userID := range userIDs {
		if err := am.SendAlert(AlertMessageData{
			UserID:   userID,
			SystemID: systemID,
			Title:    text.Title,
			Message:  text.Message,
			Link:     am.hub.MakeLink("system", systemID),
			LinkText: text.LinkText,
		}); err != nil {
			e.App.Logger().Error("发送 SMART 告警失败", "logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack()), "userID", userID, "system", systemName)
		}
	}

	return e.Next()
}

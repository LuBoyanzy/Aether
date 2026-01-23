package alerts

import (
	"fmt"
	"runtime/debug"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

type alertTask struct {
	action      string // "schedule" or "cancel"
	systemName  string
	alertRecord *core.Record
	delay       time.Duration
}

type alertInfo struct {
	systemName  string
	alertRecord *core.Record
	expireTime  time.Time
}

// startWorker is a long-running goroutine that processes alert tasks
// every x seconds. It must be running to process status alerts.
func (am *AlertManager) startWorker() {
	processPendingAlerts := time.Tick(15 * time.Second)

	// check for status alerts that are not resolved when system comes up
	// (can be removed if we figure out core bug in #1052)
	checkStatusAlerts := time.Tick(561 * time.Second)

	for {
		select {
		case <-am.stopChan:
			return
		case task := <-am.alertQueue:
			switch task.action {
			case "schedule":
				am.pendingAlerts.Store(task.alertRecord.Id, &alertInfo{
					systemName:  task.systemName,
					alertRecord: task.alertRecord,
					expireTime:  time.Now().Add(task.delay),
				})
			case "cancel":
				am.pendingAlerts.Delete(task.alertRecord.Id)
			}
		case <-checkStatusAlerts:
			resolveStatusAlerts(am.hub)
		case <-processPendingAlerts:
			// Check for expired alerts every tick
			now := time.Now()
			for key, value := range am.pendingAlerts.Range {
				info := value.(*alertInfo)
				if now.After(info.expireTime) {
					// Downtime delay has passed, process alert
					am.sendStatusAlert("down", info.systemName, info.alertRecord)
					am.pendingAlerts.Delete(key)
				}
			}
		}
	}
}

// StopWorker shuts down the AlertManager.worker goroutine
func (am *AlertManager) StopWorker() {
	close(am.stopChan)
}

// HandleStatusAlerts manages the logic when system status changes.
func (am *AlertManager) HandleStatusAlerts(newStatus string, systemRecord *core.Record) error {
	if newStatus != "up" && newStatus != "down" {
		return nil
	}

	alertRecords, err := am.getSystemStatusAlerts(systemRecord.Id)
	if err != nil {
		return err
	}
	if len(alertRecords) == 0 {
		return nil
	}

	systemName := systemRecord.GetString("name")
	if newStatus == "down" {
		am.handleSystemDown(systemName, alertRecords)
	} else {
		am.handleSystemUp(systemName, alertRecords)
	}
	return nil
}

// getSystemStatusAlerts retrieves all "Status" alert records for a given system ID.
func (am *AlertManager) getSystemStatusAlerts(systemID string) ([]*core.Record, error) {
	alertRecords, err := am.hub.FindAllRecords("alerts", dbx.HashExp{
		"system": systemID,
		"name":   "Status",
	})
	if err != nil {
		return nil, err
	}
	return alertRecords, nil
}

// Schedules delayed "down" alerts for each alert record.
func (am *AlertManager) handleSystemDown(systemName string, alertRecords []*core.Record) {
	for _, alertRecord := range alertRecords {
		// Continue if alert is already scheduled
		if _, exists := am.pendingAlerts.Load(alertRecord.Id); exists {
			continue
		}
		// Schedule by adding to queue
		min := alertRecord.GetInt("min")
		if min < 1 {
			min = 1
		}
		am.alertQueue <- alertTask{
			action:      "schedule",
			systemName:  systemName,
			alertRecord: alertRecord,
			delay:       time.Duration(min) * time.Minute,
		}
	}
}

// handleSystemUp manages the logic when a system status changes to "up".
// It cancels any pending alerts and sends "up" alerts.
func (am *AlertManager) handleSystemUp(systemName string, alertRecords []*core.Record) {
	for _, alertRecord := range alertRecords {
		alertRecordID := alertRecord.Id
		// If alert exists for record, delete and continue (down alert not sent)
		if _, exists := am.pendingAlerts.Load(alertRecordID); exists {
			am.alertQueue <- alertTask{
				action:      "cancel",
				alertRecord: alertRecord,
			}
			continue
		}
		// No alert scheduled for this record, send "up" alert
		if err := am.sendStatusAlert("up", systemName, alertRecord); err != nil {
			am.hub.Logger().Error("Failed to send alert", "logger", "alerts", "err", err)
		}
	}
}

// sendStatusAlert sends a status alert ("up" or "down") to the users associated with the alert records.
func (am *AlertManager) sendStatusAlert(alertStatus string, systemName string, alertRecord *core.Record) error {
	switch alertStatus {
	case "up":
		alertRecord.Set("triggered", false)
	case "down":
		alertRecord.Set("triggered", true)
	}
	if err := am.hub.Save(alertRecord); err != nil {
		am.hub.Logger().Error("保存状态告警记录失败", "logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack()), "alertStatus", alertStatus, "system", systemName, "alertId", alertRecord.Id)
		return fmt.Errorf("保存状态告警记录失败: %w", err)
	}

	lang, err := am.NotificationLanguage()
	if err != nil {
		am.hub.Logger().Error("读取通知语言失败", "logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack()), "alertStatus", alertStatus, "system", systemName)
		return fmt.Errorf("读取通知语言失败: %w", err)
	}

	state := NotificationStateResolved
	if alertStatus == "down" {
		state = NotificationStateTriggered
	}
	alertType := "Status"
	details := ""
	if displayTitle, displayDetails, ok := AlertDisplayText("Status", lang); ok {
		alertType = displayTitle
		details = displayDetails
	}
	durationMinutes := alertRecord.GetInt("min")
	if durationMinutes < 1 {
		durationMinutes = 1
	}
	duration := FormatDurationMinutes(durationMinutes, lang)
	content := NotificationContent{
		SystemName:   systemName,
		AlertType:    alertType,
		State:        state,
		CurrentValue: alertStatus,
		Threshold:    "up",
		Duration:     duration,
		Details:      details,
	}
	text, err := FormatNotification(lang, content)
	if err != nil {
		am.hub.Logger().Error("状态告警格式化失败", "logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack()), "alertStatus", alertStatus, "system", systemName, "durationMinutes", alertRecord.GetInt("min"))
		return fmt.Errorf("格式化状态告警失败: %w", err)
	}

	// Get system ID for the link
	systemID := alertRecord.GetString("system")

	return am.SendAlert(AlertMessageData{
		UserID:   alertRecord.GetString("user"),
		SystemID: systemID,
		Title:    text.Title,
		Message:  text.Message,
		Link:     am.hub.MakeLink("system", systemID),
		LinkText: text.LinkText,
	})
}

// resolveStatusAlerts resolves any status alerts that weren't resolved
// when system came up (https://aether/issues/1052)
func resolveStatusAlerts(app core.App) error {
	db := app.DB()
	// Find all active status alerts where the system is actually up
	var alertIds []string
	err := db.NewQuery(`
		SELECT a.id 
		FROM alerts a
		JOIN systems s ON a.system = s.id
		WHERE a.name = 'Status' 
		AND a.triggered = true
		AND s.status = 'up'
	`).Column(&alertIds)
	if err != nil {
		return err
	}
	// resolve all matching alert records
	for _, alertId := range alertIds {
		alert, err := app.FindRecordById("alerts", alertId)
		if err != nil {
			return err
		}
		alert.Set("triggered", false)
		err = app.Save(alert)
		if err != nil {
			return err
		}
	}
	return nil
}

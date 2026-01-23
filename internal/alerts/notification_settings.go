// 通知设置读取与默认值处理，保证告警发送可获得语言配置。
package alerts

import (
	"database/sql"
	"errors"
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const notificationSettingsCollection = "notification_settings"

func GetNotificationLanguage(app core.App) (NotificationLanguage, error) {
	record, err := GetOrCreateNotificationSettings(app)
	if err != nil {
		return "", err
	}
	languageRaw := strings.TrimSpace(record.GetString("language"))
	if languageRaw == "" {
		return "", fmt.Errorf("通知语言缺失，记录ID: %s", record.Id)
	}
	lang, err := ParseNotificationLanguage(languageRaw)
	if err != nil {
		return "", err
	}
	return lang, nil
}

func GetOrCreateNotificationSettings(app core.App) (*core.Record, error) {
	record, err := app.FindFirstRecordByFilter(notificationSettingsCollection, "", dbx.Params{})
	if err == nil {
		return record, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		collection, findErr := app.FindCollectionByNameOrId(notificationSettingsCollection)
		if findErr != nil {
			logNotificationSettingsError(app, "通知设置集合查询失败", findErr, map[string]any{"collection": notificationSettingsCollection})
			return nil, fmt.Errorf("通知设置集合查询失败: %w", findErr)
		}
		record = core.NewRecord(collection)
		record.Set("language", DefaultNotificationLanguage())
		if saveErr := app.Save(record); saveErr != nil {
			logNotificationSettingsError(app, "通知设置默认记录创建失败", saveErr, map[string]any{"language": DefaultNotificationLanguage()})
			return nil, fmt.Errorf("通知设置默认记录创建失败: %w", saveErr)
		}
		return record, nil
	}
	logNotificationSettingsError(app, "通知设置查询失败", err, map[string]any{"collection": notificationSettingsCollection})
	return nil, fmt.Errorf("通知设置查询失败: %w", err)
}

func (am *AlertManager) NotificationLanguage() (NotificationLanguage, error) {
	return GetNotificationLanguage(am.hub)
}

func logNotificationSettingsError(app core.App, message string, err error, fields map[string]any) {
	if app == nil || err == nil {
		return
	}
	logFields := []any{"logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack())}
	for key, value := range fields {
		logFields = append(logFields, key, value)
	}
	app.Logger().Error(message, logFields...)
}

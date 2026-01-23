// 通知语言设置接口：提供全局通知语言读取与更新。
package hub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"

	"aether/internal/alerts"

	"github.com/pocketbase/pocketbase/core"
)

type notificationSettingsResponse struct {
	Language string `json:"language"`
}

type notificationSettingsUpdateRequest struct {
	Language string `json:"language"`
}

func (h *Hub) GetNotificationSettings(e *core.RequestEvent) error {
	record, err := alerts.GetOrCreateNotificationSettings(h)
	if err != nil {
		h.logNotificationSettingsError("读取通知设置失败", err, map[string]any{"action": "get"})
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("读取通知设置失败: %v", err)})
	}
	return e.JSON(http.StatusOK, notificationSettingsResponse{Language: record.GetString("language")})
}

func (h *Hub) UpdateNotificationSettings(e *core.RequestEvent) error {
	var payload notificationSettingsUpdateRequest
	if err := decodeNotificationSettingsBody(e, &payload); err != nil {
		h.logNotificationSettingsError("解析通知设置失败", err, map[string]any{"action": "update"})
		return e.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("解析通知设置失败: %v", err)})
	}
	languageRaw := strings.TrimSpace(payload.Language)
	if languageRaw == "" {
		err := errors.New("language is required")
		h.logNotificationSettingsError("通知语言缺失", err, map[string]any{"action": "update"})
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "通知语言不能为空"})
	}
	language, err := alerts.ParseNotificationLanguage(languageRaw)
	if err != nil {
		h.logNotificationSettingsError("通知语言非法", err, map[string]any{"action": "update", "language": languageRaw})
		return e.JSON(http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("通知语言非法: %s", languageRaw)})
	}
	record, err := alerts.GetOrCreateNotificationSettings(h)
	if err != nil {
		h.logNotificationSettingsError("读取通知设置失败", err, map[string]any{"action": "update", "language": language})
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("读取通知设置失败: %v", err)})
	}
	record.Set("language", string(language))
	if err := h.Save(record); err != nil {
		h.logNotificationSettingsError("保存通知设置失败", err, map[string]any{"action": "update", "language": language})
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("保存通知设置失败: %v", err)})
	}
	h.Logger().Info("通知语言设置已更新", "logger", "hub", "language", language, "user", e.Auth.Id)
	return e.JSON(http.StatusOK, notificationSettingsResponse{Language: string(language)})
}

func decodeNotificationSettingsBody(e *core.RequestEvent, target any) error {
	decoder := json.NewDecoder(e.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("invalid json payload")
	}
	return nil
}

func (h *Hub) logNotificationSettingsError(message string, err error, fields map[string]any) {
	if err == nil {
		return
	}
	payload := []any{
		"logger", "hub",
		"err", err,
		"errType", fmt.Sprintf("%T", err),
		"stack", string(debug.Stack()),
	}
	for key, value := range fields {
		payload = append(payload, key, value)
	}
	h.Logger().Error(message, payload...)
}

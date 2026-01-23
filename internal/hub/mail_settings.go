// Package hub 提供通知页 SMTP 设置接口。
// 该文件负责读取、更新 SMTP 配置并支持发送测试邮件。
package hub

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"runtime/debug"
	"strings"

	"aether/internal/alerts"

	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mailer"
)

type mailSettingsMeta struct {
	SenderName    string `json:"senderName"`
	SenderAddress string `json:"senderAddress"`
}

type mailSettingsSMTPResponse struct {
	Enabled     bool   `json:"enabled"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	AuthMethod  string `json:"authMethod"`
	TLS         bool   `json:"tls"`
	LocalName   string `json:"localName"`
	PasswordSet bool   `json:"passwordSet"`
}

type mailSettingsSMTPUpdate struct {
	Enabled    bool   `json:"enabled"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	AuthMethod string `json:"authMethod"`
	TLS        bool   `json:"tls"`
	LocalName  string `json:"localName"`
}

type mailSettingsResponse struct {
	Meta mailSettingsMeta         `json:"meta"`
	SMTP mailSettingsSMTPResponse `json:"smtp"`
}

type mailSettingsUpdateRequest struct {
	Meta mailSettingsMeta       `json:"meta"`
	SMTP mailSettingsSMTPUpdate `json:"smtp"`
}

type mailSettingsTestRequest struct {
	Email string `json:"email"`
}

func (h *Hub) GetMailSettings(e *core.RequestEvent) error {
	settings, err := e.App.Settings().Clone()
	if err != nil {
		return h.respondMailSettingsError(e, http.StatusInternalServerError, "failed to load smtp settings", err, map[string]any{
			"action": "get",
		})
	}

	return e.JSON(http.StatusOK, mailSettingsResponse{
		Meta: mailSettingsMeta{
			SenderName:    settings.Meta.SenderName,
			SenderAddress: settings.Meta.SenderAddress,
		},
		SMTP: mailSettingsSMTPResponse{
			Enabled:     settings.SMTP.Enabled,
			Host:        settings.SMTP.Host,
			Port:        settings.SMTP.Port,
			Username:    settings.SMTP.Username,
			AuthMethod:  settings.SMTP.AuthMethod,
			TLS:         settings.SMTP.TLS,
			LocalName:   settings.SMTP.LocalName,
			PasswordSet: settings.SMTP.Password != "",
		},
	})
}

func (h *Hub) UpdateMailSettings(e *core.RequestEvent) error {
	if err := h.requireMailSettingsAdmin(e, "update"); err != nil {
		return err
	}

	var payload mailSettingsUpdateRequest
	if err := decodeMailSettingsBody(e, &payload); err != nil {
		return h.respondMailSettingsError(e, http.StatusBadRequest, "failed to parse smtp settings request", err, map[string]any{
			"action": "update",
		})
	}

	if strings.TrimSpace(payload.Meta.SenderName) == "" {
		return h.respondMailSettingsError(e, http.StatusBadRequest, "sender name is required", errors.New("senderName is required"), map[string]any{
			"action":      "update",
			"senderName":  payload.Meta.SenderName,
			"senderEmail": payload.Meta.SenderAddress,
		})
	}
	if strings.TrimSpace(payload.Meta.SenderAddress) == "" {
		return h.respondMailSettingsError(e, http.StatusBadRequest, "sender address is required", errors.New("senderAddress is required"), map[string]any{
			"action":      "update",
			"senderName":  payload.Meta.SenderName,
			"senderEmail": payload.Meta.SenderAddress,
		})
	}
	if payload.SMTP.Enabled {
		if strings.TrimSpace(payload.SMTP.Host) == "" {
			return h.respondMailSettingsError(e, http.StatusBadRequest, "smtp host is required", errors.New("host is required"), map[string]any{
				"action":       "update",
				"senderName":   payload.Meta.SenderName,
				"senderEmail":  payload.Meta.SenderAddress,
				"smtpEnabled":  payload.SMTP.Enabled,
				"smtpHost":     payload.SMTP.Host,
				"smtpPort":     payload.SMTP.Port,
				"smtpUsername": payload.SMTP.Username,
			})
		}
		if payload.SMTP.Port <= 0 {
			return h.respondMailSettingsError(e, http.StatusBadRequest, "smtp port must be a positive integer", errors.New("invalid port"), map[string]any{
				"action":       "update",
				"senderName":   payload.Meta.SenderName,
				"senderEmail":  payload.Meta.SenderAddress,
				"smtpEnabled":  payload.SMTP.Enabled,
				"smtpHost":     payload.SMTP.Host,
				"smtpPort":     payload.SMTP.Port,
				"smtpUsername": payload.SMTP.Username,
			})
		}
	}

	if payload.SMTP.AuthMethod != "" && payload.SMTP.AuthMethod != mailer.SMTPAuthPlain && payload.SMTP.AuthMethod != mailer.SMTPAuthLogin {
		return h.respondMailSettingsError(e, http.StatusBadRequest, "invalid smtp auth method", errors.New("invalid authMethod"), map[string]any{
			"action":      "update",
			"authMethod":  payload.SMTP.AuthMethod,
			"smtpEnabled": payload.SMTP.Enabled,
		})
	}

	settings, err := e.App.Settings().Clone()
	if err != nil {
		return h.respondMailSettingsError(e, http.StatusInternalServerError, "failed to load smtp settings", err, map[string]any{
			"action": "update",
		})
	}

	existingPassword := settings.SMTP.Password
	settings.Meta.SenderName = payload.Meta.SenderName
	settings.Meta.SenderAddress = payload.Meta.SenderAddress
	settings.SMTP.Enabled = payload.SMTP.Enabled
	settings.SMTP.Host = payload.SMTP.Host
	settings.SMTP.Port = payload.SMTP.Port
	settings.SMTP.Username = payload.SMTP.Username
	settings.SMTP.AuthMethod = payload.SMTP.AuthMethod
	settings.SMTP.TLS = payload.SMTP.TLS
	settings.SMTP.LocalName = payload.SMTP.LocalName
	if payload.SMTP.Password != "" {
		settings.SMTP.Password = payload.SMTP.Password
	} else {
		settings.SMTP.Password = existingPassword
	}

	if err := e.App.Save(settings); err != nil {
		status := http.StatusInternalServerError
		if isValidationError(err) {
			status = http.StatusBadRequest
		}
		return h.respondMailSettingsError(e, status, "failed to save smtp settings", err, map[string]any{
			"action":           "update",
			"senderName":       payload.Meta.SenderName,
			"senderEmail":      payload.Meta.SenderAddress,
			"smtpEnabled":      payload.SMTP.Enabled,
			"smtpHost":         payload.SMTP.Host,
			"smtpPort":         payload.SMTP.Port,
			"smtpUsername":     payload.SMTP.Username,
			"smtpAuthMethod":   payload.SMTP.AuthMethod,
			"smtpTLS":          payload.SMTP.TLS,
			"smtpLocalName":    payload.SMTP.LocalName,
			"passwordProvided": payload.SMTP.Password != "",
		})
	}

	h.Logger().Info("SMTP settings updated", "logger", "hub", "user", e.Auth.Id)
	return e.JSON(http.StatusOK, mailSettingsResponse{
		Meta: mailSettingsMeta{
			SenderName:    settings.Meta.SenderName,
			SenderAddress: settings.Meta.SenderAddress,
		},
		SMTP: mailSettingsSMTPResponse{
			Enabled:     settings.SMTP.Enabled,
			Host:        settings.SMTP.Host,
			Port:        settings.SMTP.Port,
			Username:    settings.SMTP.Username,
			AuthMethod:  settings.SMTP.AuthMethod,
			TLS:         settings.SMTP.TLS,
			LocalName:   settings.SMTP.LocalName,
			PasswordSet: settings.SMTP.Password != "",
		},
	})
}

func (h *Hub) TestMailSettings(e *core.RequestEvent) error {
	if err := h.requireMailSettingsAdmin(e, "test"); err != nil {
		return err
	}

	var payload mailSettingsTestRequest
	if err := decodeMailSettingsBody(e, &payload); err != nil {
		return h.respondMailSettingsError(e, http.StatusBadRequest, "failed to parse test email request", err, map[string]any{
			"action": "test",
		})
	}

	if strings.TrimSpace(payload.Email) == "" {
		return h.respondMailSettingsError(e, http.StatusBadRequest, "test email is required", errors.New("email is required"), map[string]any{
			"action": "test",
			"email":  payload.Email,
		})
	}
	settings, err := e.App.Settings().Clone()
	if err != nil {
		return h.respondMailSettingsError(e, http.StatusInternalServerError, "failed to load smtp settings", err, map[string]any{
			"action": "test",
			"email":  payload.Email,
		})
	}
	lang, err := alerts.GetNotificationLanguage(h)
	if err != nil {
		return h.respondMailSettingsError(e, http.StatusInternalServerError, "读取通知语言失败", err, map[string]any{
			"action": "test",
			"email":  payload.Email,
		})
	}
	appName := strings.TrimSpace(settings.Meta.AppName)
	if appName == "" {
		appName = "Aether"
	}
	alertType := "Test Notification"
	currentValue := "Test"
	threshold := "N/A"
	if lang == alerts.NotificationLanguageZhCN {
		alertType = "测试通知"
		currentValue = "测试"
		threshold = "不适用"
	}
	content := alerts.NotificationContent{
		SystemName:   appName,
		AlertType:    alertType,
		State:        alerts.NotificationStateTriggered,
		CurrentValue: currentValue,
		Threshold:    threshold,
		Duration:     alerts.FormatImmediateDuration(lang),
	}
	text, err := alerts.FormatNotification(lang, content)
	if err != nil {
		return h.respondMailSettingsError(e, http.StatusInternalServerError, "failed to format test email", err, map[string]any{
			"action": "test",
			"email":  payload.Email,
		})
	}
	message := mailer.Message{
		To: []mail.Address{{Address: payload.Email}},
		From: mail.Address{
			Address: settings.Meta.SenderAddress,
			Name:    settings.Meta.SenderName,
		},
		Subject: text.Title,
		Text:    text.Message + fmt.Sprintf("\n\n%s", settings.Meta.AppURL),
	}
	if err := h.NewMailClient().Send(&message); err != nil {
		status := http.StatusInternalServerError
		if isValidationError(err) {
			status = http.StatusBadRequest
		}
		return h.respondMailSettingsError(e, status, "failed to send test email", err, map[string]any{
			"action": "test",
			"email":  payload.Email,
		})
	}

	h.Logger().Info("SMTP test email sent", "logger", "hub", "user", e.Auth.Id)
	return e.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func decodeMailSettingsBody(e *core.RequestEvent, target any) error {
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

func (h *Hub) requireMailSettingsAdmin(e *core.RequestEvent, action string) error {
	role := ""
	userID := ""
	if e.Auth != nil {
		role = e.Auth.GetString("role")
		userID = e.Auth.Id
	}
	if role == "admin" {
		return nil
	}
	return h.respondMailSettingsError(e, http.StatusForbidden, "admin role required", errors.New("requires admin role"), map[string]any{
		"action": action,
		"role":   role,
		"user":   userID,
	})
}

func (h *Hub) respondMailSettingsError(e *core.RequestEvent, status int, context string, err error, fields map[string]any) error {
	if err == nil {
		err = errors.New("unknown error")
	}
	if fields == nil {
		fields = map[string]any{}
	}
	if _, ok := fields["path"]; !ok && e.Request != nil && e.Request.URL != nil {
		fields["path"] = e.Request.URL.Path
	}
	if _, ok := fields["method"]; !ok && e.Request != nil {
		fields["method"] = e.Request.Method
	}
	if e.Auth != nil {
		if _, ok := fields["user"]; !ok {
			fields["user"] = e.Auth.Id
		}
		if _, ok := fields["role"]; !ok {
			fields["role"] = e.Auth.GetString("role")
		}
	}
	formatted := formatMailSettingsError(context, err, fields)
	h.logMailSettingsError(context, formatted, "status", status)
	return e.JSON(status, map[string]string{"error": formatted.Error()})
}

func (h *Hub) logMailSettingsError(message string, err error, fields ...any) {
	if err == nil {
		return
	}
	payload := []any{
		"logger", "hub",
		"err", err,
		"errType", fmt.Sprintf("%T", err),
		"stack", string(debug.Stack()),
	}
	payload = append(payload, fields...)
	h.Logger().Error(message, payload...)
}

func formatMailSettingsError(context string, err error, fields map[string]any) error {
	return fmt.Errorf("%s | errType=%T | err=%v | fields=%v | stack=%s", context, err, err, fields, string(debug.Stack()))
}

func isValidationError(err error) bool {
	var validationErr validation.Errors
	return errors.As(err, &validationErr)
}

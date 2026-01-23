// Package alerts handles alert management and delivery.
package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/mail"
	"net/url"
	"runtime/debug"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nicholas-fedor/shoutrrr"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/mailer"
)

type hubLike interface {
	core.App
	MakeLink(parts ...string) string
}

type AlertManager struct {
	hub           hubLike
	alertQueue    chan alertTask
	stopChan      chan struct{}
	pendingAlerts sync.Map
}

type AlertMessageData struct {
	UserID   string
	SystemID string
	Title    string
	Message  string
	Link     string
	LinkText string
}

type WebhookConfig struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type WebhookList []WebhookConfig

type UserNotificationSettings struct {
	Emails   []string    `json:"emails"`
	Webhooks WebhookList `json:"webhooks"`
}

func (w *WebhookList) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		*w = WebhookList{}
		return nil
	}
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	parsed := make([]WebhookConfig, 0, len(raw))
	for index, item := range raw {
		var url string
		if err := json.Unmarshal(item, &url); err == nil {
			if strings.TrimSpace(url) == "" {
				return fmt.Errorf("webhook url is required at index %d", index)
			}
			parsed = append(parsed, WebhookConfig{URL: url})
			continue
		}
		var entry WebhookConfig
		if err := json.Unmarshal(item, &entry); err != nil {
			return err
		}
		if strings.TrimSpace(entry.URL) == "" {
			return fmt.Errorf("webhook url is required at index %d", index)
		}
		parsed = append(parsed, entry)
	}
	*w = parsed
	return nil
}

type SystemAlertStats struct {
	Cpu          float64                       `json:"cpu"`
	Mem          float64                       `json:"mp"`
	Disk         float64                       `json:"dp"`
	NetSent      float64                       `json:"ns"`
	NetRecv      float64                       `json:"nr"`
	GPU          map[string]SystemAlertGPUData `json:"g"`
	Temperatures map[string]float32            `json:"t"`
	LoadAvg      [3]float64                    `json:"la"`
	Battery      [2]uint8                      `json:"bat"`
}

type SystemAlertGPUData struct {
	Usage float64 `json:"u"`
}

type SystemAlertData struct {
	systemRecord *core.Record
	alertRecord  *core.Record
	name         string
	unit         string
	val          float64
	threshold    float64
	triggered    bool
	time         time.Time
	count        uint8
	min          uint8
	mapSums      map[string]float32
	descriptor   string // override descriptor in notification body (for temp sensor, disk partition, etc)
}

const (
	weLinkHost = "open.welink.huaweicloud.com"
	weLinkPath = "/api/werobot/v1/webhook/send"
)

type weLinkWebhookContent struct {
	Text string `json:"text"`
}

type weLinkWebhookPayload struct {
	MessageType string               `json:"messageType"`
	Content     weLinkWebhookContent `json:"content"`
	TimeStamp   int64                `json:"timeStamp"`
	UUID        string               `json:"uuid"`
	IsAt        bool                 `json:"isAt"`
	IsAtAll     bool                 `json:"isAtAll"`
	AtAccounts  []string             `json:"atAccounts"`
}

type weLinkWebhookResponse struct {
	Code    string `json:"code"`
	Data    string `json:"data"`
	Message string `json:"message"`
}

// notification services that support title param
var supportsTitle = map[string]struct{}{
	"bark":       {},
	"discord":    {},
	"gotify":     {},
	"ifttt":      {},
	"join":       {},
	"lark":       {},
	"ntfy":       {},
	"opsgenie":   {},
	"pushbullet": {},
	"pushover":   {},
	"slack":      {},
	"teams":      {},
	"telegram":   {},
	"zulip":      {},
}

// NewAlertManager creates a new AlertManager instance.
func NewAlertManager(app hubLike) *AlertManager {
	am := &AlertManager{
		hub:        app,
		alertQueue: make(chan alertTask, 5),
		stopChan:   make(chan struct{}),
	}
	am.bindEvents()
	go am.startWorker()
	return am
}

// Bind events to the alerts collection lifecycle
func (am *AlertManager) bindEvents() {
	am.hub.OnRecordAfterUpdateSuccess("alerts").BindFunc(updateHistoryOnAlertUpdate)
	am.hub.OnRecordAfterDeleteSuccess("alerts").BindFunc(resolveHistoryOnAlertDelete)
	am.hub.OnRecordAfterUpdateSuccess("smart_devices").BindFunc(am.handleSmartDeviceAlert)
}

// IsNotificationSilenced checks if a notification should be silenced based on configured quiet hours
func (am *AlertManager) IsNotificationSilenced(userID, systemID string) bool {
	// Query for quiet hours windows that match this user and system
	// Include both global windows (system is null/empty) and system-specific windows
	var filter string
	var params dbx.Params

	if systemID == "" {
		// If no systemID provided, only check global windows
		filter = "user={:user} AND system=''"
		params = dbx.Params{"user": userID}
	} else {
		// Check both global and system-specific windows
		filter = "user={:user} AND (system='' OR system={:system})"
		params = dbx.Params{
			"user":   userID,
			"system": systemID,
		}
	}

	quietHourWindows, err := am.hub.FindAllRecords("quiet_hours", dbx.NewExp(filter, params))
	if err != nil || len(quietHourWindows) == 0 {
		return false
	}

	now := time.Now().UTC()

	for _, window := range quietHourWindows {
		windowType := window.GetString("type")
		start := window.GetDateTime("start").Time()
		end := window.GetDateTime("end").Time()

		if windowType == "daily" {
			// For daily recurring windows, extract just the time portion and compare
			// The start/end are stored as full datetime but we only care about HH:MM
			startHour, startMin, _ := start.Clock()
			endHour, endMin, _ := end.Clock()
			nowHour, nowMin, _ := now.Clock()

			// Convert to minutes since midnight for easier comparison
			startMinutes := startHour*60 + startMin
			endMinutes := endHour*60 + endMin
			nowMinutes := nowHour*60 + nowMin

			// Handle case where window crosses midnight
			if endMinutes < startMinutes {
				// Window crosses midnight (e.g., 23:00 - 01:00)
				if nowMinutes >= startMinutes || nowMinutes < endMinutes {
					return true
				}
			} else {
				// Normal case (e.g., 09:00 - 17:00)
				if nowMinutes >= startMinutes && nowMinutes < endMinutes {
					return true
				}
			}
		} else {
			// One-time window: check if current time is within the date range
			if (now.After(start) || now.Equal(start)) && now.Before(end) {
				return true
			}
		}
	}

	return false
}

// SendAlert sends an alert to the user
func (am *AlertManager) SendAlert(data AlertMessageData) error {
	// Check if alert is silenced
	if am.IsNotificationSilenced(data.UserID, data.SystemID) {
		am.hub.Logger().Info("Notification silenced", "logger", "alerts", "user", data.UserID, "system", data.SystemID, "title", data.Title)
		return nil
	}

	// get user settings
	record, err := am.hub.FindFirstRecordByFilter(
		"user_settings", "user={:user}",
		dbx.Params{"user": data.UserID},
	)
	if err != nil {
		return err
	}
	// unmarshal user settings
	userAlertSettings := UserNotificationSettings{
		Emails:   []string{},
		Webhooks: WebhookList{},
	}
	if err := record.UnmarshalJSONField("settings", &userAlertSettings); err != nil {
		am.hub.Logger().Error("Failed to unmarshal user settings", "logger", "alerts", "err", err)
	}
	// send alerts via webhooks
	for _, webhook := range userAlertSettings.Webhooks {
		if err := am.SendWebhookAlert(webhook.URL, data.Title, data.Message, data.Link, data.LinkText); err != nil {
			am.hub.Logger().Error("Failed to send webhook alert", "logger", "alerts", "err", err)
		}
	}
	// send alerts via email
	if len(userAlertSettings.Emails) == 0 {
		return nil
	}
	addresses := []mail.Address{}
	for _, email := range userAlertSettings.Emails {
		addresses = append(addresses, mail.Address{Address: email})
	}
	message := mailer.Message{
		To:      addresses,
		Subject: data.Title,
		Text:    data.Message + fmt.Sprintf("\n\n%s", data.Link),
		From: mail.Address{
			Address: am.hub.Settings().Meta.SenderAddress,
			Name:    am.hub.Settings().Meta.SenderName,
		},
	}
	err = am.hub.NewMailClient().Send(&message)
	if err != nil {
		return err
	}
	am.hub.Logger().Info("Sent email alert", "logger", "alerts", "to", message.To, "subj", message.Subject)
	return nil
}

// SendShoutrrrAlert sends an alert via a Shoutrrr URL
func (am *AlertManager) SendShoutrrrAlert(notificationUrl, title, message, link, linkText string) error {
	// Parse the URL
	parsedURL, err := url.Parse(notificationUrl)
	if err != nil {
		return fmt.Errorf("error parsing URL: %v", err)
	}
	scheme := parsedURL.Scheme
	queryParams := parsedURL.Query()

	// Add title
	if _, ok := supportsTitle[scheme]; ok {
		queryParams.Add("title", title)
	} else if scheme == "mattermost" {
		// use markdown title for mattermost
		message = "##### " + title + "\n\n" + message
	} else if scheme == "generic" && queryParams.Has("template") {
		// add title as property if using generic with template json
		titleKey := queryParams.Get("titlekey")
		if titleKey == "" {
			titleKey = "title"
		}
		queryParams.Add("$"+titleKey, title)
	} else {
		// otherwise just add title to message
		message = title + "\n\n" + message
	}

	// Add link
	if scheme == "ntfy" {
		queryParams.Add("Actions", fmt.Sprintf("view, %s, %s", linkText, link))
	} else if scheme == "lark" {
		queryParams.Add("link", link)
	} else if scheme == "bark" {
		queryParams.Add("url", link)
	} else {
		message += "\n\n" + link
	}

	// Encode the modified query parameters back into the URL
	parsedURL.RawQuery = queryParams.Encode()
	// log.Println("URL after modification:", parsedURL.String())

	err = shoutrrr.Send(parsedURL.String(), message)

	if err == nil {
		am.hub.Logger().Info("Sent shoutrrr alert", "logger", "alerts", "title", title)
	} else {
		am.hub.Logger().Error("Error sending shoutrrr alert", "logger", "alerts", "err", err)
		return err
	}
	return nil
}

// SendWebhookAlert sends an alert via WeLink or Shoutrrr depending on the URL.
func (am *AlertManager) SendWebhookAlert(notificationUrl, title, message, link, linkText string) error {
	parsedURL, err := url.Parse(notificationUrl)
	if err != nil {
		return fmt.Errorf("failed to parse webhook url: %v", err)
	}
	if isWeLinkWebhookURL(parsedURL) {
		return am.SendWeLinkAlert(parsedURL, title, message, link)
	}
	return am.SendShoutrrrAlert(notificationUrl, title, message, link, linkText)
}

// SendWeLinkAlert sends an alert via WeLink webhook URL.
func (am *AlertManager) SendWeLinkAlert(parsedURL *url.URL, title, message, link string) error {
	queryParams := parsedURL.Query()
	token := queryParams.Get("token")
	channel := queryParams.Get("channel")
	if token == "" || channel == "" {
		return fmt.Errorf("welink webhook missing token or channel (url=%s)", redactWeLinkURL(parsedURL))
	}
	text := buildWeLinkText(title, message, link)
	textLen := utf8.RuneCountInString(text)
	if textLen < 1 || textLen > 500 {
		return fmt.Errorf("welink content length out of range: %d (expected 1-500)", textLen)
	}
	payload := weLinkWebhookPayload{
		MessageType: "text",
		Content: weLinkWebhookContent{
			Text: text,
		},
		TimeStamp:  time.Now().UnixMilli(),
		UUID:       uuid.NewString(),
		IsAt:       false,
		IsAtAll:    false,
		AtAccounts: []string{},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal welink payload: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, parsedURL.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create welink request: %v", err)
	}
	req.Header.Set("Accept-Charset", "UTF-8")
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		am.hub.Logger().Error("Failed to send welink alert", "logger", "alerts", "err", err)
		return fmt.Errorf("welink request failed: %v", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		am.hub.Logger().Error("Failed to read welink response", "logger", "alerts", "err", err)
		return fmt.Errorf("failed to read welink response body: %v", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		err = fmt.Errorf("welink response status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
		am.hub.Logger().Error("WeLink request returned non-2xx", "logger", "alerts", "err", err)
		return err
	}
	var result weLinkWebhookResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("failed to parse welink response: %v (body=%s)", err, strings.TrimSpace(string(respBody)))
	}
	if result.Code != "0" {
		err = fmt.Errorf("welink response code=%s message=%s data=%s", result.Code, result.Message, result.Data)
		am.hub.Logger().Error("WeLink responded with error", "logger", "alerts", "err", err)
		return err
	}
	am.hub.Logger().Info("Sent welink alert", "logger", "alerts")
	return nil
}

func isWeLinkWebhookURL(parsedURL *url.URL) bool {
	if parsedURL == nil {
		return false
	}
	path := strings.TrimSuffix(parsedURL.Path, "/")
	return strings.EqualFold(parsedURL.Host, weLinkHost) && path == weLinkPath
}

func redactWeLinkURL(parsedURL *url.URL) string {
	if parsedURL == nil {
		return ""
	}
	redacted := *parsedURL
	query := redacted.Query()
	if query.Has("token") {
		query.Set("token", "REDACTED")
	}
	redacted.RawQuery = query.Encode()
	return redacted.String()
}

func buildWeLinkText(title, message, link string) string {
	text := title + "\n\n" + message
	text += "\n\n" + link
	return text
}

func (am *AlertManager) SendTestNotification(e *core.RequestEvent) error {
	var data struct {
		URL string `json:"url"`
	}
	err := e.BindBody(&data)
	if err != nil || data.URL == "" {
		return e.BadRequestError("URL is required", err)
	}
	lang, langErr := am.NotificationLanguage()
	if langErr != nil {
		am.hub.Logger().Error("读取通知语言失败", "logger", "alerts", "err", langErr, "errType", fmt.Sprintf("%T", langErr), "stack", string(debug.Stack()), "url", data.URL)
		return e.JSON(200, map[string]string{"err": fmt.Sprintf("通知语言错误: %v", langErr)})
	}
	appName := strings.TrimSpace(am.hub.Settings().Meta.AppName)
	if appName == "" {
		appName = "Aether"
	}
	alertType := "Test Notification"
	currentValue := "Test"
	threshold := "N/A"
	if lang == NotificationLanguageZhCN {
		alertType = "测试通知"
		currentValue = "测试"
		threshold = "不适用"
	}
	content := NotificationContent{
		SystemName:   appName,
		AlertType:    alertType,
		State:        NotificationStateTriggered,
		CurrentValue: currentValue,
		Threshold:    threshold,
		Duration:     FormatImmediateDuration(lang),
		LinkText:     formatDefaultLinkText(lang, appName),
	}
	text, formatErr := FormatNotification(lang, content)
	if formatErr != nil {
		am.hub.Logger().Error("测试通知格式化失败", "logger", "alerts", "err", formatErr, "errType", fmt.Sprintf("%T", formatErr), "stack", string(debug.Stack()), "url", data.URL)
		return e.JSON(200, map[string]string{"err": fmt.Sprintf("通知格式错误: %v", formatErr)})
	}
	err = am.SendWebhookAlert(data.URL, text.Title, text.Message, am.hub.Settings().Meta.AppURL, text.LinkText)
	if err != nil {
		return e.JSON(200, map[string]string{"err": err.Error()})
	}
	return e.JSON(200, map[string]bool{"err": false})
}

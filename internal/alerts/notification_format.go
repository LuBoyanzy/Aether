// 告警通知格式化器：统一生成多语言标题、正文与链接文本。
package alerts

import (
	"fmt"
	"strings"
)

type NotificationLanguage string

const (
	NotificationLanguageZhCN NotificationLanguage = "zh-CN"
	NotificationLanguageEn   NotificationLanguage = "en"
)

type NotificationState string

const (
	NotificationStateTriggered NotificationState = "triggered"
	NotificationStateResolved  NotificationState = "resolved"
)

type NotificationContent struct {
	SystemName   string
	Host         string
	AlertType    string
	Descriptor   string
	State        NotificationState
	CurrentValue string
	Threshold    string
	Duration     string
	Details      string
	LinkText     string
}

type NotificationText struct {
	Title    string
	Message  string
	LinkText string
}

func DefaultNotificationLanguage() NotificationLanguage {
	return NotificationLanguageZhCN
}

func ParseNotificationLanguage(raw string) (NotificationLanguage, error) {
	lang := NotificationLanguage(strings.TrimSpace(raw))
	switch lang {
	case NotificationLanguageZhCN, NotificationLanguageEn:
		return lang, nil
	default:
		return "", fmt.Errorf("通知语言无效: %s", raw)
	}
}

func FormatNotification(lang NotificationLanguage, content NotificationContent) (NotificationText, error) {
	if _, err := ParseNotificationLanguage(string(lang)); err != nil {
		return NotificationText{}, err
	}
	if strings.TrimSpace(content.SystemName) == "" {
		return NotificationText{}, fmt.Errorf("通知系统名称不能为空")
	}
	if strings.TrimSpace(content.AlertType) == "" {
		return NotificationText{}, fmt.Errorf("通知告警类型不能为空")
	}
	if strings.TrimSpace(content.CurrentValue) == "" {
		return NotificationText{}, fmt.Errorf("通知当前值不能为空")
	}
	if strings.TrimSpace(content.Threshold) == "" {
		return NotificationText{}, fmt.Errorf("通知阈值不能为空")
	}
	if strings.TrimSpace(content.Duration) == "" {
		return NotificationText{}, fmt.Errorf("通知持续时长不能为空")
	}
	if content.State != NotificationStateTriggered && content.State != NotificationStateResolved {
		return NotificationText{}, fmt.Errorf("通知状态无效: %s", content.State)
	}

	alertType := formatAlertType(content.AlertType, content.Descriptor)
	stateLabel := formatStateLabel(lang, content.State)
	titlePrefix := formatTitlePrefix(lang, content.State)
	messageLines := []string{
		fmt.Sprintf("%s: %s", formatLabel(lang, "system"), content.SystemName),
	}
	if strings.TrimSpace(content.Host) != "" {
		messageLines = append(messageLines, fmt.Sprintf("%s: %s", formatLabel(lang, "host"), content.Host))
	}
	messageLines = append(messageLines,
		fmt.Sprintf("%s: %s", formatLabel(lang, "alertType"), alertType),
		fmt.Sprintf("%s: %s", formatLabel(lang, "state"), stateLabel),
		fmt.Sprintf("%s: %s", formatLabel(lang, "currentValue"), content.CurrentValue),
		fmt.Sprintf("%s: %s", formatLabel(lang, "threshold"), content.Threshold),
		fmt.Sprintf("%s: %s", formatLabel(lang, "duration"), content.Duration),
	)
	if strings.TrimSpace(content.Details) != "" {
		messageLines = append(messageLines, fmt.Sprintf("%s: %s", formatLabel(lang, "details"), content.Details))
	}

	linkText := strings.TrimSpace(content.LinkText)
	if linkText == "" {
		linkText = formatDefaultLinkText(lang, content.SystemName)
	}

	return NotificationText{
		Title:    fmt.Sprintf("%s %s %s", titlePrefix, content.SystemName, alertType),
		Message:  strings.Join(messageLines, "\n"),
		LinkText: linkText,
	}, nil
}

func FormatDurationMinutes(minutes int, lang NotificationLanguage) string {
	if minutes <= 0 {
		minutes = 1
	}
	if lang == NotificationLanguageZhCN {
		return fmt.Sprintf("%d 分钟", minutes)
	}
	label := "minutes"
	if minutes == 1 {
		label = "minute"
	}
	return fmt.Sprintf("%d %s", minutes, label)
}

func FormatImmediateDuration(lang NotificationLanguage) string {
	if lang == NotificationLanguageZhCN {
		return "即时"
	}
	return "Immediate"
}

func FormatMetricValue(value float64, unit string) string {
	if unit == "" {
		return fmt.Sprintf("%.2f", value)
	}
	return fmt.Sprintf("%.2f%s", value, unit)
}

func formatTitlePrefix(lang NotificationLanguage, state NotificationState) string {
	if lang == NotificationLanguageZhCN {
		if state == NotificationStateTriggered {
			return "【告警触发】"
		}
		return "【告警恢复】"
	}
	if state == NotificationStateTriggered {
		return "[ALERT]"
	}
	return "[RESOLVED]"
}

func formatStateLabel(lang NotificationLanguage, state NotificationState) string {
	if lang == NotificationLanguageZhCN {
		if state == NotificationStateTriggered {
			return "触发"
		}
		return "恢复"
	}
	if state == NotificationStateTriggered {
		return "Triggered"
	}
	return "Resolved"
}

func formatDefaultLinkText(lang NotificationLanguage, systemName string) string {
	if lang == NotificationLanguageZhCN {
		return "查看 " + systemName
	}
	return "View " + systemName
}

func formatLabel(lang NotificationLanguage, key string) string {
	if lang == NotificationLanguageZhCN {
		switch key {
		case "system":
			return "系统"
		case "alertType":
			return "告警类型"
		case "host":
			return "主机/IP"
		case "state":
			return "状态"
		case "currentValue":
			return "当前值"
		case "threshold":
			return "阈值"
		case "duration":
			return "持续时长"
		case "details":
			return "详情"
		}
	} else {
		switch key {
		case "system":
			return "System"
		case "alertType":
			return "Alert Type"
		case "host":
			return "Host/IP"
		case "state":
			return "State"
		case "currentValue":
			return "Current Value"
		case "threshold":
			return "Threshold"
		case "duration":
			return "Duration"
		case "details":
			return "Details"
		}
	}
	return key
}

func formatAlertType(alertType, descriptor string) string {
	if strings.TrimSpace(descriptor) == "" {
		return alertType
	}
	if strings.EqualFold(strings.TrimSpace(descriptor), strings.TrimSpace(alertType)) {
		return alertType
	}
	return fmt.Sprintf("%s (%s)", alertType, descriptor)
}

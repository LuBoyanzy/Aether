// Docker 关注服务告警处理逻辑。
package hub

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"aether/internal/alerts"
	"aether/internal/entities/docker"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

const (
	dockerFocusRulesCollection  = "docker_focus_services"
	dockerFocusAlertsCollection = "docker_focus_alerts"
	composeProjectLabel         = "com.docker.compose.project"
	composeServiceLabel         = "com.docker.compose.service"
	dockerFocusRecoverySeconds  = 15
	dockerFocusAlertTypeZh      = "Docker 关注服务"
	dockerFocusAlertTypeEn      = "Docker Focus Service"
	maxFocusAlertContainers     = 5
)

type dockerFocusAlertAction struct {
	ShouldSend      bool
	State           alerts.NotificationState
	RuleLabel       string
	RuleDescription string
	RunningCount    int
	TotalCount      int
	DownContainers  []docker.Container
}

// HandleDockerFocusAlerts evaluates focus rules against docker containers and sends notifications.
func (h *Hub) HandleDockerFocusAlerts(systemRecord *core.Record) error {
	if systemRecord == nil {
		return fmt.Errorf("system record is required")
	}
	systemID := strings.TrimSpace(systemRecord.Id)
	if systemID == "" {
		return fmt.Errorf("system id is required")
	}
	rules, err := h.FindRecordsByFilter(
		dockerFocusRulesCollection,
		"system={:system}",
		"match_type,value",
		-1,
		0,
		dbx.Params{"system": systemID},
	)
	if err != nil {
		return fmt.Errorf("读取关注规则失败: %w", err)
	}
	if len(rules) == 0 {
		return h.cleanupDockerFocusAlerts(systemID)
	}
	system, err := h.sm.GetSystem(systemID)
	if err != nil {
		return fmt.Errorf("系统未找到: %w", err)
	}
	containers, err := system.FetchDockerContainersFromAgent(true)
	if err != nil {
		return fmt.Errorf("获取容器列表失败: %w", err)
	}
	alertRecords, err := h.FindRecordsByFilter(
		dockerFocusAlertsCollection,
		"system={:system}",
		"-updated",
		-1,
		0,
		dbx.Params{"system": systemID},
	)
	if err != nil {
		return fmt.Errorf("读取关注告警状态失败: %w", err)
	}
	alertByRule := map[string]*core.Record{}
	for _, record := range alertRecords {
		alertByRule[record.GetString("focus_rule")] = record
	}
	ruleIDs := map[string]struct{}{}
	for _, rule := range rules {
		ruleIDs[rule.Id] = struct{}{}
	}
	for _, record := range alertRecords {
		if _, ok := ruleIDs[record.GetString("focus_rule")]; ok {
			continue
		}
		if err := h.Delete(record); err != nil {
			return fmt.Errorf("清理过期关注告警失败: %w", err)
		}
	}
	lang, err := alerts.GetNotificationLanguage(h)
	if err != nil {
		return fmt.Errorf("读取通知语言失败: %w", err)
	}
	systemName := strings.TrimSpace(systemRecord.GetString("name"))
	if systemName == "" {
		systemName = systemID
	}
	userIDs := systemRecord.GetStringSlice("users")
	if len(userIDs) == 0 {
		return fmt.Errorf("关注告警未找到系统用户: %s", systemID)
	}
	alertsCollection, err := h.FindCollectionByNameOrId(dockerFocusAlertsCollection)
	if err != nil {
		return fmt.Errorf("读取关注告警集合失败: %w", err)
	}

	for _, rule := range rules {
		ruleLabel := formatDockerFocusRuleLabel(rule)
		matched := filterDockerFocusContainers(containers, rule)
		totalCount := len(matched)
		runningCount, downContainers := countDockerFocusStatus(matched)
		shouldTrigger := totalCount == 0 || runningCount < totalCount

		alertRecord := alertByRule[rule.Id]
		if alertRecord == nil {
			alertRecord = core.NewRecord(alertsCollection)
			alertRecord.Set("system", systemID)
			alertRecord.Set("focus_rule", rule.Id)
		}
		triggered := alertRecord.GetBool("triggered")
		action := dockerFocusAlertAction{
			RuleLabel:       ruleLabel,
			RuleDescription: strings.TrimSpace(rule.GetString("description")),
			RunningCount:    runningCount,
			TotalCount:      totalCount,
			DownContainers:  downContainers,
		}
		now := time.Now().UTC()
		if shouldTrigger {
			if !triggered {
				action.ShouldSend = true
				action.State = alerts.NotificationStateTriggered
			}
			triggered = true
			alertRecord.Set("recovery_since", nil)
		} else {
			if triggered {
				recoverySince := alertRecord.GetDateTime("recovery_since").Time()
				if recoverySince.IsZero() {
					alertRecord.Set("recovery_since", now)
				} else if now.Sub(recoverySince) >= dockerFocusRecoveryDuration() {
					action.ShouldSend = true
					action.State = alerts.NotificationStateResolved
					triggered = false
					alertRecord.Set("recovery_since", nil)
				}
			} else {
				alertRecord.Set("recovery_since", nil)
			}
		}
		alertRecord.Set("triggered", triggered)
		alertRecord.Set("running_count", runningCount)
		alertRecord.Set("total_count", totalCount)

		if err := h.Save(alertRecord); err != nil {
			return fmt.Errorf("保存关注告警状态失败: %w", err)
		}
		if action.ShouldSend {
			if err := h.sendDockerFocusAlert(systemID, systemName, userIDs, lang, action); err != nil {
				return err
			}
			h.Logger().Info(
				"docker focus alert sent",
				"logger",
				"alerts",
				"system",
				systemName,
				"rule",
				ruleLabel,
				"state",
				action.State,
				"running",
				runningCount,
				"total",
				totalCount,
			)
		}
	}
	return nil
}

func (h *Hub) cleanupDockerFocusAlerts(systemID string) error {
	if strings.TrimSpace(systemID) == "" {
		return fmt.Errorf("system id is required")
	}
	records, err := h.FindRecordsByFilter(
		dockerFocusAlertsCollection,
		"system={:system}",
		"-updated",
		-1,
		0,
		dbx.Params{"system": systemID},
	)
	if err != nil {
		return fmt.Errorf("读取关注告警记录失败: %w", err)
	}
	for _, record := range records {
		if err := h.Delete(record); err != nil {
			return fmt.Errorf("清理关注告警记录失败: %w", err)
		}
	}
	return nil
}

func filterDockerFocusContainers(containers []docker.Container, rule *core.Record) []docker.Container {
	if rule == nil {
		return nil
	}
	matched := make([]docker.Container, 0)
	for _, container := range containers {
		if matchesDockerFocusRule(container, rule) {
			matched = append(matched, container)
		}
	}
	return matched
}

func matchesDockerFocusRule(container docker.Container, rule *core.Record) bool {
	matchType := strings.TrimSpace(rule.GetString("match_type"))
	value := strings.TrimSpace(rule.GetString("value"))
	value2 := strings.TrimSpace(rule.GetString("value2"))
	if value == "" {
		return false
	}
	switch matchType {
	case "container_name":
		return container.Name == value
	case "image":
		return container.Image == value
	case "compose_project":
		project := dockerComposeProject(container)
		return project == value
	case "compose_service":
		if value2 == "" {
			return false
		}
		project := dockerComposeProject(container)
		service := dockerComposeService(container)
		return project == value && service == value2
	case "label":
		if value2 == "" {
			return false
		}
		if container.Labels == nil {
			return false
		}
		return container.Labels[value] == value2
	default:
		return false
	}
}

func dockerComposeProject(container docker.Container) string {
	if container.Labels != nil {
		if project := strings.TrimSpace(container.Labels[composeProjectLabel]); project != "" {
			return project
		}
	}
	return strings.TrimSpace(container.CreatedBy)
}

func dockerComposeService(container docker.Container) string {
	if container.Labels == nil {
		return ""
	}
	return strings.TrimSpace(container.Labels[composeServiceLabel])
}

func countDockerFocusStatus(containers []docker.Container) (int, []docker.Container) {
	runningCount := 0
	downContainers := make([]docker.Container, 0)
	for _, container := range containers {
		if container.State == "running" {
			runningCount++
			continue
		}
		downContainers = append(downContainers, container)
	}
	return runningCount, downContainers
}

func formatDockerFocusRuleLabel(rule *core.Record) string {
	if rule == nil {
		return ""
	}
	value := strings.TrimSpace(rule.GetString("value"))
	value2 := strings.TrimSpace(rule.GetString("value2"))
	switch strings.TrimSpace(rule.GetString("match_type")) {
	case "compose_service":
		if value2 != "" {
			return value + " / " + value2
		}
		return value
	case "label":
		if value2 != "" {
			return value + "=" + value2
		}
		return value
	default:
		return value
	}
}

func (h *Hub) sendDockerFocusAlert(systemID, systemName string, userIDs []string, lang alerts.NotificationLanguage, action dockerFocusAlertAction) error {
	alertType := dockerFocusAlertType(lang)
	currentValue, thresholdValue := formatDockerFocusValues(lang, action.RunningCount, action.TotalCount)
	duration := alerts.FormatImmediateDuration(lang)
	if action.State == alerts.NotificationStateResolved {
		duration = formatDockerFocusRecoveryDuration(lang)
	}
	details := ""
	if action.State == alerts.NotificationStateTriggered {
		details = formatDockerFocusDetails(lang, action)
	}
	content := alerts.NotificationContent{
		SystemName:   systemName,
		AlertType:    alertType,
		Descriptor:   action.RuleLabel,
		State:        action.State,
		CurrentValue: currentValue,
		Threshold:    thresholdValue,
		Duration:     duration,
		Details:      details,
	}
	text, err := alerts.FormatNotification(lang, content)
	if err != nil {
		h.Logger().Error("关注服务告警格式化失败", "logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack()), "system", systemName, "rule", action.RuleLabel)
		return fmt.Errorf("关注服务告警格式化失败: %w", err)
	}

	var failures []string
	for _, userID := range userIDs {
		if strings.TrimSpace(userID) == "" {
			continue
		}
		err := h.AlertManager.SendAlert(alerts.AlertMessageData{
			UserID:   userID,
			SystemID: systemID,
			Title:    text.Title,
			Message:  text.Message,
			Link:     h.MakeLink("system", systemID),
			LinkText: text.LinkText,
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("user=%s err=%v", userID, err))
			h.Logger().Error("发送关注服务告警失败", "logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack()), "system", systemName, "rule", action.RuleLabel, "user", userID)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("发送关注服务告警失败: %s", strings.Join(failures, "; "))
	}
	return nil
}

func dockerFocusAlertType(lang alerts.NotificationLanguage) string {
	if lang == alerts.NotificationLanguageZhCN {
		return dockerFocusAlertTypeZh
	}
	return dockerFocusAlertTypeEn
}

func formatDockerFocusValues(lang alerts.NotificationLanguage, runningCount, totalCount int) (string, string) {
	if lang == alerts.NotificationLanguageZhCN {
		return fmt.Sprintf("运行 %d/%d", runningCount, totalCount), fmt.Sprintf("应运行 %d/%d", totalCount, totalCount)
	}
	return fmt.Sprintf("running %d/%d", runningCount, totalCount), fmt.Sprintf("expected %d/%d", totalCount, totalCount)
}

func formatDockerFocusRecoveryDuration(lang alerts.NotificationLanguage) string {
	seconds := dockerFocusRecoverySeconds
	if lang == alerts.NotificationLanguageZhCN {
		return fmt.Sprintf("%d 秒", seconds)
	}
	return fmt.Sprintf("%d seconds", seconds)
}

func formatDockerFocusDetails(lang alerts.NotificationLanguage, action dockerFocusAlertAction) string {
	parts := make([]string, 0, 2)
	if action.TotalCount == 0 {
		if lang == alerts.NotificationLanguageZhCN {
			parts = append(parts, "未匹配到容器")
		} else {
			parts = append(parts, "No containers matched")
		}
	} else if len(action.DownContainers) > 0 {
		parts = append(parts, formatDockerFocusDownContainers(lang, action.DownContainers))
	}
	if action.RuleDescription != "" {
		if lang == alerts.NotificationLanguageZhCN {
			parts = append(parts, "规则说明: "+action.RuleDescription)
		} else {
			parts = append(parts, "Rule: "+action.RuleDescription)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	if lang == alerts.NotificationLanguageZhCN {
		return strings.Join(parts, "；")
	}
	return strings.Join(parts, "; ")
}

func formatDockerFocusDownContainers(lang alerts.NotificationLanguage, containers []docker.Container) string {
	limit := maxFocusAlertContainers
	if len(containers) < limit {
		limit = len(containers)
	}
	labels := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		name := strings.TrimSpace(containers[i].Name)
		if name == "" {
			name = strings.TrimSpace(containers[i].ID)
		}
		state := strings.TrimSpace(containers[i].State)
		if state == "" {
			labels = append(labels, name)
			continue
		}
		labels = append(labels, fmt.Sprintf("%s(%s)", name, state))
	}
	if len(containers) > limit {
		labels = append(labels, "...")
	}
	if lang == alerts.NotificationLanguageZhCN {
		return "异常容器: " + strings.Join(labels, ", ")
	}
	return "Affected containers: " + strings.Join(labels, ", ")
}

func dockerFocusRecoveryDuration() time.Duration {
	return time.Duration(dockerFocusRecoverySeconds) * time.Second
}

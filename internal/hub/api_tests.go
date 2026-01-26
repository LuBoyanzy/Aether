// Package hub 提供接口管理（API 监控）的执行器、调度器与 API 路由实现。
// 包含用例执行、全局定时巡检、告警发送与历史记录写入。
package hub

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"aether/internal/alerts"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	apiTestCollectionsCollection = "api_test_collections"
	apiTestCasesCollection       = "api_test_cases"
	apiTestRunsCollection        = "api_test_runs"
	apiTestScheduleCollection    = "api_test_schedule_config"
)

const (
	apiTestDefaultIntervalMinutes            = 5
	apiTestDefaultHistoryRetentionDays       = 7
	apiTestDefaultAlertThreshold             = 1
	apiTestMaxResponseSnippetBytes     int64 = 800
	apiTestMaxPerPage                        = 200
)

type apiTestRunSource string

const (
	apiTestRunSourceManual   apiTestRunSource = "manual"
	apiTestRunSourceSchedule apiTestRunSource = "schedule"
)

type apiTestKeyValue struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

type apiTestRunCaseRequest struct {
	CaseId string `json:"caseId"`
}

type apiTestRunCollectionRequest struct {
	CollectionId string `json:"collectionId"`
}

type apiTestScheduleUpdateRequest struct {
	Enabled              *bool `json:"enabled"`
	IntervalMinutes      *int  `json:"intervalMinutes"`
	AlertEnabled         *bool `json:"alertEnabled"`
	AlertOnRecover       *bool `json:"alertOnRecover"`
	HistoryRetentionDays *int  `json:"historyRetentionDays"`
}

type apiTestScheduleResponse struct {
	Id                   string `json:"id"`
	Enabled              bool   `json:"enabled"`
	IntervalMinutes      int    `json:"intervalMinutes"`
	LastRunAt            string `json:"lastRunAt"`
	NextRunAt            string `json:"nextRunAt"`
	LastError            string `json:"lastError"`
	AlertEnabled         bool   `json:"alertEnabled"`
	AlertOnRecover       bool   `json:"alertOnRecover"`
	HistoryRetentionDays int    `json:"historyRetentionDays"`
}

type apiTestRunResult struct {
	CaseId          string `json:"caseId"`
	CollectionId    string `json:"collectionId"`
	Name            string `json:"name"`
	Status          int    `json:"status"`
	DurationMs      int    `json:"durationMs"`
	Success         bool   `json:"success"`
	Error           string `json:"error"`
	ResponseSnippet string `json:"responseSnippet"`
	RunAt           string `json:"runAt"`
}

type apiTestCollectionRunSummary struct {
	CollectionId string             `json:"collectionId"`
	Collection   string             `json:"collection"`
	Cases        int                `json:"cases"`
	Success      int                `json:"success"`
	Failed       int                `json:"failed"`
	Results      []apiTestRunResult `json:"results"`
}

type apiTestRunAllSummary struct {
	Collections int                `json:"collections"`
	Cases       int                `json:"cases"`
	Success     int                `json:"success"`
	Failed      int                `json:"failed"`
	Results     []apiTestRunResult `json:"results"`
}

type apiTestRunsResponse struct {
	Items      []apiTestRunItem `json:"items"`
	Page       int              `json:"page"`
	PerPage    int              `json:"perPage"`
	TotalItems int              `json:"totalItems"`
	TotalPages int              `json:"totalPages"`
}

type apiTestRunItem struct {
	Id              string `json:"id"`
	CaseId          string `json:"caseId"`
	CollectionId    string `json:"collectionId"`
	Status          int    `json:"status"`
	DurationMs      int    `json:"durationMs"`
	Success         bool   `json:"success"`
	Error           string `json:"error"`
	ResponseSnippet string `json:"responseSnippet"`
	Source          string `json:"source"`
	Created         string `json:"created"`
}

type apiTestExecutionResult struct {
	Status          int
	DurationMs      int
	Success         bool
	Error           string
	ResponseSnippet string
	RunAt           types.DateTime
}

type apiTestAlertAction struct {
	ShouldSend          bool
	State               alerts.NotificationState
	CaseName            string
	ConsecutiveFailures int
	Threshold           int
	DurationMinutes     int
	StatusCode          int
	ErrorMessage        string
}

var apiTestRunning int32

func apiTestAcquireRunLock() bool {
	return atomic.CompareAndSwapInt32(&apiTestRunning, 0, 1)
}

func apiTestReleaseRunLock() {
	atomic.StoreInt32(&apiTestRunning, 0)
}

func (h *Hub) logApiTestError(message string, err error, fields ...any) {
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

func formatApiTestError(context string, err error, fields map[string]any) error {
	return fmt.Errorf(
		"%s | errType=%T | err=%v | fields=%v | stack=%s",
		context,
		err,
		err,
		fields,
		string(debug.Stack()),
	)
}

func apiTestDateTimeString(dt types.DateTime) string {
	if dt.IsZero() {
		return ""
	}
	return dt.Time().UTC().Format(time.RFC3339)
}

func apiTestNowDateTime() types.DateTime {
	return types.NowDateTime()
}

func apiTestParseBody(e *core.RequestEvent, payload any) error {
	decoder := json.NewDecoder(e.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(payload); err != nil {
		return err
	}
	return nil
}

func apiTestValueListToMap(items []apiTestKeyValue) map[string]string {
	result := make(map[string]string)
	for _, item := range items {
		if !item.Enabled {
			continue
		}
		key := strings.TrimSpace(item.Key)
		if key == "" {
			continue
		}
		result[key] = item.Value
	}
	return result
}

func (h *Hub) buildApiTestHeaders(record *core.Record) (map[string]string, error) {
	var items []apiTestKeyValue
	if err := record.UnmarshalJSONField("headers", &items); err != nil {
		return nil, err
	}
	headers := apiTestValueListToMap(items)
	bodyType := strings.ToLower(record.GetString("body_type"))
	if bodyType == "json" {
		if _, ok := headers["Content-Type"]; !ok {
			headers["Content-Type"] = "application/json"
		}
	}
	return headers, nil
}

func (h *Hub) buildApiTestParams(record *core.Record) (map[string]string, error) {
	var items []apiTestKeyValue
	if err := record.UnmarshalJSONField("params", &items); err != nil {
		return nil, err
	}
	return apiTestValueListToMap(items), nil
}

func (h *Hub) buildApiTestBody(record *core.Record) (io.Reader, string, error) {
	method := strings.ToUpper(strings.TrimSpace(record.GetString("method")))
	if method == http.MethodGet || method == http.MethodHead {
		return nil, "", nil
	}
	body := record.GetString("body")
	if strings.TrimSpace(body) == "" {
		return nil, "", nil
	}
	bodyType := strings.ToLower(record.GetString("body_type"))
	switch bodyType {
	case "json":
		if !json.Valid([]byte(body)) {
			return nil, "", errors.New("请求体不是有效的 JSON")
		}
		return bytes.NewBufferString(body), "application/json", nil
	case "text":
		return bytes.NewBufferString(body), "text/plain", nil
	case "form":
		values := url.Values{}
		var raw any
		if err := json.Unmarshal([]byte(body), &raw); err != nil {
			return nil, "", err
		}
		switch typed := raw.(type) {
		case []any:
			for _, item := range typed {
				itemMap, ok := item.(map[string]any)
				if !ok {
					continue
				}
				enabled, _ := itemMap["enabled"].(bool)
				if itemMap["enabled"] != nil && !enabled {
					continue
				}
				key := strings.TrimSpace(fmt.Sprintf("%v", itemMap["key"]))
				if key == "" {
					continue
				}
				value := ""
				if rawValue, exists := itemMap["value"]; exists && rawValue != nil {
					value = fmt.Sprintf("%v", rawValue)
				}
				values.Add(key, value)
			}
		case map[string]any:
			for key, value := range typed {
				trimmedKey := strings.TrimSpace(key)
				if trimmedKey == "" {
					continue
				}
				values.Add(trimmedKey, fmt.Sprintf("%v", value))
			}
		default:
			return nil, "", errors.New("表单请求体格式不正确")
		}
		return bytes.NewBufferString(values.Encode()), "application/x-www-form-urlencoded", nil
	default:
		return nil, "", fmt.Errorf("未知的请求体类型: %s", bodyType)
	}
}

func (h *Hub) resolveApiTestURL(collectionRecord *core.Record, caseRecord *core.Record) (string, error) {
	rawURL := strings.TrimSpace(caseRecord.GetString("url"))
	if rawURL == "" {
		return "", errors.New("请求地址不能为空")
	}
	if strings.HasPrefix(strings.ToLower(rawURL), "http://") || strings.HasPrefix(strings.ToLower(rawURL), "https://") {
		return rawURL, nil
	}
	base := strings.TrimSpace(collectionRecord.GetString("base_url"))
	if base == "" {
		return "", errors.New("合集未设置基础地址，无法拼接相对路径")
	}
	baseURL, err := url.Parse(base)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return "", errors.New("合集基础地址不合法")
	}
	return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(rawURL, "/"), nil
}

func apiTestParseAllowedHosts(raw string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, item := range strings.Split(raw, ",") {
		host := strings.ToLower(strings.TrimSpace(item))
		if host == "" {
			continue
		}
		result[host] = struct{}{}
	}
	return result
}

func apiTestParseAllowedCIDRs(raw string) ([]*net.IPNet, []string) {
	var (
		result []*net.IPNet
		errors []string
	)
	for _, item := range strings.Split(raw, ",") {
		cidr := strings.TrimSpace(item)
		if cidr == "" {
			continue
		}
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			errors = append(errors, cidr)
			continue
		}
		result = append(result, network)
	}
	return result, errors
}

func (h *Hub) validateApiTestTarget(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("解析 URL 失败: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("仅允许 http/https 协议")
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return errors.New("目标地址缺少主机名")
	}
	enableFilter, _ := GetEnv("API_TEST_ENABLE_SSRF_FILTER")
	if strings.ToLower(enableFilter) != "true" {
		return nil
	}
	allowedHostsRaw, _ := GetEnv("API_TEST_ALLOWED_HOSTS")
	allowedHosts := apiTestParseAllowedHosts(allowedHostsRaw)
	if _, ok := allowedHosts[host]; ok {
		return nil
	}
	if host == "localhost" || host == "127.0.0.1" || host == "0.0.0.0" {
		return errors.New("禁止访问本地回环地址")
	}
	allowedCIDRsRaw, _ := GetEnv("API_TEST_ALLOWED_CIDRS")
	allowedCIDRs, invalidCIDRs := apiTestParseAllowedCIDRs(allowedCIDRsRaw)
	if len(invalidCIDRs) > 0 {
		return fmt.Errorf("存在无效白名单网段: %s", strings.Join(invalidCIDRs, ","))
	}
	ip := net.ParseIP(host)
	if ip != nil {
		if apiTestIPBlocked(ip, allowedCIDRs) {
			return errors.New("禁止访问内网或本地地址")
		}
		return nil
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("解析域名失败: %w", err)
	}
	for _, addr := range addrs {
		if apiTestIPBlocked(addr, allowedCIDRs) {
			return errors.New("禁止访问内网或本地地址")
		}
	}
	return nil
}

func apiTestIPBlocked(ip net.IP, allowed []*net.IPNet) bool {
	if ip == nil {
		return false
	}
	for _, network := range allowed {
		if network.Contains(ip) {
			return false
		}
	}
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() {
		return true
	}
	return false
}

func (h *Hub) getOrCreateApiTestScheduleConfig() (*core.Record, error) {
	collection, err := h.FindCollectionByNameOrId(apiTestScheduleCollection)
	if err != nil {
		return nil, err
	}
	record, err := h.FindFirstRecordByFilter(collection, "", dbx.Params{})
	if err == nil {
		return record, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	newRecord := core.NewRecord(collection)
	newRecord.Set("enabled", false)
	newRecord.Set("interval_minutes", apiTestDefaultIntervalMinutes)
	newRecord.Set("alert_enabled", false)
	newRecord.Set("alert_on_recover", true)
	newRecord.Set("history_retention_days", apiTestDefaultHistoryRetentionDays)
	newRecord.Set("last_error", "")
	if err := h.Save(newRecord); err != nil {
		return nil, err
	}
	return newRecord, nil
}

func (h *Hub) buildApiTestScheduleResponse(record *core.Record) apiTestScheduleResponse {
	return apiTestScheduleResponse{
		Id:                   record.Id,
		Enabled:              record.GetBool("enabled"),
		IntervalMinutes:      record.GetInt("interval_minutes"),
		LastRunAt:            apiTestDateTimeString(record.GetDateTime("last_run_at")),
		NextRunAt:            apiTestDateTimeString(record.GetDateTime("next_run_at")),
		LastError:            record.GetString("last_error"),
		AlertEnabled:         record.GetBool("alert_enabled"),
		AlertOnRecover:       record.GetBool("alert_on_recover"),
		HistoryRetentionDays: record.GetInt("history_retention_days"),
	}
}

func (h *Hub) getApiTestScheduleConfig(e *core.RequestEvent) error {
	record, err := h.getOrCreateApiTestScheduleConfig()
	if err != nil {
		h.logApiTestError("获取接口定时配置失败", err)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": formatApiTestError("获取接口定时配置失败", err, nil).Error()})
	}
	return e.JSON(http.StatusOK, h.buildApiTestScheduleResponse(record))
}

func (h *Hub) updateApiTestScheduleConfig(e *core.RequestEvent) error {
	var payload apiTestScheduleUpdateRequest
	if err := apiTestParseBody(e, &payload); err != nil {
		h.logApiTestError("解析接口定时配置失败", err)
		return e.JSON(http.StatusBadRequest, map[string]string{"error": formatApiTestError("解析接口定时配置失败", err, nil).Error()})
	}
	record, err := h.getOrCreateApiTestScheduleConfig()
	if err != nil {
		h.logApiTestError("读取接口定时配置失败", err)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": formatApiTestError("读取接口定时配置失败", err, nil).Error()})
	}
	if payload.Enabled != nil {
		record.Set("enabled", *payload.Enabled)
	}
	if payload.IntervalMinutes != nil {
		if *payload.IntervalMinutes <= 0 {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": formatApiTestError("intervalMinutes 无效", errors.New("必须大于 0"), map[string]any{"intervalMinutes": *payload.IntervalMinutes}).Error()})
		}
		record.Set("interval_minutes", *payload.IntervalMinutes)
	}
	if payload.AlertEnabled != nil {
		record.Set("alert_enabled", *payload.AlertEnabled)
	}
	if payload.AlertOnRecover != nil {
		record.Set("alert_on_recover", *payload.AlertOnRecover)
	}
	if payload.HistoryRetentionDays != nil {
		if *payload.HistoryRetentionDays <= 0 {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": formatApiTestError("historyRetentionDays 无效", errors.New("必须大于 0"), map[string]any{"historyRetentionDays": *payload.HistoryRetentionDays}).Error()})
		}
		record.Set("history_retention_days", *payload.HistoryRetentionDays)
	}
	if record.GetBool("enabled") && record.GetDateTime("next_run_at").IsZero() {
		interval := record.GetInt("interval_minutes")
		record.Set("next_run_at", apiTestNowDateTime().Add(time.Duration(interval)*time.Minute))
	}
	if err := h.Save(record); err != nil {
		h.logApiTestError("保存接口定时配置失败", err)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": formatApiTestError("保存接口定时配置失败", err, nil).Error()})
	}
	return e.JSON(http.StatusOK, h.buildApiTestScheduleResponse(record))
}

func (h *Hub) runApiTestCase(e *core.RequestEvent) error {
	var payload apiTestRunCaseRequest
	if err := apiTestParseBody(e, &payload); err != nil {
		h.logApiTestError("解析执行用例请求失败", err)
		return e.JSON(http.StatusBadRequest, map[string]string{"error": formatApiTestError("解析执行用例请求失败", err, nil).Error()})
	}
	caseId := strings.TrimSpace(payload.CaseId)
	if caseId == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": formatApiTestError("caseId 不能为空", errors.New("caseId 缺失"), nil).Error()})
	}
	if !apiTestAcquireRunLock() {
		return e.JSON(http.StatusConflict, map[string]string{"error": formatApiTestError("接口测试执行中", errors.New("已有任务在执行"), nil).Error()})
	}
	defer apiTestReleaseRunLock()
	result, err := h.executeApiTestCaseById(caseId, apiTestRunSourceManual, nil)
	if err != nil {
		h.logApiTestError("执行接口用例失败", err, "caseId", caseId)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": formatApiTestError("执行接口用例失败", err, map[string]any{"caseId": caseId}).Error()})
	}
	return e.JSON(http.StatusOK, result)
}

func (h *Hub) runApiTestCollection(e *core.RequestEvent) error {
	var payload apiTestRunCollectionRequest
	if err := apiTestParseBody(e, &payload); err != nil {
		h.logApiTestError("解析执行合集请求失败", err)
		return e.JSON(http.StatusBadRequest, map[string]string{"error": formatApiTestError("解析执行合集请求失败", err, nil).Error()})
	}
	collectionId := strings.TrimSpace(payload.CollectionId)
	if collectionId == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": formatApiTestError("collectionId 不能为空", errors.New("collectionId 缺失"), nil).Error()})
	}
	if !apiTestAcquireRunLock() {
		return e.JSON(http.StatusConflict, map[string]string{"error": formatApiTestError("接口测试执行中", errors.New("已有任务在执行"), nil).Error()})
	}
	defer apiTestReleaseRunLock()
	summary, err := h.executeApiTestCollection(collectionId, apiTestRunSourceManual)
	if err != nil {
		h.logApiTestError("执行接口合集失败", err, "collectionId", collectionId)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": formatApiTestError("执行接口合集失败", err, map[string]any{"collectionId": collectionId}).Error()})
	}
	return e.JSON(http.StatusOK, summary)
}

func (h *Hub) runAllApiTests(e *core.RequestEvent) error {
	if !apiTestAcquireRunLock() {
		return e.JSON(http.StatusConflict, map[string]string{"error": formatApiTestError("接口测试执行中", errors.New("已有任务在执行"), nil).Error()})
	}
	defer apiTestReleaseRunLock()
	summary, err := h.executeApiTestAll(apiTestRunSourceManual)
	if err != nil {
		h.logApiTestError("执行全部接口用例失败", err)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": formatApiTestError("执行全部接口用例失败", err, nil).Error()})
	}
	return e.JSON(http.StatusOK, summary)
}

func (h *Hub) listApiTestRuns(e *core.RequestEvent) error {
	query := e.Request.URL.Query()
	caseId := strings.TrimSpace(query.Get("case"))
	collectionId := strings.TrimSpace(query.Get("collection"))
	page := apiTestParseInt(query.Get("page"), 1)
	perPage := apiTestParseInt(query.Get("perPage"), 50)
	if perPage <= 0 {
		perPage = 50
	}
	if perPage > apiTestMaxPerPage {
		perPage = apiTestMaxPerPage
	}
	filterParts := []string{}
	countFilterParts := []string{}
	params := dbx.Params{}
	if caseId != "" {
		filterParts = append(filterParts, "case = {:case}")
		countFilterParts = append(countFilterParts, "`case` = {:case}")
		params["case"] = caseId
	}
	if collectionId != "" {
		filterParts = append(filterParts, "collection = {:collection}")
		countFilterParts = append(countFilterParts, "collection = {:collection}")
		params["collection"] = collectionId
	}
	filter := strings.Join(filterParts, " && ")
	countFilter := strings.Join(countFilterParts, " AND ")
	var exp dbx.Expression
	if countFilter != "" {
		exp = dbx.NewExp(countFilter, params)
	}
	totalItems64, err := h.CountRecords(apiTestRunsCollection, exp)
	if err != nil {
		h.logApiTestError("统计接口执行记录失败", err)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": formatApiTestError("统计接口执行记录失败", err, nil).Error()})
	}
	totalItems := int(totalItems64)
	totalPages := totalItems / perPage
	if totalItems%perPage != 0 {
		totalPages++
	}
	if page <= 0 {
		page = 1
	}
	if totalPages > 0 && page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * perPage
	records, err := h.FindRecordsByFilter(apiTestRunsCollection, filter, "-created", perPage, offset, params)
	if err != nil {
		h.logApiTestError("读取接口执行记录失败", err)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": formatApiTestError("读取接口执行记录失败", err, nil).Error()})
	}
	items := make([]apiTestRunItem, 0, len(records))
	for _, record := range records {
		items = append(items, apiTestRunItem{
			Id:              record.Id,
			CaseId:          record.GetString("case"),
			CollectionId:    record.GetString("collection"),
			Status:          record.GetInt("status"),
			DurationMs:      record.GetInt("duration_ms"),
			Success:         record.GetBool("success"),
			Error:           record.GetString("error"),
			ResponseSnippet: record.GetString("response_snippet"),
			Source:          record.GetString("source"),
			Created:         apiTestDateTimeString(record.GetDateTime("created")),
		})
	}
	return e.JSON(http.StatusOK, apiTestRunsResponse{
		Items:      items,
		Page:       page,
		PerPage:    perPage,
		TotalItems: totalItems,
		TotalPages: totalPages,
	})
}

func apiTestParseInt(raw string, fallback int) int {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func (h *Hub) executeApiTestCaseById(caseId string, source apiTestRunSource, config *core.Record) (apiTestRunResult, error) {
	caseRecord, err := h.FindRecordById(apiTestCasesCollection, caseId)
	if err != nil {
		return apiTestRunResult{}, err
	}
	collectionId := caseRecord.GetString("collection")
	collectionRecord, err := h.FindRecordById(apiTestCollectionsCollection, collectionId)
	if err != nil {
		return apiTestRunResult{}, err
	}
	return h.executeApiTestCase(caseRecord, collectionRecord, source, config)
}

func (h *Hub) executeApiTestCase(caseRecord *core.Record, collectionRecord *core.Record, source apiTestRunSource, config *core.Record) (apiTestRunResult, error) {
	start := time.Now()
	result := apiTestExecutionResult{
		Status:          0,
		DurationMs:      0,
		Success:         false,
		Error:           "",
		ResponseSnippet: "",
		RunAt:           apiTestNowDateTime(),
	}
	method := strings.ToUpper(strings.TrimSpace(caseRecord.GetString("method")))
	if method == "" {
		result.Error = "HTTP 方法不能为空"
		return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
	}
	if method != http.MethodGet && method != http.MethodPost && method != http.MethodPut && method != http.MethodDelete && method != http.MethodPatch && method != http.MethodHead {
		result.Error = fmt.Sprintf("不支持的 HTTP 方法: %s", method)
		return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
	}
	expectedStatus := caseRecord.GetInt("expected_status")
	if expectedStatus <= 0 {
		result.Error = "期望状态码必须大于 0"
		return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
	}
	timeoutMs := caseRecord.GetInt("timeout_ms")
	if timeoutMs <= 0 {
		result.Error = "超时时间必须大于 0"
		return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
	}
	headers, err := h.buildApiTestHeaders(caseRecord)
	if err != nil {
		result.Error = fmt.Sprintf("解析请求头失败: %v", err)
		return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
	}
	params, err := h.buildApiTestParams(caseRecord)
	if err != nil {
		result.Error = fmt.Sprintf("解析查询参数失败: %v", err)
		return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
	}
	bodyReader, contentType, err := h.buildApiTestBody(caseRecord)
	if err != nil {
		result.Error = fmt.Sprintf("解析请求体失败: %v", err)
		return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
	}
	targetURL, err := h.resolveApiTestURL(collectionRecord, caseRecord)
	if err != nil {
		result.Error = fmt.Sprintf("构建请求地址失败: %v", err)
		return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
	}
	if err := h.validateApiTestTarget(targetURL); err != nil {
		result.Error = fmt.Sprintf("请求地址校验失败: %v", err)
		return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
	}
	request, err := http.NewRequest(method, targetURL, bodyReader)
	if err != nil {
		result.Error = fmt.Sprintf("创建请求失败: %v", err)
		return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
	}
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	if contentType != "" && request.Header.Get("Content-Type") == "" {
		request.Header.Set("Content-Type", contentType)
	}
	if len(params) > 0 {
		query := request.URL.Query()
		for key, value := range params {
			query.Add(key, value)
		}
		request.URL.RawQuery = query.Encode()
	}
	client := &http.Client{Timeout: time.Duration(timeoutMs) * time.Millisecond}
	response, err := client.Do(request)
	if err != nil {
		result.Error = fmt.Sprintf("请求执行失败: %v", err)
		result.DurationMs = int(time.Since(start).Milliseconds())
		return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
	}
	defer response.Body.Close()
	result.Status = response.StatusCode
	snippetReader := io.LimitReader(response.Body, apiTestMaxResponseSnippetBytes+1)
	payload, readErr := io.ReadAll(snippetReader)
	if readErr != nil {
		result.Error = fmt.Sprintf("读取响应失败: %v", readErr)
		result.DurationMs = int(time.Since(start).Milliseconds())
		return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
	}
	result.ResponseSnippet = strings.TrimSpace(string(payload))
	result.Success = result.Status == expectedStatus
	if !result.Success {
		if result.ResponseSnippet != "" {
			result.Error = result.ResponseSnippet
		} else {
			result.Error = fmt.Sprintf("期望状态码 %d，实际 %d", expectedStatus, result.Status)
		}
	}
	result.DurationMs = int(time.Since(start).Milliseconds())
	return h.persistApiTestRun(caseRecord, collectionRecord, result, source, config)
}

func (h *Hub) persistApiTestRun(caseRecord *core.Record, collectionRecord *core.Record, result apiTestExecutionResult, source apiTestRunSource, config *core.Record) (apiTestRunResult, error) {
	var alertAction apiTestAlertAction
	err := h.RunInTransaction(func(txApp core.App) error {
		caseRecord.Set("last_status", result.Status)
		caseRecord.Set("last_duration_ms", result.DurationMs)
		caseRecord.Set("last_run_at", result.RunAt)
		caseRecord.Set("last_success", result.Success)
		caseRecord.Set("last_error", result.Error)
		caseRecord.Set("last_response_snippet", result.ResponseSnippet)

		threshold := caseRecord.GetInt("alert_threshold")
		if threshold <= 0 {
			threshold = apiTestDefaultAlertThreshold
		}
		consecutive := caseRecord.GetInt("consecutive_failures")
		triggered := caseRecord.GetBool("alert_triggered")
		previousConsecutive := consecutive
		intervalMinutes := apiTestDefaultIntervalMinutes
		if config != nil && config.GetInt("interval_minutes") > 0 {
			intervalMinutes = config.GetInt("interval_minutes")
		}

		if result.Success {
			if consecutive > 0 {
				consecutive = 0
			}
			if triggered && config != nil && config.GetBool("alert_on_recover") {
				alertAction = apiTestAlertAction{
					ShouldSend:          true,
					State:               alerts.NotificationStateResolved,
					CaseName:            caseRecord.GetString("name"),
					ConsecutiveFailures: previousConsecutive,
					Threshold:           threshold,
					DurationMinutes:     previousConsecutive * intervalMinutes,
					StatusCode:          result.Status,
				}
			}
			triggered = false
		} else {
			consecutive++
			if config != nil && config.GetBool("alert_enabled") && !triggered && consecutive >= threshold {
				alertAction = apiTestAlertAction{
					ShouldSend:          true,
					State:               alerts.NotificationStateTriggered,
					CaseName:            caseRecord.GetString("name"),
					ConsecutiveFailures: consecutive,
					Threshold:           threshold,
					DurationMinutes:     consecutive * intervalMinutes,
					StatusCode:          result.Status,
					ErrorMessage:        result.Error,
				}
				triggered = true
			}
		}
		caseRecord.Set("consecutive_failures", consecutive)
		caseRecord.Set("alert_triggered", triggered)
		if err := txApp.Save(caseRecord); err != nil {
			return err
		}
		runsCollection, err := txApp.FindCollectionByNameOrId(apiTestRunsCollection)
		if err != nil {
			return err
		}
		runRecord := core.NewRecord(runsCollection)
		runRecord.Set("collection", collectionRecord.Id)
		runRecord.Set("case", caseRecord.Id)
		runRecord.Set("status", result.Status)
		runRecord.Set("duration_ms", result.DurationMs)
		runRecord.Set("success", result.Success)
		runRecord.Set("error", result.Error)
		runRecord.Set("response_snippet", result.ResponseSnippet)
		runRecord.Set("source", string(source))
		if err := txApp.Save(runRecord); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return apiTestRunResult{}, err
	}
	if alertAction.ShouldSend && source == apiTestRunSourceSchedule {
		if sendErr := h.sendApiTestAlert(alertAction); sendErr != nil {
			return apiTestRunResult{}, sendErr
		}
	}
	return apiTestRunResult{
		CaseId:          caseRecord.Id,
		CollectionId:    collectionRecord.Id,
		Name:            caseRecord.GetString("name"),
		Status:          result.Status,
		DurationMs:      result.DurationMs,
		Success:         result.Success,
		Error:           result.Error,
		ResponseSnippet: result.ResponseSnippet,
		RunAt:           apiTestDateTimeString(result.RunAt),
	}, nil
}

func (h *Hub) sendApiTestAlert(action apiTestAlertAction) error {
	if !action.ShouldSend {
		return nil
	}
	lang, err := alerts.GetNotificationLanguage(h)
	if err != nil {
		h.logApiTestError("读取通知语言失败", err, "action", action)
		return err
	}
	appName := strings.TrimSpace(h.Settings().Meta.AppName)
	if appName == "" {
		appName = "Aether"
	}
	alertType := "API Test"
	if strings.TrimSpace(action.CaseName) != "" {
		alertType = fmt.Sprintf("%s: %s", alertType, action.CaseName)
	}
	thresholdValue := action.Threshold
	if thresholdValue <= 0 {
		thresholdValue = apiTestDefaultAlertThreshold
	}
	currentValue := fmt.Sprintf("%d", action.ConsecutiveFailures)
	threshold := fmt.Sprintf("%d", thresholdValue)
	if lang == alerts.NotificationLanguageZhCN {
		currentValue = fmt.Sprintf("%d 次", action.ConsecutiveFailures)
		threshold = fmt.Sprintf("%d 次", thresholdValue)
	} else {
		currentValue = fmt.Sprintf("%d times", action.ConsecutiveFailures)
		threshold = fmt.Sprintf("%d times", thresholdValue)
	}
	duration := alerts.FormatImmediateDuration(lang)
	if action.DurationMinutes > 0 {
		duration = alerts.FormatDurationMinutes(action.DurationMinutes, lang)
	}
	details := strings.TrimSpace(action.ErrorMessage)
	if details == "" && action.StatusCode > 0 {
		if lang == alerts.NotificationLanguageZhCN {
			details = fmt.Sprintf("状态码: %d", action.StatusCode)
		} else {
			details = fmt.Sprintf("Status Code: %d", action.StatusCode)
		}
	}
	linkText := "View API tests"
	if lang == alerts.NotificationLanguageZhCN {
		linkText = "查看接口管理"
	}
	content := alerts.NotificationContent{
		SystemName:   appName,
		AlertType:    alertType,
		State:        action.State,
		CurrentValue: currentValue,
		Threshold:    threshold,
		Duration:     duration,
		Details:      details,
		LinkText:     linkText,
	}
	text, err := alerts.FormatNotification(lang, content)
	if err != nil {
		h.logApiTestError("接口告警格式化失败", err, "action", action)
		return err
	}
	userSettings, err := h.FindAllRecords("user_settings", nil)
	if err != nil {
		return err
	}
	if len(userSettings) == 0 {
		return errors.New("未找到用户通知配置")
	}
	var failures []string
	for _, record := range userSettings {
		userID := record.GetString("user")
		if userID == "" {
			h.Logger().Warn("接口告警未找到用户ID", "logger", "hub", "recordId", record.Id)
			continue
		}
		err := h.AlertManager.SendAlert(alerts.AlertMessageData{
			UserID:   userID,
			SystemID: "",
			Title:    text.Title,
			Message:  text.Message,
			Link:     h.MakeLink("api-tests"),
			LinkText: text.LinkText,
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("user=%s err=%v", userID, err))
			h.logApiTestError("发送接口告警失败", err, "userId", userID)
		}
	}
	if len(failures) > 0 {
		return fmt.Errorf("发送接口告警失败: %s", strings.Join(failures, "; "))
	}
	return nil
}

func (h *Hub) executeApiTestCollection(collectionId string, source apiTestRunSource) (apiTestCollectionRunSummary, error) {
	collectionRecord, err := h.FindRecordById(apiTestCollectionsCollection, collectionId)
	if err != nil {
		return apiTestCollectionRunSummary{}, err
	}
	cases, err := h.FindRecordsByFilter(apiTestCasesCollection, "collection = {:collection}", "sort_order,created", -1, 0, dbx.Params{"collection": collectionId})
	if err != nil {
		return apiTestCollectionRunSummary{}, err
	}
	scheduleConfig, err := h.getOrCreateApiTestScheduleConfig()
	if err != nil {
		return apiTestCollectionRunSummary{}, err
	}
	summary := apiTestCollectionRunSummary{
		CollectionId: collectionRecord.Id,
		Collection:   collectionRecord.GetString("name"),
		Cases:        0,
		Success:      0,
		Failed:       0,
		Results:      []apiTestRunResult{},
	}
	for _, caseRecord := range cases {
		summary.Cases++
		result, runErr := h.executeApiTestCase(caseRecord, collectionRecord, source, nil)
		if runErr != nil {
			return apiTestCollectionRunSummary{}, runErr
		}
		summary.Results = append(summary.Results, result)
		if result.Success {
			summary.Success++
		} else {
			summary.Failed++
		}
	}
	if err := h.cleanupApiTestRuns(scheduleConfig); err != nil {
		return apiTestCollectionRunSummary{}, err
	}
	return summary, nil
}

func (h *Hub) executeApiTestAll(source apiTestRunSource) (apiTestRunAllSummary, error) {
	collections, err := h.FindRecordsByFilter(apiTestCollectionsCollection, "", "sort_order,created", -1, 0, nil)
	if err != nil {
		return apiTestRunAllSummary{}, err
	}
	collectionMap := make(map[string]*core.Record)
	for _, record := range collections {
		collectionMap[record.Id] = record
	}
	cases, err := h.FindRecordsByFilter(apiTestCasesCollection, "", "collection,sort_order,created", -1, 0, nil)
	if err != nil {
		return apiTestRunAllSummary{}, err
	}
	scheduleConfig, err := h.getOrCreateApiTestScheduleConfig()
	if err != nil {
		return apiTestRunAllSummary{}, err
	}
	summary := apiTestRunAllSummary{
		Collections: len(collections),
		Cases:       0,
		Success:     0,
		Failed:      0,
		Results:     []apiTestRunResult{},
	}
	for _, caseRecord := range cases {
		collectionRecord := collectionMap[caseRecord.GetString("collection")]
		if collectionRecord == nil {
			continue
		}
		summary.Cases++
		result, runErr := h.executeApiTestCase(caseRecord, collectionRecord, source, nil)
		if runErr != nil {
			return apiTestRunAllSummary{}, runErr
		}
		summary.Results = append(summary.Results, result)
		if result.Success {
			summary.Success++
		} else {
			summary.Failed++
		}
	}
	if err := h.cleanupApiTestRuns(scheduleConfig); err != nil {
		return apiTestRunAllSummary{}, err
	}
	return summary, nil
}

func (h *Hub) runApiTestScheduleTick() {
	config, err := h.getOrCreateApiTestScheduleConfig()
	if err != nil {
		h.logApiTestError("读取接口定时配置失败", err)
		return
	}
	if !config.GetBool("enabled") {
		return
	}
	intervalMinutes := config.GetInt("interval_minutes")
	if intervalMinutes <= 0 {
		intervalMinutes = apiTestDefaultIntervalMinutes
	}
	now := time.Now()
	nextRun := config.GetDateTime("next_run_at")
	if nextRun.IsZero() {
		nextRun = apiTestNowDateTime().Add(time.Duration(intervalMinutes) * time.Minute)
		config.Set("next_run_at", nextRun)
		if err := h.Save(config); err != nil {
			h.logApiTestError("初始化接口定时配置失败", err)
		}
		return
	}
	if nextRun.Time().After(now) {
		return
	}
	if !apiTestAcquireRunLock() {
		config.Set("last_error", "已有任务在执行，本次跳过")
		config.Set("next_run_at", apiTestNowDateTime().Add(time.Duration(intervalMinutes)*time.Minute))
		if err := h.Save(config); err != nil {
			h.logApiTestError("更新接口定时配置失败", err)
		}
		return
	}
	defer apiTestReleaseRunLock()

	runErr := h.executeScheduledApiTests(config, now, intervalMinutes)
	if runErr != nil {
		h.logApiTestError("接口定时巡检失败", runErr)
		config.Set("last_error", runErr.Error())
	} else {
		config.Set("last_error", "")
	}
	config.Set("last_run_at", apiTestNowDateTime())
	config.Set("next_run_at", apiTestNowDateTime().Add(time.Duration(intervalMinutes)*time.Minute))
	if err := h.Save(config); err != nil {
		h.logApiTestError("保存接口定时配置失败", err)
	}
}

func (h *Hub) executeScheduledApiTests(config *core.Record, now time.Time, intervalMinutes int) error {
	cases, err := h.FindRecordsByFilter(apiTestCasesCollection, "schedule_enabled = true", "collection,sort_order,created", -1, 0, nil)
	if err != nil {
		return err
	}
	collectionIds := map[string]struct{}{}
	for _, caseRecord := range cases {
		collectionIds[caseRecord.GetString("collection")] = struct{}{}
	}
	collectionMap := make(map[string]*core.Record)
	for id := range collectionIds {
		record, err := h.FindRecordById(apiTestCollectionsCollection, id)
		if err != nil {
			return err
		}
		collectionMap[id] = record
	}
	var errorsList []string
	for _, caseRecord := range cases {
		caseInterval := caseRecord.GetInt("schedule_minutes")
		if caseInterval <= 0 {
			caseInterval = intervalMinutes
		}
		lastRun := caseRecord.GetDateTime("last_run_at")
		if !lastRun.IsZero() {
			nextDue := lastRun.Time().Add(time.Duration(caseInterval) * time.Minute)
			if nextDue.After(now) {
				continue
			}
		}
		collectionRecord := collectionMap[caseRecord.GetString("collection")]
		if collectionRecord == nil {
			continue
		}
		_, runErr := h.executeApiTestCase(caseRecord, collectionRecord, apiTestRunSourceSchedule, config)
		if runErr != nil {
			errorsList = append(errorsList, runErr.Error())
		}
	}
	if err := h.cleanupApiTestRuns(config); err != nil {
		errorsList = append(errorsList, err.Error())
	}
	if len(errorsList) > 0 {
		return errors.New(strings.Join(errorsList, " | "))
	}
	return nil
}

func (h *Hub) cleanupApiTestRuns(config *core.Record) error {
	retentionDays := config.GetInt("history_retention_days")
	if retentionDays <= 0 {
		return nil
	}
	cutoff := apiTestNowDateTime().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	_, err := h.DB().NewQuery("DELETE FROM " + apiTestRunsCollection + " WHERE created < {:cutoff}").Bind(dbx.Params{
		"cutoff": cutoff.String(),
	}).Execute()
	if err != nil {
		return err
	}
	return nil
}

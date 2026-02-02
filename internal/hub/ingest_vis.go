package hub

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

type ingestVisConfig struct {
	EsURL         string
	EsUsername    string
	EsPassword    string
	IndexPattern  string
	PollInterval  time.Duration
	DefaultWindow time.Duration
	CacheTTL      time.Duration
	MaxEvents     int
}

type ingestVisService struct {
	hub *Hub

	enabled bool
	cfg     ingestVisConfig
	cfgErr  string

	client    *http.Client
	startOnce sync.Once

	mu sync.RWMutex

	started     bool
	lastQueryAt time.Time
	lastPollAt  time.Time

	lastErrorAt time.Time
	lastError   string

	truncated bool

	seen map[string]time.Time
	runs map[string]*ingestVisRunState
}

type ingestVisRunState struct {
	itemCode  string
	traceId   string
	stage     string
	status    string
	lastEvent ingestVisEventDTO

	lastSeenAt time.Time
	expiresAt  time.Time
}

type ingestVisEventDTO struct {
	Id        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
	Action    string `json:"action"`
	Outcome   string `json:"outcome"`
	ItemCode  string `json:"itemCode"`
	TraceId   string `json:"traceId,omitempty"`
	Message   string `json:"message"`
	// workflow extra (optional)
	ProcessType     *int   `json:"processType,omitempty"`
	Ingest          string `json:"ingest,omitempty"`
	InsertType      []int  `json:"insertType,omitempty"`
	Force           *int   `json:"force,omitempty"`
	FileType        *int   `json:"fileType,omitempty"`
	TaskId          string `json:"taskId,omitempty"`
	InferTaskId     string `json:"inferTaskId,omitempty"`
	InferResultPath string `json:"inferResultPath,omitempty"`
	InferType       *int   `json:"inferType,omitempty"`
	// rabbitmq extra (optional)
	RabbitMQQueue       string `json:"rabbitmqQueue,omitempty"`
	RabbitMQDeliveryTag *int64 `json:"rabbitmqDeliveryTag,omitempty"`
	RabbitMQRedelivered *bool  `json:"rabbitmqRedelivered,omitempty"`
	RabbitMQRequeue     *bool  `json:"rabbitmqRequeue,omitempty"`
	ErrorMessage        string `json:"errorMessage,omitempty"`
	ErrorType           string `json:"errorType,omitempty"`
	ErrorStackTrace     string `json:"errorStackTrace,omitempty"`
}

type ingestVisRunDTO struct {
	Key       string            `json:"key"`
	ItemCode  string            `json:"itemCode"`
	TraceId   string            `json:"traceId,omitempty"`
	Stage     string            `json:"stage"`
	Status    string            `json:"status"`
	LastEvent ingestVisEventDTO `json:"lastEvent"`
}

func newIngestVisService(hub *Hub) *ingestVisService {
	s := &ingestVisService{
		hub:  hub,
		seen: map[string]time.Time{},
		runs: map[string]*ingestVisRunState{},
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
	}

	cfg, enabled, cfgErr := loadIngestVisConfig()
	s.cfg = cfg
	s.enabled = enabled
	s.cfgErr = cfgErr
	return s
}

func loadIngestVisConfig() (cfg ingestVisConfig, enabled bool, cfgErr string) {
	esURL, _ := GetEnv("WORKFLOW_ES_URL")
	esUsername, _ := GetEnv("WORKFLOW_ES_USERNAME")
	esPassword, _ := GetEnv("WORKFLOW_ES_PASSWORD")
	indexPattern, _ := GetEnv("WORKFLOW_ES_INDEX")

	if strings.TrimSpace(esURL) == "" {
		return ingestVisConfig{}, false, "缺少环境变量 AETHER_HUB_WORKFLOW_ES_URL（或 WORKFLOW_ES_URL）"
	}
	// ES 鉴权可选：如果启用了 xpack.security，需要同时提供用户名/密码；否则可留空走匿名。
	if strings.TrimSpace(esUsername) != "" || strings.TrimSpace(esPassword) != "" {
		if strings.TrimSpace(esUsername) == "" || strings.TrimSpace(esPassword) == "" {
			return ingestVisConfig{}, false, "ES Basic Auth 需同时提供 AETHER_HUB_WORKFLOW_ES_USERNAME 与 AETHER_HUB_WORKFLOW_ES_PASSWORD（或无前缀同名变量）；若 ES 未启用鉴权可留空"
		}
	}
	if strings.TrimSpace(indexPattern) == "" {
		indexPattern = "logs-workflow-*"
	}

	pollInterval := time.Second
	if v, ok := GetEnv("WORKFLOW_VIS_POLL_INTERVAL_MS"); ok && strings.TrimSpace(v) != "" {
		ms, err := strconv.Atoi(v)
		if err != nil || ms < 200 {
			return ingestVisConfig{}, false, "环境变量 WORKFLOW_VIS_POLL_INTERVAL_MS 必须为 >=200 的整数（毫秒）"
		}
		pollInterval = time.Duration(ms) * time.Millisecond
	}

	defaultWindow := 10 * time.Minute
	if v, ok := GetEnv("WORKFLOW_VIS_DEFAULT_WINDOW_SEC"); ok && strings.TrimSpace(v) != "" {
		sec, err := strconv.Atoi(v)
		if err != nil || sec < 10 || sec > 86400 {
			return ingestVisConfig{}, false, "环境变量 WORKFLOW_VIS_DEFAULT_WINDOW_SEC 必须为 10~86400 的整数（秒）"
		}
		defaultWindow = time.Duration(sec) * time.Second
	}

	cacheTTL := 30 * time.Minute
	if v, ok := GetEnv("WORKFLOW_VIS_CACHE_TTL_SEC"); ok && strings.TrimSpace(v) != "" {
		sec, err := strconv.Atoi(v)
		if err != nil || sec < 60 || sec > 604800 {
			return ingestVisConfig{}, false, "环境变量 WORKFLOW_VIS_CACHE_TTL_SEC 必须为 60~604800 的整数（秒）"
		}
		cacheTTL = time.Duration(sec) * time.Second
	}

	maxEvents := 2000
	if v, ok := GetEnv("WORKFLOW_VIS_MAX_EVENTS"); ok && strings.TrimSpace(v) != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 100 || n > 20000 {
			return ingestVisConfig{}, false, "环境变量 WORKFLOW_VIS_MAX_EVENTS 必须为 100~20000 的整数"
		}
		maxEvents = n
	}

	cfg = ingestVisConfig{
		EsURL:         strings.TrimSpace(esURL),
		EsUsername:    strings.TrimSpace(esUsername),
		EsPassword:    strings.TrimSpace(esPassword),
		IndexPattern:  strings.TrimSpace(indexPattern),
		PollInterval:  pollInterval,
		DefaultWindow: defaultWindow,
		CacheTTL:      cacheTTL,
		MaxEvents:     maxEvents,
	}
	return cfg, true, ""
}

func (s *ingestVisService) Start() {
	if !s.enabled {
		return
	}
	s.startOnce.Do(func() {
		s.mu.Lock()
		s.started = true
		if s.lastQueryAt.IsZero() {
			s.lastQueryAt = time.Now().Add(-s.cfg.DefaultWindow)
		}
		s.mu.Unlock()
		go s.pollLoop()
	})
}

func (s *ingestVisService) pollLoop() {
	ticker := time.NewTicker(s.cfg.PollInterval)
	defer ticker.Stop()

	for {
		s.pollOnce(context.Background())
		<-ticker.C
	}
}

type esSearchResponse struct {
	Hits struct {
		Hits []struct {
			Index  string          `json:"_index"`
			ID     string          `json:"_id"`
			Source json.RawMessage `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type esWorkflowEventSource struct {
	Timestamp string `json:"@timestamp"`
	Service   struct {
		Name string `json:"name"`
	} `json:"service"`
	Event struct {
		Action  string `json:"action"`
		Outcome string `json:"outcome"`
		Dataset string `json:"dataset"`
	} `json:"event"`
	Workflow struct {
		ItemCode    string `json:"item_code"`
		TraceId     string `json:"trace_id"`
		ProcessType any    `json:"process_type"`
		Ingest      string `json:"ingest"`
		InsertType  any    `json:"insert_type"`
		Force       any    `json:"force"`
		FileType    any    `json:"file_type"`
		TaskId      string `json:"task_id"`
		InferTaskId string `json:"infer_task_id"`
		InferType   any    `json:"infer_type"`
		InferResult string `json:"infer_result_path"`
	} `json:"workflow"`
	Message string `json:"message"`
	Error   struct {
		Message    string `json:"message"`
		Type       string `json:"type"`
		StackTrace string `json:"stack_trace"`
	} `json:"error"`
	RabbitMQ struct {
		Queue       string `json:"queue"`
		DeliveryTag any    `json:"delivery_tag"`
		Redelivered *bool  `json:"redelivered"`
		Requeue     *bool  `json:"requeue"`
	} `json:"rabbitmq"`
}

func (s *ingestVisService) pollOnce(ctx context.Context) {
	s.mu.RLock()
	queryFrom := s.lastQueryAt
	s.mu.RUnlock()

	queryTo := time.Now()

	events, truncated, err := s.searchWorkflowEvents(ctx, ingestVisSearchParams{
		from:     queryFrom,
		to:       queryTo,
		size:     s.cfg.MaxEvents,
		sortAsc:  true,
		itemCode: "",
		traceId:  "",
	})
	if err != nil {
		s.mu.Lock()
		s.lastPollAt = time.Now()
		s.lastErrorAt = time.Now()
		s.lastError = err.Error()
		s.mu.Unlock()

		s.hub.Logger().Error(
			"ingestVis poll ES failed",
			"logger",
			"hub",
			"module",
			"ingest-vis",
			"es_url",
			s.cfg.EsURL,
			"index",
			s.cfg.IndexPattern,
			"from",
			queryFrom.Format(time.RFC3339Nano),
			"to",
			queryTo.Format(time.RFC3339Nano),
			"err",
			err,
		)
		return
	}

	now := time.Now()
	var lastEventTime time.Time
	seenNew := 0

	s.mu.Lock()
	s.lastPollAt = now
	s.truncated = truncated
	s.lastError = ""
	s.lastErrorAt = time.Time{}

	for _, e := range events {
		if e.ItemCode == "" {
			continue
		}

		if expiresAt, ok := s.seen[e.Id]; ok && expiresAt.After(now) {
			continue
		}
		s.seen[e.Id] = now.Add(s.cfg.CacheTTL)
		seenNew++

		ts, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err != nil {
			s.lastErrorAt = now
			s.lastError = fmt.Sprintf("解析 ES @timestamp 失败: %v (timestamp=%q, id=%q)", err, e.Timestamp, e.Id)
			continue
		}

		if ts.After(lastEventTime) {
			lastEventTime = ts
		}

		runKey := ingestRunKey(e.ItemCode, e.TraceId)
		stage := ingestStageForEvent(e)
		status := ingestStatusForEvent(e)

		rs, ok := s.runs[runKey]
		if !ok {
			rs = &ingestVisRunState{
				itemCode: e.ItemCode,
				traceId:  e.TraceId,
			}
			s.runs[runKey] = rs
		}

		rs.stage = stage
		rs.status = status
		rs.lastEvent = e
		rs.lastSeenAt = ts
		rs.expiresAt = now.Add(s.cfg.CacheTTL)
	}

	s.cleanupLocked(now)

	if !lastEventTime.IsZero() && lastEventTime.After(s.lastQueryAt) {
		s.lastQueryAt = lastEventTime
	}
	minQueryAt := queryTo.Add(-s.cfg.DefaultWindow)
	if s.lastQueryAt.Before(minQueryAt) {
		s.lastQueryAt = minQueryAt
	}
	s.mu.Unlock()
}

func (s *ingestVisService) cleanupLocked(now time.Time) {
	for k, expiresAt := range s.seen {
		if expiresAt.Before(now) {
			delete(s.seen, k)
		}
	}
	for k, rs := range s.runs {
		if rs.expiresAt.Before(now) {
			delete(s.runs, k)
		}
	}
}

type ingestVisSearchParams struct {
	from    time.Time
	to      time.Time
	size    int
	sortAsc bool

	itemCode string
	traceId  string
}

func (s *ingestVisService) searchWorkflowEvents(ctx context.Context, p ingestVisSearchParams) ([]ingestVisEventDTO, bool, error) {
	if !s.enabled {
		if s.cfgErr != "" {
			return nil, false, errors.New(s.cfgErr)
		}
		return nil, false, errors.New("ingest-vis 未启用")
	}

	searchURL, err := buildEsSearchURL(s.cfg.EsURL, s.cfg.IndexPattern)
	if err != nil {
		return nil, false, err
	}

	filters := []any{
		map[string]any{"term": map[string]any{"event.dataset": "workflow"}},
		map[string]any{"range": map[string]any{"@timestamp": map[string]any{
			"gte": p.from.Format(time.RFC3339Nano),
			"lte": p.to.Format(time.RFC3339Nano),
		}}},
	}
	if strings.TrimSpace(p.itemCode) != "" {
		filters = append(filters, map[string]any{"term": map[string]any{"workflow.item_code": strings.TrimSpace(p.itemCode)}})
	}
	if strings.TrimSpace(p.traceId) != "" {
		filters = append(filters, map[string]any{"term": map[string]any{"workflow.trace_id": strings.TrimSpace(p.traceId)}})
	}

	sortOrder := "asc"
	if !p.sortAsc {
		sortOrder = "desc"
	}

	body := map[string]any{
		"size": p.size,
		"sort": []any{
			map[string]any{"@timestamp": map[string]any{"order": sortOrder}},
		},
		"query": map[string]any{
			"bool": map[string]any{
				"filter": filters,
			},
		},
	}

	raw, err := json.Marshal(body)
	if err != nil {
		return nil, false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, searchURL, bytes.NewReader(raw))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.cfg.EsUsername != "" && s.cfg.EsPassword != "" {
		req.SetBasicAuth(s.cfg.EsUsername, s.cfg.EsPassword)
	}

	res, err := s.client.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, false, err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, false, fmt.Errorf("ES 查询失败: status=%d body=%s", res.StatusCode, string(resBody))
	}

	var parsed esSearchResponse
	if err := json.Unmarshal(resBody, &parsed); err != nil {
		return nil, false, err
	}

	events := make([]ingestVisEventDTO, 0, len(parsed.Hits.Hits))
	for _, hit := range parsed.Hits.Hits {
		var src esWorkflowEventSource
		if err := json.Unmarshal(hit.Source, &src); err != nil {
			return nil, false, fmt.Errorf("解析 ES _source 失败: %w", err)
		}

		id := hit.Index + ":" + hit.ID
		processType := coerceIntPtr(src.Workflow.ProcessType)
		force := coerceIntPtr(src.Workflow.Force)
		fileType := coerceIntPtr(src.Workflow.FileType)
		inferType := coerceIntPtr(src.Workflow.InferType)
		insertType := coerceIntSlice(src.Workflow.InsertType)

		rabbitMQDeliveryTag := coerceInt64Ptr(src.RabbitMQ.DeliveryTag)

		events = append(events, ingestVisEventDTO{
			Id:                  id,
			Timestamp:           src.Timestamp,
			Service:             src.Service.Name,
			Action:              src.Event.Action,
			Outcome:             src.Event.Outcome,
			ItemCode:            src.Workflow.ItemCode,
			TraceId:             src.Workflow.TraceId,
			Message:             src.Message,
			ProcessType:         processType,
			Ingest:              strings.TrimSpace(src.Workflow.Ingest),
			InsertType:          insertType,
			Force:               force,
			FileType:            fileType,
			TaskId:              strings.TrimSpace(src.Workflow.TaskId),
			InferTaskId:         strings.TrimSpace(src.Workflow.InferTaskId),
			InferResultPath:     strings.TrimSpace(src.Workflow.InferResult),
			InferType:           inferType,
			RabbitMQQueue:       strings.TrimSpace(src.RabbitMQ.Queue),
			RabbitMQDeliveryTag: rabbitMQDeliveryTag,
			RabbitMQRedelivered: src.RabbitMQ.Redelivered,
			RabbitMQRequeue:     src.RabbitMQ.Requeue,
			ErrorMessage:        src.Error.Message,
			ErrorType:           src.Error.Type,
			ErrorStackTrace:     src.Error.StackTrace,
		})
	}

	return events, len(parsed.Hits.Hits) >= p.size, nil
}

func buildEsSearchURL(baseURL string, indexPattern string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		return "", errors.New("WORKFLOW_ES_URL 必须包含 scheme（例如 http://127.0.0.1:9200）")
	}
	if strings.TrimSpace(indexPattern) == "" {
		return "", errors.New("WORKFLOW_ES_INDEX 不能为空")
	}

	base := strings.TrimRight(u.Path, "/")
	u.Path = base + "/" + strings.TrimLeft(indexPattern, "/") + "/_search"
	return u.String(), nil
}

func ingestRunKey(itemCode string, traceId string) string {
	return itemCode + ":" + traceId
}

func coerceIntPtr(v any) *int {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case float64:
		if math.Trunc(t) != t {
			return nil
		}
		n := int(t)
		return &n
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		return &n
	case json.Number:
		i64, err := t.Int64()
		if err != nil {
			return nil
		}
		n := int(i64)
		return &n
	default:
		return nil
	}
}

func coerceInt64Ptr(v any) *int64 {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case float64:
		if math.Trunc(t) != t {
			return nil
		}
		n := int64(t)
		return &n
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return nil
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil
		}
		return &n
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return nil
		}
		return &n
	default:
		return nil
	}
}

func coerceIntSlice(v any) []int {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []any:
		out := make([]int, 0, len(t))
		for _, it := range t {
			n := coerceIntPtr(it)
			if n == nil {
				return nil
			}
			out = append(out, *n)
		}
		return out
	default:
		n := coerceIntPtr(t)
		if n == nil {
			return nil
		}
		return []int{*n}
	}
}

func shouldTreatAsStageSuccess(e ingestVisEventDTO) bool {
	if e.Outcome != "success" {
		return false
	}
	switch e.Action {
	case "minio.ingest.skip", "minio.upload_only.execute", "minio.query.execute", "es.write":
		return true
	case "minio.task.end":
		// 当不需要推理时，minio-api 的链路到此为止（阶段性出厂）。
		// insertType 为空数组/缺失时视为“无推理”。
		return len(e.InsertType) == 0
	default:
		return false
	}
}

func ingestStatusForEvent(e ingestVisEventDTO) string {
	if e.Outcome == "failure" {
		return "failure"
	}
	if shouldTreatAsStageSuccess(e) {
		return "success"
	}
	return "running"
}

func ingestStageForEvent(e ingestVisEventDTO) string {
	if e.Outcome == "failure" {
		return "trash"
	}
	if shouldTreatAsStageSuccess(e) {
		return "out"
	}
	switch {
	case strings.HasPrefix(e.Action, "mq."):
		return "mq"
	case strings.HasPrefix(e.Action, "minio."):
		return "minio"
	case strings.HasPrefix(e.Action, "infer."):
		return "infer"
	case strings.HasPrefix(e.Action, "es."):
		return "es"
	default:
		return "other"
	}
}

func (h *Hub) getIngestVisRuns(e *core.RequestEvent) error {
	if h.ingest == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-vis 未初始化"})
	}
	if !h.ingest.enabled {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": h.ingest.cfgErr})
	}
	h.ingest.Start()

	query := e.Request.URL.Query()
	windowSec := query.Get("windowSec")
	limitStr := query.Get("limit")

	window := h.ingest.cfg.DefaultWindow
	if strings.TrimSpace(windowSec) != "" {
		sec, err := strconv.Atoi(windowSec)
		if err != nil || sec < 10 || sec > 86400 {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "windowSec 必须为 10~86400 的整数（秒）"})
		}
		window = time.Duration(sec) * time.Second
	}

	limit := 200
	if strings.TrimSpace(limitStr) != "" {
		n, err := strconv.Atoi(limitStr)
		if err != nil || n < 1 || n > 2000 {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "limit 必须为 1~2000 的整数"})
		}
		limit = n
	}

	now := time.Now()
	cutoff := now.Add(-window)

	h.ingest.mu.RLock()
	type runItem struct {
		dto        ingestVisRunDTO
		lastSeenAt time.Time
	}
	items := make([]runItem, 0, len(h.ingest.runs))
	for k, rs := range h.ingest.runs {
		if rs.lastSeenAt.Before(cutoff) {
			continue
		}
		items = append(items, runItem{
			dto: ingestVisRunDTO{
				Key:       k,
				ItemCode:  rs.itemCode,
				TraceId:   rs.traceId,
				Stage:     rs.stage,
				Status:    rs.status,
				LastEvent: rs.lastEvent,
			},
			lastSeenAt: rs.lastSeenAt,
		})
	}
	h.ingest.mu.RUnlock()

	sort.Slice(items, func(i, j int) bool {
		return items[i].lastSeenAt.After(items[j].lastSeenAt)
	})

	if len(items) > limit {
		items = items[:limit]
	}

	runs := make([]ingestVisRunDTO, 0, len(items))
	for _, it := range items {
		runs = append(runs, it.dto)
	}
	return e.JSON(http.StatusOK, map[string]any{"items": runs})
}

func (h *Hub) getIngestVisEvents(e *core.RequestEvent) error {
	if h.ingest == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-vis 未初始化"})
	}
	if !h.ingest.enabled {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": h.ingest.cfgErr})
	}
	h.ingest.Start()

	q := e.Request.URL.Query()
	itemCode := strings.TrimSpace(q.Get("itemCode"))
	traceId := strings.TrimSpace(q.Get("traceId"))
	if itemCode == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "itemCode 为必填"})
	}

	windowSec := strings.TrimSpace(q.Get("windowSec"))
	window := 24 * time.Hour
	if windowSec != "" {
		sec, err := strconv.Atoi(windowSec)
		if err != nil || sec < 60 || sec > 2592000 {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "windowSec 必须为 60~2592000 的整数（秒）"})
		}
		window = time.Duration(sec) * time.Second
	}

	limitStr := strings.TrimSpace(q.Get("limit"))
	limit := 5000
	if limitStr != "" {
		n, err := strconv.Atoi(limitStr)
		if err != nil || n < 1 || n > 20000 {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "limit 必须为 1~20000 的整数"})
		}
		limit = n
	}

	now := time.Now()
	from := now.Add(-window)
	events, truncated, err := h.ingest.searchWorkflowEvents(e.Request.Context(), ingestVisSearchParams{
		from:     from,
		to:       now,
		size:     limit,
		sortAsc:  true,
		itemCode: itemCode,
		traceId:  traceId,
	})
	if err != nil {
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"items": events, "truncated": truncated})
}

func (h *Hub) getIngestVisCacheStatus(e *core.RequestEvent) error {
	if h.ingest == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-vis 未初始化"})
	}
	if !h.ingest.enabled {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": h.ingest.cfgErr})
	}
	h.ingest.Start()

	h.ingest.mu.RLock()
	status := map[string]any{
		"enabled":          h.ingest.enabled,
		"started":          h.ingest.started,
		"pollIntervalMs":   int(h.ingest.cfg.PollInterval / time.Millisecond),
		"defaultWindowSec": int(h.ingest.cfg.DefaultWindow / time.Second),
		"cacheTtlSec":      int(h.ingest.cfg.CacheTTL / time.Second),
		"maxEvents":        h.ingest.cfg.MaxEvents,
		"index":            h.ingest.cfg.IndexPattern,
		"lastQueryAt":      h.ingest.lastQueryAt.Format(time.RFC3339Nano),
		"lastPollAt":       h.ingest.lastPollAt.Format(time.RFC3339Nano),
		"lastErrorAt":      h.ingest.lastErrorAt.Format(time.RFC3339Nano),
		"lastError":        h.ingest.lastError,
		"truncated":        h.ingest.truncated,
		"runsCount":        len(h.ingest.runs),
		"seenCount":        len(h.ingest.seen),
	}
	h.ingest.mu.RUnlock()

	return e.JSON(http.StatusOK, status)
}

func (h *Hub) clearIngestVisCache(e *core.RequestEvent) error {
	if h.ingest == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-vis 未初始化"})
	}
	if !h.ingest.enabled {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": h.ingest.cfgErr})
	}
	// RBAC: only admin / non-readonly allowed
	if e.Auth == nil || e.Auth.GetString("role") == "readonly" {
		return e.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}

	h.ingest.mu.Lock()
	h.ingest.seen = map[string]time.Time{}
	h.ingest.runs = map[string]*ingestVisRunState{}
	h.ingest.lastQueryAt = time.Now().Add(-h.ingest.cfg.DefaultWindow)
	h.ingest.lastPollAt = time.Time{}
	h.ingest.lastErrorAt = time.Time{}
	h.ingest.lastError = ""
	h.ingest.truncated = false
	h.ingest.mu.Unlock()

	return e.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

package hub

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/pocketbase/pocketbase/core"
)

const (
	ingestMonitorDefaultPort      = 5432
	ingestMonitorDefaultSSLMode   = "disable"
	ingestMonitorRecentLimit      = 20
	ingestMonitorFailureLimit     = 20
	ingestMonitorQueryTimeout     = 10 * time.Second
	ingestMonitorApplicationName  = "aether_ingest_monitor"
	ingestMonitorFormalRecordType = "formal_ingest"
	ingestMonitorStatusSuccess    = "success"
	ingestMonitorStatusFailure    = "failure"
	ingestMonitorStatusPending    = "pending"
	ingestMonitorStatusUnknown    = "unknown"
	ingestMonitorFormalFilter     = `
WHERE COALESCE(is_deleted, false) = false
	AND COALESCE(is_temporary, false) = false
`
	ingestMonitorSummaryStatusQuery = `
SELECT
	COUNT(*) FILTER (WHERE status = 'success') AS success,
	COUNT(*) FILTER (WHERE status = 'failure') AS failure,
	COUNT(*) FILTER (WHERE status = 'pending') AS pending,
	COUNT(*) FILTER (WHERE status = 'unknown') AS unknown,
	COUNT(*) AS total
FROM (
	SELECT
		CASE
			WHEN is_complete = -1 OR COALESCE(error_msg, '') <> '' THEN 'failure'
			WHEN is_complete = 1
				AND COALESCE(source_file_path, '') <> ''
				AND COALESCE(converted_file_path, '') <> ''
				AND COALESCE(pc_address, '') <> ''
				AND COALESCE(glb_address, '') <> '' THEN 'success'
		WHEN is_complete = 2 THEN 'pending'
		ELSE 'unknown'
		END AS status
	FROM product_info
` + ingestMonitorFormalFilter + `
) records
`
)

type ingestMonitorConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Tenant   string
	SSLMode  string
}

type ingestMonitorConfigError struct {
	message string
}

func (e *ingestMonitorConfigError) Error() string {
	return e.message
}

type ingestMonitorService struct {
	hub *Hub

	mu  sync.Mutex
	db  *sql.DB
	cfg *ingestMonitorConfig
}

type ingestMonitorScopeDTO struct {
	Tenant     string `json:"tenant"`
	RecordType string `json:"recordType"`
}

type ingestMonitorSummaryCountsDTO struct {
	Total   int `json:"total"`
	Success int `json:"success"`
	Failure int `json:"failure"`
	Pending int `json:"pending"`
	Unknown int `json:"unknown"`
}

type ingestMonitorRecordDTO struct {
	ItemCode          string `json:"itemCode"`
	ProductName       string `json:"productName"`
	Status            string `json:"status"`
	IsComplete        *int   `json:"isComplete,omitempty"`
	IsTemporary       bool   `json:"isTemporary"`
	ErrorMsg          string `json:"errorMsg"`
	SourceFilePath    string `json:"sourceFilePath"`
	ConvertedFilePath string `json:"convertedFilePath"`
	PcAddress         string `json:"pcAddress"`
	GlbAddress        string `json:"glbAddress"`
	HasSourceFilePath bool   `json:"hasSourceFilePath"`
	HasConvertedFile  bool   `json:"hasConvertedFile"`
	HasPcAddress      bool   `json:"hasPcAddress"`
	HasGlbAddress     bool   `json:"hasGlbAddress"`
	PathReadyCount    int    `json:"pathReadyCount"`
	PathReadyTotal    int    `json:"pathReadyTotal"`
	InferenceTypes    []int  `json:"inferenceTypes"`
	UpdateTime        string `json:"updateTime"`
	CreateTime        string `json:"createTime"`
}

type ingestMonitorSummaryResponse struct {
	Scope    ingestMonitorScopeDTO         `json:"scope"`
	Summary  ingestMonitorSummaryCountsDTO `json:"summary"`
	Recent   []ingestMonitorRecordDTO      `json:"recent"`
	Failures []ingestMonitorRecordDTO      `json:"failures"`
}

type ingestMonitorDetailResponse struct {
	Scope ingestMonitorScopeDTO  `json:"scope"`
	Item  ingestMonitorRecordDTO `json:"item"`
}

type sqlScanner interface {
	Scan(dest ...any) error
}

func newIngestMonitorService(hub *Hub) *ingestMonitorService {
	return &ingestMonitorService{hub: hub}
}

func loadIngestMonitorConfig() (ingestMonitorConfig, error) {
	getRequired := func(key string) (string, error) {
		value, _ := GetEnv(key)
		value = strings.TrimSpace(value)
		if value == "" {
			return "", &ingestMonitorConfigError{
				message: fmt.Sprintf("缺少环境变量 AETHER_HUB_%s（或 %s）", key, key),
			}
		}
		return value, nil
	}

	host, err := getRequired("INGEST_MONITOR_PG_HOST")
	if err != nil {
		return ingestMonitorConfig{}, err
	}
	user, err := getRequired("INGEST_MONITOR_PG_USER")
	if err != nil {
		return ingestMonitorConfig{}, err
	}
	database, err := getRequired("INGEST_MONITOR_PG_DATABASE")
	if err != nil {
		return ingestMonitorConfig{}, err
	}
	tenant, err := getRequired("INGEST_MONITOR_PG_TENANT")
	if err != nil {
		return ingestMonitorConfig{}, err
	}

	port := ingestMonitorDefaultPort
	if rawPort, ok := GetEnv("INGEST_MONITOR_PG_PORT"); ok && strings.TrimSpace(rawPort) != "" {
		parsedPort, parseErr := strconv.Atoi(strings.TrimSpace(rawPort))
		if parseErr != nil || parsedPort <= 0 || parsedPort > 65535 {
			return ingestMonitorConfig{}, &ingestMonitorConfigError{
				message: "环境变量 INGEST_MONITOR_PG_PORT 必须为 1~65535 的整数",
			}
		}
		port = parsedPort
	}

	sslMode := ingestMonitorDefaultSSLMode
	if rawSSLMode, ok := GetEnv("INGEST_MONITOR_PG_SSLMODE"); ok && strings.TrimSpace(rawSSLMode) != "" {
		sslMode = strings.TrimSpace(rawSSLMode)
	}

	password, _ := GetEnv("INGEST_MONITOR_PG_PASSWORD")

	return ingestMonitorConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: strings.TrimSpace(password),
		Database: database,
		Tenant:   tenant,
		SSLMode:  sslMode,
	}, nil
}

func (s *ingestMonitorService) getDB(ctx context.Context) (*sql.DB, ingestMonitorConfig, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.db != nil && s.cfg != nil {
		return s.db, *s.cfg, nil
	}

	cfg, err := loadIngestMonitorConfig()
	if err != nil {
		return nil, ingestMonitorConfig{}, err
	}

	dsnURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		Path:   cfg.Database,
	}

	query := dsnURL.Query()
	query.Set("sslmode", cfg.SSLMode)
	query.Set("application_name", ingestMonitorApplicationName)
	dsnURL.RawQuery = query.Encode()

	db, err := sql.Open("postgres", dsnURL.String())
	if err != nil {
		return nil, ingestMonitorConfig{}, err
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(10 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, ingestMonitorQueryTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, ingestMonitorConfig{}, err
	}

	s.db = db
	s.cfg = &cfg
	return s.db, cfg, nil
}

// DB returns the underlying *sql.DB connection.
func (s *ingestMonitorService) DB() *sql.DB {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db
}

func (s *ingestMonitorService) withTenantTx(ctx context.Context, fn func(*sql.Tx, ingestMonitorConfig, context.Context) error) error {
	return s.withTenantTxOptions(ctx, true, fn)
}

func (s *ingestMonitorService) withTenantTxWrite(ctx context.Context, fn func(*sql.Tx, ingestMonitorConfig, context.Context) error) error {
	return s.withTenantTxOptions(ctx, false, fn)
}

func (s *ingestMonitorService) withTenantTxOptions(ctx context.Context, readOnly bool, fn func(*sql.Tx, ingestMonitorConfig, context.Context) error) error {
	db, cfg, err := s.getDB(ctx)
	if err != nil {
		return err
	}

	queryCtx, cancel := context.WithTimeout(ctx, ingestMonitorQueryTimeout)
	defer cancel()

	tx, err := db.BeginTx(queryCtx, &sql.TxOptions{ReadOnly: readOnly})
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(queryCtx, "SELECT set_config('app.current_tenant', $1, true)", cfg.Tenant); err != nil {
		return err
	}

	if err := fn(tx, cfg, queryCtx); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *ingestMonitorService) GetSummary(ctx context.Context) (*ingestMonitorSummaryResponse, error) {
	response := &ingestMonitorSummaryResponse{}

	err := s.withTenantTx(ctx, func(tx *sql.Tx, cfg ingestMonitorConfig, queryCtx context.Context) error {
		response.Scope = ingestMonitorScopeDTO{
			Tenant:     cfg.Tenant,
			RecordType: ingestMonitorFormalRecordType,
		}

		row := tx.QueryRowContext(queryCtx, ingestMonitorSummaryStatusQuery)
		if err := row.Scan(
			&response.Summary.Success,
			&response.Summary.Failure,
			&response.Summary.Pending,
			&response.Summary.Unknown,
			&response.Summary.Total,
		); err != nil {
			return err
		}

		recent, err := listIngestMonitorRecords(tx, queryCtx, "", ingestMonitorRecentLimit)
		if err != nil {
			return err
		}
		response.Recent = recent

		failures, err := listIngestMonitorRecords(tx, queryCtx, ingestMonitorStatusFailure, ingestMonitorFailureLimit)
		if err != nil {
			return err
		}
		response.Failures = failures
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (s *ingestMonitorService) GetDetail(ctx context.Context, itemCode string) (*ingestMonitorDetailResponse, error) {
	response := &ingestMonitorDetailResponse{}

	err := s.withTenantTx(ctx, func(tx *sql.Tx, cfg ingestMonitorConfig, queryCtx context.Context) error {
		response.Scope = ingestMonitorScopeDTO{
			Tenant:     cfg.Tenant,
			RecordType: ingestMonitorFormalRecordType,
		}

		record, err := getIngestMonitorRecordByItemCode(tx, queryCtx, itemCode)
		if err != nil {
			return err
		}
		response.Item = record
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func listIngestMonitorRecords(tx *sql.Tx, ctx context.Context, statusFilter string, limit int) ([]ingestMonitorRecordDTO, error) {
	baseQuery := `
SELECT
	item_code,
	COALESCE(product_name, '') AS product_name,
	is_complete,
	is_temporary,
	COALESCE(error_msg, '') AS error_msg,
	COALESCE(source_file_path, '') AS source_file_path,
	COALESCE(converted_file_path, '') AS converted_file_path,
	COALESCE(pc_address, '') AS pc_address,
	COALESCE(glb_address, '') AS glb_address,
	COALESCE(inference_types, '') AS inference_types,
	update_time,
	create_time,
	CASE
		WHEN is_complete = -1 OR COALESCE(error_msg, '') <> '' THEN 'failure'
		WHEN is_complete = 1
			AND COALESCE(source_file_path, '') <> ''
			AND COALESCE(converted_file_path, '') <> ''
			AND COALESCE(pc_address, '') <> ''
			AND COALESCE(glb_address, '') <> '' THEN 'success'
		WHEN is_complete = 2 THEN 'pending'
		ELSE 'unknown'
	END AS status
	FROM product_info
` + ingestMonitorFormalFilter

	args := []any{}
	if statusFilter != "" {
		baseQuery = `
SELECT *
FROM (` + baseQuery + `
) records
WHERE status = $1
`
		args = append(args, statusFilter)
	}

	baseQuery += `
ORDER BY update_time DESC NULLS LAST, create_time DESC NULLS LAST, item_code DESC
LIMIT $` + strconv.Itoa(len(args)+1)
	args = append(args, limit)

	rows, err := tx.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ingestMonitorRecordDTO, 0, limit)
	for rows.Next() {
		record, err := scanIngestMonitorRecord(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func getIngestMonitorRecordByItemCode(tx *sql.Tx, ctx context.Context, itemCode string) (ingestMonitorRecordDTO, error) {
	row := tx.QueryRowContext(ctx, `
SELECT
	item_code,
	COALESCE(product_name, '') AS product_name,
	is_complete,
	is_temporary,
	COALESCE(error_msg, '') AS error_msg,
	COALESCE(source_file_path, '') AS source_file_path,
	COALESCE(converted_file_path, '') AS converted_file_path,
	COALESCE(pc_address, '') AS pc_address,
	COALESCE(glb_address, '') AS glb_address,
	COALESCE(inference_types, '') AS inference_types,
	update_time,
	create_time,
	CASE
		WHEN is_complete = -1 OR COALESCE(error_msg, '') <> '' THEN 'failure'
		WHEN is_complete = 1
			AND COALESCE(source_file_path, '') <> ''
			AND COALESCE(converted_file_path, '') <> ''
			AND COALESCE(pc_address, '') <> ''
			AND COALESCE(glb_address, '') <> '' THEN 'success'
		WHEN is_complete = 2 THEN 'pending'
		ELSE 'unknown'
	END AS status
FROM product_info
`+ingestMonitorFormalFilter+`
	AND item_code = $1
ORDER BY update_time DESC NULLS LAST, create_time DESC NULLS LAST
LIMIT 1
`, itemCode)

	return scanIngestMonitorRecord(row)
}

func scanIngestMonitorRecord(scanner sqlScanner) (ingestMonitorRecordDTO, error) {
	var (
		itemCode          string
		productName       string
		isComplete        sql.NullInt64
		isTemporary       sql.NullBool
		errorMsg          string
		sourceFilePath    string
		convertedFilePath string
		pcAddress         string
		glbAddress        string
		inferenceTypesRaw string
		updateTime        sql.NullTime
		createTime        sql.NullTime
		status            string
	)

	if err := scanner.Scan(
		&itemCode,
		&productName,
		&isComplete,
		&isTemporary,
		&errorMsg,
		&sourceFilePath,
		&convertedFilePath,
		&pcAddress,
		&glbAddress,
		&inferenceTypesRaw,
		&updateTime,
		&createTime,
		&status,
	); err != nil {
		return ingestMonitorRecordDTO{}, err
	}

	record := ingestMonitorRecordDTO{
		ItemCode:          itemCode,
		ProductName:       productName,
		Status:            normalizeIngestMonitorStatus(status),
		IsTemporary:       isTemporary.Valid && isTemporary.Bool,
		ErrorMsg:          errorMsg,
		SourceFilePath:    sourceFilePath,
		ConvertedFilePath: convertedFilePath,
		PcAddress:         pcAddress,
		GlbAddress:        glbAddress,
		HasSourceFilePath: sourceFilePath != "",
		HasConvertedFile:  convertedFilePath != "",
		HasPcAddress:      pcAddress != "",
		HasGlbAddress:     glbAddress != "",
		PathReadyTotal:    4,
		InferenceTypes:    parseInferenceTypes(inferenceTypesRaw),
		UpdateTime:        formatNullableTime(updateTime),
		CreateTime:        formatNullableTime(createTime),
	}

	if isComplete.Valid {
		value := int(isComplete.Int64)
		record.IsComplete = &value
	}

	if record.HasSourceFilePath {
		record.PathReadyCount++
	}
	if record.HasConvertedFile {
		record.PathReadyCount++
	}
	if record.HasPcAddress {
		record.PathReadyCount++
	}
	if record.HasGlbAddress {
		record.PathReadyCount++
	}

	return record, nil
}

func normalizeIngestMonitorStatus(status string) string {
	switch status {
	case ingestMonitorStatusSuccess, ingestMonitorStatusFailure, ingestMonitorStatusPending:
		return status
	default:
		return ingestMonitorStatusUnknown
	}
}

func parseInferenceTypes(raw string) []int {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "[")
	trimmed = strings.TrimSuffix(trimmed, "]")
	if trimmed == "" {
		return nil
	}

	parts := strings.Split(trimmed, ",")
	items := make([]int, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		number, err := strconv.Atoi(value)
		if err != nil {
			continue
		}
		items = append(items, number)
	}

	if len(items) == 0 {
		return nil
	}
	return items
}

func formatNullableTime(value sql.NullTime) string {
	if !value.Valid {
		return ""
	}
	return value.Time.UTC().Format(time.RFC3339)
}

func (h *Hub) getIngestMonitorSummary(e *core.RequestEvent) error {
	if h.ingestMonitor == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-monitor 未初始化"})
	}

	response, err := h.ingestMonitor.GetSummary(e.Request.Context())
	if err != nil {
		return h.handleIngestMonitorError(e, "get ingest monitor summary failed", err)
	}

	return e.JSON(http.StatusOK, response)
}

func (h *Hub) getIngestMonitorDetail(e *core.RequestEvent) error {
	if h.ingestMonitor == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-monitor 未初始化"})
	}

	itemCode := strings.TrimSpace(e.Request.URL.Query().Get("itemCode"))
	if itemCode == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "itemCode 参数不能为空"})
	}

	response, err := h.ingestMonitor.GetDetail(e.Request.Context(), itemCode)
	if err != nil {
		return h.handleIngestMonitorError(e, "get ingest monitor detail failed", err)
	}

	return e.JSON(http.StatusOK, response)
}

func (h *Hub) handleIngestMonitorError(e *core.RequestEvent, logMessage string, err error) error {
	var cfgErr *ingestMonitorConfigError
	switch {
	case errors.As(err, &cfgErr):
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": cfgErr.Error()})
	case errors.Is(err, sql.ErrNoRows):
		return e.JSON(http.StatusNotFound, map[string]string{"error": "未找到对应的正式入库记录"})
	default:
		h.Logger().Error(logMessage, "logger", "hub", "err", err)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "查询入库状态失败"})
	}
}

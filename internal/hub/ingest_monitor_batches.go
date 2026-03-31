package hub

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

const (
	ingestMonitorBatchRecordType = "ingest_batch"
	ingestMonitorBatchListLimit  = 20
	ingestMonitorBatchStatusCase = `
CASE
	WHEN p.is_complete = -1
		OR COALESCE(p.error_msg, '') <> ''
		OR COALESCE(c.process_status, '') IN ('failed', 'error') THEN 'failure'
	WHEN p.is_complete = 1
		AND COALESCE(p.source_file_path, '') <> ''
		AND COALESCE(p.converted_file_path, '') <> ''
		AND COALESCE(p.pc_address, '') <> ''
		AND COALESCE(p.glb_address, '') <> '' THEN 'success'
	ELSE 'pending'
END
`
	ingestMonitorBatchTerminalTimeCase = `
CASE
	WHEN p.is_complete = -1
		OR COALESCE(p.error_msg, '') <> '' THEN p.update_time
	WHEN p.is_complete = 1
		AND COALESCE(p.source_file_path, '') <> ''
		AND COALESCE(p.converted_file_path, '') <> ''
		AND COALESCE(p.pc_address, '') <> ''
		AND COALESCE(p.glb_address, '') <> '' THEN p.update_time
	WHEN COALESCE(c.process_status, '') IN ('failed', 'error') THEN c.process_end_time
	ELSE NULL
END
`
)

type ingestMonitorBatchDTO struct {
	BatchRunID               string   `json:"batchRunId"`
	SourceType               string   `json:"sourceType"`
	XXLJobID                 string   `json:"xxlJobId"`
	XXLLogID                 string   `json:"xxlLogId"`
	ScanPaths                []string `json:"scanPaths"`
	FileType                 *int     `json:"fileType,omitempty"`
	BatchSize                int      `json:"batchSize"`
	Force                    bool     `json:"force"`
	Status                   string   `json:"status"`
	ErrorMessage             string   `json:"errorMessage"`
	ScanStartedAt            string   `json:"scanStartedAt"`
	ScanFinishedAt           string   `json:"scanFinishedAt"`
	XXLScanElapsedSeconds    *float64 `json:"xxlScanElapsedSeconds,omitempty"`
	FinalIngestElapsedSecond *float64 `json:"finalIngestElapsedSeconds,omitempty"`
	TotalDirsScanned         int      `json:"totalDirsScanned"`
	TotalFilesScanned        int      `json:"totalFilesScanned"`
	TotalFilesFiltered       int      `json:"totalFilesFiltered"`
	TotalFilesProcessed      int      `json:"totalFilesProcessed"`
	TotalBatches             int      `json:"totalBatches"`
	TotalTracked             int      `json:"totalTracked"`
	SuccessCount             int      `json:"successCount"`
	FailureCount             int      `json:"failureCount"`
	PendingCount             int      `json:"pendingCount"`
}

type ingestMonitorBatchItemDTO struct {
	ItemCode          string `json:"itemCode"`
	CadNumber         string `json:"cadNumber"`
	FileName          string `json:"fileName"`
	ProcessStatus     string `json:"processStatus"`
	IngestStatus      string `json:"ingestStatus"`
	ProductName       string `json:"productName"`
	IsComplete        *int   `json:"isComplete,omitempty"`
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
	ProcessStartTime  string `json:"processStartTime"`
	ProcessEndTime    string `json:"processEndTime"`
	ProductUpdateTime string `json:"productUpdateTime"`
	UpdateTime        string `json:"updateTime"`
	CreateTime        string `json:"createTime"`
}

type ingestMonitorBatchListResponse struct {
	Scope   ingestMonitorScopeDTO   `json:"scope"`
	Batches []ingestMonitorBatchDTO `json:"batches"`
}

type ingestMonitorBatchDetailResponse struct {
	Scope ingestMonitorScopeDTO       `json:"scope"`
	Batch ingestMonitorBatchDTO       `json:"batch"`
	Items []ingestMonitorBatchItemDTO `json:"items"`
}

func (s *ingestMonitorService) GetBatchList(ctx context.Context) (*ingestMonitorBatchListResponse, error) {
	response := &ingestMonitorBatchListResponse{}

	err := s.withTenantTx(ctx, func(tx *sql.Tx, cfg ingestMonitorConfig, queryCtx context.Context) error {
		response.Scope = ingestMonitorScopeDTO{
			Tenant:     cfg.Tenant,
			RecordType: ingestMonitorBatchRecordType,
		}

		batches, err := listIngestMonitorBatches(tx, queryCtx, ingestMonitorBatchListLimit)
		if err != nil {
			return err
		}
		response.Batches = batches
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func (s *ingestMonitorService) GetBatchDetail(ctx context.Context, batchRunID string) (*ingestMonitorBatchDetailResponse, error) {
	response := &ingestMonitorBatchDetailResponse{}

	err := s.withTenantTx(ctx, func(tx *sql.Tx, cfg ingestMonitorConfig, queryCtx context.Context) error {
		response.Scope = ingestMonitorScopeDTO{
			Tenant:     cfg.Tenant,
			RecordType: ingestMonitorBatchRecordType,
		}

		batch, err := getIngestMonitorBatchByID(tx, queryCtx, batchRunID)
		if err != nil {
			return err
		}
		response.Batch = batch

		items, err := listIngestMonitorBatchItems(tx, queryCtx, batchRunID)
		if err != nil {
			return err
		}
		response.Items = items
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

func listIngestMonitorBatches(tx *sql.Tx, ctx context.Context, limit int) ([]ingestMonitorBatchDTO, error) {
	query := fmt.Sprintf(`
WITH recent_batches AS (
	SELECT
		batch_run_id,
		source_type,
		COALESCE(xxl_job_id, '') AS xxl_job_id,
		COALESCE(xxl_log_id, '') AS xxl_log_id,
		COALESCE(scan_paths, '[]'::jsonb)::text AS scan_paths_json,
		file_type,
		batch_size,
		force,
		status,
		COALESCE(error_message, '') AS error_message,
		scan_started_at,
		scan_finished_at,
		xxl_elapsed_seconds,
		total_dirs_scanned,
		total_files_scanned,
		total_files_filtered,
		total_files_processed,
		total_batches,
		create_time
	FROM ingest_batch_run
	WHERE COALESCE(is_deleted, false) = false
	ORDER BY scan_started_at DESC NULLS LAST, create_time DESC NULLS LAST, batch_run_id DESC
	LIMIT $1
)
SELECT
	b.batch_run_id,
	b.source_type,
	b.xxl_job_id,
	b.xxl_log_id,
	b.scan_paths_json,
	b.file_type,
	b.batch_size,
	b.force,
	b.status,
	b.error_message,
	b.scan_started_at,
	b.scan_finished_at,
	b.xxl_elapsed_seconds,
	b.total_dirs_scanned,
	b.total_files_scanned,
	b.total_files_filtered,
	b.total_files_processed,
	b.total_batches,
	COUNT(c.item_code) AS total_tracked,
	COUNT(*) FILTER (WHERE c.item_code IS NOT NULL AND (%s) = 'success') AS success_count,
	COUNT(*) FILTER (WHERE c.item_code IS NOT NULL AND (%s) = 'failure') AS failure_count,
	COUNT(*) FILTER (WHERE c.item_code IS NOT NULL AND (%s) = 'pending') AS pending_count,
	EXTRACT(EPOCH FROM (MAX((%s)) - b.scan_started_at)) AS final_ingest_elapsed_seconds
FROM recent_batches b
LEFT JOIN cad_file_process_status c
	ON c.batch_run_id = b.batch_run_id
	AND COALESCE(c.is_deleted, 0) = 0
LEFT JOIN LATERAL (
	SELECT
		product_name,
		is_complete,
		error_msg,
		source_file_path,
		converted_file_path,
		pc_address,
		glb_address,
		update_time,
		create_time
	FROM product_info
	WHERE item_code = c.item_code
		AND COALESCE(is_deleted, false) = false
	ORDER BY update_time DESC NULLS LAST, create_time DESC NULLS LAST
	LIMIT 1
) p ON true
GROUP BY
	b.batch_run_id,
	b.source_type,
	b.xxl_job_id,
	b.xxl_log_id,
	b.scan_paths_json,
	b.file_type,
	b.batch_size,
	b.force,
	b.status,
	b.error_message,
	b.scan_started_at,
	b.scan_finished_at,
	b.xxl_elapsed_seconds,
	b.total_dirs_scanned,
	b.total_files_scanned,
	b.total_files_filtered,
	b.total_files_processed,
	b.total_batches,
	b.create_time
ORDER BY b.scan_started_at DESC NULLS LAST, b.create_time DESC NULLS LAST, b.batch_run_id DESC
`, ingestMonitorBatchStatusCase, ingestMonitorBatchStatusCase, ingestMonitorBatchStatusCase, ingestMonitorBatchTerminalTimeCase)

	rows, err := tx.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ingestMonitorBatchDTO, 0, limit)
	for rows.Next() {
		item, err := scanIngestMonitorBatch(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func getIngestMonitorBatchByID(tx *sql.Tx, ctx context.Context, batchRunID string) (ingestMonitorBatchDTO, error) {
	query := fmt.Sprintf(`
SELECT
	b.batch_run_id,
	b.source_type,
	COALESCE(b.xxl_job_id, '') AS xxl_job_id,
	COALESCE(b.xxl_log_id, '') AS xxl_log_id,
	COALESCE(b.scan_paths, '[]'::jsonb)::text AS scan_paths_json,
	b.file_type,
	b.batch_size,
	b.force,
	b.status,
	COALESCE(b.error_message, '') AS error_message,
	b.scan_started_at,
	b.scan_finished_at,
	b.xxl_elapsed_seconds,
	b.total_dirs_scanned,
	b.total_files_scanned,
	b.total_files_filtered,
	b.total_files_processed,
	b.total_batches,
	COUNT(c.item_code) AS total_tracked,
	COUNT(*) FILTER (WHERE c.item_code IS NOT NULL AND (%s) = 'success') AS success_count,
	COUNT(*) FILTER (WHERE c.item_code IS NOT NULL AND (%s) = 'failure') AS failure_count,
	COUNT(*) FILTER (WHERE c.item_code IS NOT NULL AND (%s) = 'pending') AS pending_count,
	EXTRACT(EPOCH FROM (MAX((%s)) - b.scan_started_at)) AS final_ingest_elapsed_seconds
FROM ingest_batch_run b
LEFT JOIN cad_file_process_status c
	ON c.batch_run_id = b.batch_run_id
	AND COALESCE(c.is_deleted, 0) = 0
LEFT JOIN LATERAL (
	SELECT
		product_name,
		is_complete,
		error_msg,
		source_file_path,
		converted_file_path,
		pc_address,
		glb_address,
		update_time,
		create_time
	FROM product_info
	WHERE item_code = c.item_code
		AND COALESCE(is_deleted, false) = false
	ORDER BY update_time DESC NULLS LAST, create_time DESC NULLS LAST
	LIMIT 1
) p ON true
WHERE COALESCE(b.is_deleted, false) = false
	AND b.batch_run_id = $1
GROUP BY
	b.batch_run_id,
	b.source_type,
	b.xxl_job_id,
	b.xxl_log_id,
	b.scan_paths,
	b.file_type,
	b.batch_size,
	b.force,
	b.status,
	b.error_message,
	b.scan_started_at,
	b.scan_finished_at,
	b.xxl_elapsed_seconds,
	b.total_dirs_scanned,
	b.total_files_scanned,
	b.total_files_filtered,
	b.total_files_processed,
	b.total_batches
`, ingestMonitorBatchStatusCase, ingestMonitorBatchStatusCase, ingestMonitorBatchStatusCase, ingestMonitorBatchTerminalTimeCase)

	row := tx.QueryRowContext(ctx, query, batchRunID)
	return scanIngestMonitorBatch(row)
}

func listIngestMonitorBatchItems(tx *sql.Tx, ctx context.Context, batchRunID string) ([]ingestMonitorBatchItemDTO, error) {
	query := fmt.Sprintf(`
SELECT
	c.item_code,
	COALESCE(c.cad_number, '') AS cad_number,
	COALESCE(c.file_name, '') AS file_name,
	COALESCE(c.process_status, '') AS process_status,
	COALESCE(p.product_name, '') AS product_name,
	p.is_complete,
	COALESCE(NULLIF(p.error_msg, ''), NULLIF(c.error_message, ''), '') AS error_msg,
	COALESCE(p.source_file_path, '') AS source_file_path,
	COALESCE(p.converted_file_path, '') AS converted_file_path,
	COALESCE(p.pc_address, '') AS pc_address,
	COALESCE(p.glb_address, '') AS glb_address,
	c.process_start_time,
	c.process_end_time,
	p.update_time AS product_update_time,
	COALESCE(p.update_time, c.process_end_time, c.process_start_time, c.update_time, c.create_time) AS update_time,
	c.create_time,
	%s AS ingest_status
FROM cad_file_process_status c
LEFT JOIN LATERAL (
	SELECT
		product_name,
		is_complete,
		error_msg,
		source_file_path,
		converted_file_path,
		pc_address,
		glb_address,
		update_time,
		create_time
	FROM product_info
	WHERE item_code = c.item_code
		AND COALESCE(is_deleted, false) = false
	ORDER BY update_time DESC NULLS LAST, create_time DESC NULLS LAST
	LIMIT 1
) p ON true
WHERE c.batch_run_id = $1
	AND COALESCE(c.is_deleted, 0) = 0
ORDER BY
	CASE (%s)
		WHEN 'failure' THEN 0
		WHEN 'pending' THEN 1
		ELSE 2
	END,
	COALESCE(p.update_time, c.process_end_time, c.process_start_time, c.update_time, c.create_time) DESC NULLS LAST,
	c.item_code DESC
`, ingestMonitorBatchStatusCase, ingestMonitorBatchStatusCase)

	rows, err := tx.QueryContext(ctx, query, batchRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]ingestMonitorBatchItemDTO, 0, 64)
	for rows.Next() {
		item, err := scanIngestMonitorBatchItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func scanIngestMonitorBatch(scanner sqlScanner) (ingestMonitorBatchDTO, error) {
	var (
		batchRunID               string
		sourceType               string
		xxlJobID                 string
		xxlLogID                 string
		scanPathsJSON            string
		fileType                 sql.NullInt64
		batchSize                int
		force                    bool
		status                   string
		errorMessage             string
		scanStartedAt            sql.NullTime
		scanFinishedAt           sql.NullTime
		xxlScanElapsedSeconds    sql.NullFloat64
		totalDirsScanned         int
		totalFilesScanned        int
		totalFilesFiltered       int
		totalFilesProcessed      int
		totalBatches             int
		totalTracked             int
		successCount             int
		failureCount             int
		pendingCount             int
		finalIngestElapsedSecond sql.NullFloat64
	)

	if err := scanner.Scan(
		&batchRunID,
		&sourceType,
		&xxlJobID,
		&xxlLogID,
		&scanPathsJSON,
		&fileType,
		&batchSize,
		&force,
		&status,
		&errorMessage,
		&scanStartedAt,
		&scanFinishedAt,
		&xxlScanElapsedSeconds,
		&totalDirsScanned,
		&totalFilesScanned,
		&totalFilesFiltered,
		&totalFilesProcessed,
		&totalBatches,
		&totalTracked,
		&successCount,
		&failureCount,
		&pendingCount,
		&finalIngestElapsedSecond,
	); err != nil {
		return ingestMonitorBatchDTO{}, err
	}

	record := ingestMonitorBatchDTO{
		BatchRunID:               batchRunID,
		SourceType:               sourceType,
		XXLJobID:                 xxlJobID,
		XXLLogID:                 xxlLogID,
		ScanPaths:                parseJSONStringArray(scanPathsJSON),
		BatchSize:                batchSize,
		Force:                    force,
		Status:                   strings.TrimSpace(status),
		ErrorMessage:             errorMessage,
		ScanStartedAt:            formatNullableTime(scanStartedAt),
		ScanFinishedAt:           formatNullableTime(scanFinishedAt),
		XXLScanElapsedSeconds:    nullableFloatPointer(xxlScanElapsedSeconds),
		FinalIngestElapsedSecond: nullableFloatPointer(finalIngestElapsedSecond),
		TotalDirsScanned:         totalDirsScanned,
		TotalFilesScanned:        totalFilesScanned,
		TotalFilesFiltered:       totalFilesFiltered,
		TotalFilesProcessed:      totalFilesProcessed,
		TotalBatches:             totalBatches,
		TotalTracked:             totalTracked,
		SuccessCount:             successCount,
		FailureCount:             failureCount,
		PendingCount:             pendingCount,
	}

	if fileType.Valid {
		value := int(fileType.Int64)
		record.FileType = &value
	}

	return record, nil
}

func scanIngestMonitorBatchItem(scanner sqlScanner) (ingestMonitorBatchItemDTO, error) {
	var (
		itemCode          string
		cadNumber         string
		fileName          string
		processStatus     string
		productName       string
		isComplete        sql.NullInt64
		errorMsg          string
		sourceFilePath    string
		convertedFilePath string
		pcAddress         string
		glbAddress        string
		processStartTime  sql.NullTime
		processEndTime    sql.NullTime
		productUpdateTime sql.NullTime
		updateTime        sql.NullTime
		createTime        sql.NullTime
		ingestStatus      string
	)

	if err := scanner.Scan(
		&itemCode,
		&cadNumber,
		&fileName,
		&processStatus,
		&productName,
		&isComplete,
		&errorMsg,
		&sourceFilePath,
		&convertedFilePath,
		&pcAddress,
		&glbAddress,
		&processStartTime,
		&processEndTime,
		&productUpdateTime,
		&updateTime,
		&createTime,
		&ingestStatus,
	); err != nil {
		return ingestMonitorBatchItemDTO{}, err
	}

	record := ingestMonitorBatchItemDTO{
		ItemCode:          itemCode,
		CadNumber:         cadNumber,
		FileName:          fileName,
		ProcessStatus:     processStatus,
		IngestStatus:      normalizeIngestMonitorStatus(ingestStatus),
		ProductName:       productName,
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
		ProcessStartTime:  formatNullableTime(processStartTime),
		ProcessEndTime:    formatNullableTime(processEndTime),
		ProductUpdateTime: formatNullableTime(productUpdateTime),
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

func parseJSONStringArray(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	items := make([]string, 0)
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil
	}
	if len(items) == 0 {
		return nil
	}
	return items
}

func nullableFloatPointer(value sql.NullFloat64) *float64 {
	if !value.Valid {
		return nil
	}
	number := value.Float64
	return &number
}

func (h *Hub) getIngestMonitorBatches(e *core.RequestEvent) error {
	if h.ingestMonitor == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-monitor 未初始化"})
	}

	response, err := h.ingestMonitor.GetBatchList(e.Request.Context())
	if err != nil {
		return h.handleIngestMonitorBatchError(e, "get ingest monitor batches failed", err)
	}

	return e.JSON(http.StatusOK, response)
}

func (h *Hub) getIngestMonitorBatchDetail(e *core.RequestEvent) error {
	if h.ingestMonitor == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-monitor 未初始化"})
	}

	batchRunID := strings.TrimSpace(e.Request.URL.Query().Get("batchRunId"))
	if batchRunID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "batchRunId 参数不能为空"})
	}

	response, err := h.ingestMonitor.GetBatchDetail(e.Request.Context(), batchRunID)
	if err != nil {
		return h.handleIngestMonitorBatchError(e, "get ingest monitor batch detail failed", err)
	}

	return e.JSON(http.StatusOK, response)
}

func (h *Hub) handleIngestMonitorBatchError(e *core.RequestEvent, logMessage string, err error) error {
	var cfgErr *ingestMonitorConfigError
	switch {
	case errors.As(err, &cfgErr):
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": cfgErr.Error()})
	case errors.Is(err, sql.ErrNoRows):
		return e.JSON(http.StatusNotFound, map[string]string{"error": "未找到对应的入库批次"})
	default:
		h.Logger().Error(logMessage, "logger", "hub", "err", err)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "查询入库批次失败"})
	}
}

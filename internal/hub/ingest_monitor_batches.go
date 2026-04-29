package hub

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pocketbase/pocketbase/core"
)

const (
	ingestMonitorBatchListLimit       = 20
	ingestMonitorBatchDetailPageSize  = 200
	ingestMonitorBatchStatusOrderCase = `
CASE c.ingest_status
	WHEN 'failure' THEN 0
	WHEN 'processing' THEN 1
	WHEN 'success' THEN 2
	ELSE 3
END
`
	ingestMonitorBatchProcessingOrderCase = `
CASE
	WHEN c.process_status IN ('processing', 'preview_ready') THEN 0
	WHEN c.process_status IN ('completed', 'success') THEN 1
	WHEN c.process_status = 'pending' THEN 2
	ELSE 3
END
`
)

var errInvalidIngestMonitorBatchCursor = errors.New("invalid ingest monitor batch cursor")

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
	TotalFilesLargeFiltered  int      `json:"totalFilesLargeFiltered"`
	TotalFilesRegistered     int      `json:"totalFilesRegistered"`
	TotalFilesRegisterFailed int      `json:"totalFilesRegisterFailed"`
	TotalFilesEnqueued       int      `json:"totalFilesEnqueued"`
	TotalFilesEnqueueFailed  int      `json:"totalFilesEnqueueFailed"`
	TotalFilesProcessed      int      `json:"totalFilesProcessed"`
	TotalBatches             int      `json:"totalBatches"`
	TotalTracked             int      `json:"totalTracked"`
	SuccessCount             int      `json:"successCount"`
	FailureCount             int      `json:"failureCount"`
	ProcessingCount          int      `json:"processingCount"`
}

type ingestMonitorBatchItemDTO struct {
	ItemCode          string   `json:"itemCode"`
	CadNumber         string   `json:"cadNumber"`
	FileName          string   `json:"fileName"`
	ProcessStatus     string   `json:"processStatus"`
	IngestStatus      string   `json:"ingestStatus"`
	ProductName       string   `json:"productName"`
	IsComplete        *int     `json:"isComplete,omitempty"`
	ErrorMsg          string   `json:"errorMsg"`
	SourceFilePath    string   `json:"sourceFilePath"`
	ConvertedFilePath string   `json:"convertedFilePath"`
	PcAddress         string   `json:"pcAddress"`
	GlbAddress        string   `json:"glbAddress"`
	HasSourceFilePath bool     `json:"hasSourceFilePath"`
	HasConvertedFile  bool     `json:"hasConvertedFile"`
	HasPcAddress      bool     `json:"hasPcAddress"`
	HasGlbAddress     bool     `json:"hasGlbAddress"`
	PathReadyCount    int      `json:"pathReadyCount"`
	PathReadyTotal    int      `json:"pathReadyTotal"`
	StageStatus       string   `json:"stageStatus"`
	DiagnosticMessage string   `json:"diagnosticMessage"`
	MissingPaths      []string `json:"missingPaths"`
	HasFormalRecord   bool     `json:"hasFormalRecord"`
	IsStalled         bool     `json:"isStalled"`
	StalledMinutes    *int     `json:"stalledMinutes,omitempty"`
	ProcessStartTime  string   `json:"processStartTime"`
	ProcessEndTime    string   `json:"processEndTime"`
	ProductUpdateTime string   `json:"productUpdateTime"`
	UpdateTime        string   `json:"updateTime"`
	CreateTime        string   `json:"createTime"`
}

type ingestMonitorBatchListResponse struct {
	Scope   ingestMonitorScopeDTO   `json:"scope"`
	Batches []ingestMonitorBatchDTO `json:"batches"`
}

type ingestMonitorBatchDetailResponse struct {
	Scope      ingestMonitorScopeDTO       `json:"scope"`
	Batch      ingestMonitorBatchDTO       `json:"batch"`
	Items      []ingestMonitorBatchItemDTO `json:"items"`
	TotalItems int                         `json:"totalItems"`
	PageSize   int                         `json:"pageSize"`
	NextCursor string                      `json:"nextCursor"`
	HasMore    bool                        `json:"hasMore"`
}

type ingestMonitorBatchCursor struct {
	StatusRank  int    `json:"s"`
	ProcessRank int    `json:"p"`
	SortTime    string `json:"t"`
	ItemCode    string `json:"i"`
}

type ingestMonitorBatchItemRow struct {
	Item          ingestMonitorBatchItemDTO
	StatusRank    int
	ProcessRank   int
	SortTime      time.Time
	SortTimeValid bool
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

func (s *ingestMonitorService) GetBatchDetail(ctx context.Context, batchRunID string, cursor string) (*ingestMonitorBatchDetailResponse, error) {
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
		response.PageSize = ingestMonitorBatchDetailPageSize
		response.TotalItems = batch.TotalTracked

		items, nextCursor, hasMore, err := listIngestMonitorBatchItems(tx, queryCtx, batchRunID, cursor, ingestMonitorBatchDetailPageSize)
		if err != nil {
			return err
		}
		response.Items = items
		response.NextCursor = nextCursor
		response.HasMore = hasMore
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
		total_files_large_filtered,
		total_files_registered,
		total_files_register_failed,
		total_files_enqueued,
		total_files_enqueue_failed,
		total_files_processed,
		total_batches,
		create_time
	FROM ingest_batch_run
	WHERE COALESCE(is_deleted, false) = false
		AND tenant_id = current_setting('app.current_tenant', true)
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
	b.total_files_large_filtered,
	b.total_files_registered,
	b.total_files_register_failed,
	b.total_files_enqueued,
	b.total_files_enqueue_failed,
	b.total_files_processed,
	b.total_batches,
	COALESCE(stats.total_tracked, 0) AS total_tracked,
	COALESCE(stats.success_count, 0) AS success_count,
	COALESCE(stats.failure_count, 0) AS failure_count,
	COALESCE(stats.processing_count, 0) AS processing_count,
	EXTRACT(EPOCH FROM (stats.max_ingest_terminal_time - b.scan_started_at)) AS final_ingest_elapsed_seconds
FROM recent_batches b
LEFT JOIN LATERAL (
	SELECT
		COUNT(*) AS total_tracked,
		COUNT(*) FILTER (WHERE c.ingest_status = 'success') AS success_count,
		COUNT(*) FILTER (WHERE c.ingest_status = 'failure') AS failure_count,
		COUNT(*) FILTER (WHERE c.ingest_status = 'processing') AS processing_count,
		MAX(c.ingest_terminal_time) AS max_ingest_terminal_time
	FROM cad_file_process_status c
	WHERE c.batch_run_id = b.batch_run_id
		AND c.tenant_id = current_setting('app.current_tenant', true)
		AND COALESCE(c.is_deleted, 0) = 0
) stats ON true
ORDER BY b.scan_started_at DESC NULLS LAST, b.create_time DESC NULLS LAST, b.batch_run_id DESC
`)

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
	b.total_files_large_filtered,
	b.total_files_registered,
	b.total_files_register_failed,
	b.total_files_enqueued,
	b.total_files_enqueue_failed,
	b.total_files_processed,
	b.total_batches,
	COALESCE(stats.total_tracked, 0) AS total_tracked,
	COALESCE(stats.success_count, 0) AS success_count,
	COALESCE(stats.failure_count, 0) AS failure_count,
	COALESCE(stats.processing_count, 0) AS processing_count,
	EXTRACT(EPOCH FROM (stats.max_ingest_terminal_time - b.scan_started_at)) AS final_ingest_elapsed_seconds
FROM ingest_batch_run b
LEFT JOIN LATERAL (
	SELECT
		COUNT(*) AS total_tracked,
		COUNT(*) FILTER (WHERE c.ingest_status = 'success') AS success_count,
		COUNT(*) FILTER (WHERE c.ingest_status = 'failure') AS failure_count,
		COUNT(*) FILTER (WHERE c.ingest_status = 'processing') AS processing_count,
		MAX(c.ingest_terminal_time) AS max_ingest_terminal_time
	FROM cad_file_process_status c
	WHERE c.batch_run_id = b.batch_run_id
		AND c.tenant_id = current_setting('app.current_tenant', true)
		AND COALESCE(c.is_deleted, 0) = 0
) stats ON true
WHERE COALESCE(b.is_deleted, false) = false
	AND b.tenant_id = current_setting('app.current_tenant', true)
	AND b.batch_run_id = $1
`)

	row := tx.QueryRowContext(ctx, query, batchRunID)
	return scanIngestMonitorBatch(row)
}

func listIngestMonitorBatchItems(tx *sql.Tx, ctx context.Context, batchRunID string, cursor string, pageSize int) ([]ingestMonitorBatchItemDTO, string, bool, error) {
	decodedCursor, err := decodeIngestMonitorBatchCursor(cursor)
	if err != nil {
		return nil, "", false, err
	}

	args := []any{batchRunID}
	cursorFilter := ""
	if decodedCursor != nil {
		cursorSortTime, parseErr := time.Parse(time.RFC3339Nano, decodedCursor.SortTime)
		if parseErr != nil {
			return nil, "", false, errInvalidIngestMonitorBatchCursor
		}

		args = append(args, decodedCursor.StatusRank, decodedCursor.ProcessRank, cursorSortTime, decodedCursor.ItemCode)
		cursorFilter = fmt.Sprintf(`
	AND (
		status_rank > $2
		OR (status_rank = $2 AND process_rank > $3)
		OR (status_rank = $2 AND process_rank = $3 AND sort_time < $4)
		OR (status_rank = $2 AND process_rank = $3 AND sort_time = $4 AND item_code < $5)
	)
`)
	}

	args = append(args, pageSize+1)
	limitPlaceholder := len(args)
	query := fmt.Sprintf(`
	WITH ordered_items AS (
		SELECT
			c.item_code,
			COALESCE(c.product_item_code, '') AS product_item_code,
			COALESCE(NULLIF(c.product_item_code, ''), c.item_code) AS lookup_item_code,
			COALESCE(c.cad_number, '') AS cad_number,
			COALESCE(c.file_name, '') AS file_name,
			COALESCE(c.process_status, '') AS process_status,
		c.ingest_status,
		COALESCE(c.error_message, '') AS error_message,
		c.process_start_time,
		c.process_end_time,
		c.ingest_terminal_time,
		c.update_time,
		c.create_time,
		%s AS status_rank,
		%s AS process_rank,
		COALESCE(c.ingest_terminal_time, c.process_end_time, c.process_start_time, c.update_time, c.create_time) AS sort_time
	FROM cad_file_process_status c
	WHERE c.batch_run_id = $1
		AND c.tenant_id = current_setting('app.current_tenant', true)
		AND COALESCE(c.is_deleted, 0) = 0
),
	paged_items AS (
		SELECT *
		FROM ordered_items
	WHERE 1 = 1
	%s
	ORDER BY
		status_rank ASC,
		process_rank ASC,
		sort_time DESC NULLS LAST,
		item_code DESC
		LIMIT $%d
	),
	lookup_keys AS MATERIALIZED (
		SELECT DISTINCT lookup_item_code
		FROM paged_items
	),
	product_lookup AS MATERIALIZED (
		SELECT
			k.lookup_item_code,
			p.product_name,
			p.is_complete,
			p.error_msg,
			p.source_file_path,
			p.converted_file_path,
			p.pc_address,
			p.glb_address,
			p.update_time,
			p.create_time
		FROM lookup_keys k
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
			WHERE item_code = k.lookup_item_code
				AND tenant_id = current_setting('app.current_tenant', true)
				AND COALESCE(is_deleted, false) = false
			ORDER BY update_time DESC NULLS LAST, create_time DESC NULLS LAST
			LIMIT 1
		) p ON true
	)
SELECT
	i.item_code,
	i.cad_number,
	i.file_name,
	i.process_status,
	i.ingest_status,
	COALESCE(p.product_name, '') AS product_name,
	p.is_complete,
	CASE WHEN p.create_time IS NOT NULL THEN true ELSE false END AS has_formal_record,
	COALESCE(NULLIF(p.error_msg, ''), NULLIF(i.error_message, ''), '') AS error_msg,
	COALESCE(p.source_file_path, '') AS source_file_path,
	COALESCE(p.converted_file_path, '') AS converted_file_path,
	COALESCE(p.pc_address, '') AS pc_address,
	COALESCE(p.glb_address, '') AS glb_address,
	i.process_start_time,
	i.process_end_time,
	p.update_time AS product_update_time,
	COALESCE(i.ingest_terminal_time, p.update_time, i.process_end_time, i.process_start_time, i.update_time, i.create_time) AS update_time,
	i.create_time,
	i.status_rank,
	i.process_rank,
	i.sort_time
FROM paged_items i
LEFT JOIN product_lookup p
	ON p.lookup_item_code = i.lookup_item_code
ORDER BY
	i.status_rank ASC,
	i.process_rank ASC,
	i.sort_time DESC NULLS LAST,
	i.item_code DESC
`, ingestMonitorBatchStatusOrderCase, ingestMonitorBatchProcessingOrderCase, cursorFilter, limitPlaceholder)

	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, "", false, err
	}
	defer rows.Close()

	itemRows := make([]ingestMonitorBatchItemRow, 0, pageSize+1)
	for rows.Next() {
		item, err := scanIngestMonitorBatchItem(rows)
		if err != nil {
			return nil, "", false, err
		}
		itemRows = append(itemRows, item)
	}
	if err := rows.Err(); err != nil {
		return nil, "", false, err
	}

	hasMore := len(itemRows) > pageSize
	if hasMore {
		itemRows = itemRows[:pageSize]
	}

	nextCursor := ""
	if hasMore && len(itemRows) > 0 {
		nextCursor, err = encodeIngestMonitorBatchCursor(itemRows[len(itemRows)-1])
		if err != nil {
			return nil, "", false, err
		}
	}

	items := make([]ingestMonitorBatchItemDTO, 0, len(itemRows))
	for _, row := range itemRows {
		items = append(items, row.Item)
	}

	return items, nextCursor, hasMore, nil
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
		totalFilesLargeFiltered  int
		totalFilesRegistered     int
		totalFilesRegisterFailed int
		totalFilesEnqueued       int
		totalFilesEnqueueFailed  int
		totalFilesProcessed      int
		totalBatches             int
		totalTracked             int
		successCount             int
		failureCount             int
		processingCount          int
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
		&totalFilesLargeFiltered,
		&totalFilesRegistered,
		&totalFilesRegisterFailed,
		&totalFilesEnqueued,
		&totalFilesEnqueueFailed,
		&totalFilesProcessed,
		&totalBatches,
		&totalTracked,
		&successCount,
		&failureCount,
		&processingCount,
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
		TotalFilesLargeFiltered:  totalFilesLargeFiltered,
		TotalFilesRegistered:     totalFilesRegistered,
		TotalFilesRegisterFailed: totalFilesRegisterFailed,
		TotalFilesEnqueued:       totalFilesEnqueued,
		TotalFilesEnqueueFailed:  totalFilesEnqueueFailed,
		TotalFilesProcessed:      totalFilesProcessed,
		TotalBatches:             totalBatches,
		TotalTracked:             totalTracked,
		SuccessCount:             successCount,
		FailureCount:             failureCount,
		ProcessingCount:          processingCount,
	}

	if fileType.Valid {
		value := int(fileType.Int64)
		record.FileType = &value
	}

	return record, nil
}

func scanIngestMonitorBatchItem(scanner sqlScanner) (ingestMonitorBatchItemRow, error) {
	var (
		itemCode          string
		cadNumber         string
		fileName          string
		processStatus     string
		ingestStatus      string
		productName       string
		isComplete        sql.NullInt64
		hasFormalRecord   bool
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
		statusRank        int
		processRank       int
		sortTime          sql.NullTime
	)

	if err := scanner.Scan(
		&itemCode,
		&cadNumber,
		&fileName,
		&processStatus,
		&ingestStatus,
		&productName,
		&isComplete,
		&hasFormalRecord,
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
		&statusRank,
		&processRank,
		&sortTime,
	); err != nil {
		return ingestMonitorBatchItemRow{}, err
	}

	trackedRecord := ingestMonitorRecordDTO{
		ItemCode:          itemCode,
		ProductName:       productName,
		HasFormalRecord:   hasFormalRecord,
		RecordSource:      ingestMonitorBatchRecordType,
		CadNumber:         cadNumber,
		FileName:          fileName,
		ProcessStatus:     strings.TrimSpace(processStatus),
		ErrorMsg:          errorMsg,
		SourceFilePath:    sourceFilePath,
		ConvertedFilePath: convertedFilePath,
		PcAddress:         pcAddress,
		GlbAddress:        glbAddress,
		PathReadyTotal:    4,
		ProcessStartTime:  formatNullableTime(processStartTime),
		ProcessEndTime:    formatNullableTime(processEndTime),
		ProductUpdateTime: formatNullableTime(productUpdateTime),
		UpdateTime:        formatNullableTime(updateTime),
		CreateTime:        formatNullableTime(createTime),
		Status:            strings.TrimSpace(ingestStatus),
	}
	if isComplete.Valid {
		value := int(isComplete.Int64)
		trackedRecord.IsComplete = &value
	}
	populateIngestMonitorDiagnostics(&trackedRecord)

	record := ingestMonitorBatchItemRow{
		StatusRank:    statusRank,
		ProcessRank:   processRank,
		SortTimeValid: sortTime.Valid,
	}
	if sortTime.Valid {
		record.SortTime = sortTime.Time
	}

	record.Item = ingestMonitorBatchItemDTO{
		ItemCode:          itemCode,
		CadNumber:         cadNumber,
		FileName:          fileName,
		ProcessStatus:     processStatus,
		IngestStatus:      trackedRecord.Status,
		ProductName:       productName,
		ErrorMsg:          errorMsg,
		SourceFilePath:    sourceFilePath,
		ConvertedFilePath: convertedFilePath,
		PcAddress:         pcAddress,
		GlbAddress:        glbAddress,
		HasSourceFilePath: trackedRecord.HasSourceFilePath,
		HasConvertedFile:  trackedRecord.HasConvertedFile,
		HasPcAddress:      trackedRecord.HasPcAddress,
		HasGlbAddress:     trackedRecord.HasGlbAddress,
		PathReadyCount:    trackedRecord.PathReadyCount,
		PathReadyTotal:    trackedRecord.PathReadyTotal,
		StageStatus:       trackedRecord.StageStatus,
		DiagnosticMessage: trackedRecord.DiagnosticMessage,
		MissingPaths:      trackedRecord.MissingPaths,
		HasFormalRecord:   trackedRecord.HasFormalRecord,
		IsStalled:         trackedRecord.IsStalled,
		StalledMinutes:    trackedRecord.StalledMinutes,
		ProcessStartTime:  trackedRecord.ProcessStartTime,
		ProcessEndTime:    trackedRecord.ProcessEndTime,
		ProductUpdateTime: trackedRecord.ProductUpdateTime,
		UpdateTime:        trackedRecord.UpdateTime,
		CreateTime:        trackedRecord.CreateTime,
	}

	record.Item.IsComplete = trackedRecord.IsComplete

	return record, nil
}

func decodeIngestMonitorBatchCursor(raw string) (*ingestMonitorBatchCursor, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	decoded, err := base64.RawURLEncoding.DecodeString(trimmed)
	if err != nil {
		return nil, errInvalidIngestMonitorBatchCursor
	}

	cursor := &ingestMonitorBatchCursor{}
	if err := json.Unmarshal(decoded, cursor); err != nil {
		return nil, errInvalidIngestMonitorBatchCursor
	}
	if cursor.ItemCode == "" || cursor.SortTime == "" {
		return nil, errInvalidIngestMonitorBatchCursor
	}
	if _, err := time.Parse(time.RFC3339Nano, cursor.SortTime); err != nil {
		return nil, errInvalidIngestMonitorBatchCursor
	}

	return cursor, nil
}

func encodeIngestMonitorBatchCursor(row ingestMonitorBatchItemRow) (string, error) {
	if !row.SortTimeValid || row.Item.ItemCode == "" {
		return "", errInvalidIngestMonitorBatchCursor
	}

	payload, err := json.Marshal(ingestMonitorBatchCursor{
		StatusRank:  row.StatusRank,
		ProcessRank: row.ProcessRank,
		SortTime:    row.SortTime.UTC().Format(time.RFC3339Nano),
		ItemCode:    row.Item.ItemCode,
	})
	if err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(payload), nil
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

	cursor := strings.TrimSpace(e.Request.URL.Query().Get("cursor"))

	response, err := h.ingestMonitor.GetBatchDetail(e.Request.Context(), batchRunID, cursor)
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
	case errors.Is(err, errInvalidIngestMonitorBatchCursor):
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "cursor 参数不合法"})
	default:
		h.Logger().Error(logMessage, "logger", "hub", "err", err)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "查询入库批次失败"})
	}
}

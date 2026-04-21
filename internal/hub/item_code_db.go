package hub

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/pocketbase/pocketbase/core"
)

// ------------------------------------------------------------------
// DTOs
// ------------------------------------------------------------------

type itemCodeDBListItem struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Updated     string `json:"updated"`
}

type itemCodeDBDetail struct {
	itemCodeDBListItem
	Has3DModel        bool    `json:"has3dModel"`
	Has2DImage        bool    `json:"has2dImage"`
	FilePath          string  `json:"filePath"`
	GlbAddress        string  `json:"glbAddress"`
	SourceFilePath    string  `json:"sourceFilePath"`
	ConvertedFilePath string  `json:"convertedFilePath"`
	MaterialType      string  `json:"materialType"`
	XLength           float64 `json:"xLength"`
	YLength           float64 `json:"yLength"`
	ZLength           float64 `json:"zLength"`
	PartNumber        string  `json:"partNumber"`
	ModelMd5          string  `json:"modelMd5"`
	CreateTime        string  `json:"createTime"`

	CadNumber    string `json:"cadNumber,omitempty"`
	DrawingURL   string `json:"drawingUrl,omitempty"`
	DesignState  string `json:"designState,omitempty"`
	LifeCycle    string `json:"lifeCycle,omitempty"`
	PipeDiameter string `json:"pipeDiameter,omitempty"`
	PackLength   string `json:"packLength,omitempty"`
	PackWidth    string `json:"packWidth,omitempty"`
	PackHeight   string `json:"packHeight,omitempty"`
	ItemLength   string `json:"itemLength,omitempty"`
	ItemWidth    string `json:"itemWidth,omitempty"`
	ItemHeight   string `json:"itemHeight,omitempty"`

	DownloadStatus string `json:"downloadStatus,omitempty"`
	UploadStatus   string `json:"uploadStatus,omitempty"`
	ProcessStatus  string `json:"processStatus,omitempty"`
}

type itemCodeDBListResponse struct {
	Items []itemCodeDBListItem `json:"items"`
	Total int                  `json:"total"`
}

// ------------------------------------------------------------------
// SQL helpers
// ------------------------------------------------------------------

func itemCodeStatusCase() string {
	return `
		CASE
			WHEN COALESCE(is_deleted, false) = true THEN 'obsolete'
			WHEN has_3d_model = true AND has_2d_image = true THEN 'active'
			ELSE 'inactive'
		END
	`
}

func buildItemCodeListQuery(countOnly bool) string {
	statusCase := itemCodeStatusCase()

	var selectClause string
	if countOnly {
		selectClause = "SELECT COUNT(*)"
	} else {
		selectClause = fmt.Sprintf(`
			SELECT
				item_code,
				COALESCE(product_name, '') AS product_name,
				COALESCE(category_name, '') AS category_name,
				COALESCE(description, '') AS description,
				update_time,
				%s AS status
		`, statusCase)
	}

	query := selectClause + `
		FROM product_info
		WHERE tenant_id = current_setting('app.current_tenant')
			AND COALESCE(is_deleted, false) = false
			AND ($1 = '' OR item_code ILIKE '%' || $1 || '%' OR product_name ILIKE '%' || $1 || '%')
			AND ($2 = '' OR category_name = $2)
			AND ($3 = 'all' OR ` + statusCase + ` = $3)
	`

	if !countOnly {
		query += `
			ORDER BY update_time DESC NULLS LAST
			LIMIT $4 OFFSET $5
		`
	}

	return query
}

// ------------------------------------------------------------------
// Service methods (on ingestMonitorService so we reuse its DB pool)
// ------------------------------------------------------------------

func (s *ingestMonitorService) ListItemCodesFromDB(ctx context.Context, search, category, status string, page, perPage int) (*itemCodeDBListResponse, error) {
	response := &itemCodeDBListResponse{}

	err := s.withTenantTx(ctx, func(tx *sql.Tx, cfg ingestMonitorConfig, queryCtx context.Context) error {
		// total count
		countQuery := buildItemCodeListQuery(true)
		var total int
		if err := tx.QueryRowContext(queryCtx, countQuery, search, category, status).Scan(&total); err != nil {
			return err
		}
		response.Total = total

		// list
		listQuery := buildItemCodeListQuery(false)
		offset := (page - 1) * perPage
		rows, err := tx.QueryContext(queryCtx, listQuery, search, category, status, perPage, offset)
		if err != nil {
			return err
		}
		defer rows.Close()

		items := make([]itemCodeDBListItem, 0, perPage)
		for rows.Next() {
			item, err := scanItemCodeDBListItem(rows)
			if err != nil {
				return err
			}
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
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

func (s *ingestMonitorService) GetItemCodeDetailFromDB(ctx context.Context, code string) (*itemCodeDBDetail, error) {
	response := &itemCodeDBDetail{}

	err := s.withTenantTx(ctx, func(tx *sql.Tx, cfg ingestMonitorConfig, queryCtx context.Context) error {
		row := tx.QueryRowContext(queryCtx, `
			SELECT
				p.item_code,
				COALESCE(p.product_name, '') AS product_name,
				COALESCE(p.category_name, '') AS category_name,
				COALESCE(p.description, '') AS description,
				p.update_time,
				CASE
					WHEN COALESCE(p.is_deleted, false) = true THEN 'obsolete'
					WHEN p.has_3d_model = true AND p.has_2d_image = true THEN 'active'
					ELSE 'inactive'
				END AS status,
				p.has_3d_model,
				p.has_2d_image,
				COALESCE(p.file_path, '') AS file_path,
				COALESCE(p.glb_address, '') AS glb_address,
				COALESCE(p.source_file_path, '') AS source_file_path,
				COALESCE(p.converted_file_path, '') AS converted_file_path,
				COALESCE(p.material_type, '') AS material_type,
				p.x_length,
				p.y_length,
				p.z_length,
				COALESCE(p.part_number, '') AS part_number,
				COALESCE(p.model_md5, '') AS model_md5,
				p.create_time,
				COALESCE(c.cad_number, '') AS cad_number,
				COALESCE(c.drawing_url, '') AS drawing_url,
				COALESCE(c.design_state, '') AS design_state,
				COALESCE(c.life_cycle, '') AS life_cycle,
				COALESCE(c.pipe_diameter, '') AS pipe_diameter,
				COALESCE(c.estimated_pack_length, '') AS estimated_pack_length,
				COALESCE(c.estimated_pack_width, '') AS estimated_pack_width,
				COALESCE(c.estimated_pack_height, '') AS estimated_pack_height,
				COALESCE(c.item_length, '') AS item_length,
				COALESCE(c.item_width, '') AS item_width,
				COALESCE(c.item_height, '') AS item_height,
				COALESCE(s.download_status, '') AS download_status,
				COALESCE(s.upload_status, '') AS upload_status,
				COALESCE(s.process_status, '') AS process_status
			FROM product_info p
			LEFT JOIN cad_file_plm c
				ON p.item_code = c.item_code AND c.tenant_id = p.tenant_id
			LEFT JOIN cad_file_process_status s
				ON p.item_code = s.item_code
			WHERE p.tenant_id = current_setting('app.current_tenant')
				AND p.item_code = $1
				AND COALESCE(p.is_deleted, false) = false
			LIMIT 1
		`, code)

		detail, err := scanItemCodeDBDetail(row)
		if err != nil {
			return err
		}
		*response = detail
		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

// ------------------------------------------------------------------
// Scan helpers
// ------------------------------------------------------------------

func scanItemCodeDBListItem(scanner sqlScanner) (itemCodeDBListItem, error) {
	var (
		code, name, category, description string
		updated                           sql.NullTime
		status                            string
	)
	if err := scanner.Scan(&code, &name, &category, &description, &updated, &status); err != nil {
		return itemCodeDBListItem{}, err
	}
	return itemCodeDBListItem{
		Code:        code,
		Name:        name,
		Category:    category,
		Status:      status,
		Description: description,
		Updated:     formatNullableTime(updated),
	}, nil
}

func scanItemCodeDBDetail(scanner sqlScanner) (itemCodeDBDetail, error) {
	var (
		code, name, category, description string
		updated                           sql.NullTime
		status                            string
		has3dModel, has2dImage            bool
		filePath, glbAddress              string
		sourceFilePath, convertedFilePath string
		materialType                      string
		xLength, yLength, zLength         sql.NullFloat64
		partNumber, modelMd5              string
		createTime                        sql.NullTime
		cadNumber, drawingURL             string
		designState, lifeCycle            string
		pipeDiameter                      string
		packLength, packWidth, packHeight string
		itemLength, itemWidth, itemHeight string
		downloadStatus, uploadStatus      string
		processStatus                     string
	)

	if err := scanner.Scan(
		&code, &name, &category, &description, &updated, &status,
		&has3dModel, &has2dImage, &filePath, &glbAddress, &sourceFilePath, &convertedFilePath,
		&materialType, &xLength, &yLength, &zLength, &partNumber, &modelMd5, &createTime,
		&cadNumber, &drawingURL, &designState, &lifeCycle, &pipeDiameter,
		&packLength, &packWidth, &packHeight, &itemLength, &itemWidth, &itemHeight,
		&downloadStatus, &uploadStatus, &processStatus,
	); err != nil {
		return itemCodeDBDetail{}, err
	}

	detail := itemCodeDBDetail{
		itemCodeDBListItem: itemCodeDBListItem{
			Code:        code,
			Name:        name,
			Category:    category,
			Status:      status,
			Description: description,
			Updated:     formatNullableTime(updated),
		},
		Has3DModel:        has3dModel,
		Has2DImage:        has2dImage,
		FilePath:          filePath,
		GlbAddress:        glbAddress,
		SourceFilePath:    sourceFilePath,
		ConvertedFilePath: convertedFilePath,
		MaterialType:      materialType,
		PartNumber:        partNumber,
		ModelMd5:          modelMd5,
		CreateTime:        formatNullableTime(createTime),
		CadNumber:         cadNumber,
		DrawingURL:        drawingURL,
		DesignState:       designState,
		LifeCycle:         lifeCycle,
		PipeDiameter:      pipeDiameter,
		PackLength:        packLength,
		PackWidth:         packWidth,
		PackHeight:        packHeight,
		ItemLength:        itemLength,
		ItemWidth:         itemWidth,
		ItemHeight:        itemHeight,
		DownloadStatus:    downloadStatus,
		UploadStatus:      uploadStatus,
		ProcessStatus:     processStatus,
	}
	if xLength.Valid {
		detail.XLength = xLength.Float64
	}
	if yLength.Valid {
		detail.YLength = yLength.Float64
	}
	if zLength.Valid {
		detail.ZLength = zLength.Float64
	}
	return detail, nil
}

// ------------------------------------------------------------------
// HTTP handlers
// ------------------------------------------------------------------

func (h *Hub) getItemCodeDBList(e *core.RequestEvent) error {
	if h.ingestMonitor == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-monitor 未初始化"})
	}

	query := e.Request.URL.Query()
	search := strings.TrimSpace(query.Get("search"))
	category := strings.TrimSpace(query.Get("category"))
	status := strings.TrimSpace(query.Get("status"))
	if status == "" {
		status = "all"
	}

	page := 1
	if p, err := strconv.Atoi(query.Get("page")); err == nil && p > 0 {
		page = p
	}
	perPage := 50
	if pp, err := strconv.Atoi(query.Get("perPage")); err == nil && pp > 0 {
		perPage = pp
	}

	response, err := h.ingestMonitor.ListItemCodesFromDB(e.Request.Context(), search, category, status, page, perPage)
	if err != nil {
		return h.handleItemCodeDBError(e, "list item codes from db failed", err)
	}
	return e.JSON(http.StatusOK, response)
}

func (h *Hub) getItemCodeDBDetail(e *core.RequestEvent) error {
	if h.ingestMonitor == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-monitor 未初始化"})
	}

	code := strings.TrimSpace(e.Request.URL.Query().Get("code"))
	if code == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "code is required"})
	}

	response, err := h.ingestMonitor.GetItemCodeDetailFromDB(e.Request.Context(), code)
	if err != nil {
		return h.handleItemCodeDBError(e, "get item code detail from db failed", err)
	}
	return e.JSON(http.StatusOK, response)
}

func (s *ingestMonitorService) MarkItemCodeDeletedInDB(ctx context.Context, code string) error {
	return s.withTenantTxWrite(ctx, func(tx *sql.Tx, cfg ingestMonitorConfig, queryCtx context.Context) error {
		_, err := tx.ExecContext(queryCtx, `
			DELETE FROM product_info
			WHERE tenant_id = current_setting('app.current_tenant')
			  AND item_code = $1
		`, code)
		return err
	})
}

func (s *ingestMonitorService) UpdateItemCodeInDB(ctx context.Context, code string, name, category, description string) error {
	return s.withTenantTxWrite(ctx, func(tx *sql.Tx, cfg ingestMonitorConfig, queryCtx context.Context) error {
		_, err := tx.ExecContext(queryCtx, `
			UPDATE product_info
			SET product_name = $1,
			    category_name = $2,
			    description = $3,
			    update_time = NOW()
			WHERE tenant_id = current_setting('app.current_tenant')
			  AND item_code = $4
		`, name, category, description, code)
		return err
	})
}

func (h *Hub) updateItemCodeInDB(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	if h.ingestMonitor == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-monitor 未初始化"})
	}

	var payload struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Category    string `json:"category"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	payload.Code = strings.TrimSpace(payload.Code)
	if payload.Code == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "code is required"})
	}

	if err := h.ingestMonitor.UpdateItemCodeInDB(e.Request.Context(), payload.Code, payload.Name, payload.Category, payload.Description); err != nil {
		return h.handleItemCodeDBError(e, "update item code in db failed", err)
	}
	return e.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Hub) deleteItemCodeByCodeFromDB(e *core.RequestEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}
	if h.ingestMonitor == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-monitor 未初始化"})
	}

	code := strings.TrimSpace(e.Request.URL.Query().Get("code"))
	if code == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "code is required"})
	}

	password := strings.TrimSpace(e.Request.URL.Query().Get("password"))
	if password == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "password is required"})
	}
	if !e.Auth.ValidatePassword(password) {
		return e.JSON(http.StatusForbidden, map[string]string{"error": "invalid password"})
	}

	if err := h.ingestMonitor.MarkItemCodeDeletedInDB(e.Request.Context(), code); err != nil {
		return h.handleItemCodeDBError(e, "delete item code from db failed", err)
	}

	if auditErr := h.recordItemCodeAudit(itemCodeAuditEntry{
		UserID:    e.Auth.Id,
		Action:    "single_delete",
		TargetIDs: code,
		Status:    itemCodeAuditStatusSuccess,
		IPAddress: e.RealIP(),
	}); auditErr != nil {
		h.Logger().Error("record item code audit failed", "logger", "hub", "err", auditErr)
	}

	return e.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Hub) batchDeleteItemCodesByCodeFromDB(e *core.RequestEvent) error {
	if err := requireAdmin(e); err != nil {
		return err
	}
	if h.ingestMonitor == nil {
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": "ingest-monitor 未初始化"})
	}

	var payload struct {
		Codes    []string `json:"codes"`
		Password string   `json:"password"`
	}
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if len(payload.Codes) == 0 {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "codes is required"})
	}

	password := strings.TrimSpace(payload.Password)
	if password == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "password is required"})
	}
	if !e.Auth.ValidatePassword(password) {
		return e.JSON(http.StatusForbidden, map[string]string{"error": "invalid password"})
	}

	deleted := 0
	failed := 0
	var failedCodes []string

	for _, code := range payload.Codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if err := h.ingestMonitor.MarkItemCodeDeletedInDB(e.Request.Context(), code); err != nil {
			failed++
			failedCodes = append(failedCodes, code)
			h.Logger().Error("batch delete item code from db failed", "logger", "hub", "code", code, "err", err)
			continue
		}
		deleted++
	}

	status := itemCodeAuditStatusSuccess
	detail := ""
	if failed > 0 {
		status = itemCodeAuditStatusFailed
		detail = "failed: " + strings.Join(failedCodes, ", ")
	}
	if auditErr := h.recordItemCodeAudit(itemCodeAuditEntry{
		UserID:    e.Auth.Id,
		Action:    "batch_delete",
		TargetIDs: strings.Join(payload.Codes, ","),
		Status:    status,
		Detail:    detail,
		IPAddress: e.RealIP(),
	}); auditErr != nil {
		h.Logger().Error("record item code audit failed", "logger", "hub", "err", auditErr)
	}

	return e.JSON(http.StatusOK, map[string]any{
		"deleted": deleted,
		"failed":  failed,
	})
}

func (h *Hub) handleItemCodeDBError(e *core.RequestEvent, logMessage string, err error) error {
	var cfgErr *ingestMonitorConfigError
	switch {
	case errors.As(err, &cfgErr):
		return e.JSON(http.StatusServiceUnavailable, map[string]string{"error": cfgErr.Error()})
	case errors.Is(err, sql.ErrNoRows):
		return e.JSON(http.StatusNotFound, map[string]string{"error": "未找到对应的 Item Code"})
	default:
		h.Logger().Error(logMessage, "logger", "hub", "err", err)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": logMessage + ": " + err.Error()})
	}
}

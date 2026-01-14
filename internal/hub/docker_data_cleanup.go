// Package hub 提供 Docker 数据清理相关 API 与任务编排。
// 该文件负责清理配置保存、资源拉取、任务执行与进度查询。
package hub

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"aether/internal/common"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/security"
	"github.com/pocketbase/pocketbase/tools/types"
)

const (
	dataCleanupConfigCollection = "docker_data_cleanup_configs"
	dataCleanupRunsCollection   = "docker_data_cleanup_runs"
	dataCleanupKeyEnv           = "DATA_CLEANUP_KEY"
)

var dataCleanupRedisPatterns = []string{
	"task:*",
	"pending_queue",
	"processing",
	"task_stats",
	"last_process_time",
	"processing_lock",
	"celery-task-meta-*",
	"file_processor",
	"celery",
	"unacked*",
	"unacked_mutex*",
	"processing:*",
}

type dataCleanupMySQLStored struct {
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Username string   `json:"username,omitempty"`
	Database string   `json:"database,omitempty"`
	Tables   []string `json:"tables,omitempty"`
}

type dataCleanupRedisStored struct {
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Username string   `json:"username,omitempty"`
	DB       int      `json:"db,omitempty"`
	Patterns []string `json:"patterns,omitempty"`
}

type dataCleanupMinioStored struct {
	Host      string   `json:"host"`
	Port      int      `json:"port"`
	AccessKey string   `json:"accessKey,omitempty"`
	Bucket    string   `json:"bucket,omitempty"`
	Prefixes  []string `json:"prefixes,omitempty"`
}

type dataCleanupESStored struct {
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Username string   `json:"username,omitempty"`
	Indices  []string `json:"indices,omitempty"`
}

type dataCleanupConfigResponse struct {
	ID     string                  `json:"id"`
	System string                  `json:"system"`
	MySQL  dataCleanupMySQLPayload `json:"mysql"`
	Redis  dataCleanupRedisPayload `json:"redis"`
	Minio  dataCleanupMinioPayload `json:"minio"`
	ES     dataCleanupESPayload    `json:"es"`
}

type dataCleanupMySQLPayload struct {
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	Username    string   `json:"username,omitempty"`
	Password    string   `json:"password,omitempty"`
	Database    string   `json:"database,omitempty"`
	Tables      []string `json:"tables,omitempty"`
	HasPassword bool     `json:"hasPassword,omitempty"`
}

type dataCleanupRedisPayload struct {
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	Username    string   `json:"username,omitempty"`
	Password    string   `json:"password,omitempty"`
	DB          int      `json:"db,omitempty"`
	Patterns    []string `json:"patterns,omitempty"`
	HasPassword bool     `json:"hasPassword,omitempty"`
}

type dataCleanupMinioPayload struct {
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	AccessKey    string   `json:"accessKey,omitempty"`
	SecretKey    string   `json:"secretKey,omitempty"`
	Bucket       string   `json:"bucket,omitempty"`
	Prefixes     []string `json:"prefixes,omitempty"`
	HasSecretKey bool     `json:"hasSecretKey,omitempty"`
}

type dataCleanupESPayload struct {
	Host        string   `json:"host"`
	Port        int      `json:"port"`
	Username    string   `json:"username,omitempty"`
	Password    string   `json:"password,omitempty"`
	Indices     []string `json:"indices,omitempty"`
	HasPassword bool     `json:"hasPassword,omitempty"`
}

type dataCleanupListPayload struct {
	System            string `json:"system"`
	Host              string `json:"host"`
	Port              int    `json:"port"`
	Username          string `json:"username"`
	Password          string `json:"password"`
	UseStoredPassword bool   `json:"useStoredPassword"`
	Database          string `json:"database"`
}

type dataCleanupMinioListPayload struct {
	System          string `json:"system"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	AccessKey       string `json:"accessKey"`
	SecretKey       string `json:"secretKey"`
	UseStoredSecret bool   `json:"useStoredSecret"`
	Bucket          string `json:"bucket"`
}

type dataCleanupRunPayload struct {
	System string `json:"system"`
}

type dataCleanupRunResult struct {
	Module string `json:"module"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

func (h *Hub) getDataCleanupEncryptionKey() (string, error) {
	key, ok := GetEnv(dataCleanupKeyEnv)
	if !ok || strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("missing encryption key env: %s (or AETHER_HUB_%s)", dataCleanupKeyEnv, dataCleanupKeyEnv)
	}
	if len(key) != 32 {
		return "", fmt.Errorf("invalid encryption key length: expected 32 bytes, got %d", len(key))
	}
	return key, nil
}

func (h *Hub) encryptDataCleanupSecret(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	key, err := h.getDataCleanupEncryptionKey()
	if err != nil {
		return "", err
	}
	encrypted, err := security.Encrypt([]byte(value), key)
	if err != nil {
		return "", err
	}
	return encrypted, nil
}

func (h *Hub) decryptDataCleanupSecret(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	key, err := h.getDataCleanupEncryptionKey()
	if err != nil {
		return "", err
	}
	decrypted, err := security.Decrypt(value, key)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}

func (h *Hub) logDataCleanupError(message string, err error, fields ...any) {
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

func formatDataCleanupError(context string, err error, fields map[string]any) error {
	return fmt.Errorf(
		"%s | errType=%T | err=%v | fields=%v | stack=%s",
		context,
		err,
		err,
		fields,
		string(debug.Stack()),
	)
}

func normalizeStringSlice(items []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func parseJSONField[T any](record *core.Record, field string, target *T) error {
	raw, err := types.ParseJSONRaw(record.Get(field))
	if err != nil {
		return err
	}
	if len(raw) == 0 || raw.String() == "null" {
		return nil
	}
	if err := json.Unmarshal(raw, target); err != nil {
		return err
	}
	return nil
}

func toJSONRaw(value any) (types.JSONRaw, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return types.JSONRaw(encoded), nil
}

func (h *Hub) findCleanupConfig(systemID string) (*core.Record, error) {
	if strings.TrimSpace(systemID) == "" {
		return nil, errors.New("system is required")
	}
	records, err := h.FindRecordsByFilter(
		dataCleanupConfigCollection,
		"system = {:system}",
		"-created",
		1,
		0,
		dbx.Params{"system": systemID},
	)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}

func (h *Hub) getDockerDataCleanupConfig(e *core.RequestEvent) error {
	systemID := strings.TrimSpace(e.Request.URL.Query().Get("system"))
	if systemID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system is required"})
	}
	if _, err := h.resolveSystemRecordForUser(e, systemID); err != nil {
		return respondSystemAccessError(e, err)
	}
	record, err := h.findCleanupConfig(systemID)
	if err != nil {
		h.logDataCleanupError("load cleanup config failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	response := dataCleanupConfigResponse{
		System: systemID,
		MySQL:  dataCleanupMySQLPayload{Tables: []string{}},
		Redis:  dataCleanupRedisPayload{Patterns: append([]string{}, dataCleanupRedisPatterns...)},
		Minio:  dataCleanupMinioPayload{Prefixes: []string{}},
		ES:     dataCleanupESPayload{Indices: []string{}},
	}
	if record == nil {
		return e.JSON(http.StatusOK, response)
	}

	var mysqlStored dataCleanupMySQLStored
	var redisStored dataCleanupRedisStored
	var minioStored dataCleanupMinioStored
	var esStored dataCleanupESStored

	if err := parseJSONField(record, "mysql", &mysqlStored); err != nil {
		h.logDataCleanupError("parse mysql config failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if err := parseJSONField(record, "redis", &redisStored); err != nil {
		h.logDataCleanupError("parse redis config failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if err := parseJSONField(record, "minio", &minioStored); err != nil {
		h.logDataCleanupError("parse minio config failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if err := parseJSONField(record, "es", &esStored); err != nil {
		h.logDataCleanupError("parse es config failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	response.ID = record.Id
	response.MySQL = dataCleanupMySQLPayload{
		Host:        mysqlStored.Host,
		Port:        mysqlStored.Port,
		Username:    mysqlStored.Username,
		Database:    mysqlStored.Database,
		Tables:      normalizeStringSlice(mysqlStored.Tables),
		HasPassword: record.GetString("mysql_password") != "",
	}
	response.Redis = dataCleanupRedisPayload{
		Host:        redisStored.Host,
		Port:        redisStored.Port,
		Username:    redisStored.Username,
		DB:          redisStored.DB,
		Patterns:    normalizeStringSlice(redisStored.Patterns),
		HasPassword: record.GetString("redis_password") != "",
	}
	if len(response.Redis.Patterns) == 0 {
		response.Redis.Patterns = append([]string{}, dataCleanupRedisPatterns...)
	}
	response.Minio = dataCleanupMinioPayload{
		Host:         minioStored.Host,
		Port:         minioStored.Port,
		AccessKey:    minioStored.AccessKey,
		Bucket:       minioStored.Bucket,
		Prefixes:     normalizeStringSlice(minioStored.Prefixes),
		HasSecretKey: record.GetString("minio_secret_key") != "",
	}
	response.ES = dataCleanupESPayload{
		Host:        esStored.Host,
		Port:        esStored.Port,
		Username:    esStored.Username,
		Indices:     normalizeStringSlice(esStored.Indices),
		HasPassword: record.GetString("es_password") != "",
	}

	return e.JSON(http.StatusOK, response)
}

func (h *Hub) upsertDockerDataCleanupConfig(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dataCleanupConfigResponse
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	systemID := strings.TrimSpace(payload.System)
	if systemID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system is required"})
	}
	if _, err := h.resolveSystemRecordForUser(e, systemID); err != nil {
		return respondSystemAccessError(e, err)
	}

	record, err := h.findCleanupConfig(systemID)
	if err != nil {
		h.logDataCleanupError("load cleanup config failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	isCreate := record == nil
	if record == nil {
		collection, err := h.FindCollectionByNameOrId(dataCleanupConfigCollection)
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		record = core.NewRecord(collection)
		record.Set("system", systemID)
	}

	mysqlStored := dataCleanupMySQLStored{
		Host:     strings.TrimSpace(payload.MySQL.Host),
		Port:     payload.MySQL.Port,
		Username: strings.TrimSpace(payload.MySQL.Username),
		Database: strings.TrimSpace(payload.MySQL.Database),
		Tables:   normalizeStringSlice(payload.MySQL.Tables),
	}
	redisStored := dataCleanupRedisStored{
		Host:     strings.TrimSpace(payload.Redis.Host),
		Port:     payload.Redis.Port,
		Username: strings.TrimSpace(payload.Redis.Username),
		DB:       payload.Redis.DB,
		Patterns: normalizeStringSlice(payload.Redis.Patterns),
	}
	if len(redisStored.Patterns) == 0 {
		redisStored.Patterns = append([]string{}, dataCleanupRedisPatterns...)
	}
	minioStored := dataCleanupMinioStored{
		Host:      strings.TrimSpace(payload.Minio.Host),
		Port:      payload.Minio.Port,
		AccessKey: strings.TrimSpace(payload.Minio.AccessKey),
		Bucket:    strings.TrimSpace(payload.Minio.Bucket),
		Prefixes:  normalizeStringSlice(payload.Minio.Prefixes),
	}
	esStored := dataCleanupESStored{
		Host:     strings.TrimSpace(payload.ES.Host),
		Port:     payload.ES.Port,
		Username: strings.TrimSpace(payload.ES.Username),
		Indices:  normalizeStringSlice(payload.ES.Indices),
	}

	mysqlRaw, err := toJSONRaw(mysqlStored)
	if err != nil {
		h.logDataCleanupError("encode mysql config failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	redisRaw, err := toJSONRaw(redisStored)
	if err != nil {
		h.logDataCleanupError("encode redis config failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	minioRaw, err := toJSONRaw(minioStored)
	if err != nil {
		h.logDataCleanupError("encode minio config failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	esRaw, err := toJSONRaw(esStored)
	if err != nil {
		h.logDataCleanupError("encode es config failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	record.Set("mysql", mysqlRaw)
	record.Set("redis", redisRaw)
	record.Set("minio", minioRaw)
	record.Set("es", esRaw)

	mysqlPassword := strings.TrimSpace(payload.MySQL.Password)
	if mysqlPassword != "" {
		encrypted, err := h.encryptDataCleanupSecret(mysqlPassword)
		if err != nil {
			h.logDataCleanupError("encrypt mysql password failed", err, "system", systemID)
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		record.Set("mysql_password", encrypted)
	}
	redisPassword := strings.TrimSpace(payload.Redis.Password)
	if redisPassword != "" {
		encrypted, err := h.encryptDataCleanupSecret(redisPassword)
		if err != nil {
			h.logDataCleanupError("encrypt redis password failed", err, "system", systemID)
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		record.Set("redis_password", encrypted)
	}
	minioSecret := strings.TrimSpace(payload.Minio.SecretKey)
	if minioSecret != "" {
		encrypted, err := h.encryptDataCleanupSecret(minioSecret)
		if err != nil {
			h.logDataCleanupError("encrypt minio secret failed", err, "system", systemID)
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		record.Set("minio_secret_key", encrypted)
	}
	esPassword := strings.TrimSpace(payload.ES.Password)
	if esPassword != "" {
		encrypted, err := h.encryptDataCleanupSecret(esPassword)
		if err != nil {
			h.logDataCleanupError("encrypt es password failed", err, "system", systemID)
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		record.Set("es_password", encrypted)
	}

	if err := h.Save(record); err != nil {
		h.logDataCleanupError("save cleanup config failed", err, "system", systemID, "create", isCreate)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"id": record.Id, "status": "ok"})
}

func (h *Hub) resolveCleanupPassword(
	systemID string,
	field string,
	password string,
	useStored bool,
) (string, error) {
	if strings.TrimSpace(password) != "" {
		return password, nil
	}
	if !useStored {
		return "", nil
	}
	record, err := h.findCleanupConfig(systemID)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", errors.New("cleanup config not found")
	}
	encrypted := record.GetString(field)
	if encrypted == "" {
		return "", nil
	}
	return h.decryptDataCleanupSecret(encrypted)
}

func (h *Hub) listDataCleanupMySQLDatabases(e *core.RequestEvent) error {
	var payload dataCleanupListPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	payload.Host = strings.TrimSpace(payload.Host)
	if payload.System == "" || payload.Host == "" || payload.Port <= 0 {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system, host and port are required"})
	}
	if _, err := h.resolveSystemRecordForUser(e, payload.System); err != nil {
		return respondSystemAccessError(e, err)
	}
	password, err := h.resolveCleanupPassword(payload.System, "mysql_password", payload.Password, payload.UseStoredPassword)
	if err != nil {
		h.logDataCleanupError("resolve mysql password failed", err, "system", payload.System)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	items, err := system.FetchDataCleanupMySQLDatabasesFromAgent(common.DataCleanupMySQLDatabasesRequest{
		Host:     payload.Host,
		Port:     payload.Port,
		Username: payload.Username,
		Password: password,
	})
	if err != nil {
		h.logDataCleanupError("list mysql databases failed", err, "system", payload.System, "host", payload.Host, "port", payload.Port)
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Hub) listDataCleanupMySQLTables(e *core.RequestEvent) error {
	var payload dataCleanupListPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	payload.Host = strings.TrimSpace(payload.Host)
	payload.Database = strings.TrimSpace(payload.Database)
	if payload.System == "" || payload.Host == "" || payload.Port <= 0 || payload.Database == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system, host, port, database are required"})
	}
	if _, err := h.resolveSystemRecordForUser(e, payload.System); err != nil {
		return respondSystemAccessError(e, err)
	}
	password, err := h.resolveCleanupPassword(payload.System, "mysql_password", payload.Password, payload.UseStoredPassword)
	if err != nil {
		h.logDataCleanupError("resolve mysql password failed", err, "system", payload.System)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	items, err := system.FetchDataCleanupMySQLTablesFromAgent(common.DataCleanupMySQLTablesRequest{
		Host:     payload.Host,
		Port:     payload.Port,
		Username: payload.Username,
		Password: password,
		Database: payload.Database,
	})
	if err != nil {
		h.logDataCleanupError("list mysql tables failed", err, "system", payload.System, "database", payload.Database)
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Hub) listDataCleanupRedisDatabases(e *core.RequestEvent) error {
	var payload dataCleanupListPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	payload.Host = strings.TrimSpace(payload.Host)
	if payload.System == "" || payload.Host == "" || payload.Port <= 0 {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system, host and port are required"})
	}
	if _, err := h.resolveSystemRecordForUser(e, payload.System); err != nil {
		return respondSystemAccessError(e, err)
	}
	password, err := h.resolveCleanupPassword(payload.System, "redis_password", payload.Password, payload.UseStoredPassword)
	if err != nil {
		h.logDataCleanupError("resolve redis password failed", err, "system", payload.System)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	items, err := system.FetchDataCleanupRedisDatabasesFromAgent(common.DataCleanupRedisDatabasesRequest{
		Host:     payload.Host,
		Port:     payload.Port,
		Username: payload.Username,
		Password: password,
	})
	if err != nil {
		h.logDataCleanupError("list redis databases failed", err, "system", payload.System, "host", payload.Host, "port", payload.Port)
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Hub) listDataCleanupMinioBuckets(e *core.RequestEvent) error {
	var payload dataCleanupMinioListPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	payload.Host = strings.TrimSpace(payload.Host)
	payload.AccessKey = strings.TrimSpace(payload.AccessKey)
	if payload.System == "" || payload.Host == "" || payload.Port <= 0 {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system, host and port are required"})
	}
	if _, err := h.resolveSystemRecordForUser(e, payload.System); err != nil {
		return respondSystemAccessError(e, err)
	}
	secret, err := h.resolveCleanupPassword(payload.System, "minio_secret_key", payload.SecretKey, payload.UseStoredSecret)
	if err != nil {
		h.logDataCleanupError("resolve minio secret failed", err, "system", payload.System)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	items, err := system.FetchDataCleanupMinioBucketsFromAgent(common.DataCleanupMinioBucketsRequest{
		Host:      payload.Host,
		Port:      payload.Port,
		AccessKey: payload.AccessKey,
		SecretKey: secret,
	})
	if err != nil {
		h.logDataCleanupError("list minio buckets failed", err, "system", payload.System, "host", payload.Host, "port", payload.Port)
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Hub) listDataCleanupMinioPrefixes(e *core.RequestEvent) error {
	var payload dataCleanupMinioListPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	payload.Host = strings.TrimSpace(payload.Host)
	payload.AccessKey = strings.TrimSpace(payload.AccessKey)
	payload.Bucket = strings.TrimSpace(payload.Bucket)
	if payload.System == "" || payload.Host == "" || payload.Port <= 0 || payload.Bucket == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system, host, port, bucket are required"})
	}
	if _, err := h.resolveSystemRecordForUser(e, payload.System); err != nil {
		return respondSystemAccessError(e, err)
	}
	secret, err := h.resolveCleanupPassword(payload.System, "minio_secret_key", payload.SecretKey, payload.UseStoredSecret)
	if err != nil {
		h.logDataCleanupError("resolve minio secret failed", err, "system", payload.System)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	items, err := system.FetchDataCleanupMinioPrefixesFromAgent(common.DataCleanupMinioPrefixesRequest{
		Host:      payload.Host,
		Port:      payload.Port,
		AccessKey: payload.AccessKey,
		SecretKey: secret,
		Bucket:    payload.Bucket,
	})
	if err != nil {
		h.logDataCleanupError("list minio prefixes failed", err, "system", payload.System, "bucket", payload.Bucket)
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Hub) listDataCleanupESIndices(e *core.RequestEvent) error {
	var payload dataCleanupListPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	payload.Host = strings.TrimSpace(payload.Host)
	if payload.System == "" || payload.Host == "" || payload.Port <= 0 {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system, host and port are required"})
	}
	if _, err := h.resolveSystemRecordForUser(e, payload.System); err != nil {
		return respondSystemAccessError(e, err)
	}
	password, err := h.resolveCleanupPassword(payload.System, "es_password", payload.Password, payload.UseStoredPassword)
	if err != nil {
		h.logDataCleanupError("resolve es password failed", err, "system", payload.System)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	system, err := h.resolveSystem(payload.System)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	items, err := system.FetchDataCleanupESIndicesFromAgent(common.DataCleanupESIndicesRequest{
		Host:     payload.Host,
		Port:     payload.Port,
		Username: payload.Username,
		Password: password,
	})
	if err != nil {
		h.logDataCleanupError("list es indices failed", err, "system", payload.System, "host", payload.Host, "port", payload.Port)
		return e.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]any{"items": items})
}

func (h *Hub) startDataCleanupRun(e *core.RequestEvent) error {
	if err := requireWritable(e); err != nil {
		return err
	}
	var payload dataCleanupRunPayload
	if err := json.NewDecoder(e.Request.Body).Decode(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	systemID := strings.TrimSpace(payload.System)
	if systemID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "system is required"})
	}
	if _, err := h.resolveSystemRecordForUser(e, systemID); err != nil {
		return respondSystemAccessError(e, err)
	}
	configRecord, err := h.findCleanupConfig(systemID)
	if err != nil {
		h.logDataCleanupError("load cleanup config failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	if configRecord == nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "cleanup config not found"})
	}
	runCollection, err := h.FindCollectionByNameOrId(dataCleanupRunsCollection)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	runRecord := core.NewRecord(runCollection)
	runRecord.Set("system", systemID)
	runRecord.Set("config", configRecord.Id)
	runRecord.Set("status", "pending")
	runRecord.Set("progress", 0)
	runRecord.Set("step", "")
	runRecord.Set("logs", types.JSONRaw("[]"))
	runRecord.Set("results", types.JSONRaw("[]"))
	if err := h.Save(runRecord); err != nil {
		h.logDataCleanupError("create cleanup run failed", err, "system", systemID)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	go h.executeDataCleanupRun(runRecord.Id, systemID, configRecord.Id, e.Auth.Id)

	return e.JSON(http.StatusOK, map[string]any{"runId": runRecord.Id})
}

func (h *Hub) getDataCleanupRun(e *core.RequestEvent) error {
	runID := strings.TrimSpace(e.Request.URL.Query().Get("id"))
	if runID == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "id is required"})
	}
	record, err := h.FindRecordById(dataCleanupRunsCollection, runID)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "run not found"})
	}
	systemID := record.GetString("system")
	if _, err := h.resolveSystemRecordForUser(e, systemID); err != nil {
		return respondSystemAccessError(e, err)
	}
	return e.JSON(http.StatusOK, map[string]any{
		"id":       record.Id,
		"status":   record.GetString("status"),
		"progress": record.GetInt("progress"),
		"step":     record.GetString("step"),
		"logs":     record.Get("logs"),
		"results":  record.Get("results"),
	})
}

func (h *Hub) retryDataCleanupRun(e *core.RequestEvent) error {
	return h.startDataCleanupRun(e)
}

func (h *Hub) executeDataCleanupRun(runID, systemID, configID, userID string) {
	steps := []string{"mysql", "redis", "minio", "es"}
	_ = steps
	logs := make([]string, 0, 16)
	results := make([]dataCleanupRunResult, 0, 4)

	updateErr := h.updateDataCleanupRun(runID, "running", 0, "init", logs, results)
	if updateErr != nil {
		h.logDataCleanupError("update cleanup run failed", updateErr, "run", runID)
		return
	}

	system, err := h.resolveSystem(systemID)
	if err != nil {
		err = formatDataCleanupError("resolve system failed", err, map[string]any{"system": systemID})
		h.logDataCleanupError("resolve system failed", err, "system", systemID)
		_ = h.failDataCleanupRun(runID, logs, results, err)
		return
	}

	configRecord, err := h.FindRecordById(dataCleanupConfigCollection, configID)
	if err != nil {
		err = formatDataCleanupError("load cleanup config failed", err, map[string]any{"system": systemID})
		h.logDataCleanupError("load cleanup config failed", err, "system", systemID)
		_ = h.failDataCleanupRun(runID, logs, results, err)
		return
	}

	var mysqlStored dataCleanupMySQLStored
	var redisStored dataCleanupRedisStored
	var minioStored dataCleanupMinioStored
	var esStored dataCleanupESStored

	if err := parseJSONField(configRecord, "mysql", &mysqlStored); err != nil {
		err = formatDataCleanupError("parse mysql config failed", err, map[string]any{"system": systemID})
		h.logDataCleanupError("parse mysql config failed", err, "system", systemID)
		_ = h.failDataCleanupRun(runID, logs, results, err)
		return
	}
	if err := parseJSONField(configRecord, "redis", &redisStored); err != nil {
		err = formatDataCleanupError("parse redis config failed", err, map[string]any{"system": systemID})
		h.logDataCleanupError("parse redis config failed", err, "system", systemID)
		_ = h.failDataCleanupRun(runID, logs, results, err)
		return
	}
	if err := parseJSONField(configRecord, "minio", &minioStored); err != nil {
		err = formatDataCleanupError("parse minio config failed", err, map[string]any{"system": systemID})
		h.logDataCleanupError("parse minio config failed", err, "system", systemID)
		_ = h.failDataCleanupRun(runID, logs, results, err)
		return
	}
	if err := parseJSONField(configRecord, "es", &esStored); err != nil {
		err = formatDataCleanupError("parse es config failed", err, map[string]any{"system": systemID})
		h.logDataCleanupError("parse es config failed", err, "system", systemID)
		_ = h.failDataCleanupRun(runID, logs, results, err)
		return
	}

	mysqlPassword, err := h.decryptDataCleanupSecret(configRecord.GetString("mysql_password"))
	if err != nil {
		err = formatDataCleanupError("decrypt mysql password failed", err, map[string]any{"system": systemID})
		h.logDataCleanupError("decrypt mysql password failed", err, "system", systemID)
		_ = h.failDataCleanupRun(runID, logs, results, err)
		return
	}
	redisPassword, err := h.decryptDataCleanupSecret(configRecord.GetString("redis_password"))
	if err != nil {
		err = formatDataCleanupError("decrypt redis password failed", err, map[string]any{"system": systemID})
		h.logDataCleanupError("decrypt redis password failed", err, "system", systemID)
		_ = h.failDataCleanupRun(runID, logs, results, err)
		return
	}
	minioSecret, err := h.decryptDataCleanupSecret(configRecord.GetString("minio_secret_key"))
	if err != nil {
		err = formatDataCleanupError("decrypt minio secret failed", err, map[string]any{"system": systemID})
		h.logDataCleanupError("decrypt minio secret failed", err, "system", systemID)
		_ = h.failDataCleanupRun(runID, logs, results, err)
		return
	}
	esPassword, err := h.decryptDataCleanupSecret(configRecord.GetString("es_password"))
	if err != nil {
		err = formatDataCleanupError("decrypt es password failed", err, map[string]any{"system": systemID})
		h.logDataCleanupError("decrypt es password failed", err, "system", systemID)
		_ = h.failDataCleanupRun(runID, logs, results, err)
		return
	}

	mysqlTables := normalizeStringSlice(mysqlStored.Tables)
	minioPrefixes := normalizeStringSlice(minioStored.Prefixes)
	esIndices := normalizeStringSlice(esStored.Indices)
	redisPatterns := normalizeStringSlice(redisStored.Patterns)
	if len(redisPatterns) == 0 {
		redisPatterns = append([]string{}, dataCleanupRedisPatterns...)
	}

	mysqlTargets := 0
	if mysqlStored.Host != "" && mysqlStored.Port > 0 && mysqlStored.Database != "" {
		mysqlTargets = len(mysqlTables)
	}
	redisTargets := 0
	if redisStored.Host != "" && redisStored.Port > 0 {
		redisTargets = len(redisPatterns)
	}
	minioTargets := 0
	if minioStored.Host != "" && minioStored.Port > 0 && minioStored.Bucket != "" {
		minioTargets = len(minioPrefixes)
	}
	esTargets := 0
	if esStored.Host != "" && esStored.Port > 0 {
		esTargets = len(esIndices)
	}

	totalOps := mysqlTargets + redisTargets + minioTargets + esTargets
	if totalOps == 0 {
		err = formatDataCleanupError("no cleanup target", errors.New("no cleanup target"), map[string]any{"system": systemID})
		_ = h.failDataCleanupRun(runID, logs, results, err)
		return
	}

	completedOps := 0
	failures := 0

	if mysqlStored.Host != "" && mysqlStored.Port > 0 && mysqlStored.Database != "" && len(mysqlTables) > 0 {
		logs = append(logs, fmt.Sprintf("[%s] start mysql cleanup", time.Now().Format(time.RFC3339)))
		mysqlFailed := false
		for _, table := range mysqlTables {
			result, err := system.CleanupMySQLTablesFromAgent(common.DataCleanupMySQLDeleteTablesRequest{
				Host:     mysqlStored.Host,
				Port:     mysqlStored.Port,
				Username: mysqlStored.Username,
				Password: mysqlPassword,
				Database: mysqlStored.Database,
				Tables:   []string{table},
			})
			if err != nil {
				mysqlFailed = true
				failures++
				logs = append(logs, fmt.Sprintf("[%s] mysql table %s failed: %s", time.Now().Format(time.RFC3339), table, err.Error()))
				results = append(results, dataCleanupRunResult{
					Module: "mysql",
					Status: "failed",
					Detail: err.Error(),
				})
				break
			}
			logs = append(logs, fmt.Sprintf("[%s] mysql table %s deleted %d rows", time.Now().Format(time.RFC3339), table, result.Deleted))
			completedOps++
			progress := int(float64(completedOps) / float64(totalOps) * 100)
			if err := h.updateDataCleanupRun(runID, "running", progress, "mysql", logs, results); err != nil {
				h.logDataCleanupError("update cleanup run failed", err, "run", runID)
				return
			}
		}
		if !mysqlFailed {
			results = append(results, dataCleanupRunResult{
				Module: "mysql",
				Status: "success",
			})
		}
	}

	if redisStored.Host != "" && redisStored.Port > 0 && len(redisPatterns) > 0 {
		logs = append(logs, fmt.Sprintf("[%s] start redis cleanup", time.Now().Format(time.RFC3339)))
		redisFailed := false
		for _, pattern := range redisPatterns {
			result, err := system.CleanupRedisFromAgent(common.DataCleanupRedisCleanupRequest{
				Host:     redisStored.Host,
				Port:     redisStored.Port,
				Username: redisStored.Username,
				Password: redisPassword,
				DB:       redisStored.DB,
				Patterns: []string{pattern},
			})
			if err != nil {
				redisFailed = true
				failures++
				logs = append(logs, fmt.Sprintf("[%s] redis pattern %s failed: %s", time.Now().Format(time.RFC3339), pattern, err.Error()))
				results = append(results, dataCleanupRunResult{
					Module: "redis",
					Status: "failed",
					Detail: err.Error(),
				})
				break
			}
			logs = append(logs, fmt.Sprintf("[%s] redis pattern %s deleted %d keys", time.Now().Format(time.RFC3339), pattern, result.Deleted))
			completedOps++
			progress := int(float64(completedOps) / float64(totalOps) * 100)
			if err := h.updateDataCleanupRun(runID, "running", progress, "redis", logs, results); err != nil {
				h.logDataCleanupError("update cleanup run failed", err, "run", runID)
				return
			}
		}
		if !redisFailed {
			results = append(results, dataCleanupRunResult{
				Module: "redis",
				Status: "success",
			})
		}
	}

	if minioStored.Host != "" && minioStored.Port > 0 && minioStored.Bucket != "" && len(minioPrefixes) > 0 {
		logs = append(logs, fmt.Sprintf("[%s] start minio cleanup", time.Now().Format(time.RFC3339)))
		minioFailed := false
		for _, prefix := range minioPrefixes {
			result, err := system.CleanupMinioFromAgent(common.DataCleanupMinioCleanupRequest{
				Host:      minioStored.Host,
				Port:      minioStored.Port,
				AccessKey: minioStored.AccessKey,
				SecretKey: minioSecret,
				Bucket:    minioStored.Bucket,
				Prefixes:  []string{prefix},
			})
			if err != nil {
				minioFailed = true
				failures++
				logs = append(logs, fmt.Sprintf("[%s] minio prefix %s failed: %s", time.Now().Format(time.RFC3339), prefix, err.Error()))
				results = append(results, dataCleanupRunResult{
					Module: "minio",
					Status: "failed",
					Detail: err.Error(),
				})
				break
			}
			logs = append(logs, fmt.Sprintf("[%s] minio prefix %s deleted %d objects", time.Now().Format(time.RFC3339), prefix, result.Deleted))
			completedOps++
			progress := int(float64(completedOps) / float64(totalOps) * 100)
			if err := h.updateDataCleanupRun(runID, "running", progress, "minio", logs, results); err != nil {
				h.logDataCleanupError("update cleanup run failed", err, "run", runID)
				return
			}
		}
		if !minioFailed {
			results = append(results, dataCleanupRunResult{
				Module: "minio",
				Status: "success",
			})
		}
	}

	if esStored.Host != "" && esStored.Port > 0 && len(esIndices) > 0 {
		logs = append(logs, fmt.Sprintf("[%s] start es cleanup", time.Now().Format(time.RFC3339)))
		esFailed := false
		for _, index := range esIndices {
			result, err := system.CleanupESFromAgent(common.DataCleanupESCleanupRequest{
				Host:     esStored.Host,
				Port:     esStored.Port,
				Username: esStored.Username,
				Password: esPassword,
				Indices:  []string{index},
			})
			if err != nil {
				esFailed = true
				failures++
				logs = append(logs, fmt.Sprintf("[%s] es index %s failed: %s", time.Now().Format(time.RFC3339), index, err.Error()))
				results = append(results, dataCleanupRunResult{
					Module: "es",
					Status: "failed",
					Detail: err.Error(),
				})
				break
			}
			logs = append(logs, fmt.Sprintf("[%s] es index %s deleted %d docs", time.Now().Format(time.RFC3339), index, result.Deleted))
			completedOps++
			progress := int(float64(completedOps) / float64(totalOps) * 100)
			if err := h.updateDataCleanupRun(runID, "running", progress, "es", logs, results); err != nil {
				h.logDataCleanupError("update cleanup run failed", err, "run", runID)
				return
			}
		}
		if !esFailed {
			results = append(results, dataCleanupRunResult{
				Module: "es",
				Status: "success",
			})
		}
	}

	status := "success"
	if failures > 0 {
		status = "failed"
	}
	if err := h.updateDataCleanupRun(runID, status, 100, "done", logs, results); err != nil {
		h.logDataCleanupError("finalize cleanup run failed", err, "run", runID)
		return
	}

	if auditErr := h.recordDockerAudit(dockerAuditEntry{
		SystemID:     systemID,
		UserID:       userID,
		Action:       "data_cleanup.run",
		ResourceType: "data_cleanup",
		ResourceID:   runID,
		Status:       status,
		Detail:       fmt.Sprintf("cleanup run %s", status),
	}); auditErr != nil {
		h.logDataCleanupError("record cleanup audit failed", auditErr, "run", runID)
	}
}

func (h *Hub) updateDataCleanupRun(
	runID string,
	status string,
	progress int,
	step string,
	logs []string,
	results []dataCleanupRunResult,
) error {
	record, err := h.FindRecordById(dataCleanupRunsCollection, runID)
	if err != nil {
		return err
	}
	record.Set("status", status)
	record.Set("progress", progress)
	record.Set("step", step)
	logsRaw, err := toJSONRaw(logs)
	if err != nil {
		return err
	}
	resultsRaw, err := toJSONRaw(results)
	if err != nil {
		return err
	}
	record.Set("logs", logsRaw)
	record.Set("results", resultsRaw)
	return h.Save(record)
}

func (h *Hub) failDataCleanupRun(runID string, logs []string, results []dataCleanupRunResult, err error) error {
	logs = append(logs, fmt.Sprintf("[%s] cleanup failed: %s", time.Now().Format(time.RFC3339), err.Error()))
	results = append(results, dataCleanupRunResult{
		Module: "run",
		Status: "failed",
		Detail: err.Error(),
	})
	return h.updateDataCleanupRun(runID, "failed", 100, "failed", logs, results)
}

func (h *Hub) parsePortOrDefault(value string, fallback int) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}
	port, err := strconv.Atoi(value)
	if err != nil {
		return 0, err
	}
	return port, nil
}

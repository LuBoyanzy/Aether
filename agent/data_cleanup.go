// data_cleanup.go 提供数据清理相关的 Agent 能力实现。
// 覆盖 MySQL/Redis/MinIO/ES 的资源拉取与清理操作。
package agent

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"

	"aether/internal/common"

	"github.com/fxamacker/cbor/v2"
	"github.com/go-sql-driver/mysql"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"
	"golang.org/x/exp/slog"
)

const (
	dataCleanupListTimeout        = 20 * time.Second
	dataCleanupActionTimeout      = 30 * time.Minute
	dataCleanupScanCount          = 500
	dataCleanupMinioProgressBatch = 5000
)

type dataCleanupIndexItem struct {
	Index string `json:"index"`
}

type dataCleanupDeleteResponse struct {
	Deleted  int64           `json:"deleted"`
	Failures []any           `json:"failures"`
	Error    json.RawMessage `json:"error"`
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

func encodeDataCleanupJobStatusDetail(snapshot dataCleanupJobSnapshot) (string, error) {
	detail := common.DataCleanupJobStatusDetail{
		JobID:   snapshot.JobID,
		Module:  snapshot.Module,
		Status:  snapshot.Status,
		Current: snapshot.Current,
		Done:    snapshot.Done,
		Total:   snapshot.Total,
		Seq:     snapshot.Seq,
		Error:   snapshot.Error,
	}
	encoded, err := json.Marshal(detail)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func requireHostPort(host string, port int, fields map[string]any) (string, error) {
	trimmed := strings.TrimSpace(host)
	if trimmed == "" {
		return "", formatDataCleanupError("host is required", errors.New("host is required"), fields)
	}
	if port <= 0 {
		return "", formatDataCleanupError("port is required", errors.New("port is required"), fields)
	}
	return net.JoinHostPort(trimmed, strconv.Itoa(port)), nil
}

func newMySQLConfig(
	req common.DataCleanupMySQLDatabasesRequest,
	dbName string,
	timeout time.Duration,
) (*mysql.Config, error) {
	addr, err := requireHostPort(req.Host, req.Port, map[string]any{"host": req.Host, "port": req.Port})
	if err != nil {
		return nil, err
	}
	cfg := mysql.NewConfig()
	cfg.User = strings.TrimSpace(req.Username)
	cfg.Passwd = req.Password
	cfg.Net = "tcp"
	cfg.Addr = addr
	cfg.DBName = strings.TrimSpace(dbName)
	cfg.ParseTime = true
	cfg.Timeout = timeout
	cfg.ReadTimeout = timeout
	cfg.WriteTimeout = timeout
	return cfg, nil
}

func openMySQL(ctx context.Context, cfg *mysql.Config) (*sql.DB, error) {
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, formatDataCleanupError("open mysql failed", err, map[string]any{"addr": cfg.Addr, "db": cfg.DBName})
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, formatDataCleanupError("ping mysql failed", err, map[string]any{"addr": cfg.Addr, "db": cfg.DBName})
	}
	return db, nil
}

func listMySQLDatabases(ctx context.Context, req common.DataCleanupMySQLDatabasesRequest) ([]string, error) {
	cfg, err := newMySQLConfig(req, "", dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	db, err := openMySQL(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, formatDataCleanupError("list mysql databases failed", err, map[string]any{"addr": cfg.Addr})
	}
	defer rows.Close()

	items := make([]string, 0, 16)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, formatDataCleanupError("scan mysql database failed", err, map[string]any{"addr": cfg.Addr})
		}
		if name != "" {
			items = append(items, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, formatDataCleanupError("iterate mysql databases failed", err, map[string]any{"addr": cfg.Addr})
	}
	sort.Strings(items)
	return items, nil
}

func listMySQLTables(ctx context.Context, req common.DataCleanupMySQLTablesRequest) ([]string, error) {
	cfg, err := newMySQLConfig(common.DataCleanupMySQLDatabasesRequest{
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
	}, req.Database, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.Database) == "" {
		return nil, formatDataCleanupError("database is required", errors.New("database is required"), map[string]any{"addr": cfg.Addr})
	}
	db, err := openMySQL(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, "SHOW TABLES")
	if err != nil {
		return nil, formatDataCleanupError("list mysql tables failed", err, map[string]any{"addr": cfg.Addr, "db": cfg.DBName})
	}
	defer rows.Close()

	items := make([]string, 0, 32)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, formatDataCleanupError("scan mysql table failed", err, map[string]any{"addr": cfg.Addr, "db": cfg.DBName})
		}
		if name != "" {
			items = append(items, name)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, formatDataCleanupError("iterate mysql tables failed", err, map[string]any{"addr": cfg.Addr, "db": cfg.DBName})
	}
	sort.Strings(items)
	return items, nil
}

func escapeMySQLIdentifier(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", formatDataCleanupError("table name is required", errors.New("table is required"), map[string]any{})
	}
	if strings.Contains(trimmed, "\x00") {
		return "", formatDataCleanupError("invalid mysql identifier", errors.New("invalid table name"), map[string]any{"table": trimmed})
	}
	escaped := strings.ReplaceAll(trimmed, "`", "``")
	return fmt.Sprintf("`%s`", escaped), nil
}

func deleteMySQLTables(ctx context.Context, req common.DataCleanupMySQLDeleteTablesRequest) (int64, error) {
	cfg, err := newMySQLConfig(common.DataCleanupMySQLDatabasesRequest{
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
	}, req.Database, dataCleanupActionTimeout)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(req.Database) == "" {
		return 0, formatDataCleanupError("database is required", errors.New("database is required"), map[string]any{"addr": cfg.Addr})
	}
	if len(req.Tables) == 0 {
		return 0, formatDataCleanupError("tables are required", errors.New("tables are required"), map[string]any{"addr": cfg.Addr, "db": cfg.DBName})
	}
	db, err := openMySQL(ctx, cfg)
	if err != nil {
		return 0, err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, formatDataCleanupError("begin mysql transaction failed", err, map[string]any{"addr": cfg.Addr, "db": cfg.DBName})
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if _, err := tx.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0"); err != nil {
		return 0, formatDataCleanupError("disable foreign key checks failed", err, map[string]any{"addr": cfg.Addr, "db": cfg.DBName})
	}

	var deleted int64
	for _, table := range req.Tables {
		escaped, err := escapeMySQLIdentifier(table)
		if err != nil {
			return 0, err
		}
		result, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s", escaped))
		if err != nil {
			return 0, formatDataCleanupError("delete mysql table failed", err, map[string]any{"addr": cfg.Addr, "db": cfg.DBName, "table": table})
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return 0, formatDataCleanupError("get mysql rows affected failed", err, map[string]any{"addr": cfg.Addr, "db": cfg.DBName, "table": table})
		}
		deleted += rows
	}

	if err := tx.Commit(); err != nil {
		return 0, formatDataCleanupError("commit mysql delete failed", err, map[string]any{"addr": cfg.Addr, "db": cfg.DBName})
	}
	return deleted, nil
}

func newRedisClient(req common.DataCleanupRedisDatabasesRequest, db int) (*redis.Client, error) {
	addr, err := requireHostPort(req.Host, req.Port, map[string]any{"host": req.Host, "port": req.Port})
	if err != nil {
		return nil, err
	}
	opts := &redis.Options{
		Addr:     addr,
		Username: strings.TrimSpace(req.Username),
		Password: req.Password,
		DB:       db,
	}
	return redis.NewClient(opts), nil
}

func redisConfigKeyMatches(raw any, key string) bool {
	switch v := raw.(type) {
	case string:
		return v == key
	case []byte:
		return string(v) == key
	default:
		return false
	}
}

func extractRedisConfigValue(raw any, key string) (any, bool) {
	switch v := raw.(type) {
	case []interface{}:
		if len(v) < 2 || !redisConfigKeyMatches(v[0], key) {
			return nil, false
		}
		return v[1], true
	case map[string]any:
		value, ok := v[key]
		return value, ok
	case map[string]string:
		value, ok := v[key]
		if !ok {
			return nil, false
		}
		return value, true
	case map[interface{}]interface{}:
		for mapKey, value := range v {
			if redisConfigKeyMatches(mapKey, key) {
				return value, true
			}
		}
		return nil, false
	default:
		return nil, false
	}
}

func parseRedisConfigCount(value any, host string, port int) (int, error) {
	switch v := value.(type) {
	case string:
		return parseRedisConfigCountString(v, host, port)
	case []byte:
		return parseRedisConfigCountString(string(v), host, port)
	case int:
		if v <= 0 {
			return 0, formatDataCleanupError("redis databases count invalid", errors.New("redis databases count invalid"), map[string]any{"host": host, "port": port, "count": v})
		}
		return v, nil
	case int64:
		if v <= 0 {
			return 0, formatDataCleanupError("redis databases count invalid", errors.New("redis databases count invalid"), map[string]any{"host": host, "port": port, "count": v})
		}
		maxInt := int64(int(^uint(0) >> 1))
		if v > maxInt {
			return 0, formatDataCleanupError("redis databases count invalid", errors.New("redis databases count overflow"), map[string]any{"host": host, "port": port, "count": v})
		}
		return int(v), nil
	case uint64:
		if v == 0 {
			return 0, formatDataCleanupError("redis databases count invalid", errors.New("redis databases count invalid"), map[string]any{"host": host, "port": port, "count": v})
		}
		maxInt := uint64(int(^uint(0) >> 1))
		if v > maxInt {
			return 0, formatDataCleanupError("redis databases count invalid", errors.New("redis databases count overflow"), map[string]any{"host": host, "port": port, "count": v})
		}
		return int(v), nil
	default:
		return 0, formatDataCleanupError("redis config value invalid", errors.New("unexpected config value type"), map[string]any{"host": host, "port": port, "valueType": fmt.Sprintf("%T", value)})
	}
}

func parseRedisConfigCountString(countStr string, host string, port int) (int, error) {
	count, err := strconv.Atoi(strings.TrimSpace(countStr))
	if err != nil {
		return 0, formatDataCleanupError("parse redis databases count failed", err, map[string]any{"host": host, "port": port, "value": countStr})
	}
	if count <= 0 {
		return 0, formatDataCleanupError("redis databases count invalid", errors.New("redis databases count invalid"), map[string]any{"host": host, "port": port, "count": count})
	}
	return count, nil
}

func listRedisDatabases(ctx context.Context, req common.DataCleanupRedisDatabasesRequest) ([]int, error) {
	client, err := newRedisClient(req, 0)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, formatDataCleanupError("ping redis failed", err, map[string]any{"host": req.Host, "port": req.Port})
	}

	raw, err := client.Do(ctx, "CONFIG", "GET", "databases").Result()
	if err != nil {
		return nil, formatDataCleanupError("redis config get databases failed", err, map[string]any{"host": req.Host, "port": req.Port})
	}

	value, ok := extractRedisConfigValue(raw, "databases")
	if !ok {
		return nil, formatDataCleanupError("redis config response invalid", errors.New("unexpected config response"), map[string]any{"host": req.Host, "port": req.Port, "raw": raw})
	}

	count, err := parseRedisConfigCount(value, req.Host, req.Port)
	if err != nil {
		return nil, err
	}

	dbs := make([]int, count)
	for i := 0; i < count; i++ {
		dbs[i] = i
	}
	return dbs, nil
}

func cleanupRedis(ctx context.Context, req common.DataCleanupRedisCleanupRequest) (int64, error) {
	if len(req.Patterns) == 0 {
		return 0, formatDataCleanupError("redis patterns required", errors.New("patterns are required"), map[string]any{"host": req.Host, "port": req.Port})
	}
	client, err := newRedisClient(common.DataCleanupRedisDatabasesRequest{
		Host:     req.Host,
		Port:     req.Port,
		Username: req.Username,
		Password: req.Password,
	}, req.DB)
	if err != nil {
		return 0, err
	}
	defer client.Close()

	if err := client.Ping(ctx).Err(); err != nil {
		return 0, formatDataCleanupError("ping redis failed", err, map[string]any{"host": req.Host, "port": req.Port, "db": req.DB})
	}

	var deleted int64
	for _, pattern := range req.Patterns {
		cursor := uint64(0)
		for {
			keys, nextCursor, err := client.Scan(ctx, cursor, pattern, dataCleanupScanCount).Result()
			if err != nil {
				return deleted, formatDataCleanupError("redis scan failed", err, map[string]any{"host": req.Host, "port": req.Port, "db": req.DB, "pattern": pattern})
			}
			if len(keys) > 0 {
				count, err := client.Del(ctx, keys...).Result()
				if err != nil {
					return deleted, formatDataCleanupError("redis delete failed", err, map[string]any{"host": req.Host, "port": req.Port, "db": req.DB, "pattern": pattern})
				}
				deleted += count
			}
			if nextCursor == 0 {
				break
			}
			cursor = nextCursor
		}
	}

	return deleted, nil
}

func newMinioClient(req common.DataCleanupMinioBucketsRequest) (*minio.Client, error) {
	addr, err := requireHostPort(req.Host, req.Port, map[string]any{"host": req.Host, "port": req.Port})
	if err != nil {
		return nil, err
	}
	accessKey := strings.TrimSpace(req.AccessKey)
	client, err := minio.New(addr, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, req.SecretKey, ""),
		Secure: false,
	})
	if err != nil {
		return nil, formatDataCleanupError("init minio client failed", err, map[string]any{"addr": addr})
	}
	return client, nil
}

func listMinioBuckets(ctx context.Context, req common.DataCleanupMinioBucketsRequest) ([]string, error) {
	client, err := newMinioClient(req)
	if err != nil {
		return nil, err
	}
	buckets, err := client.ListBuckets(ctx)
	if err != nil {
		return nil, formatDataCleanupError("list minio buckets failed", err, map[string]any{"host": req.Host, "port": req.Port})
	}
	items := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		if bucket.Name != "" {
			items = append(items, bucket.Name)
		}
	}
	sort.Strings(items)
	return items, nil
}

func listMinioPrefixes(ctx context.Context, req common.DataCleanupMinioPrefixesRequest) ([]string, error) {
	if strings.TrimSpace(req.Bucket) == "" {
		return nil, formatDataCleanupError("bucket is required", errors.New("bucket is required"), map[string]any{"host": req.Host, "port": req.Port})
	}
	client, err := newMinioClient(common.DataCleanupMinioBucketsRequest{
		Host:      req.Host,
		Port:      req.Port,
		AccessKey: req.AccessKey,
		SecretKey: req.SecretKey,
	})
	if err != nil {
		return nil, err
	}

	prefixSet := make(map[string]struct{})
	opts := minio.ListObjectsOptions{Prefix: "", Recursive: false}
	for object := range client.ListObjects(ctx, req.Bucket, opts) {
		if object.Err != nil {
			return nil, formatDataCleanupError("list minio prefixes failed", object.Err, map[string]any{"host": req.Host, "port": req.Port, "bucket": req.Bucket})
		}
		if object.Key != "" &&
			strings.HasSuffix(object.Key, "/") &&
			object.ETag == "" &&
			object.LastModified.IsZero() {
			prefixSet[object.Key] = struct{}{}
		}
	}

	items := make([]string, 0, len(prefixSet))
	for prefix := range prefixSet {
		items = append(items, prefix)
	}
	sort.Strings(items)
	return items, nil
}

func normalizeMinioPrefix(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	if strings.HasSuffix(trimmed, "/") {
		return trimmed
	}
	return trimmed + "/"
}

func cleanupMinioPrefix(ctx context.Context, client *minio.Client, bucket, prefix string) (int64, error) {
	target := normalizeMinioPrefix(prefix)
	if target == "" {
		return 0, formatDataCleanupError("minio prefix is required", errors.New("prefix is required"), map[string]any{"bucket": bucket})
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	objectsCh := make(chan minio.ObjectInfo)
	listErrCh := make(chan error, 1)
	go func() {
		defer close(objectsCh)
		opts := minio.ListObjectsOptions{Prefix: target, Recursive: true}
		for object := range client.ListObjects(ctx, bucket, opts) {
			if object.Err != nil {
				listErrCh <- object.Err
				cancel()
				return
			}
			objectsCh <- object
		}
	}()

	var deleted int64
	for result := range client.RemoveObjectsWithResult(ctx, bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if result.Err != nil {
			select {
			case err := <-listErrCh:
				if err != nil {
					return deleted, formatDataCleanupError("list minio objects failed", err, map[string]any{"bucket": bucket, "prefix": target})
				}
			default:
			}
			return deleted, formatDataCleanupError("remove minio objects failed", result.Err, map[string]any{"bucket": bucket, "prefix": target})
		}
		deleted++
	}

	select {
	case err := <-listErrCh:
		if err != nil {
			return deleted, formatDataCleanupError("list minio objects failed", err, map[string]any{"bucket": bucket, "prefix": target})
		}
	default:
	}

	return deleted, nil
}

func cleanupMinioPrefixWithProgress(
	ctx context.Context,
	client *minio.Client,
	bucket, prefix string,
	onBatchDeleted func(int64),
) (int64, error) {
	target := normalizeMinioPrefix(prefix)
	if target == "" {
		return 0, formatDataCleanupError("minio prefix is required", errors.New("prefix is required"), map[string]any{"bucket": bucket})
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	objectsCh := make(chan minio.ObjectInfo)
	listErrCh := make(chan error, 1)
	go func() {
		defer close(objectsCh)
		opts := minio.ListObjectsOptions{Prefix: target, Recursive: true}
		for object := range client.ListObjects(ctx, bucket, opts) {
			if object.Err != nil {
				listErrCh <- object.Err
				cancel()
				return
			}
			objectsCh <- object
		}
	}()

	var deleted int64
	var batch int64
	for result := range client.RemoveObjectsWithResult(ctx, bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if result.Err != nil {
			select {
			case err := <-listErrCh:
				if err != nil {
					return deleted, formatDataCleanupError("list minio objects failed", err, map[string]any{"bucket": bucket, "prefix": target})
				}
			default:
			}
			return deleted, formatDataCleanupError("remove minio objects failed", result.Err, map[string]any{"bucket": bucket, "prefix": target})
		}
		deleted++
		batch++
		if batch >= dataCleanupMinioProgressBatch {
			if onBatchDeleted != nil {
				onBatchDeleted(batch)
			}
			batch = 0
		}
	}

	if batch > 0 && onBatchDeleted != nil {
		onBatchDeleted(batch)
	}

	select {
	case err := <-listErrCh:
		if err != nil {
			return deleted, formatDataCleanupError("list minio objects failed", err, map[string]any{"bucket": bucket, "prefix": target})
		}
	default:
	}

	return deleted, nil
}

func cleanupMinio(ctx context.Context, req common.DataCleanupMinioCleanupRequest) (int64, error) {
	if strings.TrimSpace(req.Bucket) == "" {
		return 0, formatDataCleanupError("bucket is required", errors.New("bucket is required"), map[string]any{"host": req.Host, "port": req.Port})
	}
	if len(req.Prefixes) == 0 {
		return 0, formatDataCleanupError("minio prefixes required", errors.New("prefixes are required"), map[string]any{"bucket": req.Bucket})
	}
	client, err := newMinioClient(common.DataCleanupMinioBucketsRequest{
		Host:      req.Host,
		Port:      req.Port,
		AccessKey: req.AccessKey,
		SecretKey: req.SecretKey,
	})
	if err != nil {
		return 0, err
	}

	var deleted int64
	for _, prefix := range req.Prefixes {
		count, err := cleanupMinioPrefix(ctx, client, req.Bucket, prefix)
		if err != nil {
			return deleted, err
		}
		deleted += count
	}
	return deleted, nil
}

func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}

func buildHTTPURL(host string, port int, path string) (string, error) {
	addr, err := requireHostPort(host, port, map[string]any{"host": host, "port": port})
	if err != nil {
		return "", err
	}
	if strings.Contains(host, "://") {
		return "", formatDataCleanupError("host contains scheme", errors.New("host should not include scheme"), map[string]any{"host": host})
	}
	u := url.URL{
		Scheme: "http",
		Host:   addr,
		Path:   path,
	}
	return u.String(), nil
}

func listESIndices(ctx context.Context, req common.DataCleanupESIndicesRequest) ([]string, error) {
	endpoint, err := buildHTTPURL(req.Host, req.Port, "/_cat/indices")
	if err != nil {
		return nil, err
	}
	queryURL := endpoint + "?format=json"
	httpClient := newHTTPClient(dataCleanupListTimeout)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, queryURL, nil)
	if err != nil {
		return nil, formatDataCleanupError("build es indices request failed", err, map[string]any{"endpoint": queryURL})
	}
	if strings.TrimSpace(req.Username) != "" || strings.TrimSpace(req.Password) != "" {
		request.SetBasicAuth(req.Username, req.Password)
	}

	resp, err := httpClient.Do(request)
	if err != nil {
		return nil, formatDataCleanupError("request es indices failed", err, map[string]any{"endpoint": queryURL})
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, formatDataCleanupError("es indices response error", errors.New(string(body)), map[string]any{"status": resp.StatusCode, "endpoint": queryURL})
	}

	var items []dataCleanupIndexItem
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, formatDataCleanupError("decode es indices failed", err, map[string]any{"endpoint": queryURL})
	}

	indices := make([]string, 0, len(items))
	for _, item := range items {
		if item.Index != "" {
			indices = append(indices, item.Index)
		}
	}
	sort.Strings(indices)
	return indices, nil
}

func cleanupESIndices(ctx context.Context, req common.DataCleanupESCleanupRequest) (int64, error) {
	if len(req.Indices) == 0 {
		return 0, formatDataCleanupError("es indices required", errors.New("indices are required"), map[string]any{"host": req.Host, "port": req.Port})
	}
	httpClient := newHTTPClient(dataCleanupActionTimeout)
	var deleted int64

	for _, index := range req.Indices {
		escaped := url.PathEscape(strings.TrimSpace(index))
		if escaped == "" {
			return deleted, formatDataCleanupError("es index required", errors.New("index is required"), map[string]any{"host": req.Host, "port": req.Port})
		}
		endpoint, err := buildHTTPURL(req.Host, req.Port, "/"+escaped+"/_delete_by_query")
		if err != nil {
			return deleted, err
		}
		endpoint += "?conflicts=proceed"
		body := strings.NewReader(`{"query":{"match_all":{}}}`)
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
		if err != nil {
			return deleted, formatDataCleanupError("build es delete request failed", err, map[string]any{"endpoint": endpoint})
		}
		request.Header.Set("Content-Type", "application/json")
		if strings.TrimSpace(req.Username) != "" || strings.TrimSpace(req.Password) != "" {
			request.SetBasicAuth(req.Username, req.Password)
		}

		resp, err := httpClient.Do(request)
		if err != nil {
			return deleted, formatDataCleanupError("request es delete failed", err, map[string]any{"endpoint": endpoint})
		}

		if resp.StatusCode >= http.StatusBadRequest {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			_ = resp.Body.Close()
			return deleted, formatDataCleanupError("es delete response error", errors.New(string(body)), map[string]any{"status": resp.StatusCode, "endpoint": endpoint})
		}

		var payload dataCleanupDeleteResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			_ = resp.Body.Close()
			return deleted, formatDataCleanupError("decode es delete response failed", err, map[string]any{"endpoint": endpoint})
		}
		_ = resp.Body.Close()
		if len(payload.Failures) > 0 || len(payload.Error) > 0 {
			errMsg := strings.TrimSpace(string(payload.Error))
			if errMsg == "" {
				errMsg = "elasticsearch delete failures detected"
			}
			return deleted, formatDataCleanupError("es delete failures detected", errors.New(errMsg), map[string]any{"endpoint": endpoint, "failures": payload.Failures})
		}
		deleted += payload.Deleted
	}
	return deleted, nil
}

type DataCleanupMySQLDatabasesHandler struct{}

func (h *DataCleanupMySQLDatabasesHandler) Handle(hctx *HandlerContext) error {
	var req common.DataCleanupMySQLDatabasesRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return formatDataCleanupError("decode mysql databases request failed", err, map[string]any{})
	}
	ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
	defer cancel()

	items, err := listMySQLDatabases(ctx, req)
	if err != nil {
		slog.Error("mysql databases list failed", "err", err, "host", req.Host, "port", req.Port)
		return err
	}
	return hctx.SendResponse(&common.DockerDataCleanupList{Databases: items}, hctx.RequestID)
}

type DataCleanupMySQLTablesHandler struct{}

func (h *DataCleanupMySQLTablesHandler) Handle(hctx *HandlerContext) error {
	var req common.DataCleanupMySQLTablesRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return formatDataCleanupError("decode mysql tables request failed", err, map[string]any{})
	}
	ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
	defer cancel()

	items, err := listMySQLTables(ctx, req)
	if err != nil {
		slog.Error("mysql tables list failed", "err", err, "host", req.Host, "port", req.Port, "db", req.Database)
		return err
	}
	return hctx.SendResponse(&common.DockerDataCleanupList{Tables: items}, hctx.RequestID)
}

type DataCleanupMySQLDeleteTablesHandler struct{}

func (h *DataCleanupMySQLDeleteTablesHandler) Handle(hctx *HandlerContext) error {
	var req common.DataCleanupMySQLDeleteTablesRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return formatDataCleanupError("decode mysql delete request failed", err, map[string]any{})
	}
	jobID := strings.TrimSpace(req.JobID)
	if jobID != "" {
		if len(req.Tables) == 0 {
			return formatDataCleanupError("mysql tables required", errors.New("tables are required"), map[string]any{"host": req.Host, "port": req.Port, "db": req.Database})
		}

		snapshot, err := hctx.Agent.dataCleanupJobs.Start(jobID, "mysql", len(req.Tables), dataCleanupActionTimeout, func(ctx context.Context, job *dataCleanupJob) error {
			slog.Info("mysql cleanup job start", "jobId", jobID, "host", req.Host, "port", req.Port, "db", req.Database, "tables", len(req.Tables))
			var totalDeleted int64

			for _, table := range req.Tables {
				table = strings.TrimSpace(table)
				if table == "" {
					return formatDataCleanupError("mysql table required", errors.New("table is required"), map[string]any{"host": req.Host, "port": req.Port, "db": req.Database})
				}
				job.setCurrent(table)

				perReq := req
				perReq.Tables = []string{table}
				perReq.JobID = ""

				deleted, err := deleteMySQLTables(ctx, perReq)
				if err != nil {
					slog.Error("mysql cleanup failed", "err", err, "jobId", jobID, "host", req.Host, "port", req.Port, "db", req.Database, "table", table)
					return err
				}
				totalDeleted += deleted
				job.markItemDoneWithDeleted(deleted)
			}

			slog.Info("mysql cleanup job done", "jobId", jobID, "host", req.Host, "port", req.Port, "db", req.Database, "deleted", totalDeleted)
			return nil
		})
		if err != nil {
			return err
		}
		detail, err := encodeDataCleanupJobStatusDetail(snapshot)
		if err != nil {
			return formatDataCleanupError("encode data cleanup job status failed", err, map[string]any{"jobId": jobID, "module": "mysql"})
		}
		return hctx.SendResponse(&common.DockerDataCleanupResult{Deleted: snapshot.Deleted, Detail: detail}, hctx.RequestID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dataCleanupActionTimeout)
	defer cancel()

	slog.Info("mysql cleanup start", "host", req.Host, "port", req.Port, "db", req.Database, "tables", len(req.Tables))
	deleted, err := deleteMySQLTables(ctx, req)
	if err != nil {
		slog.Error("mysql cleanup failed", "err", err, "host", req.Host, "port", req.Port, "db", req.Database)
		return err
	}
	slog.Info("mysql cleanup done", "host", req.Host, "port", req.Port, "db", req.Database, "deleted", deleted)
	return hctx.SendResponse(&common.DockerDataCleanupResult{Deleted: deleted}, hctx.RequestID)
}

type DataCleanupRedisDatabasesHandler struct{}

func (h *DataCleanupRedisDatabasesHandler) Handle(hctx *HandlerContext) error {
	var req common.DataCleanupRedisDatabasesRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return formatDataCleanupError("decode redis databases request failed", err, map[string]any{})
	}
	ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
	defer cancel()

	items, err := listRedisDatabases(ctx, req)
	if err != nil {
		slog.Error("redis databases list failed", "err", err, "host", req.Host, "port", req.Port)
		return err
	}
	return hctx.SendResponse(&common.DockerDataCleanupList{RedisDBs: items}, hctx.RequestID)
}

type DataCleanupRedisCleanupHandler struct{}

func (h *DataCleanupRedisCleanupHandler) Handle(hctx *HandlerContext) error {
	var req common.DataCleanupRedisCleanupRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return formatDataCleanupError("decode redis cleanup request failed", err, map[string]any{})
	}
	jobID := strings.TrimSpace(req.JobID)
	if jobID != "" {
		if len(req.Patterns) == 0 {
			return formatDataCleanupError("redis patterns required", errors.New("patterns are required"), map[string]any{"host": req.Host, "port": req.Port, "db": req.DB})
		}

		snapshot, err := hctx.Agent.dataCleanupJobs.Start(jobID, "redis", len(req.Patterns), dataCleanupActionTimeout, func(ctx context.Context, job *dataCleanupJob) error {
			slog.Info("redis cleanup job start", "jobId", jobID, "host", req.Host, "port", req.Port, "db", req.DB, "patterns", len(req.Patterns))
			var totalDeleted int64

			for _, pattern := range req.Patterns {
				pattern = strings.TrimSpace(pattern)
				if pattern == "" {
					return formatDataCleanupError("redis pattern required", errors.New("pattern is required"), map[string]any{"host": req.Host, "port": req.Port, "db": req.DB})
				}
				job.setCurrent(pattern)

				perReq := req
				perReq.Patterns = []string{pattern}
				perReq.JobID = ""

				deleted, err := cleanupRedis(ctx, perReq)
				if err != nil {
					slog.Error("redis cleanup failed", "err", err, "jobId", jobID, "host", req.Host, "port", req.Port, "db", req.DB, "pattern", pattern)
					return err
				}
				totalDeleted += deleted
				job.markItemDoneWithDeleted(deleted)
			}

			slog.Info("redis cleanup job done", "jobId", jobID, "host", req.Host, "port", req.Port, "db", req.DB, "deleted", totalDeleted)
			return nil
		})
		if err != nil {
			return err
		}
		detail, err := encodeDataCleanupJobStatusDetail(snapshot)
		if err != nil {
			return formatDataCleanupError("encode data cleanup job status failed", err, map[string]any{"jobId": jobID, "module": "redis"})
		}
		return hctx.SendResponse(&common.DockerDataCleanupResult{Deleted: snapshot.Deleted, Detail: detail}, hctx.RequestID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dataCleanupActionTimeout)
	defer cancel()

	slog.Info("redis cleanup start", "host", req.Host, "port", req.Port, "db", req.DB, "patterns", len(req.Patterns))
	deleted, err := cleanupRedis(ctx, req)
	if err != nil {
		slog.Error("redis cleanup failed", "err", err, "host", req.Host, "port", req.Port, "db", req.DB)
		return err
	}
	slog.Info("redis cleanup done", "host", req.Host, "port", req.Port, "db", req.DB, "deleted", deleted)
	return hctx.SendResponse(&common.DockerDataCleanupResult{Deleted: deleted}, hctx.RequestID)
}

type DataCleanupMinioBucketsHandler struct{}

func (h *DataCleanupMinioBucketsHandler) Handle(hctx *HandlerContext) error {
	var req common.DataCleanupMinioBucketsRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return formatDataCleanupError("decode minio buckets request failed", err, map[string]any{})
	}
	ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
	defer cancel()

	items, err := listMinioBuckets(ctx, req)
	if err != nil {
		slog.Error("minio buckets list failed", "err", err, "host", req.Host, "port", req.Port)
		return err
	}
	return hctx.SendResponse(&common.DockerDataCleanupList{Buckets: items}, hctx.RequestID)
}

type DataCleanupMinioPrefixesHandler struct{}

func (h *DataCleanupMinioPrefixesHandler) Handle(hctx *HandlerContext) error {
	var req common.DataCleanupMinioPrefixesRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return formatDataCleanupError("decode minio prefixes request failed", err, map[string]any{})
	}
	ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
	defer cancel()

	items, err := listMinioPrefixes(ctx, req)
	if err != nil {
		slog.Error("minio prefixes list failed", "err", err, "host", req.Host, "port", req.Port, "bucket", req.Bucket)
		return err
	}
	return hctx.SendResponse(&common.DockerDataCleanupList{Prefixes: items}, hctx.RequestID)
}

type DataCleanupMinioCleanupHandler struct{}

func (h *DataCleanupMinioCleanupHandler) Handle(hctx *HandlerContext) error {
	var req common.DataCleanupMinioCleanupRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return formatDataCleanupError("decode minio cleanup request failed", err, map[string]any{})
	}
	jobID := strings.TrimSpace(req.JobID)
	if jobID != "" {
		if strings.TrimSpace(req.Bucket) == "" {
			return formatDataCleanupError("bucket is required", errors.New("bucket is required"), map[string]any{"host": req.Host, "port": req.Port})
		}
		if len(req.Prefixes) == 0 {
			return formatDataCleanupError("minio prefixes required", errors.New("prefixes are required"), map[string]any{"bucket": req.Bucket})
		}

		snapshot, err := hctx.Agent.dataCleanupJobs.Start(jobID, "minio", len(req.Prefixes), dataCleanupActionTimeout, func(ctx context.Context, job *dataCleanupJob) error {
			slog.Info("minio cleanup job start", "jobId", jobID, "host", req.Host, "port", req.Port, "bucket", req.Bucket, "prefixes", len(req.Prefixes))

			client, err := newMinioClient(common.DataCleanupMinioBucketsRequest{
				Host:      req.Host,
				Port:      req.Port,
				AccessKey: req.AccessKey,
				SecretKey: req.SecretKey,
			})
			if err != nil {
				slog.Error("minio cleanup failed", "err", err, "jobId", jobID, "host", req.Host, "port", req.Port, "bucket", req.Bucket)
				return err
			}

			var totalDeleted int64
			for _, prefix := range req.Prefixes {
				prefix = strings.TrimSpace(prefix)
				if prefix == "" {
					return formatDataCleanupError("minio prefix is required", errors.New("prefix is required"), map[string]any{"bucket": req.Bucket})
				}
				job.setCurrent(prefix)

				count, err := cleanupMinioPrefixWithProgress(ctx, client, req.Bucket, prefix, func(batch int64) {
					job.addDeleted(batch)
				})
				totalDeleted += count
				if err != nil {
					slog.Error("minio cleanup failed", "err", err, "jobId", jobID, "host", req.Host, "port", req.Port, "bucket", req.Bucket, "prefix", prefix)
					return err
				}
				job.markItemDone()
			}

			slog.Info("minio cleanup job done", "jobId", jobID, "host", req.Host, "port", req.Port, "bucket", req.Bucket, "deleted", totalDeleted)
			return nil
		})
		if err != nil {
			return err
		}
		detail, err := encodeDataCleanupJobStatusDetail(snapshot)
		if err != nil {
			return formatDataCleanupError("encode data cleanup job status failed", err, map[string]any{"jobId": jobID, "module": "minio"})
		}
		return hctx.SendResponse(&common.DockerDataCleanupResult{Deleted: snapshot.Deleted, Detail: detail}, hctx.RequestID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dataCleanupActionTimeout)
	defer cancel()

	slog.Info("minio cleanup start", "host", req.Host, "port", req.Port, "bucket", req.Bucket, "prefixes", len(req.Prefixes))
	deleted, err := cleanupMinio(ctx, req)
	if err != nil {
		slog.Error("minio cleanup failed", "err", err, "host", req.Host, "port", req.Port, "bucket", req.Bucket)
		return err
	}
	slog.Info("minio cleanup done", "host", req.Host, "port", req.Port, "bucket", req.Bucket, "deleted", deleted)
	return hctx.SendResponse(&common.DockerDataCleanupResult{Deleted: deleted}, hctx.RequestID)
}

type DataCleanupESIndicesHandler struct{}

func (h *DataCleanupESIndicesHandler) Handle(hctx *HandlerContext) error {
	var req common.DataCleanupESIndicesRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return formatDataCleanupError("decode es indices request failed", err, map[string]any{})
	}
	ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
	defer cancel()

	items, err := listESIndices(ctx, req)
	if err != nil {
		slog.Error("es indices list failed", "err", err, "host", req.Host, "port", req.Port)
		return err
	}
	return hctx.SendResponse(&common.DockerDataCleanupList{Indices: items}, hctx.RequestID)
}

type DataCleanupESCleanupHandler struct{}

func (h *DataCleanupESCleanupHandler) Handle(hctx *HandlerContext) error {
	var req common.DataCleanupESCleanupRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return formatDataCleanupError("decode es cleanup request failed", err, map[string]any{})
	}
	jobID := strings.TrimSpace(req.JobID)
	if jobID != "" {
		if len(req.Indices) == 0 {
			return formatDataCleanupError("es indices required", errors.New("indices are required"), map[string]any{"host": req.Host, "port": req.Port})
		}

		snapshot, err := hctx.Agent.dataCleanupJobs.Start(jobID, "es", len(req.Indices), dataCleanupActionTimeout, func(ctx context.Context, job *dataCleanupJob) error {
			slog.Info("es cleanup job start", "jobId", jobID, "host", req.Host, "port", req.Port, "indices", len(req.Indices))
			var totalDeleted int64

			for _, index := range req.Indices {
				index = strings.TrimSpace(index)
				if index == "" {
					return formatDataCleanupError("es index required", errors.New("index is required"), map[string]any{"host": req.Host, "port": req.Port})
				}
				job.setCurrent(index)

				perReq := req
				perReq.Indices = []string{index}
				perReq.JobID = ""

				deleted, err := cleanupESIndices(ctx, perReq)
				if err != nil {
					slog.Error("es cleanup failed", "err", err, "jobId", jobID, "host", req.Host, "port", req.Port, "index", index)
					return err
				}
				totalDeleted += deleted
				job.markItemDoneWithDeleted(deleted)
			}

			slog.Info("es cleanup job done", "jobId", jobID, "host", req.Host, "port", req.Port, "deleted", totalDeleted)
			return nil
		})
		if err != nil {
			return err
		}
		detail, err := encodeDataCleanupJobStatusDetail(snapshot)
		if err != nil {
			return formatDataCleanupError("encode data cleanup job status failed", err, map[string]any{"jobId": jobID, "module": "es"})
		}
		return hctx.SendResponse(&common.DockerDataCleanupResult{Deleted: snapshot.Deleted, Detail: detail}, hctx.RequestID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), dataCleanupActionTimeout)
	defer cancel()

	slog.Info("es cleanup start", "host", req.Host, "port", req.Port, "indices", len(req.Indices))
	deleted, err := cleanupESIndices(ctx, req)
	if err != nil {
		slog.Error("es cleanup failed", "err", err, "host", req.Host, "port", req.Port)
		return err
	}
	slog.Info("es cleanup done", "host", req.Host, "port", req.Port, "deleted", deleted)
	return hctx.SendResponse(&common.DockerDataCleanupResult{Deleted: deleted}, hctx.RequestID)
}

type DataCleanupJobStatusHandler struct{}

func (h *DataCleanupJobStatusHandler) Handle(hctx *HandlerContext) error {
	var req common.DataCleanupJobStatusRequest
	if err := cbor.Unmarshal(hctx.Request.Data, &req); err != nil {
		return formatDataCleanupError("decode data cleanup job status request failed", err, map[string]any{})
	}
	jobID := strings.TrimSpace(req.JobID)
	if jobID == "" {
		return formatDataCleanupError("jobId is required", errors.New("jobId is required"), map[string]any{})
	}
	snapshot, err := hctx.Agent.dataCleanupJobs.Snapshot(jobID)
	if err != nil {
		return formatDataCleanupError("data cleanup job not found", err, map[string]any{"jobId": jobID})
	}
	detail, err := encodeDataCleanupJobStatusDetail(snapshot)
	if err != nil {
		return formatDataCleanupError("encode data cleanup job status failed", err, map[string]any{"jobId": jobID})
	}
	return hctx.SendResponse(&common.DockerDataCleanupResult{Deleted: snapshot.Deleted, Detail: detail}, hctx.RequestID)
}

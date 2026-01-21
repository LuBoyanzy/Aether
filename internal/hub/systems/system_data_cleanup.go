package systems

import (
	"context"
	"errors"
	"time"

	"aether/internal/common"
)

const (
	dataCleanupListTimeout   = 20 * time.Second
	dataCleanupActionTimeout = 30 * time.Minute
)

func (sys *System) FetchDataCleanupMySQLDatabasesFromAgent(
	req common.DataCleanupMySQLDatabasesRequest,
) ([]string, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
		defer cancel()
		return sys.WsConn.RequestDataCleanupMySQLDatabases(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.DataCleanupMySQLDatabases, req, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	if resp.DataCleanupList == nil {
		return nil, errors.New("no mysql databases in response")
	}
	return resp.DataCleanupList.Databases, nil
}

func (sys *System) FetchDataCleanupMySQLTablesFromAgent(
	req common.DataCleanupMySQLTablesRequest,
) ([]string, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
		defer cancel()
		return sys.WsConn.RequestDataCleanupMySQLTables(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.DataCleanupMySQLTables, req, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	if resp.DataCleanupList == nil {
		return nil, errors.New("no mysql tables in response")
	}
	return resp.DataCleanupList.Tables, nil
}

func (sys *System) CleanupMySQLTablesFromAgent(
	req common.DataCleanupMySQLDeleteTablesRequest,
) (common.DockerDataCleanupResult, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), dataCleanupActionTimeout)
		defer cancel()
		return sys.WsConn.RequestDataCleanupMySQLDeleteTables(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.DataCleanupMySQLDeleteTables, req, dataCleanupActionTimeout)
	if err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	if resp.DataCleanupResult == nil {
		return common.DockerDataCleanupResult{}, errors.New("no mysql cleanup result in response")
	}
	return *resp.DataCleanupResult, nil
}

func (sys *System) FetchDataCleanupRedisDatabasesFromAgent(
	req common.DataCleanupRedisDatabasesRequest,
) ([]int, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
		defer cancel()
		return sys.WsConn.RequestDataCleanupRedisDatabases(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.DataCleanupRedisDatabases, req, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	if resp.DataCleanupList == nil {
		return nil, errors.New("no redis databases in response")
	}
	return resp.DataCleanupList.RedisDBs, nil
}

func (sys *System) CleanupRedisFromAgent(
	req common.DataCleanupRedisCleanupRequest,
) (common.DockerDataCleanupResult, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), dataCleanupActionTimeout)
		defer cancel()
		return sys.WsConn.RequestDataCleanupRedisCleanup(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.DataCleanupRedisCleanup, req, dataCleanupActionTimeout)
	if err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	if resp.DataCleanupResult == nil {
		return common.DockerDataCleanupResult{}, errors.New("no redis cleanup result in response")
	}
	return *resp.DataCleanupResult, nil
}

func (sys *System) FetchDataCleanupMinioBucketsFromAgent(
	req common.DataCleanupMinioBucketsRequest,
) ([]string, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
		defer cancel()
		return sys.WsConn.RequestDataCleanupMinioBuckets(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.DataCleanupMinioBuckets, req, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	if resp.DataCleanupList == nil {
		return nil, errors.New("no minio buckets in response")
	}
	return resp.DataCleanupList.Buckets, nil
}

func (sys *System) FetchDataCleanupMinioPrefixesFromAgent(
	req common.DataCleanupMinioPrefixesRequest,
) ([]string, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
		defer cancel()
		return sys.WsConn.RequestDataCleanupMinioPrefixes(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.DataCleanupMinioPrefixes, req, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	if resp.DataCleanupList == nil {
		return nil, errors.New("no minio prefixes in response")
	}
	return resp.DataCleanupList.Prefixes, nil
}

func (sys *System) CleanupMinioFromAgent(
	req common.DataCleanupMinioCleanupRequest,
) (common.DockerDataCleanupResult, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), dataCleanupActionTimeout)
		defer cancel()
		return sys.WsConn.RequestDataCleanupMinioCleanup(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.DataCleanupMinioCleanup, req, dataCleanupActionTimeout)
	if err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	if resp.DataCleanupResult == nil {
		return common.DockerDataCleanupResult{}, errors.New("no minio cleanup result in response")
	}
	return *resp.DataCleanupResult, nil
}

func (sys *System) FetchDataCleanupESIndicesFromAgent(
	req common.DataCleanupESIndicesRequest,
) ([]string, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
		defer cancel()
		return sys.WsConn.RequestDataCleanupESIndices(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.DataCleanupESIndices, req, dataCleanupListTimeout)
	if err != nil {
		return nil, err
	}
	if resp.DataCleanupList == nil {
		return nil, errors.New("no es indices in response")
	}
	return resp.DataCleanupList.Indices, nil
}

func (sys *System) CleanupESFromAgent(
	req common.DataCleanupESCleanupRequest,
) (common.DockerDataCleanupResult, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), dataCleanupActionTimeout)
		defer cancel()
		return sys.WsConn.RequestDataCleanupESCleanup(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.DataCleanupESCleanup, req, dataCleanupActionTimeout)
	if err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	if resp.DataCleanupResult == nil {
		return common.DockerDataCleanupResult{}, errors.New("no es cleanup result in response")
	}
	return *resp.DataCleanupResult, nil
}

func (sys *System) FetchDataCleanupJobStatusFromAgent(
	req common.DataCleanupJobStatusRequest,
) (common.DockerDataCleanupResult, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), dataCleanupListTimeout)
		defer cancel()
		return sys.WsConn.RequestDataCleanupJobStatus(ctx, req)
	}
	resp, err := sys.fetchDockerResponseViaSSH(common.DataCleanupJobStatus, req, dataCleanupListTimeout)
	if err != nil {
		return common.DockerDataCleanupResult{}, err
	}
	if resp.DataCleanupResult == nil {
		return common.DockerDataCleanupResult{}, errors.New("no data cleanup job status in response")
	}
	return *resp.DataCleanupResult, nil
}

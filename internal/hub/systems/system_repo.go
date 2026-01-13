package systems

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"aether/internal/common"
	"aether/internal/entities/repo"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

func (sys *System) FetchRepoSourcesFromAgent(check bool) ([]repo.Source, error) {
	if sys.WsConn != nil && sys.WsConn.IsConnected() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		return sys.WsConn.RequestRepoSources(ctx, common.RepoSourcesRequest{Check: check})
	}

	resp, err := sys.fetchDockerResponseViaSSH(common.GetRepoSources, common.RepoSourcesRequest{Check: check}, 60*time.Second)
	if err != nil {
		return nil, err
	}
	if resp.RepoSources == nil {
		return nil, errors.New("no repo sources in response")
	}
	return resp.RepoSources, nil
}

func (sys *System) FetchAndSaveRepoSources(check bool) error {
	sources, err := sys.FetchRepoSourcesFromAgent(check)
	if err != nil {
		return err
	}
	return sys.manager.hub.RunInTransaction(func(txApp core.App) error {
		return syncSystemRepoSources(txApp, sys.Id, sources, !check)
	})
}

func syncSystemRepoSources(app core.App, systemID string, sources []repo.Source, preserveStatus bool) error {
	if sources == nil {
		return nil
	}
	if len(sources) == 0 {
		return deleteStaleSystemRecords(app, "system_repo_sources", systemID, nil)
	}

	params := dbx.Params{
		"system":  systemID,
		"updated": time.Now().UTC().UnixMilli(),
	}
	valueStrings := make([]string, 0, len(sources))
	ids := make([]string, 0, len(sources))

	for i, source := range sources {
		suffix := fmt.Sprintf("%d", i)
		id := makeStableHashId(systemID, source.Manager, source.RepoID, source.URL)
		ids = append(ids, id)
		valueStrings = append(valueStrings, fmt.Sprintf(
			"({:id%[1]s}, {:system}, {:manager%[1]s}, {:repoID%[1]s}, {:name%[1]s}, {:url%[1]s}, {:enabled%[1]s}, {:status%[1]s}, {:error%[1]s}, {:checkedAt%[1]s}, {:updated})",
			suffix,
		))
		params["id"+suffix] = id
		params["manager"+suffix] = source.Manager
		params["repoID"+suffix] = source.RepoID
		params["name"+suffix] = source.Name
		params["url"+suffix] = source.URL
		params["enabled"+suffix] = source.Enabled
		params["status"+suffix] = source.Status
		params["error"+suffix] = source.Error
		params["checkedAt"+suffix] = source.CheckedAt
	}

	if len(valueStrings) == 0 {
		return deleteStaleSystemRecords(app, "system_repo_sources", systemID, nil)
	}

	updateFields := "system = excluded.system, manager = excluded.manager, repo_id = excluded.repo_id, name = excluded.name, url = excluded.url, enabled = excluded.enabled, updated = excluded.updated"
	if !preserveStatus {
		updateFields += ", status = excluded.status, error = excluded.error, checked_at = excluded.checked_at"
	}

	query := fmt.Sprintf(
		"INSERT INTO system_repo_sources (id, system, manager, repo_id, name, url, enabled, status, error, checked_at, updated) VALUES %s ON CONFLICT(id) DO UPDATE SET %s",
		strings.Join(valueStrings, ","),
		updateFields,
	)
	if _, err := app.DB().NewQuery(query).Bind(params).Execute(); err != nil {
		return err
	}
	return deleteStaleSystemRecords(app, "system_repo_sources", systemID, ids)
}

package systems

import (
	"fmt"
	"strings"
	"time"

	"aether/internal/entities/system"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

func syncSystemNetworkMounts(app core.App, systemID string, mounts []*system.NetworkMount) error {
	if mounts == nil {
		return nil
	}
	if len(mounts) == 0 {
		return deleteStaleSystemRecords(app, "system_network_mounts", systemID, nil)
	}

	params := dbx.Params{
		"system":  systemID,
		"updated": time.Now().UTC().UnixMilli(),
	}
	valueStrings := make([]string, 0, len(mounts))
	ids := make([]string, 0, len(mounts))

	for i, mount := range mounts {
		if mount == nil {
			continue
		}
		suffix := fmt.Sprintf("%d", i)
		id := makeStableHashId(systemID, mount.MountPoint, mount.Source, mount.FsType)
		ids = append(ids, id)
		valueStrings = append(valueStrings, fmt.Sprintf(
			"({:id%[1]s}, {:system}, {:source%[1]s}, {:sourceHost%[1]s}, {:sourcePath%[1]s}, {:mountPoint%[1]s}, {:fstype%[1]s}, {:totalBytes%[1]s}, {:usedBytes%[1]s}, {:usedPct%[1]s}, {:error%[1]s}, {:updated})",
			suffix,
		))
		params["id"+suffix] = id
		params["source"+suffix] = mount.Source
		params["sourceHost"+suffix] = mount.SourceHost
		params["sourcePath"+suffix] = mount.SourcePath
		params["mountPoint"+suffix] = mount.MountPoint
		params["fstype"+suffix] = mount.FsType
		params["totalBytes"+suffix] = mount.TotalBytes
		params["usedBytes"+suffix] = mount.UsedBytes
		params["usedPct"+suffix] = mount.UsedPct
		params["error"+suffix] = mount.Error
	}

	if len(valueStrings) == 0 {
		return deleteStaleSystemRecords(app, "system_network_mounts", systemID, nil)
	}

	query := fmt.Sprintf(
		"INSERT INTO system_network_mounts (id, system, source, source_host, source_path, mount_point, fstype, total_bytes, used_bytes, used_pct, error, updated) VALUES %s ON CONFLICT(id) DO UPDATE SET system = excluded.system, source = excluded.source, source_host = excluded.source_host, source_path = excluded.source_path, mount_point = excluded.mount_point, fstype = excluded.fstype, total_bytes = excluded.total_bytes, used_bytes = excluded.used_bytes, used_pct = excluded.used_pct, error = excluded.error, updated = excluded.updated",
		strings.Join(valueStrings, ","),
	)
	if _, err := app.DB().NewQuery(query).Bind(params).Execute(); err != nil {
		return err
	}
	return deleteStaleSystemRecords(app, "system_network_mounts", systemID, ids)
}

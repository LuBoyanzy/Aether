//go:build testing
// +build testing

package alerts_test

import (
	"encoding/json"
	"testing"
	"time"

	"aether/internal/entities/system"
	aetherTests "aether/internal/tests"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiskIOAlertAveragedSamples(t *testing.T) {
	hub, user := aetherTests.GetHubWithUser(t)
	defer hub.Cleanup()

	systems, err := aetherTests.CreateSystems(hub, 1, user.Id, "up")
	require.NoError(t, err)
	systemRecord := systems[0]

	diskAlert, err := aetherTests.CreateRecord(hub, "alerts", map[string]any{
		"name":   "DiskIO",
		"system": systemRecord.Id,
		"user":   user.Id,
		"value":  100,
		"min":    2,
	})
	require.NoError(t, err)
	assert.False(t, diskAlert.GetBool("triggered"), "Alert should not be triggered initially")

	am := hub.GetAlertManager()
	now := time.Now().UTC()

	recordTimes := []time.Duration{
		-180 * time.Second,
		-90 * time.Second,
		-60 * time.Second,
		-30 * time.Second,
	}

	for _, offset := range recordTimes {
		statsHigh := system.Stats{
			Cpu:         10,
			MemPct:      30,
			DiskPct:     40,
			DiskReadPs:  60,
			DiskWritePs: 70,
		}
		statsHighJSON, _ := json.Marshal(statsHigh)
		recordTime := now.Add(offset)
		record, err := aetherTests.CreateRecord(hub, "system_stats", map[string]any{
			"system": systemRecord.Id,
			"type":   "1m",
			"stats":  string(statsHighJSON),
		})
		require.NoError(t, err)
		record.SetRaw("created", recordTime.Format(types.DefaultDateLayout))
		err = hub.SaveNoValidate(record)
		require.NoError(t, err)
	}

	combinedDataHigh := &system.CombinedData{
		Stats: system.Stats{
			Cpu:         10,
			MemPct:      30,
			DiskPct:     40,
			DiskReadPs:  60,
			DiskWritePs: 70,
		},
		Info: system.Info{
			AgentVersion: "0.12.0",
			Cpu:          10,
			MemPct:       30,
			DiskPct:      40,
		},
	}

	systemRecord.Set("updated", now)
	err = hub.SaveNoValidate(systemRecord)
	require.NoError(t, err)

	err = am.HandleSystemAlerts(systemRecord, combinedDataHigh)
	require.NoError(t, err)

	time.Sleep(20 * time.Millisecond)
	diskAlert, err = hub.FindFirstRecordByFilter("alerts", "id={:id}", dbx.Params{"id": diskAlert.Id})
	require.NoError(t, err)
	assert.True(t, diskAlert.GetBool("triggered"), "Alert SHOULD be triggered when average disk I/O exceeds threshold")

	newNow := now.Add(2 * time.Minute)
	for _, offset := range recordTimes {
		statsLow := system.Stats{
			Cpu:         10,
			MemPct:      30,
			DiskPct:     40,
			DiskReadPs:  10,
			DiskWritePs: 10,
		}
		statsLowJSON, _ := json.Marshal(statsLow)
		recordTime := newNow.Add(offset)
		record, err := aetherTests.CreateRecord(hub, "system_stats", map[string]any{
			"system": systemRecord.Id,
			"type":   "1m",
			"stats":  string(statsLowJSON),
		})
		require.NoError(t, err)
		record.SetRaw("created", recordTime.Format(types.DefaultDateLayout))
		err = hub.SaveNoValidate(record)
		require.NoError(t, err)
	}

	combinedDataLow := &system.CombinedData{
		Stats: system.Stats{
			Cpu:         10,
			MemPct:      30,
			DiskPct:     40,
			DiskReadPs:  10,
			DiskWritePs: 10,
		},
		Info: system.Info{
			AgentVersion: "0.12.0",
			Cpu:          10,
			MemPct:       30,
			DiskPct:      40,
		},
	}

	systemRecord.Set("updated", newNow)
	err = hub.SaveNoValidate(systemRecord)
	require.NoError(t, err)

	err = am.HandleSystemAlerts(systemRecord, combinedDataLow)
	require.NoError(t, err)

	time.Sleep(20 * time.Millisecond)
	diskAlert, err = hub.FindFirstRecordByFilter("alerts", "id={:id}", dbx.Params{"id": diskAlert.Id})
	require.NoError(t, err)
	assert.False(t, diskAlert.GetBool("triggered"), "Alert should be resolved when average disk I/O drops to threshold or below")
}

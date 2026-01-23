// 告警系统的系统指标处理与触发逻辑。
// 负责根据系统指标计算阈值并发送告警。
package alerts

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"aether/internal/entities/system"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/types"
	"github.com/spf13/cast"
)

func (am *AlertManager) HandleSystemAlerts(systemRecord *core.Record, data *system.CombinedData) error {
	alertRecords, err := am.hub.FindAllRecords("alerts",
		dbx.NewExp("system={:system} AND name!='Status'", dbx.Params{"system": systemRecord.Id}),
	)
	if err != nil || len(alertRecords) == 0 {
		// log.Println("no alerts found for system")
		return nil
	}

	var validAlerts []SystemAlertData
	now := systemRecord.GetDateTime("updated").Time().UTC()
	oldestTime := now

	for _, alertRecord := range alertRecords {
		name := alertRecord.GetString("name")
		var val float64
		unit := "%"

		switch name {
		case "CPU":
			val = data.Info.Cpu
		case "Memory":
			val = data.Info.MemPct
		case "Bandwidth":
			val = data.Info.Bandwidth
			unit = " MB/s"
		case "Disk":
			maxUsedPct := data.Info.DiskPct
			for _, fs := range data.Stats.ExtraFs {
				usedPct := fs.DiskUsed / fs.DiskTotal * 100
				if usedPct > maxUsedPct {
					maxUsedPct = usedPct
				}
			}
			val = maxUsedPct
		case "DiskIO":
			val = data.Stats.DiskReadPs + data.Stats.DiskWritePs
			unit = " MB/s"
		case "Temperature":
			if data.Info.DashboardTemp < 1 {
				continue
			}
			val = data.Info.DashboardTemp
			unit = "°C"
		case "LoadAvg1":
			val = data.Info.LoadAvg[0]
			unit = ""
		case "LoadAvg5":
			val = data.Info.LoadAvg[1]
			unit = ""
		case "LoadAvg15":
			val = data.Info.LoadAvg[2]
			unit = ""
		case "GPU":
			val = data.Info.GpuPct
		case "Battery":
			if data.Stats.Battery[0] == 0 {
				continue
			}
			val = float64(data.Stats.Battery[0])
		}

		triggered := alertRecord.GetBool("triggered")
		threshold := alertRecord.GetFloat("value")

		// Battery alert has inverted logic: trigger when value is BELOW threshold
		lowAlert := isLowAlert(name)

		// CONTINUE
		// For normal alerts: IF not triggered and curValue <= threshold, OR triggered and curValue > threshold
		// For low alerts (Battery): IF not triggered and curValue >= threshold, OR triggered and curValue < threshold
		if lowAlert {
			if (!triggered && val >= threshold) || (triggered && val < threshold) {
				continue
			}
		} else {
			if (!triggered && val <= threshold) || (triggered && val > threshold) {
				continue
			}
		}

		min := cast.ToUint8(alertRecord.Get("min"))
		if min < 1 {
			min = 1
		}

		alert := SystemAlertData{
			systemRecord: systemRecord,
			alertRecord:  alertRecord,
			name:         name,
			unit:         unit,
			val:          val,
			threshold:    threshold,
			triggered:    triggered,
			min:          min,
		}

		// send alert immediately if min is 1 - no need to sum up values.
		if min == 1 {
			if lowAlert {
				alert.triggered = val < threshold
			} else {
				alert.triggered = val > threshold
			}
			go am.sendSystemAlert(alert)
			continue
		}

		alert.time = now.Add(-time.Duration(min) * time.Minute)
		if alert.time.Before(oldestTime) {
			oldestTime = alert.time
		}

		validAlerts = append(validAlerts, alert)
	}

	systemStats := []struct {
		Stats   []byte         `db:"stats"`
		Created types.DateTime `db:"created"`
	}{}

	err = am.hub.DB().
		Select("stats", "created").
		From("system_stats").
		Where(dbx.NewExp(
			"system={:system} AND type='1m' AND created > {:created}",
			dbx.Params{
				"system": systemRecord.Id,
				// subtract some time to give us a bit of buffer
				"created": oldestTime.Add(-time.Second * 90),
			},
		)).
		OrderBy("created").
		All(&systemStats)
	if err != nil || len(systemStats) == 0 {
		return err
	}

	// get oldest record creation time from first record in the slice
	oldestRecordTime := systemStats[0].Created.Time()
	// log.Println("oldestRecordTime", oldestRecordTime.String())

	// Filter validAlerts to keep only those with time newer than oldestRecord
	filteredAlerts := make([]SystemAlertData, 0, len(validAlerts))
	for _, alert := range validAlerts {
		if alert.time.After(oldestRecordTime) {
			filteredAlerts = append(filteredAlerts, alert)
		}
	}
	validAlerts = filteredAlerts

	if len(validAlerts) == 0 {
		// log.Println("no valid alerts found")
		return nil
	}

	var stats SystemAlertStats

	// we can skip the latest systemStats record since it's the current value
	for i := range systemStats {
		stat := systemStats[i]
		// subtract 10 seconds to give a small time buffer
		systemStatsCreation := stat.Created.Time().Add(-time.Second * 10)
		if err := json.Unmarshal(stat.Stats, &stats); err != nil {
			return err
		}
		// log.Println("stats", stats)
		for j := range validAlerts {
			alert := &validAlerts[j]
			// reset alert val on first iteration
			if i == 0 {
				alert.val = 0
			}
			// continue if system_stats is older than alert time range
			if systemStatsCreation.Before(alert.time) {
				continue
			}
			// add to alert value
			switch alert.name {
			case "CPU":
				alert.val += stats.Cpu
			case "Memory":
				alert.val += stats.Mem
			case "Bandwidth":
				alert.val += stats.NetSent + stats.NetRecv
			case "DiskIO":
				alert.val += stats.DiskReadPs + stats.DiskWritePs
			case "Disk":
				if alert.mapSums == nil {
					alert.mapSums = make(map[string]float32, len(data.Stats.ExtraFs)+1)
				}
				// add root disk
				if _, ok := alert.mapSums["root"]; !ok {
					alert.mapSums["root"] = 0.0
				}
				alert.mapSums["root"] += float32(stats.Disk)
				// add extra disks
				for key, fs := range data.Stats.ExtraFs {
					if _, ok := alert.mapSums[key]; !ok {
						alert.mapSums[key] = 0.0
					}
					alert.mapSums[key] += float32(fs.DiskUsed / fs.DiskTotal * 100)
				}
			case "Temperature":
				if alert.mapSums == nil {
					alert.mapSums = make(map[string]float32, len(stats.Temperatures))
				}
				for key, temp := range stats.Temperatures {
					if _, ok := alert.mapSums[key]; !ok {
						alert.mapSums[key] = float32(0)
					}
					alert.mapSums[key] += temp
				}
			case "LoadAvg1":
				alert.val += stats.LoadAvg[0]
			case "LoadAvg5":
				alert.val += stats.LoadAvg[1]
			case "LoadAvg15":
				alert.val += stats.LoadAvg[2]
			case "GPU":
				if len(stats.GPU) == 0 {
					continue
				}
				maxUsage := 0.0
				for _, gpu := range stats.GPU {
					if gpu.Usage > maxUsage {
						maxUsage = gpu.Usage
					}
				}
				alert.val += maxUsage
			case "Battery":
				alert.val += float64(stats.Battery[0])
			default:
				continue
			}
			alert.count++
		}
	}
	// sum up vals for each alert
	for _, alert := range validAlerts {
		switch alert.name {
		case "Disk":
			maxPct := float32(0)
			for key, value := range alert.mapSums {
				sumPct := float32(value)
				if sumPct > maxPct {
					maxPct = sumPct
					alert.descriptor = fmt.Sprintf("Usage of %s", key)
				}
			}
			alert.val = float64(maxPct / float32(alert.count))
		case "Temperature":
			maxTemp := float32(0)
			for key, value := range alert.mapSums {
				sumTemp := float32(value) / float32(alert.count)
				if sumTemp > maxTemp {
					maxTemp = sumTemp
					alert.descriptor = fmt.Sprintf("Highest sensor %s", key)
				}
			}
			alert.val = float64(maxTemp)
		default:
			alert.val = alert.val / float64(alert.count)
		}
		minCount := float32(alert.min) / 1.2
		// log.Println("alert", alert.name, "val", alert.val, "threshold", alert.threshold, "triggered", alert.triggered)
		// log.Printf("%s: val %f | count %d | min-count %f | threshold %f\n", alert.name, alert.val, alert.count, minCount, alert.threshold)
		// pass through alert if count is greater than or equal to minCount
		if float32(alert.count) >= minCount {
			// Battery alert has inverted logic: trigger when value is BELOW threshold
			lowAlert := isLowAlert(alert.name)
			if lowAlert {
				if !alert.triggered && alert.val < alert.threshold {
					alert.triggered = true
					go am.sendSystemAlert(alert)
				} else if alert.triggered && alert.val >= alert.threshold {
					alert.triggered = false
					go am.sendSystemAlert(alert)
				}
			} else {
				if !alert.triggered && alert.val > alert.threshold {
					alert.triggered = true
					go am.sendSystemAlert(alert)
				} else if alert.triggered && alert.val <= alert.threshold {
					alert.triggered = false
					go am.sendSystemAlert(alert)
				}
			}
		}
	}
	return nil
}

func (am *AlertManager) sendSystemAlert(alert SystemAlertData) {
	// log.Printf("Sending alert %s: val %f | count %d | threshold %f\n", alert.name, alert.val, alert.count, alert.threshold)
	systemName := alert.systemRecord.GetString("name")

	lang, err := am.NotificationLanguage()
	if err != nil {
		am.hub.Logger().Error("读取通知语言失败", "logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack()), "system", systemName, "alertName", alert.name)
		return
	}
	alertType := alert.name
	details := ""
	if displayTitle, displayDetails, ok := AlertDisplayText(alert.name, lang); ok {
		alertType = displayTitle
		details = displayDetails
	} else {
		alertType = formatLegacySystemAlertType(alert.name)
	}
	state := NotificationStateResolved
	if alert.triggered {
		state = NotificationStateTriggered
	}
	content := NotificationContent{
		SystemName:   systemName,
		AlertType:    alertType,
		Descriptor:   alert.descriptor,
		State:        state,
		CurrentValue: FormatMetricValue(alert.val, alert.unit),
		Threshold:    FormatMetricValue(alert.threshold, alert.unit),
		Duration:     FormatDurationMinutes(int(alert.min), lang),
		Details:      details,
	}
	text, err := FormatNotification(lang, content)
	if err != nil {
		am.hub.Logger().Error("告警通知格式化失败", "logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack()), "system", systemName, "alertName", alert.name, "triggered", alert.triggered, "currentValue", alert.val, "threshold", alert.threshold, "durationMinutes", alert.min)
		return
	}

	alert.alertRecord.Set("triggered", alert.triggered)
	if err := am.hub.Save(alert.alertRecord); err != nil {
		am.hub.Logger().Error("保存告警记录失败", "logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack()), "alertId", alert.alertRecord.Id, "system", systemName)
		return
	}
	if err := am.SendAlert(AlertMessageData{
		UserID:   alert.alertRecord.GetString("user"),
		SystemID: alert.systemRecord.Id,
		Title:    text.Title,
		Message:  text.Message,
		Link:     am.hub.MakeLink("system", alert.systemRecord.Id),
		LinkText: text.LinkText,
	}); err != nil {
		am.hub.Logger().Error("发送告警通知失败", "logger", "alerts", "err", err, "errType", fmt.Sprintf("%T", err), "stack", string(debug.Stack()), "system", systemName, "alertName", alert.name)
	}
}

func isLowAlert(name string) bool {
	return name == "Battery"
}

func formatLegacySystemAlertType(alertName string) string {
	alertType := alertName
	if alertType == "Disk" {
		alertType += " usage"
	}
	if after, ok := strings.CutPrefix(alertType, "LoadAvg"); ok {
		alertType = after + "m Load"
	}
	if alertType != "CPU" && alertType != "GPU" {
		alertType = strings.ToLower(alertType)
	}
	return alertType
}

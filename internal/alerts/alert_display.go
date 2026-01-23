// 告警名称与描述映射，保持与前端告警配置弹窗一致。
package alerts

type alertDisplayText struct {
	TitleZh string
	TitleEn string
	DescZh  string
	DescEn  string
}

var alertDisplayTextByName = map[string]alertDisplayText{
	"Status": {
		TitleZh: "状态",
		TitleEn: "Status",
		DescZh:  "当状态在上线与掉线之间切换时触发",
		DescEn:  "Triggers when status switches between up and down",
	},
	"CPU": {
		TitleZh: "CPU 使用率",
		TitleEn: "CPU Usage",
		DescZh:  "当 CPU 使用率超过阈值时触发",
		DescEn:  "Triggers when CPU usage exceeds a threshold",
	},
	"Memory": {
		TitleZh: "内存使用率",
		TitleEn: "Memory Usage",
		DescZh:  "当内存使用率超过阈值时触发",
		DescEn:  "Triggers when memory usage exceeds a threshold",
	},
	"Disk": {
		TitleZh: "磁盘使用",
		TitleEn: "Disk Usage",
		DescZh:  "当任何磁盘的使用率超过阈值时触发",
		DescEn:  "Triggers when usage of any disk exceeds a threshold",
	},
	"DiskIO": {
		TitleZh: "根磁盘 I/O 吞吐量",
		TitleEn: "Root Disk I/O Throughput",
		DescZh:  "当根磁盘 I/O 吞吐量超过阈值时触发",
		DescEn:  "Triggers when root disk I/O throughput exceeds a threshold",
	},
	"Bandwidth": {
		TitleZh: "带宽",
		TitleEn: "Bandwidth",
		DescZh:  "当网络的上/下行速度超过阈值时触发",
		DescEn:  "Triggers when combined up/down exceeds a threshold",
	},
	"GPU": {
		TitleZh: "GPU 使用率",
		TitleEn: "GPU Usage",
		DescZh:  "当 GPU 使用率超过阈值时触发",
		DescEn:  "Triggers when GPU usage exceeds a threshold",
	},
	"Temperature": {
		TitleZh: "温度",
		TitleEn: "Temperature",
		DescZh:  "当任何传感器超过阈值时触发",
		DescEn:  "Triggers when any sensor exceeds a threshold",
	},
	"LoadAvg1": {
		TitleZh: "1 分钟负载平均值",
		TitleEn: "Load Average 1m",
		DescZh:  "当 1 分钟负载平均值超过阈值时触发",
		DescEn:  "Triggers when 1 minute load average exceeds a threshold",
	},
	"LoadAvg5": {
		TitleZh: "5 分钟内的平均负载",
		TitleEn: "Load Average 5m",
		DescZh:  "当 5 分钟内的平均负载超过阈值时触发",
		DescEn:  "Triggers when 5 minute load average exceeds a threshold",
	},
	"LoadAvg15": {
		TitleZh: "15 分钟内的平均负载",
		TitleEn: "Load Average 15m",
		DescZh:  "当 15 分钟负载平均值超过阈值时触发",
		DescEn:  "Triggers when 15 minute load average exceeds a threshold",
	},
	"Battery": {
		TitleZh: "电池",
		TitleEn: "Battery",
		DescZh:  "当电池电量低于阈值时触发",
		DescEn:  "Triggers when battery charge drops below a threshold",
	},
}

func AlertDisplayText(alertName string, lang NotificationLanguage) (string, string, bool) {
	info, ok := alertDisplayTextByName[alertName]
	if !ok {
		return "", "", false
	}
	if lang == NotificationLanguageZhCN {
		return info.TitleZh, info.DescZh, true
	}
	return info.TitleEn, info.DescEn, true
}

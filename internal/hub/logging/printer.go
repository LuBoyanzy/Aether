package logging

import (
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/mattn/go-isatty"
	"github.com/petermattis/goid"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/logger"
	"github.com/pocketbase/pocketbase/tools/types"

	_ "unsafe"
)

const (
	defaultLoggerName = "hub"
	loggerKey         = "logger"
	logTypeKey        = "type"
	logMetaKey        = "meta"
)

var colorEnabled = detectColorEnabled()

var (
	colorCache   = map[string]*color.Color{}
	colorCacheMu sync.Mutex
	levelDebug   = []color.Attribute{color.FgHiBlack}
	levelInfo    = []color.Attribute{color.FgGreen}
	levelWarn    = []color.Attribute{color.FgYellow}
	levelError   = []color.Attribute{color.FgRed}
	timeColor    = []color.Attribute{color.FgCyan}
	loggerColor  = []color.Attribute{color.FgBlue}
	threadColor  = []color.Attribute{color.FgMagenta}
	messageColor = []color.Attribute{color.FgWhite}
)

//go:linkname pocketbasePrintLog github.com/pocketbase/pocketbase/core.printLog
var pocketbasePrintLog func(log *logger.Log)

// Init 在启动时安装日志格式化器，并设置 slog 默认日志器。
func Init(app core.App) {
	if app == nil {
		return
	}

	app.OnBootstrap().BindFunc(func(e *core.BootstrapEvent) error {
		if err := e.Next(); err != nil {
			return err
		}

		installPrinter()
		slog.SetDefault(e.App.Logger().With(loggerKey, defaultLoggerName))
		return nil
	})
}

func installPrinter() {
	pocketbasePrintLog = printLog
}

func printLog(log *logger.Log) {
	if log == nil {
		return
	}

	levelText := formatLevel(log.Level)
	timeText := log.Time.Local().Format("2006-01-02 15:04:05")
	loggerName := resolveLoggerName(log)
	threadText := fmt.Sprintf("Thread-%d", goid.Get())
	message := formatMessage(log)

	var builder strings.Builder
	builder.WriteString("[")
	builder.WriteString(colorize(levelColorAttrs(log.Level), levelText))
	builder.WriteString("] ")
	builder.WriteString(colorize(timeColor, timeText))
	builder.WriteString(" [")
	builder.WriteString(colorize(loggerColor, loggerName))
	builder.WriteString("] [")
	builder.WriteString(colorize(threadColor, threadText))
	builder.WriteString("] ")
	builder.WriteString(colorize(messageColor, message))
	builder.WriteString("\n")

	fmt.Print(builder.String())
}

func detectColorEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd())
}

func colorize(attrs []color.Attribute, text string) string {
	if !colorEnabled {
		return text
	}
	return getColor(attrs...).Sprint(text)
}

func getColor(attrs ...color.Attribute) *color.Color {
	key := fmt.Sprint(attrs)
	colorCacheMu.Lock()
	defer colorCacheMu.Unlock()
	if c, ok := colorCache[key]; ok {
		return c
	}
	c := color.New(attrs...)
	colorCache[key] = c
	return c
}

func levelColorAttrs(level slog.Level) []color.Attribute {
	switch level {
	case slog.LevelDebug:
		return levelDebug
	case slog.LevelInfo:
		return levelInfo
	case slog.LevelWarn:
		return levelWarn
	case slog.LevelError:
		return levelError
	default:
		return levelDebug
	}
}

func formatLevel(level slog.Level) string {
	switch level {
	case slog.LevelDebug:
		return "DEBUG"
	case slog.LevelInfo:
		return "INFO"
	case slog.LevelWarn:
		return "WARN"
	case slog.LevelError:
		return "ERROR"
	default:
		return fmt.Sprintf("LEVEL(%d)", level)
	}
}

func resolveLoggerName(log *logger.Log) string {
	if log == nil || len(log.Data) == 0 {
		return defaultLoggerName
	}
	if value, ok := log.Data[loggerKey]; ok {
		name := formatValue(value)
		if name != "" {
			return name
		}
	}
	if value, ok := log.Data[logTypeKey]; ok {
		name := formatValue(value)
		if name != "" {
			return name
		}
	}
	return defaultLoggerName
}

func formatMessage(log *logger.Log) string {
	if isRequestLog(log) {
		return formatRequestMessage(log)
	}

	message := log.Message
	extra := formatExtraData(log, map[string]struct{}{
		loggerKey:  {},
		logTypeKey: {},
		logMetaKey: {},
	})
	if extra != "" {
		message += " " + extra
	}
	return message
}

func isRequestLog(log *logger.Log) bool {
	if log == nil || len(log.Data) == 0 {
		return false
	}
	value, ok := log.Data[logTypeKey]
	if !ok {
		return false
	}
	return formatValue(value) == "request"
}

func formatRequestMessage(log *logger.Log) string {
	method := formatValue(log.Data["method"])
	requestURI := formatValue(log.Data["url"])
	status := formatInt(log.Data["status"])
	proto := resolveRequestProto(log)
	if method == "" || requestURI == "" {
		return log.Message
	}

	requestLine := log.Message
	if proto != "" {
		requestLine = fmt.Sprintf("%s %s %s", method, requestURI, proto)
	}
	message := fmt.Sprintf("\"%s\" %d %d", requestLine, status, resolveRequestSize(log))

	extra := formatExtraData(log, requestSkipKeys())
	if extra != "" {
		message += " " + extra
	}
	return message
}

func resolveRequestProto(log *logger.Log) string {
	if log == nil {
		return ""
	}
	if value, ok := log.Data["proto"]; ok {
		return formatValue(value)
	}
	meta := resolveMeta(log)
	if meta == nil {
		return ""
	}
	return formatValue(meta["proto"])
}

func resolveRequestSize(log *logger.Log) int64 {
	meta := resolveMeta(log)
	if meta == nil {
		return 0
	}
	return formatInt(meta["size"])
}

func resolveMeta(log *logger.Log) map[string]any {
	if log == nil || len(log.Data) == 0 {
		return nil
	}
	value, ok := log.Data[logMetaKey]
	if !ok || value == nil {
		return nil
	}

	switch meta := value.(type) {
	case map[string]any:
		return meta
	case map[string]string:
		out := make(map[string]any, len(meta))
		for key, val := range meta {
			out[key] = val
		}
		return out
	case types.JSONMap[any]:
		return map[string]any(meta)
	default:
		return nil
	}
}

func requestSkipKeys() map[string]struct{} {
	return map[string]struct{}{
		loggerKey:   {},
		logTypeKey:  {},
		logMetaKey:  {},
		"method":    {},
		"url":       {},
		"status":    {},
		"referer":   {},
		"userAgent": {},
		"auth":      {},
		"authId":    {},
		"userIP":    {},
		"remoteIP":  {},
		"execTime":  {},
	}
}

func formatExtraData(log *logger.Log, skipKeys map[string]struct{}) string {
	if log == nil || len(log.Data) == 0 {
		return ""
	}

	keys := make([]string, 0, len(log.Data))
	for key, value := range log.Data {
		if value == nil {
			continue
		}
		if _, skip := skipKeys[key]; skip {
			continue
		}
		keys = append(keys, key)
	}
	if len(keys) == 0 {
		return ""
	}

	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, formatValue(log.Data[key])))
	}
	return strings.Join(parts, " ")
}

func formatValue(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case error:
		return v.Error()
	case time.Time:
		return v.Format(time.RFC3339)
	case fmt.Stringer:
		return v.String()
	default:
		return fmt.Sprint(v)
	}
}

func formatInt(value any) int64 {
	switch v := value.(type) {
	case int:
		return int64(v)
	case int8:
		return int64(v)
	case int16:
		return int64(v)
	case int32:
		return int64(v)
	case int64:
		return v
	case uint:
		return int64(v)
	case uint8:
		return int64(v)
	case uint16:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return int64(v)
	case float32:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return 0
	}
}

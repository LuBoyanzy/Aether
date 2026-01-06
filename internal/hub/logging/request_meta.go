package logging

import (
	"bufio"
	"io"
	"net"
	"net/http"

	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/hook"
	"github.com/pocketbase/pocketbase/tools/types"
)

const requestMetaMiddlewareID = "aetherRequestMeta"

// RequestMetaMiddleware 统计响应字节数，并写入请求日志的 meta。
func RequestMetaMiddleware() *hook.Handler[*core.RequestEvent] {
	return &hook.Handler[*core.RequestEvent]{
		Id:       requestMetaMiddlewareID,
		Priority: apis.DefaultActivityLoggerMiddlewarePriority + 1,
		Func: func(e *core.RequestEvent) error {
			if e.Response == nil || e.Request == nil {
				return e.Next()
			}

			counter := &responseSizeWriter{ResponseWriter: e.Response}
			e.Response = counter
			err := e.Next()

			meta := mergeLogMeta(e.Get(apis.RequestEventKeyLogMeta))
			meta["proto"] = e.Request.Proto
			meta["size"] = counter.size
			e.Set(apis.RequestEventKeyLogMeta, meta)

			return err
		},
	}
}

func mergeLogMeta(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}

	switch meta := value.(type) {
	case map[string]any:
		return cloneMeta(meta)
	case map[string]string:
		out := make(map[string]any, len(meta))
		for key, val := range meta {
			out[key] = val
		}
		return out
	case types.JSONMap[any]:
		return cloneMeta(map[string]any(meta))
	default:
		return map[string]any{}
	}
}

func cloneMeta(meta map[string]any) map[string]any {
	out := make(map[string]any, len(meta))
	for key, val := range meta {
		out[key] = val
	}
	return out
}

type responseSizeWriter struct {
	http.ResponseWriter
	size int64
}

func (w *responseSizeWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.size += int64(n)
	return n, err
}

func (w *responseSizeWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseSizeWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *responseSizeWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}
	return hijacker.Hijack()
}

func (w *responseSizeWriter) Push(target string, opts *http.PushOptions) error {
	pusher, ok := w.ResponseWriter.(http.Pusher)
	if !ok {
		return http.ErrNotSupported
	}
	return pusher.Push(target, opts)
}

func (w *responseSizeWriter) ReadFrom(reader io.Reader) (int64, error) {
	if rf, ok := w.ResponseWriter.(io.ReaderFrom); ok {
		n, err := rf.ReadFrom(reader)
		w.size += n
		return n, err
	}

	return io.Copy(writerOnly{Writer: w}, reader)
}

type writerOnly struct {
	io.Writer
}

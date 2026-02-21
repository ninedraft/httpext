package httpext

import (
	"bytes"
	"cmp"
	"crypto/rand"
	"log/slog"
	"net/http"
	"net/textproto"
	"runtime/debug"
	"strings"
	"time"
)

type LoggerConfig struct {
	RequestHeaders  []string
	ResponseHeaders []string
}

// LogWithRecover logs request and response data into provided slog instance.
// Additionally it logs response headers from provided allowlist.
// It tries to recover panics from handler and logs the as errors with stack and recover value.
func LogWithRecover(log *slog.Logger, cfg LoggerConfig) Middleware {
	requestHeader := headerSet(cfg.RequestHeaders)
	responseHeader := headerSet(cfg.ResponseHeaders)

	return func(next http.Handler) http.Handler {
		handle := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			reqID := slog.GroupAttrs("request", slog.String("id", requestID(r.Header)))

			response := &ResponseWriterInterceptor{ResponseWriter: w}

			panicked := true
			tick := time.Now()
			defer func() {
				dt := time.Since(tick)

				entry := log.InfoContext
				if response.StatusCode/100 == 5 {
					entry = log.ErrorContext
				}

				if panicked {
					response.StatusCode = cmp.Or(response.StatusCode, http.StatusInternalServerError)
					Error(response, http.StatusInternalServerError)

					entry = log.ErrorContext

					entry(ctx, "!!PANIC!!",
						reqID,
						slog.Any("recover", recover()),
						slog.String("stack", string(debug.Stack())))
				}

				entry(ctx, "http handler",
					reqID,
					slog.GroupAttrs("request",
						slog.String("method", r.Method),
						slog.String("pattern", r.Pattern),
						slog.String("host", r.Host),
						slog.String("uri", r.RequestURI),
						slog.String("from", r.RemoteAddr),
						headerGroup(r.Header, "header", requestHeader),
					),
					slog.GroupAttrs("response",
						slog.Duration("duration", dt),
						slog.Bool("panic", panicked),
						slog.Int("status", response.StatusCode),
						slog.Int64("body_size", response.Written),
						headerGroup(response.Header(), "header", responseHeader),
					))
			}()

			next.ServeHTTP(response, r)

			panicked = false

			// handler don't call .Write or .WriteHeader
			if response.StatusCode == 0 {
				response.StatusCode = 200
			}
		}

		return http.HandlerFunc(handle)
	}
}

func headerSet(items []string) map[string]struct{} {
	set := make(map[string]struct{}, len(items))
	for _, item := range items {
		set[textproto.CanonicalMIMEHeaderKey(item)] = struct{}{}
	}
	return set
}

func headerGroup(header http.Header, group string, set map[string]struct{}) slog.Attr {
	entries := make([]slog.Attr, 0, len(set))
	for key, values := range header {
		key = textproto.CanonicalMIMEHeaderKey(key)
		if _, ok := set[key]; !ok {
			continue
		}
		entries = append(entries, slog.String(key, strings.Join(values, ", ")))
	}

	return slog.GroupAttrs(group, entries...)
}

// ResponseWriterInterceptor allows to spy on http handler
// by catching result status code, response body size, etc.
// It's compatible with http.NewResponseController
type ResponseWriterInterceptor struct {
	http.ResponseWriter

	StatusCode int
	Written    int64
	// true means that http.NewResponseController was used in the handler
	IsUnwrapped bool

	Body *bytes.Buffer
}

func (rw *ResponseWriterInterceptor) Unwrap() http.ResponseWriter {
	rw.IsUnwrapped = true
	return rw.ResponseWriter
}

func (rw *ResponseWriterInterceptor) WriteHeader(status int) {
	rw.ResponseWriter.WriteHeader(status)

	if rw.StatusCode == 0 {
		rw.StatusCode = status
	}
}

func (rw *ResponseWriterInterceptor) Write(p []byte) (int, error) {
	if rw.StatusCode == 0 {
		rw.WriteHeader(200)
	}

	if rw.Body != nil {
		rw.Body.Write(p)
	}

	n, err := rw.ResponseWriter.Write(p)
	rw.Written += int64(n)

	return n, err
}

func requestID(header http.Header) string {
	headerID := header.Get("X-Request-ID")
	if headerID != "" {
		return headerID
	}

	return time.Now().Format(time.RFC3339 + "_" + rand.Text())
}

package httpext

import (
	"cmp"
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"time"
)

// LogWithRecover logs request and response data into provided slog instance.
// Additionally it logs response headers from provided allowlist.
// It tries to recover panics from handler and logs the as errors with stack and recover value.
func LogWithRecover(log *slog.Logger, allowedHeaders ...string) Middleware {
	headerAllowset := setOf(allowedHeaders)

	return func(next http.Handler) http.Handler {
		handle := func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

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
					entry = log.ErrorContext

					entry(ctx, "!!PANIC!!",
						slog.Any("recover", recover()),
						slog.String("stack", string(debug.Stack())))
				}

				entry(ctx, "http handler",
					slog.GroupAttrs("request",
						slog.String("method", r.Method),
						slog.String("pattern", r.Pattern),
						slog.String("host", r.Host),
						slog.String("uri", r.RequestURI),
						slog.String("from", r.RemoteAddr),
					),
					slog.GroupAttrs("response",
						slog.Duration("duration", dt),
						slog.Bool("panic", panicked),
						slog.Int("status", response.StatusCode),
						slog.Int64("body_size", response.Written),
						slog.Any("header", filterHeader(response.Header(), headerAllowset))),
				)
			}()

			next.ServeHTTP(response, r)

			panicked = false
		}

		return http.HandlerFunc(handle)
	}
}

func setOf[E comparable](items []E) map[E]struct{} {
	set := make(map[E]struct{}, len(items))

	for _, item := range items {
		set[item] = struct{}{}
	}

	return set
}

func filterHeader(header http.Header, set map[string]struct{}) http.Header {
	safe := make(http.Header, len(header))

	for key := range set {
		safe[key] = slices.Clone(header[key])
	}

	return safe
}

// ResponseWriterInterceptor allows to spy on http handler
// by catching result status code, response body size, etc.
// It's compatible with http.http.NewResponseController
type ResponseWriterInterceptor struct {
	http.ResponseWriter

	StatusCode int
	Written    int64
	// true means that http.NewResponseController was used in the handler
	IsUnwrapped bool
}

func (rw *ResponseWriterInterceptor) Unwrap() http.ResponseWriter {
	rw.IsUnwrapped = true
	return rw.ResponseWriter
}

func (rw *ResponseWriterInterceptor) WriteHeader(status int) {
	rw.ResponseWriter.WriteHeader(status)

	rw.StatusCode = status
}

func (rw *ResponseWriterInterceptor) Write(p []byte) (int, error) {
	if rw.StatusCode == 0 {
		rw.WriteHeader(200)
	}

	n, err := rw.ResponseWriter.Write(p)
	rw.Written += int64(n)

	return n, err
}

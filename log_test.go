package httpext_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/ninedraft/httpext"
	"github.com/stretchr/testify/require"
)

func TestLog_Ok(t *testing.T) {
	t.Parallel()

	const body = "test body"

	t.Run("write header", func(t *testing.T) {
		t.Parallel()

		log, logBuf := bufLog()

		rw := httptest.NewRecorder()
		rw.Body = &bytes.Buffer{}

		req := httptest.NewRequest("GET", "/test", strings.NewReader(body))
		handle := handler("/test", 200, body, nil)

		With(handle, LogWithRecover(log, LoggerConfig{})).ServeHTTP(rw, req)

		require.Equal(t, body, rw.Body.String(), "response body")
		entries := decodeLogs(t, logBuf.Bytes())
		require.Len(t, entries, 1)
		resp := getMap(t, entries[0], "response")
		requireKeyValue(t, resp, "panic", false, "panic=false is logged")
	})

	t.Run("no write header", func(t *testing.T) {
		t.Parallel()

		log, logBuf := bufLog()

		rw := httptest.NewRecorder()
		rw.Body = &bytes.Buffer{}

		req := httptest.NewRequest("GET", "/test", strings.NewReader(body))
		handle := handler("/test", 0, body, nil)

		With(handle, LogWithRecover(log, LoggerConfig{})).ServeHTTP(rw, req)

		require.Equal(t, body, rw.Body.String(), "response body")
		entries := decodeLogs(t, logBuf.Bytes())
		require.NotEmptyf(t, entries, "logs")
	})

	t.Run("header", func(t *testing.T) {
		t.Parallel()

		log, logBuf := bufLog()

		rw := httptest.NewRecorder()
		rw.Body = &bytes.Buffer{}

		req := httptest.NewRequest("GET", "/test", strings.NewReader(body))
		header := http.Header{
			"X-Test": {"test-value"},
		}
		handle := handler("/test", 200, body, header)

		With(handle, LogWithRecover(log, LoggerConfig{
			ResponseHeaders: []string{"X-Test"},
		})).ServeHTTP(rw, req)

		require.Equal(t, body, rw.Body.String(), "response body")
		entries := decodeLogs(t, logBuf.Bytes())
		require.Len(t, entries, 1)
		resp := getMap(t, entries[0], "response")
		hdr := getMap(t, resp, "header")
		requireKeyValue(t, hdr, "X-Test", "test-value", "logs contains header values")
	})

	t.Run("header filtered", func(t *testing.T) {
		t.Parallel()

		log, logBuf := bufLog()

		rw := httptest.NewRecorder()
		rw.Body = &bytes.Buffer{}

		req := httptest.NewRequest("GET", "/test", strings.NewReader(body))
		header := http.Header{
			"X-Test":   {"test-value"},
			"X-Secret": {"top-secret"},
		}
		handle := handler("/test", 200, body, header)

		With(handle, LogWithRecover(log, LoggerConfig{
			ResponseHeaders: []string{"X-Test"},
		})).ServeHTTP(rw, req)

		entries := decodeLogs(t, logBuf.Bytes())
		require.Len(t, entries, 1)
		resp := getMap(t, entries[0], "response")
		hdr := getMap(t, resp, "header")
		requireKeyValue(t, hdr, "X-Test", "test-value", "allowed header name is logged")
		require.NotContainsf(t, hdr, "X-Secret", "disallowed header is filtered out")
	})

	t.Run("request header", func(t *testing.T) {
		t.Parallel()

		log, logBuf := bufLog()

		rw := httptest.NewRecorder()
		rw.Body = &bytes.Buffer{}

		req := httptest.NewRequest("GET", "/test", strings.NewReader(body))
		req.Header.Set("X-Test", "req-value")
		handle := handler("/test", 200, body, nil)

		With(handle, LogWithRecover(log, LoggerConfig{
			RequestHeaders: []string{"X-Test"},
		})).ServeHTTP(rw, req)

		entries := decodeLogs(t, logBuf.Bytes())
		require.Len(t, entries, 1)
		reqLog := getMap(t, entries[0], "request")
		hdr := getMap(t, reqLog, "header")
		requireKeyValue(t, hdr, "X-Test", "req-value", "logs contains request header value")
	})

	t.Run("request header filtered", func(t *testing.T) {
		t.Parallel()

		log, logBuf := bufLog()

		rw := httptest.NewRecorder()
		rw.Body = &bytes.Buffer{}

		req := httptest.NewRequest("GET", "/test", strings.NewReader(body))
		req.Header.Set("X-Test", "req-value")
		req.Header.Set("X-Secret", "top-secret")
		handle := handler("/test", 200, body, nil)

		With(handle, LogWithRecover(log, LoggerConfig{
			RequestHeaders: []string{"X-Test"},
		})).ServeHTTP(rw, req)

		entries := decodeLogs(t, logBuf.Bytes())
		require.Len(t, entries, 1)
		reqLog := getMap(t, entries[0], "request")
		hdr := getMap(t, reqLog, "header")
		requireKeyValue(t, hdr, "X-Test", "req-value", "allowed header name is logged")
		require.NotContainsf(t, hdr, "X-Secret", "disallowed header is filtered out")
	})

	t.Run("request header allowlist case-insensitive", func(t *testing.T) {
		t.Parallel()

		log, logBuf := bufLog()

		rw := httptest.NewRecorder()
		rw.Body = &bytes.Buffer{}

		req := httptest.NewRequest("GET", "/test", strings.NewReader(body))
		req.Header.Set("X-Test", "req-value")
		handle := handler("/test", 200, body, nil)

		With(handle, LogWithRecover(log, LoggerConfig{
			RequestHeaders: []string{"x-test"},
		})).ServeHTTP(rw, req)

		entries := decodeLogs(t, logBuf.Bytes())
		require.Len(t, entries, 1)
		reqLog := getMap(t, entries[0], "request")
		hdr := getMap(t, reqLog, "header")
		requireKeyValue(t, hdr, "X-Test", "req-value", "logs contains request header value")
	})

	t.Run("response header allowlist case-insensitive", func(t *testing.T) {
		t.Parallel()

		log, logBuf := bufLog()

		rw := httptest.NewRecorder()
		rw.Body = &bytes.Buffer{}

		req := httptest.NewRequest("GET", "/test", strings.NewReader(body))
		header := http.Header{
			"X-Test": {"resp-value"},
		}
		handle := handler("/test", 200, body, header)

		With(handle, LogWithRecover(log, LoggerConfig{
			ResponseHeaders: []string{"x-test"},
		})).ServeHTTP(rw, req)

		entries := decodeLogs(t, logBuf.Bytes())
		require.Len(t, entries, 1)
		resp := getMap(t, entries[0], "response")
		hdr := getMap(t, resp, "header")
		requireKeyValue(t, hdr, "X-Test", "resp-value", "logs contains response header value")
	})

	t.Run("defaults status when handler silent", func(t *testing.T) {
		t.Parallel()

		log, logBuf := bufLog()

		rw := httptest.NewRecorder()
		rw.Body = &bytes.Buffer{}

		req := httptest.NewRequest(http.MethodGet, "/test", strings.NewReader(body))
		handle := handler("/test", 0, "", nil)

		With(handle, LogWithRecover(log, LoggerConfig{})).ServeHTTP(rw, req)

		entries := decodeLogs(t, logBuf.Bytes())
		require.Len(t, entries, 1)
		resp := getMap(t, entries[0], "response")
		requireKeyValue(t, resp, "status", http.StatusOK, "default success status is logged")
	})
}

func TestLog_panic(t *testing.T) {
	t.Parallel()

	const body = "test body"

	log, logBuf := bufLog()

	rw := httptest.NewRecorder()
	rw.Body = &bytes.Buffer{}

	req := httptest.NewRequest("GET", "/test", strings.NewReader(body))
	var handle http.HandlerFunc = func(w http.ResponseWriter, r *http.Request) {
		panic("test")
	}

	require.NotPanicsf(t, func() {
		With(handle, LogWithRecover(log, LoggerConfig{})).ServeHTTP(rw, req)
	}, "handler")

	entries := decodeLogs(t, logBuf.Bytes())
	require.GreaterOrEqual(t, len(entries), 1)
	resp := getMap(t, entries[len(entries)-1], "response")
	requireKeyValue(t, resp, "panic", true, "logs panic flag")
}

func TestLog_ServerError(t *testing.T) {
	t.Parallel()

	const body = "test body"

	log, logBuf := bufLog()

	rw := httptest.NewRecorder()
	rw.Body = &bytes.Buffer{}

	req := httptest.NewRequest("GET", "/test", strings.NewReader(body))
	handle := handler("/test", http.StatusServiceUnavailable, body, nil)

	With(handle, LogWithRecover(log, LoggerConfig{})).ServeHTTP(rw, req)

	entries := decodeLogs(t, logBuf.Bytes())
	require.Len(t, entries, 1)
	requireKeyValue(t, entries[0], "level", "ERROR", "server errors are logged on error level")
	resp := getMap(t, entries[0], "response")
	requireKeyValue(t, resp, "status", http.StatusServiceUnavailable, "response status is logged")
	requireKeyValue(t, resp, "panic", false, "panic flag is false for server errors")
}

func TestResponseWriterInterceptor(t *testing.T) {
	t.Parallel()

	t.Run("write sets defaults", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := &ResponseWriterInterceptor{ResponseWriter: rec}

		n, err := rw.Write([]byte("payload"))
		require.NoError(t, err)

		require.Equal(t, http.StatusOK, rw.StatusCode, "status code defaults to 200 on write")
		require.Equal(t, http.StatusOK, rec.Code, "wrapped writer receives default status")
		require.Equal(t, int64(n), rw.Written, "written bytes are tracked")
	})

	t.Run("write header propagates status", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := &ResponseWriterInterceptor{ResponseWriter: rec}

		rw.WriteHeader(http.StatusCreated)
		require.Equal(t, http.StatusCreated, rw.StatusCode, "status code is captured")
		require.Equal(t, http.StatusCreated, rec.Code, "status code propagated to wrapped writer")
	})

	t.Run("unwrap marks flag", func(t *testing.T) {
		t.Parallel()

		rec := httptest.NewRecorder()
		rw := &ResponseWriterInterceptor{ResponseWriter: rec}

		require.Same(t, rec, rw.Unwrap(), "unwrap returns underlying writer")
		require.True(t, rw.IsUnwrapped, "unwrap toggles flag")
	})

	t.Run("new response controller unwraps original writer", func(t *testing.T) {
		t.Parallel()

		spy := &responseControllerSpy{ResponseWriter: httptest.NewRecorder()}
		rw := &ResponseWriterInterceptor{ResponseWriter: spy}

		controller := http.NewResponseController(rw)
		require.NotNil(t, controller, "response controller is constructed")

		require.NoError(t, controller.Flush(), "flush is delegated to the original writer")
		require.True(t, spy.flushCalled, "flush hits the original writer")

		require.NoError(t, controller.EnableFullDuplex(), "full duplex is delegated to the original writer")
		require.True(t, spy.enableFullDuplexCalled, "full duplex hits the original writer")

		require.True(t, rw.IsUnwrapped, "creating controller unwraps the interceptor per http doc")
	})
}

func bufLog() (*slog.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}

	handler := slog.NewJSONHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	return slog.New(handler), buf
}

func decodeLogs(t testing.TB, data []byte) []map[string]any {
	t.Helper()

	dec := json.NewDecoder(bytes.NewReader(data))
	var entries []map[string]any
	for dec.More() {
		var entry map[string]any
		require.NoError(t, dec.Decode(&entry))
		entries = append(entries, entry)
	}
	return entries
}

func getMap(t testing.TB, m map[string]any, key string) map[string]any {
	t.Helper()

	require.IsType(t, map[string]any{}, m[key], "expected map for %s", key)
	v, _ := m[key].(map[string]any)
	return v
}

func requireKeyValue(t testing.TB, m map[string]any, key string, value any, msg string, args ...any) {
	t.Helper()

	require.Containsf(t, m, key, msg, args...)
	require.EqualValuesf(t, value, m[key], msg, args...)
}

func handler(pattern string, status int, body string, header http.Header) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc(pattern,
		func(w http.ResponseWriter, r *http.Request) {
			maps.Copy(w.Header(), header)
			if status != 0 {
				w.WriteHeader(status)
			}

			io.WriteString(w, body)
		})

	return mux
}

type responseControllerSpy struct {
	http.ResponseWriter

	flushCalled            bool
	enableFullDuplexCalled bool
}

func (spy *responseControllerSpy) Flush() {
	spy.flushCalled = true

	if flusher, ok := spy.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (spy *responseControllerSpy) EnableFullDuplex() error {
	spy.enableFullDuplexCalled = true
	return nil
}

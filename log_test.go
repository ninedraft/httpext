package httpext_test

import (
	"bytes"
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

		With(handle, LogWithRecover(log)).ServeHTTP(rw, req)

		require.Equal(t, body, rw.Body.String(), "response body")
		require.Containsf(t, logBuf.String(), "panic=false", "logs")
	})

	t.Run("no write header", func(t *testing.T) {
		t.Parallel()

		log, logBuf := bufLog()

		rw := httptest.NewRecorder()
		rw.Body = &bytes.Buffer{}

		req := httptest.NewRequest("GET", "/test", strings.NewReader(body))
		handle := handler("/test", 0, body, nil)

		With(handle, LogWithRecover(log)).ServeHTTP(rw, req)

		require.Equal(t, body, rw.Body.String(), "response body")
		require.NotEmptyf(t, logBuf.String(), "logs")
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

		With(handle, LogWithRecover(log, "X-Test")).ServeHTTP(rw, req)

		require.Equal(t, body, rw.Body.String(), "response body")
		require.Containsf(t, logBuf.String(), "X-Test", "logs contains header values")
		require.Containsf(t, logBuf.String(), "test-value", "logs contains header values")
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

		With(handle, LogWithRecover(log, "X-Test")).ServeHTTP(rw, req)

		require.Containsf(t, logBuf.String(), "X-Test", "allowed header name is logged")
		require.NotContainsf(t, logBuf.String(), "X-Secret", "disallowed header is filtered out")
		require.NotContainsf(t, logBuf.String(), "top-secret", "disallowed header values are filtered out")
	})

	t.Run("defaults status when handler silent", func(t *testing.T) {
		t.Parallel()

		log, logBuf := bufLog()

		rw := httptest.NewRecorder()
		rw.Body = &bytes.Buffer{}

		req := httptest.NewRequest(http.MethodGet, "/test", strings.NewReader(body))
		handle := handler("/test", 0, "", nil)

		With(handle, LogWithRecover(log)).ServeHTTP(rw, req)

		logs := logBuf.String()
		require.Contains(t, logs, "status=200", "default success status is logged")
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
		With(handle, LogWithRecover(log)).ServeHTTP(rw, req)
	}, "handler")

	t.Log(logBuf)
	require.Containsf(t, logBuf.String(), "panic=true", "logs")
}

func TestLog_ServerError(t *testing.T) {
	t.Parallel()

	const body = "test body"

	log, logBuf := bufLog()

	rw := httptest.NewRecorder()
	rw.Body = &bytes.Buffer{}

	req := httptest.NewRequest("GET", "/test", strings.NewReader(body))
	handle := handler("/test", http.StatusServiceUnavailable, body, nil)

	With(handle, LogWithRecover(log)).ServeHTTP(rw, req)

	logs := logBuf.String()
	require.Contains(t, logs, "level=ERROR", "server errors are logged on error level")
	require.Contains(t, logs, "status=503", "response status is logged")
	require.Contains(t, logs, "panic=false", "panic flag is false for server errors")
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

	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	return slog.New(handler), buf
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

package configure

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"
)

type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *recordingHandler) Handle(_ context.Context, record slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.records = append(h.records, record.Clone())
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *recordingHandler) WithGroup(string) slog.Handler {
	return h
}

func TestServerBaseContext(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "value")

	srv := Server(ctx, nil, "localhost:0")
	got := srv.BaseContext(nil)

	if got.Value(key{}) != "value" {
		t.Fatalf("BaseContext returned %v, want %q", got.Value(key{}), "value")
	}
	if got != ctx {
		t.Fatalf("expected BaseContext to reuse provided context")
	}
	if want := 100 * time.Millisecond; srv.ReadHeaderTimeout != want {
		t.Fatalf("ReadHeaderTimeout = %v, want %v", srv.ReadHeaderTimeout, want)
	}
}

func TestServerErrorLogBridgesToHandler(t *testing.T) {
	handler := &recordingHandler{}

	srv := Server(context.Background(), handler, "localhost:0")
	if srv.ErrorLog == nil {
		t.Fatal("expected ErrorLog to be set")
	}

	srv.ErrorLog.Print("oops")

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.records) == 0 {
		t.Fatal("handler did not receive any log records")
	}
	rec := handler.records[0]
	if rec.Level != slog.LevelError {
		t.Fatalf("level = %v, want %v", rec.Level, slog.LevelError)
	}
	if rec.Message != "oops" {
		t.Fatalf("message = %q, want %q", rec.Message, "oops")
	}
}

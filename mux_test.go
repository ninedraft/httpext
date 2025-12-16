package httpext_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	. "github.com/ninedraft/httpext"
	"github.com/stretchr/testify/require"
)

func TestWith_Order(t *testing.T) {
	var events []string

	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		events = append(events, "handler")
	})

	wrapped := With(handler,
		recordingMiddleware(&events, "first"),
		recordingMiddleware(&events, "second"),
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	wrapped.ServeHTTP(rec, req)

	require.Equal(t, []string{
		"before first",
		"before second",
		"handler",
		"after second",
		"after first",
	}, events)
}

func TestMux_HandleFunc(t *testing.T) {
	const pattern = "/func"

	mux := NewMux()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, pattern, nil)

	var middlewareCalled bool
	mux.HandleFunc(pattern, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}, flagMiddleware(&middlewareCalled))

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code, "handler response")
	require.True(t, middlewareCalled, "middleware is invoked")
	require.Len(t, mux.Routes(), 1, "single route registered")
	require.Equal(t, pattern, mux.Routes()[0].Pattern, "pattern stored")

	routeRec := httptest.NewRecorder()
	mux.Routes()[0].Handler.ServeHTTP(routeRec, req)
	require.Equal(t, http.StatusAccepted, routeRec.Code, "route handler contains middleware chain")
}

func TestMux_Handle(t *testing.T) {
	const pattern = "/handler"

	mux := NewMux()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, pattern, nil)

	var middlewareCalled bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	mux.Handle(pattern, handler, flagMiddleware(&middlewareCalled))

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, "handler response")
	require.True(t, middlewareCalled, "middleware is invoked")
	require.Len(t, mux.Routes(), 1, "single route registered")
	require.Equal(t, pattern, mux.Routes()[0].Pattern, "pattern stored")
}

func recordingMiddleware(events *[]string, label string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if events != nil {
				*events = append(*events, "before "+label)
			}
			next.ServeHTTP(w, r)
			if events != nil {
				*events = append(*events, "after "+label)
			}
		})
	}
}

func flagMiddleware(flag *bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			*flag = true
			next.ServeHTTP(w, r)
		})
	}
}

func TestMux_GroupSplicesPatterns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		prefix  string
		pattern string
		want    string
	}{
		{
			name:    "plain path",
			prefix:  "api",
			pattern: "/a/b",
			want:    "/api/a/b",
		},
		{
			name:    "prefix with leading slash",
			prefix:  "/api",
			pattern: "/a/b",
			want:    "/api/a/b",
		},
		{
			name:    "prefix with trailing slash",
			prefix:  "api/",
			pattern: "/a/b",
			want:    "/api/a/b",
		},
		{
			name:    "host specific pattern",
			prefix:  "v1",
			pattern: "example.com/a/b",
			want:    "example.com/v1/a/b",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mux := NewMux()
			group := mux.Group(tt.prefix)

			group.HandleFunc(tt.pattern, func(w http.ResponseWriter, r *http.Request) {})

			require.Len(t, mux.Routes(), 1)
			require.Equal(t, tt.want, mux.Routes()[0].Pattern)
		})
	}

	t.Run("nested group", func(t *testing.T) {
		mux := NewMux()
		group := mux.Group("api").Group("v1")

		group.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {})

		require.Len(t, mux.Routes(), 1)
		require.Equal(t, "/api/v1/users", mux.Routes()[0].Pattern)
	})
}

func TestMux_GroupMiddlewares(t *testing.T) {
	var events []string

	mux := NewMux(recordingMiddleware(&events, "root"))
	group := mux.Group("api", recordingMiddleware(&events, "group"))
	group.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		events = append(events, "handler")
		w.WriteHeader(http.StatusTeapot)
	}, recordingMiddleware(&events, "route"))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)

	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusTeapot, rec.Code)
	require.Equal(t, []string{
		"before root",
		"before group",
		"before route",
		"handler",
		"after route",
		"after group",
		"after root",
	}, events)
}

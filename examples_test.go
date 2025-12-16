package httpext_test

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/ninedraft/httpext"
)

func ExampleMiddlewares() {
	mux := httpext.NewMux()

	base := httpext.Middlewares(
		httpext.MaxBytes(1024*1024),
		httpext.LogWithRecover(slog.Default()),
	)

	mux.HandleFunc("/v1/hello", hello, base)
	mux.HandleFunc("/v2/hello", hello,
		base, httpext.Timeout(time.Second, "too complex query"))

	// Output:
}

func ExampleResponseWriterInterceptor() {
	mw := func(next http.Handler) http.Handler {
		handle := func(w http.ResponseWriter, r *http.Request) {
			rw := &httpext.ResponseWriterInterceptor{ResponseWriter: w}

			next.ServeHTTP(rw, r)
			fmt.Println(rw.StatusCode)
		}

		return http.HandlerFunc(handle)
	}

	mw(http.HandlerFunc(hello)).ServeHTTP(
		httptest.NewRecorder(),
		httptest.NewRequest("GET", "/hello", nil))

	// Output: 200
}

func hello(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello"))
}

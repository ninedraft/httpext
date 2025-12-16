package httpext_test

import (
	"log/slog"
	"net/http"
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

func hello(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hello"))
}

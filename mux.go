package httpext

import (
	"net/http"
	"slices"
	"time"
)

type Middleware = func(next http.Handler) http.Handler

func With(handler http.Handler, middlewares ...Middleware) http.Handler {
	for _, wrap := range slices.Backward(middlewares) {
		handler = wrap(handler)
	}

	return handler
}

type Mux struct {
	*http.ServeMux

	Routes []MuxRoute
}

type MuxRoute struct {
	Pattern string
	Handler http.Handler
}

func NewMux(middlewares ...Middleware) *Mux {
	return &Mux{
		ServeMux: http.NewServeMux(),
	}
}

func (mux *Mux) HandlFunc(pattern string, handler http.HandlerFunc, middlewares ...Middleware) {
	handler = With(handler, middlewares...).ServeHTTP

	mux.ServeMux.HandleFunc(pattern, handler)
	mux.Routes = append(mux.Routes, MuxRoute{
		Pattern: pattern,
		Handler: handler,
	})
}

func (mux *Mux) Handle(pattern string, handler http.Handler, middlewares ...Middleware) {
	handler = With(handler, middlewares...)

	mux.ServeMux.Handle(pattern, handler)
	mux.Routes = append(mux.Routes, MuxRoute{
		Pattern: pattern,
		Handler: handler,
	})
}

func Timeout(timeout time.Duration, message string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, message)
	}
}

func MaxBytes(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.MaxBytesHandler(next, maxBytes)
		})
	}
}

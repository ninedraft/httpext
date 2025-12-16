package httpext

import (
	"net/http"
	"slices"
	"time"
)

func Middlewares(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		return With(next, middlewares...)
	}
}

func With(handler http.Handler, middlewares ...Middleware) http.Handler {
	return with(handler, middlewares)
}

func with(handler http.Handler, middlewares ...[]Middleware) http.Handler {
	for _, stack := range slices.Backward(middlewares) {
		for _, wrap := range slices.Backward(stack) {
			if wrap == nil {
				continue
			}
			handler = wrap(handler)
		}
	}

	return handler
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

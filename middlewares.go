package httpext

import (
	"net/http"
	"slices"
	"time"
)

// Middlewares joines multiple middlewares into one.
// Middlewares applied in reverse, so first one being the most outward.
// Nil middlewares are ignored.
func Middlewares(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		return With(next, middlewares...)
	}
}

// With wraps handler with provided middlewares.
// Middlewares applied in reverse, so first one being the most outward.
// Nil middlewares are ignored.
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

// Timeout is just an adapter for net/http.TimeoutHandler.
//
//	httpext.Timeout(timeout, msg)(handler) == http.TimeoutHandler(handler, timeout, msg)
func Timeout(timeout time.Duration, message string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.TimeoutHandler(next, timeout, message)
	}
}

// MaxBytes is just an adapater for net/http.MaxBytesHandler
//
//	httpext.MaxBytesHandler(maxBytes)(handler) == http.MaxBytesHandler(handler, maxBytes)
func MaxBytes(maxBytes int64) Middleware {
	return func(next http.Handler) http.Handler {
		return http.MaxBytesHandler(next, maxBytes)
	}
}

package httpext

import (
	"net/http"
	"path"
	"slices"
	"strings"
	"time"
)

type Middleware = func(next http.Handler) http.Handler

func With(handler http.Handler, middlewares ...Middleware) http.Handler {
	return with(handler, middlewares)
}

func with(handler http.Handler, middlewares ...[]Middleware) http.Handler {
	for _, stack := range slices.Backward(middlewares) {
		for _, wrap := range slices.Backward(stack) {
			handler = wrap(handler)
		}
	}

	return handler
}

type Mux struct {
	group string

	*routes
	*http.ServeMux
	Middlewares []Middleware
}

// using embedded pointer to enable mutable sharing of Routes between parent and child mux groups.
type routes struct {
	Routes []MuxRoute
}

type MuxRoute struct {
	Pattern string
	Handler http.Handler
}

func NewMux(middlewares ...Middleware) *Mux {
	return &Mux{
		routes:      &routes{},
		ServeMux:    http.NewServeMux(),
		Middlewares: middlewares,
	}
}

func (mux *Mux) HandleFunc(pattern string, handler http.HandlerFunc, middlewares ...Middleware) {
	pattern = mux.spliceGroup(pattern)

	handler = with(handler, mux.Middlewares, middlewares).ServeHTTP

	mux.ServeMux.HandleFunc(pattern, handler)
	mux.Routes = append(mux.Routes, MuxRoute{
		Pattern: pattern,
		Handler: handler,
	})
}

func (mux *Mux) Handle(pattern string, handler http.Handler, middlewares ...Middleware) {
	pattern = mux.spliceGroup(pattern)

	handler = with(handler, mux.Middlewares, middlewares)

	mux.ServeMux.Handle(pattern, handler)
	mux.Routes = append(mux.Routes, MuxRoute{
		Pattern: pattern,
		Handler: handler,
	})
}

// Group creates a prefixed subrouter with additional middlewares.
// Prefix is inserted into start section of paths.
//
// Prefixes "prefix", "prefix/", "/prefix" and "/prefix/" are all valid and equal and
// will be converted into "/prefix/" form.
//
// It's valid to use "{pattern}" as a prefix.
//
// Examples:
//
//	/a/b                 -> /prefix/a/b
//	POST /a/b            -> POST /prefix/a/b
//	POST example.com/a/b -> POST example.com/prefix/a/b
func (mux *Mux) Group(prefix string, middlewares ...Middleware) *Mux {
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}

	return &Mux{
		group:       path.Join(mux.group, prefix),
		ServeMux:    mux.ServeMux,
		Middlewares: slices.Concat(mux.Middlewares, middlewares),
		routes:      mux.routes,
	}
}

func (mux *Mux) spliceGroup(pattern string) string {
	head, path, ok := strings.Cut(pattern, "/")
	if !ok {
		// something wrong, let net/http handle this
		return pattern
	}

	return head + mux.group + path
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

func Middlewares(middlewares ...Middleware) Middleware {
	return func(next http.Handler) http.Handler {
		return With(next, middlewares...)
	}
}

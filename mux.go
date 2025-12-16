package httpext

import (
	"cmp"
	"net/http"
	"path"
	"slices"
	"strings"
)

type Middleware = func(next http.Handler) http.Handler

type Mux struct {
	group string

	*routes
	*http.ServeMux
	Middlewares []Middleware
}

// using embedded pointer to enable mutable sharing of Routes between parent and child mux groups.
type routes struct {
	routes []MuxRoute
}

// Routes returnes list of registered mux routes.
// It's shared between all parent and child Mux groups.
// Handler in each returned route is wrapped with base Mux and per-handle and per-group middlewares.
// Result can be used for documentation and debug purposes.
// Also you can feed routes into other httpext.Mux or http.ServeMux to construct an equal router.
//
//	other := http.NewServeMux()
//	for _, route := range mux.Routes() {
//		other.Handle(route.Pattern, route.Handler)
//	}
func (r *routes) Routes() []MuxRoute {
	return r.routes
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
	mux.routes.routes = append(mux.routes.routes, MuxRoute{
		Pattern: pattern,
		Handler: handler,
	})
}

func (mux *Mux) Handle(pattern string, handler http.Handler, middlewares ...Middleware) {
	pattern = mux.spliceGroup(pattern)

	handler = with(handler, mux.Middlewares, middlewares)

	mux.ServeMux.Handle(pattern, handler)
	mux.routes.routes = append(mux.routes.routes, MuxRoute{
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
//	/a/b                 -> /prefix/a/b
//	POST /a/b            -> POST /prefix/a/b
//	POST example.com/a/b -> POST example.com/prefix/a/b
func (mux *Mux) Group(prefix string, middlewares ...Middleware) *Mux {
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}

	prefix = path.Join(mux.group, prefix) + "/"
	return &Mux{
		group:       prefix,
		ServeMux:    mux.ServeMux,
		Middlewares: slices.Concat(mux.Middlewares, middlewares),
		routes:      mux.routes,
	}
}

func (mux *Mux) spliceGroup(pattern string) string {
	head, path, ok := strings.Cut(pattern, "/")
	if !ok {
		// something wrong, let's net/http handle thi
		return pattern
	}

	group := cmp.Or(mux.group, "/")

	return head + group + path
}

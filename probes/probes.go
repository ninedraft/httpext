package probes

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ninedraft/httpext"
)

type probe struct {
	name       string
	mu         sync.RWMutex
	components map[string]*atomic.Bool
}

func Health() *probe {
	return &probe{
		name:       "health",
		components: map[string]*atomic.Bool{},
	}
}

func Readiness() *probe {
	return &probe{
		name:       "readiness",
		components: map[string]*atomic.Bool{},
	}
}

func (probe *probe) Component(name string) func(ok bool) {
	probe.mu.Lock()
	defer probe.mu.Unlock()

	if _, exists := probe.components[name]; exists {
		panic("[httpext/probes] .Component: duplicated component name on probe")
	}

	ok := &atomic.Bool{}
	probe.components[name] = ok

	return ok.Store
}

func (probe *probe) String() string {
	str := &strings.Builder{}

	probe.snapshot().formatString(str)

	return str.String()
}

func (probe *probe) snapshot() *probeSnapshot {
	probe.mu.RLock()
	defer probe.mu.RUnlock()

	snapshot := &probeSnapshot{
		Name:       probe.name,
		Ok:         true,
		Components: make(map[string]bool, len(probe.components)),
	}

	for name, component := range probe.components {
		ok := component.Load()
		snapshot.Components[name] = ok
		if !ok {
			snapshot.Ok = false
		}
	}

	return snapshot
}

type probeSnapshot struct {
	Name       string          `json:"probe"`
	Ok         bool            `json:"ok"`
	Components map[string]bool `json:"components"`
}

func (snap *probeSnapshot) formatString(dst io.Writer) {
	components := slices.Sorted(maps.Keys(snap.Components))

	fmt.Fprintf(dst, "%s, ok=%v\n", snap.Name, snap.Ok)

	for _, component := range components {
		fmt.Fprintf(dst, "\t- %s ok=%v\n", component, snap.Components[component])
	}
}

func (snap *probeSnapshot) formatJSON(dst io.Writer) {
	_ = json.NewEncoder(dst).Encode(snap)
}

func (probe *probe) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	snap := probe.snapshot()

	acceptsJSON := strings.Contains(
		strings.ToLower(req.Header.Get("Accept")),
		"application/json")

	status := http.StatusOK
	if !snap.Ok {
		status = http.StatusServiceUnavailable
	}

	format, contentType := snap.formatString, "text/plain; charset=utf-8"

	if acceptsJSON {
		format = snap.formatJSON
		contentType = "application/json; charset=utf-8"
	}

	rw.Header().Set("Content-Type", contentType)
	rw.WriteHeader(status)
	format(rw)
}

func Server(addr string) (*http.Server, *httpext.Mux) {
	const timeout = time.Second

	if addr == "" {
		addr = ":9090"
	}

	mux := httpext.NewMux(
		httpext.MaxBytes(10*1024),
		httpext.Timeout(timeout, "probing timeout"),
	)

	return &http.Server{
		Addr:              addr,
		MaxHeaderBytes:    1024,
		ReadHeaderTimeout: timeout,
		ReadTimeout:       timeout,
		WriteTimeout:      timeout,
		Handler:           mux,
	}, mux
}

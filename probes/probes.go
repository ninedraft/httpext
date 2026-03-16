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

// Probe is a simple collection of (name, bool) pairs.
// It serves 200 Ok if all bools are true, else false.
// It has name, which is accessible via http response.
type Probe struct {
	name       string
	mu         sync.RWMutex
	components map[string]*atomic.Bool
}

// Health constructs a probe named "health".
func Health() *Probe {
	return &Probe{
		name:       "health",
		components: map[string]*atomic.Bool{},
	}
}

// Health constructs a probe named "ready".
func Readiness() *Probe {
	return &Probe{
		name:       "ready",
		components: map[string]*atomic.Bool{},
	}
}

// Component returns a setter function, which 
func (probe *Probe) Component(name string) func(ok bool) {
	probe.mu.Lock()
	defer probe.mu.Unlock()

	if _, exists := probe.components[name]; exists {
		panic("[httpext/probes] .Component: duplicated component name on probe")
	}

	ok := &atomic.Bool{}
	probe.components[name] = ok

	return ok.Store
}

func (probe *Probe) String() string {
	str := &strings.Builder{}

	probe.snapshot().formatString(str)

	return str.String()
}

func (probe *Probe) snapshot() *probeSnapshot {
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

func (probe *Probe) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet, http.MethodHead:
	default:
		rw.Header().Set("Allow", "GET, HEAD")
		httpext.Error(rw, http.StatusMethodNotAllowed)
		return
	}

	snap := probe.snapshot()

	acceptsJSON := strings.Contains(
		strings.ToLower(req.Header.Get("Accept")),
		"application/json",
	)

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

	if req.Method == http.MethodHead {
		return
	}

	format(rw)
}

type Server struct {
	Health    *Probe
	Readiness *Probe
	*http.Server
}

// Server configures an simple probe server with good defaults.
// Returned server is unstarted.
func New(addr string) *Server {
	const timeout = time.Second

	if addr == "" {
		addr = ":9090"
	}

	mux := httpext.NewMux(
		httpext.MaxBytes(10*1024),
		httpext.Timeout(timeout, "probing timeout"),
	)

	health := Health()
	readiness := Readiness()

	mux.Handle("GET,HEAD /probes/health", health)
	mux.Handle("GET,HEAD /probes/ready", readiness)

	return &Server{
		Health:    health,
		Readiness: readiness,
		Server: &http.Server{
			Addr:              addr,
			MaxHeaderBytes:    1024,
			ReadHeaderTimeout: timeout,
			ReadTimeout:       timeout,
			WriteTimeout:      timeout,
			Handler:           mux,
		},
	}
}

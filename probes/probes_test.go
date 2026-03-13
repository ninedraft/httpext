package probes_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ninedraft/httpext/probes"
)

func TestHealth(t *testing.T) {
	testProbe(t, func() (http.Handler, newComponent) {
		probe := probes.Health()

		return probe, probe.Component
	})
}

func TestReadiness(t *testing.T) {
	testProbe(t, func() (http.Handler, newComponent) {
		probe := probes.Readiness()

		return probe, probe.Component
	})
}

type newComponent = func(name string) func(bool)

func testProbe(t *testing.T, newProbe func() (http.Handler, newComponent)) {
	t.Helper()

	t.Run("no components", func(t *testing.T) {
		t.Parallel()

		probe, _ := newProbe()

		assertProbeStatus(t, probe, http.StatusOK)
	})

	t.Run("ok ok", func(t *testing.T) {
		t.Parallel()

		probe, component := newProbe()

		component("a")(true)
		component("b")(true)

		assertProbeStatus(t, probe, http.StatusOK)
	})

	t.Run("ok failed", func(t *testing.T) {
		t.Parallel()

		probe, component := newProbe()

		component("a")(true)

		healthB := component("b")
		healthB(false)

		assertProbeStatus(t, probe, http.StatusServiceUnavailable)

		healthB(true)
		assertProbeStatus(t, probe, http.StatusOK)
	})
}

func assertProbeStatus(t *testing.T, probe http.Handler, status int) {
	t.Helper()

	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "https://example.com", nil)

	probe.ServeHTTP(rw, req)

	result := rw.Result()
	if result.StatusCode != status {
		t.Errorf("want status %d", status)
		t.Errorf(" got status %d %s", result.StatusCode, result.Status)
	}
}

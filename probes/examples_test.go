package probes_test

import (
	"github.com/ninedraft/httpext/probes"
)

func ExampleHealth() {
	health := probes.Health()

	dbOk := health.Component("db")
	dbOk(true)

	probeServer, probeMux := probes.Server(":9090")
	probeMux.Handle("/health", health)

	_ = probeServer.ListenAndServe
}

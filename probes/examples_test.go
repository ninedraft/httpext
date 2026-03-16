package probes_test

import (
	"github.com/ninedraft/httpext/probes"
)

func ExampleHealth() {
	health := probes.Health()

	dbOk := health.Component("db")
	dbOk(true)

	probeServer := probes.New(":9090")

	_ = probeServer.ListenAndServe
}

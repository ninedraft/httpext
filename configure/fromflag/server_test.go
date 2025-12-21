package fromflag

import (
	"flag"
	"net/http"
	"testing"
	"time"
)

func TestServerFlagsSetValues(t *testing.T) {
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	srv := &http.Server{
		Addr:           "0.0.0.0:0",
		IdleTimeout:    0,
		ReadTimeout:    0,
		WriteTimeout:   0,
		MaxHeaderBytes: 0,
	}

	if err := Server(flags, "custom", srv); err != nil {
		t.Fatalf("Server(...) error = %v", err)
	}

	args := []string{
		"--custom.addr=example.com:9000",
		"--custom.idle-timeout=13s",
		"--custom.read-timeout=17s",
		"--custom.read-header-timeout=5s",
		"--custom.write-timeout=2s",
		"--custom.max-header-bytes=2048",
	}
	if err := flags.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	checkDuration(t, "idle-timeout", srv.IdleTimeout, 13*time.Second)
	checkDuration(t, "read-timeout", srv.ReadTimeout, 17*time.Second)
	checkDuration(t, "read-header-timeout", srv.ReadHeaderTimeout, 5*time.Second)
	checkDuration(t, "write-timeout", srv.WriteTimeout, 2*time.Second)

	if got, want := srv.Addr, "example.com:9000"; got != want {
		t.Fatalf("Addr = %q, want %q", got, want)
	}
	if got, want := srv.MaxHeaderBytes, 2048; got != want {
		t.Fatalf("MaxHeaderBytes = %d, want %d", got, want)
	}
}

func TestServerFlagsRejectsNil(t *testing.T) {
	t.Run("flagset", func(t *testing.T) {
		if err := Server(nil, "x", &http.Server{}); err == nil {
			t.Fatal("expected error for nil FlagSet")
		}
	})
	t.Run("server", func(t *testing.T) {
		flags := flag.NewFlagSet("server", flag.ContinueOnError)
		if err := Server(flags, "x", nil); err == nil {
			t.Fatal("expected error for nil server")
		}
	})
}

func checkDuration(t *testing.T, name string, got, want time.Duration) {
	t.Helper()

	if got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

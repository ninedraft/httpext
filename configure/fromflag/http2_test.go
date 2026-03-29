package fromflag

import (
	"flag"
	"net/http"
	"testing"
	"time"
)

func TestHTTP2FlagsSetValues(t *testing.T) {
	flags := flag.NewFlagSet("http2", flag.ContinueOnError)
	cfg := &http.HTTP2Config{}

	if err := HTTP2(flags, "h2", cfg); err != nil {
		t.Fatalf("HTTP2(...) error = %v", err)
	}

	args := []string{
		"--h2.max-concurrent-streams=42",
		"--h2.max-decoder-header-table-size=1024",
		"--h2.max-encoder-header-table-size=2048",
		"--h2.max-read-frame-size=65536",
		"--h2.max-receive-buffer-per-connection=8192",
		"--h2.max-receive-buffer-per-stream=4096",
		"--h2.send-ping-timeout=3s",
		"--h2.ping-timeout=5s",
		"--h2.write-byte-timeout=1s",
		"--h2.permit-prohibited-cipher-suites=true",
	}
	if err := flags.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if got, want := cfg.MaxConcurrentStreams, 42; got != want {
		t.Fatalf("MaxConcurrentStreams = %d, want %d", got, want)
	}
	if got, want := cfg.MaxDecoderHeaderTableSize, 1024; got != want {
		t.Fatalf("MaxDecoderHeaderTableSize = %d, want %d", got, want)
	}
	if got, want := cfg.MaxEncoderHeaderTableSize, 2048; got != want {
		t.Fatalf("MaxEncoderHeaderTableSize = %d, want %d", got, want)
	}
	if got, want := cfg.MaxReadFrameSize, 65536; got != want {
		t.Fatalf("MaxReadFrameSize = %d, want %d", got, want)
	}
	if got, want := cfg.MaxReceiveBufferPerConnection, 8192; got != want {
		t.Fatalf("MaxReceiveBufferPerConnection = %d, want %d", got, want)
	}
	if got, want := cfg.MaxReceiveBufferPerStream, 4096; got != want {
		t.Fatalf("MaxReceiveBufferPerStream = %d, want %d", got, want)
	}

	checkDuration(t, "send-ping-timeout", cfg.SendPingTimeout, 3*time.Second)
	checkDuration(t, "ping-timeout", cfg.PingTimeout, 5*time.Second)
	checkDuration(t, "write-byte-timeout", cfg.WriteByteTimeout, 1*time.Second)

	if !cfg.PermitProhibitedCipherSuites {
		t.Fatal("PermitProhibitedCipherSuites = false, want true")
	}
}

func TestHTTP2FlagsRejectsNil(t *testing.T) {
	t.Run("flagset", func(t *testing.T) {
		if err := HTTP2(nil, "h2", &http.HTTP2Config{}); err == nil {
			t.Fatal("expected error for nil FlagSet")
		}
	})
	t.Run("config", func(t *testing.T) {
		flags := flag.NewFlagSet("http2", flag.ContinueOnError)
		if err := HTTP2(flags, "h2", nil); err == nil {
			t.Fatal("expected error for nil HTTP2Config")
		}
	})
}

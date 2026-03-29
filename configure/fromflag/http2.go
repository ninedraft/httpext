package fromflag

import (
	"flag"
	"fmt"
	"net/http"
)

// HTTP2 registers flag bindings for the most commonly tuned options in
// http.HTTP2Config. Function-valued knobs such as CountError are intentionally
// omitted because flags cannot configure callbacks.
func HTTP2(flags *flag.FlagSet, prefix string, cfg *http.HTTP2Config) error {
	switch {
	case flags == nil:
		return fmt.Errorf("fromflag: flag set is nil")
	case cfg == nil:
		return fmt.Errorf("fromflag: HTTP/2 config is nil")
	}

	name := prefix
	if name != "" {
		name += "."
	}

	flags.IntVar(&cfg.MaxConcurrentStreams, name+"max-concurrent-streams", cfg.MaxConcurrentStreams,
		"maximum number of concurrent streams a peer may open")
	flags.IntVar(&cfg.MaxDecoderHeaderTableSize, name+"max-decoder-header-table-size", cfg.MaxDecoderHeaderTableSize,
		"upper limit for the decoder header compression table (bytes)")
	flags.IntVar(&cfg.MaxEncoderHeaderTableSize, name+"max-encoder-header-table-size", cfg.MaxEncoderHeaderTableSize,
		"upper limit for the encoder header compression table (bytes)")
	flags.IntVar(&cfg.MaxReadFrameSize, name+"max-read-frame-size", cfg.MaxReadFrameSize,
		"largest frame size this endpoint will read (bytes)")
	flags.IntVar(&cfg.MaxReceiveBufferPerConnection, name+"max-receive-buffer-per-connection", cfg.MaxReceiveBufferPerConnection,
		"flow control window per connection (bytes)")
	flags.IntVar(&cfg.MaxReceiveBufferPerStream, name+"max-receive-buffer-per-stream", cfg.MaxReceiveBufferPerStream,
		"flow control window per stream (bytes)")

	flags.DurationVar(&cfg.SendPingTimeout, name+"send-ping-timeout", cfg.SendPingTimeout,
		"timeout before sending a health-check ping when idle")
	flags.DurationVar(&cfg.PingTimeout, name+"ping-timeout", cfg.PingTimeout,
		"timeout waiting for a ping response before closing the connection")
	flags.DurationVar(&cfg.WriteByteTimeout, name+"write-byte-timeout", cfg.WriteByteTimeout,
		"timeout for write progress on idle connections")

	flags.BoolVar(&cfg.PermitProhibitedCipherSuites, name+"permit-prohibited-cipher-suites", cfg.PermitProhibitedCipherSuites,
		"allow cipher suites that the HTTP/2 spec normally forbids")

	return nil
}

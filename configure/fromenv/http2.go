package fromenv

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	EnvHTTP2MaxConcurrentStreams         = "HTTP2_MAX_CONCURRENT_STREAMS"
	EnvHTTP2MaxDecoderHeaderTableSize    = "HTTP2_MAX_DECODER_HEADER_TABLE_SIZE"
	EnvHTTP2MaxEncoderHeaderTableSize    = "HTTP2_MAX_ENCODER_HEADER_TABLE_SIZE"
	EnvHTTP2MaxReadFrameSize             = "HTTP2_MAX_READ_FRAME_SIZE"
	EnvHTTP2MaxReceiveBufferPerConn      = "HTTP2_MAX_RECEIVE_BUFFER_PER_CONNECTION"
	EnvHTTP2MaxReceiveBufferPerStream    = "HTTP2_MAX_RECEIVE_BUFFER_PER_STREAM"
	EnvHTTP2SendPingTimeout              = "HTTP2_SEND_PING_TIMEOUT"
	EnvHTTP2PingTimeout                  = "HTTP2_PING_TIMEOUT"
	EnvHTTP2WriteByteTimeout             = "HTTP2_WRITE_BYTE_TIMEOUT"
	EnvHTTP2PermitProhibitedCipherSuites = "HTTP2_PERMIT_PROHIBITED_CIPHER_SUITES"
)

func AllHTTP2EnvKeys() []string {
	return slices.Clone(envHTTP2Keys)
}

var envHTTP2Keys = []string{
	EnvHTTP2MaxConcurrentStreams,
	EnvHTTP2MaxDecoderHeaderTableSize,
	EnvHTTP2MaxEncoderHeaderTableSize,
	EnvHTTP2MaxReadFrameSize,
	EnvHTTP2MaxReceiveBufferPerConn,
	EnvHTTP2MaxReceiveBufferPerStream,
	EnvHTTP2SendPingTimeout,
	EnvHTTP2PingTimeout,
	EnvHTTP2WriteByteTimeout,
	EnvHTTP2PermitProhibitedCipherSuites,
}

func HTTP2(lookup LookupEnv, prefix string, cfg *http.HTTP2Config) (map[string]string, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}
	if cfg == nil {
		return nil, fmt.Errorf("fromenv: http2 config is nil")
	}

	if prefix != "" {
		prefix += "_"
	}

	env := make(map[string]string, len(envHTTP2Keys))
	for _, key := range envHTTP2Keys {
		fullKey := prefix + key
		if val, ok := lookup(fullKey); ok {
			env[key] = val
		}
	}

	if err := setValue(&cfg.MaxConcurrentStreams, env, EnvHTTP2MaxConcurrentStreams, strconv.Atoi); err != nil {
		return nil, err
	}
	if err := setValue(&cfg.MaxDecoderHeaderTableSize, env, EnvHTTP2MaxDecoderHeaderTableSize, strconv.Atoi); err != nil {
		return nil, err
	}
	if err := setValue(&cfg.MaxEncoderHeaderTableSize, env, EnvHTTP2MaxEncoderHeaderTableSize, strconv.Atoi); err != nil {
		return nil, err
	}
	if err := setValue(&cfg.MaxReadFrameSize, env, EnvHTTP2MaxReadFrameSize, strconv.Atoi); err != nil {
		return nil, err
	}
	if err := setValue(&cfg.MaxReceiveBufferPerConnection, env, EnvHTTP2MaxReceiveBufferPerConn, strconv.Atoi); err != nil {
		return nil, err
	}
	if err := setValue(&cfg.MaxReceiveBufferPerStream, env, EnvHTTP2MaxReceiveBufferPerStream, strconv.Atoi); err != nil {
		return nil, err
	}

	if err := setValue(&cfg.SendPingTimeout, env, EnvHTTP2SendPingTimeout, time.ParseDuration); err != nil {
		return nil, err
	}
	if err := setValue(&cfg.PingTimeout, env, EnvHTTP2PingTimeout, time.ParseDuration); err != nil {
		return nil, err
	}
	if err := setValue(&cfg.WriteByteTimeout, env, EnvHTTP2WriteByteTimeout, time.ParseDuration); err != nil {
		return nil, err
	}

	if err := setValue(&cfg.PermitProhibitedCipherSuites, env, EnvHTTP2PermitProhibitedCipherSuites, strconv.ParseBool); err != nil {
		return nil, err
	}

	return env, nil
}

func HTTP2Usage(prefix string, defaults *http.HTTP2Config) string {
	if defaults == nil {
		defaults = &http.HTTP2Config{}
	}

	if prefix != "" {
		prefix += "_"
	}

	type entry struct {
		Key         string
		Description string
		Value       any
	}
	entries := []entry{
		{
			Key:   prefix + EnvHTTP2MaxConcurrentStreams,
			Value: defaults.MaxConcurrentStreams,
			Description: "MaxConcurrentStreams optionally specifies the number of concurrent streams " +
				"that a peer may have open at a time. If zero, the limit defaults to at least 100. " +
				"Set via this environment variable.",
		},
		{
			Key:   prefix + EnvHTTP2MaxDecoderHeaderTableSize,
			Value: defaults.MaxDecoderHeaderTableSize,
			Description: "MaxDecoderHeaderTableSize limits the size of the header compression table used " +
				"for decoding headers sent by the peer. Valid values are less than 4MiB; if zero or invalid, " +
				"a default is used.",
		},
		{
			Key:   prefix + EnvHTTP2MaxEncoderHeaderTableSize,
			Value: defaults.MaxEncoderHeaderTableSize,
			Description: "MaxEncoderHeaderTableSize limits the header compression table used for sending " +
				"headers to the peer. Valid values are less than 4MiB; if zero or invalid, a default is used.",
		},
		{
			Key:   prefix + EnvHTTP2MaxReadFrameSize,
			Value: defaults.MaxReadFrameSize,
			Description: "MaxReadFrameSize specifies the largest frame this endpoint is willing to read. " +
				"Valid values are between 16KiB and 16MiB; if zero or invalid, a default is used.",
		},
		{
			Key:   prefix + EnvHTTP2MaxReceiveBufferPerConn,
			Value: defaults.MaxReceiveBufferPerConnection,
			Description: "MaxReceiveBufferPerConnection is the maximum flow control window for data received " +
				"on a connection. Valid values are at least 64KiB and less than 4MiB; invalid values use " +
				"a default.",
		},
		{
			Key:   prefix + EnvHTTP2MaxReceiveBufferPerStream,
			Value: defaults.MaxReceiveBufferPerStream,
			Description: "MaxReceiveBufferPerStream is the maximum flow control window for data received " +
				"on a stream. Valid values are less than 4MiB; if zero or invalid, a default is used.",
		},
		{
			Key:   prefix + EnvHTTP2SendPingTimeout,
			Value: defaults.SendPingTimeout,
			Description: "SendPingTimeout is the timeout after which a health check using a ping frame is " +
				"carried out when no frame has been received; if zero, no ping is sent.",
		},
		{
			Key:   prefix + EnvHTTP2PingTimeout,
			Value: defaults.PingTimeout,
			Description: "PingTimeout is the timeout after which a connection is closed when a ping response " +
				"is not received; if zero, a 15 second default is used.",
		},
		{
			Key:   prefix + EnvHTTP2WriteByteTimeout,
			Value: defaults.WriteByteTimeout,
			Description: "WriteByteTimeout is the timeout after which the connection is closed if no data " +
				"can be written; the timeout resets whenever bytes are written.",
		},
		{
			Key:   prefix + EnvHTTP2PermitProhibitedCipherSuites,
			Value: defaults.PermitProhibitedCipherSuites,
			Description: "PermitProhibitedCipherSuites, if true, allows the use of cipher suites that the " +
				"HTTP/2 spec normally forbids.",
		},
	}

	usage := &strings.Builder{}
	for _, item := range entries {
		fmt.Fprintf(usage, "%s=%v\n\t%s\n", item.Key, item.Value, item.Description)
	}

	return usage.String()
}

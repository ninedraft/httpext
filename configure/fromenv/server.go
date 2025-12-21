package fromenv

import (
	"cmp"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

type LookupEnv = func(key string) (string, bool)

const (
	EnvServerAddr                     = "SERVE_ADDR"
	EnvServerIdleTimeout              = "IDLE_TIMEOUT"
	EnvServerReadTimeout              = "READ_TIMEOUT"
	EnvServerReadHeaderTimeout        = "READ_HEADER_TIMEOUT"
	EnvServerWriteTimeout             = "WRITE_TIMEOUT"
	EnvServerMaxHeaderBytes           = "MAX_HEADER_BYTES"
	EnvServerProtocolHTTP1            = "PROTOCOL_ENABLED_HTTP1"
	EnvServerProtocolHTTP2            = "PROTOCOL_ENABLED_HTTP2"
	EnvServerProtocolUnencryptedHTTP2 = "PROTOCOL_ENABLED_UNENCRYPTED_HTTP2"
)

func AllServerEnvKeys() []string {
	return slices.Clone(envServerAllKeys)
}

var envServerAllKeys = []string{
	EnvServerAddr,
	EnvServerIdleTimeout,
	EnvServerReadTimeout,
	EnvServerReadHeaderTimeout,
	EnvServerWriteTimeout,
	EnvServerMaxHeaderBytes,
	EnvServerProtocolHTTP1,
	EnvServerProtocolHTTP2,
	EnvServerProtocolUnencryptedHTTP2,
}

func ServerUsage(prefix string, defaults *http.Server) string {
	if prefix != "" {
		prefix += "_"
	}

	if defaults == nil {
		defaults = &http.Server{}
	}

	readTimeoutKey := prefix + EnvServerReadTimeout

	type entry struct {
		Key         string
		Description string
		Value       any
	}
	entries := []entry{
		{
			Key:   prefix + EnvServerAddr,
			Value: cmp.Or(defaults.Addr, ":http"),
			Description: "Addr optionally specifies the TCP address for the server to listen on, " +
				"in the form \"host:port\". If empty, \":http\" (port 80) is used. " +
				"The service names are defined in RFC 6335 and assigned by IANA.",
		},
		{
			Key:   prefix + EnvServerIdleTimeout,
			Value: defaults.IdleTimeout,
			Description: "IdleTimeout is the maximum amount of time to wait for the next request " +
				"when keep-alives are enabled. If zero, the value of " + readTimeoutKey + " is used. " +
				"If negative, or if zero and " + readTimeoutKey + " is zero or negative, there is no timeout.",
		},
		{
			Key:   prefix + EnvServerReadTimeout,
			Value: defaults.ReadTimeout,
			Description: "ReadTimeout is the maximum duration for reading the entire request, " +
				"including the body. A zero or negative value means there will be no timeout. " +
				"It does not let handlers make per-request decisions on deadlines, so " +
				"ReadHeaderTimeout may be preferred; both can be used together.",
		},
		{
			Key:   prefix + EnvServerReadHeaderTimeout,
			Value: defaults.ReadHeaderTimeout,
			Description: "ReadHeaderTimeout is the amount of time allowed to read request headers. " +
				"The connection's read deadline is reset after the headers are read " +
				"so the handler can decide what is considered too slow for the body. " +
				"If zero, the value of " + readTimeoutKey + " is used; if negative, or if zero and " +
				readTimeoutKey + " is zero or negative, there is no timeout.",
		},
		{
			Key:   prefix + EnvServerWriteTimeout,
			Value: defaults.WriteTimeout,
			Description: "WriteTimeout is the maximum duration before timing out writes of the response. " +
				"It is reset whenever a new request's header is read. Like ReadTimeout, " +
				"it does not let handlers make decisions on a per-request basis. " +
				"A zero or negative value means there will be no timeout.",
		},
		{
			Key:   prefix + EnvServerMaxHeaderBytes,
			Value: http.DefaultMaxHeaderBytes,
			Description: "MaxHeaderBytes controls the maximum number of bytes the server will read when " +
				"parsing the request header keys and values, including the request line. " +
				"It does not limit the size of the request body. If zero, the default header limit is used.",
		},
		{
			Key:         prefix + EnvServerProtocolHTTP1,
			Value:       "true/false",
			Description: "Enable HTTP1 to accept HTTP/1.0 and HTTP/1.1 connections (supported on both TLS and plain TCP).",
		},
		{
			Key:         prefix + EnvServerProtocolHTTP2,
			Value:       "true/false",
			Description: "Enable HTTP2 to accept TLS-based HTTP/2 connections (requires the TLS ALPN protocol to select \"h2\").",
		},
		{
			Key:         prefix + EnvServerProtocolUnencryptedHTTP2,
			Value:       "true/false",
			Description: "Enable UnencryptedHTTP2 to accept plain TCP HTTP/2 connections using the \"h2c\" upgrade path.",
		},
	}

	usage := &strings.Builder{}

	for _, item := range entries {
		fmt.Fprintf(usage, "%s=%v\n\t%s\n", item.Key, item.Value, item.Description)
	}

	return usage.String()
}

func Server(lookup LookupEnv, prefix string, srv *http.Server) (map[string]string, error) {
	if lookup == nil {
		lookup = os.LookupEnv
	}

	if prefix != "" {
		prefix += "_"
	}

	env := make(map[string]string, len(envServerAllKeys))
	for _, key := range envServerAllKeys {
		fullKey := prefix + key
		if val, ok := lookup(fullKey); ok {
			env[key] = val
		}
	}

	serveAddr, ok := env[EnvServerAddr]
	if ok {
		srv.Addr = serveAddr
	}

	if err := setValue(&srv.IdleTimeout, env, EnvServerIdleTimeout, time.ParseDuration); err != nil {
		return nil, err
	}
	if err := setValue(&srv.ReadTimeout, env, EnvServerReadTimeout, time.ParseDuration); err != nil {
		return nil, err
	}
	if err := setValue(&srv.ReadHeaderTimeout, env, EnvServerReadHeaderTimeout, time.ParseDuration); err != nil {
		return nil, err
	}
	if err := setValue(&srv.WriteTimeout, env, EnvServerWriteTimeout, time.ParseDuration); err != nil {
		return nil, err
	}
	if err := setValue(&srv.MaxHeaderBytes, env, EnvServerMaxHeaderBytes, strconv.Atoi); err != nil {
		return nil, err
	}

	protocols := &http.Protocols{}
	if srv.Protocols != nil {
		*protocols = *srv.Protocols
	}

	protocolSet := false
	setProto := func(key string, setFn func(bool)) error {
		val, ok, err := parseValue(env, key, strconv.ParseBool)
		if err != nil {
			return err
		}
		if ok {
			setFn(val)
			protocolSet = true
		}
		return nil
	}

	if err := setProto(EnvServerProtocolHTTP1, protocols.SetHTTP1); err != nil {
		return nil, err
	}
	if err := setProto(EnvServerProtocolHTTP2, protocols.SetHTTP2); err != nil {
		return nil, err
	}
	if err := setProto(EnvServerProtocolUnencryptedHTTP2, protocols.SetUnencryptedHTTP2); err != nil {
		return nil, err
	}

	if protocolSet {
		srv.Protocols = protocols
	}

	return env, nil
}

func setValue[E any](dst *E, env map[string]string, key string, parseFn func(string) (E, error)) error {
	val, ok, err := parseValue(env, key, parseFn)
	if err != nil {
		return err
	}
	if ok {
		*dst = val
	}

	return nil
}

func parseValue[E any](env map[string]string, key string, parseFn func(string) (E, error)) (E, bool, error) {
	valStr, ok := env[key]
	if !ok {
		var zero E
		return zero, false, nil
	}

	val, err := parseFn(valStr)
	if err != nil {
		var zero E
		return zero, false, fmt.Errorf("%q=%q: %w", key, valStr, err)
	}

	return val, true, nil
}

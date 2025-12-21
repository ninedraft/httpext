package fromenv

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHTTP2EnvBindings(t *testing.T) {
	const prefix = "H2"
	t.Setenv(prefix+"_"+EnvHTTP2MaxConcurrentStreams, "16")
	t.Setenv(prefix+"_"+EnvHTTP2MaxDecoderHeaderTableSize, "1024")
	t.Setenv(prefix+"_"+EnvHTTP2MaxEncoderHeaderTableSize, "2048")
	t.Setenv(prefix+"_"+EnvHTTP2MaxReadFrameSize, "65536")
	t.Setenv(prefix+"_"+EnvHTTP2MaxReceiveBufferPerConn, "16384")
	t.Setenv(prefix+"_"+EnvHTTP2MaxReceiveBufferPerStream, "8192")
	t.Setenv(prefix+"_"+EnvHTTP2SendPingTimeout, "4s")
	t.Setenv(prefix+"_"+EnvHTTP2PingTimeout, "6s")
	t.Setenv(prefix+"_"+EnvHTTP2WriteByteTimeout, "2s")
	t.Setenv(prefix+"_"+EnvHTTP2PermitProhibitedCipherSuites, "true")

	cfg := &http.HTTP2Config{}

	env, err := HTTP2(nil, prefix, cfg)
	require.NoError(t, err)

	require.Equal(t, 16, cfg.MaxConcurrentStreams)
	require.Equal(t, 1024, cfg.MaxDecoderHeaderTableSize)
	require.Equal(t, 2048, cfg.MaxEncoderHeaderTableSize)
	require.Equal(t, 65536, cfg.MaxReadFrameSize)
	require.Equal(t, 16384, cfg.MaxReceiveBufferPerConnection)
	require.Equal(t, 8192, cfg.MaxReceiveBufferPerStream)
	checkDuration(t, cfg.SendPingTimeout, 4*time.Second)
	checkDuration(t, cfg.PingTimeout, 6*time.Second)
	checkDuration(t, cfg.WriteByteTimeout, 2*time.Second)
	require.True(t, cfg.PermitProhibitedCipherSuites)

	require.Equal(t, "16", env[EnvHTTP2MaxConcurrentStreams])
}

func TestHTTP2EnvInvalid(t *testing.T) {
	t.Setenv(EnvHTTP2MaxReadFrameSize, "not-int")

	_, err := HTTP2(nil, "", &http.HTTP2Config{})
	require.Error(t, err)
}

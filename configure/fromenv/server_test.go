package fromenv

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServerUsage(t *testing.T) {
	usage := ServerUsage("", nil)
	require.NotEmptyf(t, usage, "server usage")
	t.Log(usage)
}

func TestServerEnvBindings(t *testing.T) {
	const prefix = "MYAPP"

	t.Setenv(prefix+"_"+EnvServerAddr, "1.2.3.4:1234")
	t.Setenv(prefix+"_"+EnvServerIdleTimeout, "3s")
	t.Setenv(prefix+"_"+EnvServerReadTimeout, "4s")
	t.Setenv(prefix+"_"+EnvServerReadHeaderTimeout, "2s")
	t.Setenv(prefix+"_"+EnvServerWriteTimeout, "5s")
	t.Setenv(prefix+"_"+EnvServerMaxHeaderBytes, "4096")
	t.Setenv(prefix+"_"+EnvServerProtocolHTTP1, "true")
	t.Setenv(prefix+"_"+EnvServerProtocolHTTP2, "true")
	t.Setenv(prefix+"_"+EnvServerProtocolUnencryptedHTTP2, "false")

	srv := &http.Server{}

	env, err := Server(nil, prefix, srv)
	require.NoError(t, err)

	require.Equal(t, "1.2.3.4:1234", srv.Addr)
	checkDuration(t, srv.IdleTimeout, 3*time.Second)
	checkDuration(t, srv.ReadTimeout, 4*time.Second)
	checkDuration(t, srv.ReadHeaderTimeout, 2*time.Second)
	checkDuration(t, srv.WriteTimeout, 5*time.Second)

	require.Equal(t, 4096, srv.MaxHeaderBytes)
	require.True(t, srv.Protocols.HTTP1())
	require.True(t, srv.Protocols.HTTP2())
	require.False(t, srv.Protocols.UnencryptedHTTP2())

	require.Equal(t, "1.2.3.4:1234", env[EnvServerAddr])
}

func TestServerEnvInvalid(t *testing.T) {
	t.Setenv(EnvServerMaxHeaderBytes, "not-int")

	_, err := Server(nil, "", &http.Server{})
	require.Error(t, err)
}

func checkDuration(t *testing.T, got, want time.Duration) {
	t.Helper()
	require.Equal(t, want, got)
}

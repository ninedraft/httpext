package fromflag

import (
	"cmp"
	"flag"
	"fmt"
	"net/http"
	"strconv"

	httpintern "github.com/ninedraft/httpext/configure/internal/http"
)

// Server registers flag bindings for commonly tuned http.Server fields.
func Server(flags *flag.FlagSet, prefix string, srv *http.Server) error {
	switch {
	case flags == nil:
		return fmt.Errorf("fromflag: flag set is nil")
	case srv == nil:
		return fmt.Errorf("fromflag: server is nil")
	}

	name := prefix
	if name != "" {
		name += "."
	}

	addr := cmp.Or(srv.Addr, "localhost:8080")
	flags.StringVar(&srv.Addr, name+"addr", addr, "server listen address")

	flags.DurationVar(&srv.IdleTimeout, name+"idle-timeout", srv.IdleTimeout,
		"maximum idle duration for keep-alive connections")
	flags.DurationVar(&srv.ReadTimeout, name+"read-timeout", srv.ReadTimeout,
		"maximum duration for reading the request")
	flags.DurationVar(&srv.ReadHeaderTimeout, name+"read-header-timeout", srv.ReadHeaderTimeout,
		"maximum duration for reading request headers")
	flags.DurationVar(&srv.WriteTimeout, name+"write-timeout", srv.WriteTimeout,
		"maximum duration before timing out writes")
	flags.IntVar(&srv.MaxHeaderBytes, name+"max-header-bytes", srv.MaxHeaderBytes,
		"maximum header size in bytes (0 means http.DefaultMaxHeaderBytes)")

	srv.Protocols = httpintern.Protocols(srv)

	flagProtocol := func(proto string, enabled bool, setFunc func(bool)) {
		state := "enabled"
		if !enabled {
			state = "disabled"
		}

		usage := fmt.Sprintf("enable or disable %s protocol, %s by default", proto, state)
		flags.BoolFunc(name+"protocols."+proto, usage, func(s string) error {
			ok, err := strconv.ParseBool(s)
			if err != nil {
				return err
			}
			setFunc(ok)

			return nil
		})
	}

	flagProtocol("http1", srv.Protocols.HTTP1(), srv.Protocols.SetHTTP1)
	flagProtocol("http2", srv.Protocols.HTTP2(), srv.Protocols.SetHTTP2)
	flagProtocol("unencrypted_http2", srv.Protocols.UnencryptedHTTP2(), srv.Protocols.SetUnencryptedHTTP2)

	return nil
}

package http

import (
	"net/http"
	"os"
	"strings"
)

// Adapted from go/src/net/http/server.go
//
// LICENSE.go BSD-3-Clause applies.
func Protocols(srv *http.Server) *http.Protocols {
	if srv.Protocols != nil {
		return srv.Protocols // user-configured set
	}

	// The historic way of disabling HTTP/2 is to set TLSNextProto to
	// a non-nil map with no "h2" entry.
	_, hasH2 := srv.TLSNextProto["h2"]
	http2Disabled := srv.TLSNextProto != nil && !hasH2

	if strings.Contains(os.Getenv("GODEBUG"), "http2server=0") && !hasH2 {
		http2Disabled = true
	}

	protocols := &http.Protocols{}

	protocols.SetHTTP1(true) // default always includes HTTP/1
	if !http2Disabled {
		protocols.SetHTTP2(true)
	}

	return protocols
}

package configure

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// Server constructs http.Server with sane defaults.
// If logh is not nil, it will be used for server error logging.
//
// Provided context is used to manage server lifecycle and is expected to be cancelled
// when server shutdown is desired.
// The server's Shutdown method is called with a 5 second timeout after context cancellation,
// followed by Close to ensure all resources are released.
//
// The server's BaseContext is set to the provided context.
func Server(ctx context.Context, logh slog.Handler, addr string) *http.Server {
	log := slog.Default()
	if logh != nil {
		log = slog.New(logh)
	}

	srv := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 100 * time.Millisecond,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	context.AfterFunc(ctx, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := srv.Shutdown(ctx)
		if err != nil {
			log.ErrorContext(ctx, "shutting down server", "error", err)
		}

		if err := srv.Close(); err != nil {
			log.ErrorContext(ctx, "closing server", "error", err)
		}
	})

	if logh != nil {
		srv.ErrorLog = slog.NewLogLogger(logh, slog.LevelError)
	}

	return srv
}

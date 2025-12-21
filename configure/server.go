package configure

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"time"
)

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
	})

	if logh != nil {
		srv.ErrorLog = slog.NewLogLogger(logh, slog.LevelError)
	}

	return srv
}

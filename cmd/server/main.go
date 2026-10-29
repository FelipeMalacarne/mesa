// Command server runs the Mesa service: the HTTP API and the embedded
// web app. It is the container image for Docker and Kubernetes.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/platform"
	"github.com/felipemalacarne/mesa/internal/platform/bootstrap"
	httptransport "github.com/felipemalacarne/mesa/internal/transport/http"
	mcptransport "github.com/felipemalacarne/mesa/internal/transport/mcp"
	"github.com/felipemalacarne/mesa/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mesa-server:", err)
		os.Exit(1)
	}
}

func run() error {
	configDir, err := platform.ConfigDir()
	if err != nil {
		return err
	}
	cfg := platform.FromEnv(configDir)
	log := platform.Logger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := bootstrap.Build(ctx, cfg, app.ModeServer, configDir)
	if err != nil {
		return err
	}
	defer func() { _ = a.Close() }()

	// Databases opened in Monitor keep being sampled, so its ranges have history.
	go a.Monitor.Run(ctx)

	if a.Accounts == nil {
		log.Warn("no identity provider (MESA_OIDC_ISSUER): everyone who reaches this server may do everything; keep it on a private network or behind an authenticating proxy")
	}

	handler := httptransport.New(a, httptransport.Options{
		Assets:      web.Assets(),
		CORSOrigins: cfg.CORSOrigins,
		Logger:      log,
		MCP:         mcptransport.Handler(a, mcptransport.Options{AllowWrites: cfg.MCPWrites}),
		PublicURL:   cfg.PublicURL,
	})
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("mesa-server listening", "addr", cfg.Listen, "version", platform.Version, "store", cfg.StoreDriver)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	log.Info("shutting down")
	return srv.Shutdown(shutdown)
}

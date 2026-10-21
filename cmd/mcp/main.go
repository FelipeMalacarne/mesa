// Command mcp serves Mesa's tools to agents over the Model Context
// Protocol on stdio. Read-only unless --allow-writes is given.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/platform"
	"github.com/felipemalacarne/mesa/internal/platform/bootstrap"
	mcptransport "github.com/felipemalacarne/mesa/internal/transport/mcp"
)

func main() {
	allowWrites := flag.Bool("allow-writes", false, "expose apply_sql; writes still go through policy and audit")
	flag.Parse()
	if err := run(*allowWrites); err != nil {
		fmt.Fprintln(os.Stderr, "mesa-mcp:", err)
		os.Exit(1)
	}
}

func run(allowWrites bool) error {
	configDir, err := platform.ConfigDir()
	if err != nil {
		return err
	}
	cfg := platform.FromEnv(configDir)
	cfg.LogLevel = "warn"
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	a, err := bootstrap.Build(ctx, cfg, app.ModeCLI, configDir)
	if err != nil {
		return err
	}
	defer func() { _ = a.Close() }()
	return mcptransport.Serve(ctx, mcptransport.NewServer(a, mcptransport.Options{AllowWrites: allowWrites}))
}

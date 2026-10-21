// Command cli is the Mesa CLI (installed as `mesa`).
package main

import (
	"context"
	"os"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/platform"
	"github.com/felipemalacarne/mesa/internal/platform/bootstrap"
	"github.com/felipemalacarne/mesa/internal/transport/cli"
)

func main() {
	deps := cli.Deps{
		Open: func(ctx context.Context) (*app.App, error) {
			configDir, err := platform.ConfigDir()
			if err != nil {
				return nil, err
			}
			cfg := platform.FromEnv(configDir)
			cfg.LogLevel = "warn"
			return bootstrap.Build(ctx, cfg, app.ModeCLI, configDir)
		},
	}
	os.Exit(cli.Run(context.Background(), deps, os.Args[1:]))
}

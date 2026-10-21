// Package bootstrap builds the application for a binary: store, vault,
// engines and the app, from platform.Config. Every cmd/* main uses it.
package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/zalando/go-keyring"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/engine"
	"github.com/felipemalacarne/mesa/internal/engine/postgres"
	"github.com/felipemalacarne/mesa/internal/platform"
	"github.com/felipemalacarne/mesa/internal/secret"
	"github.com/felipemalacarne/mesa/internal/session"
	"github.com/felipemalacarne/mesa/internal/store"
	storepg "github.com/felipemalacarne/mesa/internal/store/postgres"
	"github.com/felipemalacarne/mesa/internal/store/project"
	"github.com/felipemalacarne/mesa/internal/store/sqlite"
)

// Engines returns every driver this build ships.
func Engines() []engine.Engine {
	return []engine.Engine{postgres.New()}
}

// OpenStore opens the configured store.
func OpenStore(ctx context.Context, c platform.Config) (store.Store, error) {
	switch strings.ToLower(c.StoreDriver) {
	case "sqlite", "":
		return sqlite.Open(ctx, c.StoreDSN)
	case "postgres", "postgresql":
		return storepg.Open(ctx, c.StoreDSN)
	}
	return nil, errs.Newf(errs.Invalid, "unknown store driver %q (sqlite or postgres)", c.StoreDriver)
}

const (
	keychainService = "mesa"
	keychainAccount = "app-key"
)

// AppKey returns the configured key. In server mode it must be set; for
// desktop and CLI a key file is created under the config directory.
func AppKey(c platform.Config, mode app.Mode, configDir string) (string, error) {
	if c.AppKey != "" {
		return c.AppKey, nil
	}
	if mode == app.ModeServer {
		return "", errs.New(errs.Invalid, "MESA_APP_KEY is required in server mode (generate one with `mesa key`)")
	}
	// The desktop app keeps its key in the OS keychain when there is one.
	if mode == app.ModeDesktop {
		if k, err := keyring.Get(keychainService, keychainAccount); err == nil && k != "" {
			return k, nil
		}
		if _, err := os.Stat(filepath.Join(configDir, "key")); errors.Is(err, fs.ErrNotExist) {
			if k, err := secret.GenerateKey(); err == nil && keyring.Set(keychainService, keychainAccount, k) == nil {
				return k, nil
			}
		}
	}
	path := filepath.Join(configDir, "key")
	if b, err := os.ReadFile(path); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		return strings.TrimSpace(string(b)), nil
	}
	key, err := secret.GenerateKey()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(key+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("writing key file: %w", err)
	}
	return key, nil
}

// Build opens everything and returns the application. Close it when done.
func Build(ctx context.Context, c platform.Config, mode app.Mode, configDir string) (*app.App, error) {
	key, err := AppKey(c, mode, configDir)
	if err != nil {
		return nil, err
	}
	st, err := OpenStore(ctx, c)
	if err != nil {
		return nil, err
	}
	vault, err := secret.NewLocal(key, st.Secrets())
	if err != nil {
		_ = st.Close()
		return nil, err
	}
	var proj *project.Project
	if c.Project != "" && c.Project != "off" {
		if proj, err = project.Find(c.Project); err != nil {
			_ = st.Close()
			return nil, err
		}
	}
	return app.New(app.Options{
		Project: proj,
		Version: platform.Version,
		Mode:    mode,
		Engines: Engines(),
		Store:   st,
		Vault:   vault,
		Session: session.Options{IdleTimeout: c.SessionIdle, AppName: "mesa-" + string(mode)},
	}), nil
}

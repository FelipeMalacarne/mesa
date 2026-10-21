//go:build desktop

// Command desktop is Mesa as a desktop app (ADR 0003): a Wails window whose
// webview talks to the same HTTP handler mesa-server runs, in process. Data
// stays on this machine (SQLite under the config directory); the key that
// encrypts connection secrets lives in the OS keychain when there is one.
//
// Build: go build -tags desktop,production,webkit2_41 -o bin/mesa-desktop ./cmd/desktop
// (Linux needs WebKitGTK 4.1 and GTK 3 development packages; see `make desktop`.)
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/platform"
	"github.com/felipemalacarne/mesa/internal/platform/bootstrap"
	httptransport "github.com/felipemalacarne/mesa/internal/transport/http"
	"github.com/felipemalacarne/mesa/web"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mesa-desktop:", err)
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
	a, err := bootstrap.Build(context.Background(), cfg, app.ModeDesktop, configDir)
	if err != nil {
		return err
	}
	defer func() { _ = a.Close() }()

	handler := httptransport.New(a, httptransport.Options{Assets: web.Assets(), Logger: log})
	var wctx context.Context
	return wails.Run(&options.App{
		Title:            "Mesa",
		Width:            1440,
		Height:           900,
		MinWidth:         960,
		MinHeight:        600,
		BackgroundColour: &options.RGBA{R: 20, G: 20, B: 23, A: 255},
		// Every request from the webview, the API included, goes to the
		// same handler the service runs.
		AssetServer: &assetserver.Options{Handler: handler},
		Menu:        appMenu(func() context.Context { return wctx }),
		OnStartup:   func(ctx context.Context) { wctx = ctx },
	})
}

func appMenu(ctx func() context.Context) *menu.Menu {
	m := menu.NewMenu()
	// The role menus exist only on macOS, where Edit is what makes copy and
	// paste work in the webview; Linux and Windows handle those natively.
	if isMac() {
		m.Append(menu.AppMenu())
		m.Append(menu.EditMenu())
	}
	view := m.AddSubmenu("View")
	view.AddText("Reload", keys.CmdOrCtrl("r"), func(*menu.CallbackData) { runtime.WindowReload(ctx()) })
	view.AddText("Command palette", keys.CmdOrCtrl("k"), func(*menu.CallbackData) {
		runtime.WindowExecJS(ctx(), `window.dispatchEvent(new KeyboardEvent("keydown",{key:"k",metaKey:navigator.platform.includes("Mac"),ctrlKey:!navigator.platform.includes("Mac")}))`)
	})
	view.AddSeparator()
	view.AddText("Toggle full screen", keys.Key("f11"), func(*menu.CallbackData) {
		if runtime.WindowIsFullscreen(ctx()) {
			runtime.WindowUnfullscreen(ctx())
		} else {
			runtime.WindowFullscreen(ctx())
		}
	})
	return m
}

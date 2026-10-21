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
	_ "embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/felipemalacarne/mesa/internal/app"
	"github.com/felipemalacarne/mesa/internal/platform"
	"github.com/felipemalacarne/mesa/internal/platform/bootstrap"
	"github.com/felipemalacarne/mesa/internal/platform/desktop"
	httptransport "github.com/felipemalacarne/mesa/internal/transport/http"
	"github.com/felipemalacarne/mesa/web"
)

//go:embed appicon.png
var appIcon []byte

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "mesa-desktop:", err)
		os.Exit(1)
	}
}

func run() error {
	// After an update the new build waits for the old one to quit, so the
	// single-instance lock is free.
	waitForExit(os.Getenv("MESA_WAIT_PID"))
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
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	go a.Monitor.Run(ctx)

	if exe, err := os.Executable(); err == nil && os.Getenv("MESA_REGISTER_SCHEME") != "off" {
		if err := desktop.RegisterScheme(exe); err != nil {
			log.Warn("cannot register mesa:// links", "err", err)
		}
	}
	links := &linkRouter{}
	if route, ok := desktop.FromArgs(os.Args[1:]); ok {
		links.pending = route
	}
	updates := newUpdates(configDir, log)

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
		Menu:        appMenu(func() context.Context { return wctx }, updates),
		OnStartup: func(ctx context.Context) {
			wctx = ctx
			links.start(ctx)
			go updates.background(ctx)
		},
		OnDomReady: links.ready,
		// A second launch (a mesa:// link clicked on Linux or Windows) hands
		// its arguments to this window and exits.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "dev.mesa.desktop",
			OnSecondInstanceLaunch: func(d options.SecondInstanceData) {
				if route, ok := desktop.FromArgs(d.Args); ok {
					links.open(route)
				} else {
					links.focus()
				}
			},
		},
		Linux: &linux.Options{Icon: appIcon, ProgramName: "mesa-desktop"},
		Mac: &mac.Options{
			OnUrlOpen: func(url string) {
				if route, ok := desktop.Route(url); ok {
					links.open(route)
				}
			},
		},
	})
}

func appMenu(ctx func() context.Context, updates *updates) *menu.Menu {
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
	help := m.AddSubmenu("Help")
	help.AddText("Check for updates…", nil, func(*menu.CallbackData) { go updates.check(ctx(), true) })
	help.AddText("Mesa on GitHub", nil, func(*menu.CallbackData) { runtime.BrowserOpenURL(ctx(), "https://github.com/felipemalacarne/mesa") })
	return m
}

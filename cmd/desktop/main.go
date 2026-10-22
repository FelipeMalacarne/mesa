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
	"github.com/felipemalacarne/mesa/internal/platform/update"
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

	// A Nix install's path changes with every version; its package registers the links.
	if exe, err := os.Executable(); err == nil && os.Getenv("MESA_REGISTER_SCHEME") != "off" && !update.Managed(exe) {
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
		AssetServer: &assetserver.Options{Handler: links.serve(handler)},
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
	// send runs one of the web app's commands (web/src/app/commands.ts), the
	// same ones its keyboard shortcuts run; it ignores a repeat within 250 ms,
	// so a key press seen by both the menu and the page acts once.
	send := func(id string) menu.Callback {
		return func(*menu.CallbackData) {
			runtime.WindowExecJS(ctx(), `window.dispatchEvent(new CustomEvent("mesa:command",{detail:"`+id+`"}))`)
		}
	}
	m := menu.NewMenu()
	// The role menus exist only on macOS, where Edit is what makes copy and
	// paste work in the webview; Linux and Windows handle those natively.
	if isMac() {
		m.Append(menu.AppMenu())
	}
	file := m.AddSubmenu("File")
	file.AddText("New Query Tab", keys.CmdOrCtrl("t"), send("tab.new"))
	file.AddText("Reopen Closed Tab", keys.Combo("t", keys.CmdOrCtrlKey, keys.ShiftKey), send("tab.reopen"))
	file.AddSeparator()
	file.AddText("Close Tab", keys.CmdOrCtrl("w"), send("tab.close"))
	if isMac() {
		m.Append(menu.EditMenu())
	}
	view := m.AddSubmenu("View")
	view.AddText("Command Palette", keys.CmdOrCtrl("k"), send("palette"))
	view.AddText("Filter Objects", keys.CmdOrCtrl("p"), send("filter"))
	view.AddText("Show or Hide Sidebar", keys.CmdOrCtrl("b"), send("rail"))
	view.AddSeparator()
	view.AddText("Next Tab", keys.Control("tab"), send("tab.next"))
	view.AddText("Previous Tab", keys.Combo("tab", keys.ControlKey, keys.ShiftKey), send("tab.prev"))
	view.AddSeparator()
	view.AddText("Refresh", keys.CmdOrCtrl("r"), send("refresh"))
	view.AddText("Reload Window", keys.Combo("r", keys.CmdOrCtrlKey, keys.ShiftKey), func(*menu.CallbackData) { runtime.WindowReload(ctx()) })
	view.AddText("Toggle Full Screen", keys.Key("f11"), func(*menu.CallbackData) {
		if runtime.WindowIsFullscreen(ctx()) {
			runtime.WindowUnfullscreen(ctx())
		} else {
			runtime.WindowFullscreen(ctx())
		}
	})
	help := m.AddSubmenu("Help")
	help.AddText("Keyboard Shortcuts", keys.CmdOrCtrl("/"), send("help"))
	help.AddSeparator()
	help.AddText("Check for Updates…", nil, func(*menu.CallbackData) { go updates.check(ctx(), true) })
	help.AddText("Mesa on GitHub", nil, func(*menu.CallbackData) { runtime.BrowserOpenURL(ctx(), "https://github.com/felipemalacarne/mesa") })
	return m
}

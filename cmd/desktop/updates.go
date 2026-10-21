//go:build desktop

package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/felipemalacarne/mesa/internal/platform"
	"github.com/felipemalacarne/mesa/internal/platform/update"
)

// updates checks for new releases once a day (MESA_UPDATE_CHECK=off turns
// that off) and from the Help menu, and installs one when asked.
type updates struct {
	u     *update.Updater
	state string
	log   *slog.Logger
	busy  sync.Mutex
}

func newUpdates(configDir string, log *slog.Logger) *updates {
	return &updates{u: update.New(platform.Version), state: filepath.Join(configDir, "last-update-check"), log: log}
}

func (up *updates) background(ctx context.Context) {
	if os.Getenv("MESA_UPDATE_CHECK") == "off" || up.u.Development() {
		return
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(15 * time.Second):
	}
	if update.Due(up.state, time.Now(), 24*time.Hour) {
		up.check(ctx, false)
	}
}

// check looks for a release; asked is true when a person chose the menu
// item, who then also hears that there is nothing new.
func (up *updates) check(ctx context.Context, asked bool) {
	if !up.busy.TryLock() {
		return
	}
	defer up.busy.Unlock()
	say := func(title, msg string) {
		_, _ = runtime.MessageDialog(ctx, runtime.MessageDialogOptions{Type: runtime.InfoDialog, Title: title, Message: msg})
	}
	if up.u.Development() {
		if asked {
			say("Development build", "This build of Mesa ("+platform.Version+") does not update itself.")
		}
		return
	}
	rel, err := up.u.Latest(ctx)
	if err != nil {
		up.log.Warn("update check failed", "err", err)
		if asked {
			say("Cannot check for updates", err.Error())
		}
		return
	}
	if rel == nil {
		if asked {
			say("Mesa is up to date", "You have Mesa "+platform.Version+", the latest release.")
		}
		return
	}
	choice, err := runtime.MessageDialog(ctx, runtime.MessageDialogOptions{
		Type:          runtime.QuestionDialog,
		Title:         "Mesa " + rel.Version + " is available",
		Message:       "You have " + platform.Version + ". Install " + rel.Version + " and restart Mesa now?",
		Buttons:       []string{"Install and restart", "Release notes", "Later"},
		DefaultButton: "Install and restart",
		CancelButton:  "Later",
	})
	if err != nil {
		return
	}
	switch choice {
	case "Release notes":
		runtime.BrowserOpenURL(ctx, rel.Page)
		return
	case "Install and restart", "Yes", "Ok", "OK":
	default:
		return
	}
	if err := up.install(ctx, rel); err != nil {
		up.log.Error("update failed", "version", rel.Version, "err", err)
		if errors.Is(err, update.ErrUnsigned) {
			runtime.BrowserOpenURL(ctx, rel.Page)
		}
		say("Update failed", err.Error())
	}
}

// install downloads, verifies and swaps in the release, then starts it and
// quits; the new process waits for this one to exit.
func (up *updates) install(ctx context.Context, rel *update.Release) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	archive, err := up.u.Download(ctx, rel, os.TempDir())
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(archive) }()
	next, err := update.Install(archive, exe)
	if err != nil {
		return err
	}
	cmd := exec.Command(next, os.Args[1:]...)
	cmd.Env = append(os.Environ(), "MESA_WAIT_PID="+strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	runtime.Quit(ctx)
	return nil
}

// waitForExit blocks until the process pid (a previous build being
// replaced) is gone, for at most 15 seconds.
func waitForExit(pid string) {
	n, err := strconv.Atoi(pid)
	if err != nil || n <= 0 {
		return
	}
	deadline := time.Now().Add(15 * time.Second)
	for processAlive(n) && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
	}
}

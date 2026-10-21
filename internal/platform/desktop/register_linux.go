package desktop

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RegisterScheme makes exe the handler of mesa:// links for this user: a
// desktop entry under $XDG_DATA_HOME/applications declaring the scheme, set
// as the default handler with xdg-mime when it is installed.
func RegisterScheme(exe string) error {
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		dataHome = filepath.Join(home, ".local", "share")
	}
	dir := filepath.Join(dataHome, "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	file := filepath.Join(dir, "mesa.desktop")
	entry := DesktopEntry(exe)
	if old, err := os.ReadFile(file); err == nil && bytes.Equal(old, entry) {
		return nil
	}
	if err := os.WriteFile(file, entry, 0o644); err != nil {
		return err
	}
	if p, err := exec.LookPath("xdg-mime"); err == nil {
		_ = exec.Command(p, "default", "mesa.desktop", "x-scheme-handler/"+Scheme).Run()
	}
	if p, err := exec.LookPath("update-desktop-database"); err == nil {
		_ = exec.Command(p, dir).Run()
	}
	return nil
}

// DesktopEntry is the freedesktop.org entry for exe.
func DesktopEntry(exe string) []byte {
	quoted := `"` + strings.NewReplacer(`\`, `\\\\`, `"`, `\\"`, "`", "\\\\`", "$", `\\$`).Replace(exe) + `"`
	return []byte(`[Desktop Entry]
Type=Application
Name=Mesa
GenericName=SQL client
Comment=The open-source SQL control plane
Exec=` + quoted + ` %u
Icon=mesa
Terminal=false
Categories=Development;Database;
MimeType=x-scheme-handler/` + Scheme + `;
StartupWMClass=mesa-desktop
`)
}

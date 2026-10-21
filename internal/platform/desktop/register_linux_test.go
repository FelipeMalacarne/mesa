package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRegisterSchemeWritesADesktopEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	t.Setenv("PATH", "") // no xdg-mime: the entry alone must be written
	if err := RegisterScheme("/opt/Mesa Apps/mesa-desktop"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "applications", "mesa.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`Exec="/opt/Mesa Apps/mesa-desktop" %u`, "MimeType=x-scheme-handler/mesa;"} {
		if !strings.Contains(s, want) {
			t.Errorf("entry lacks %q:\n%s", want, s)
		}
	}
	if !strings.Contains(string(DesktopEntry(`/a"b$c`)), `Exec="/a\\"b\\$c" %u`) {
		t.Errorf("special characters are not escaped: %s", DesktopEntry(`/a"b$c`))
	}
}

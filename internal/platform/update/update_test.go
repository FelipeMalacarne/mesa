package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v1.2.0", "1.1.9", true},
		{"1.10.0", "1.9.3", true},
		{"1.2.0", "1.2.0", false},
		{"1.2.0", "1.2.0-rc.1", true},
		{"1.2.0-rc.2", "1.2.0-rc.1", true},
		{"1.2.0-rc.1", "1.2.0", false},
		{"1.2.0", "dev", false},
		{"nightly", "1.0.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.a, c.b, got)
		}
	}
}

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range files {
		_ = tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte(body))
	}
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetMode(0o755)
		w, _ := zw.CreateHeader(h)
		_, _ = w.Write([]byte(body))
	}
	_ = zw.Close()
	return buf.Bytes()
}

// server publishes one release with the given archive and signs its
// checksums with priv.
func server(t *testing.T, tag, archiveName string, archive []byte, priv ed25519.PrivateKey) *httptest.Server {
	t.Helper()
	sum := sha256.Sum256(archive)
	sums := []byte(hex.EncodeToString(sum[:]) + "  " + archiveName + "\n" + strings.Repeat("0", 64) + "  other.zip\n")
	sig := ed25519.Sign(priv, sums)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/o/r/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name": tag, "html_url": srv.URL + "/page", "body": "notes",
				"assets": []map[string]any{
					{"name": archiveName, "browser_download_url": srv.URL + "/a"},
					{"name": "checksums.txt", "browser_download_url": srv.URL + "/sums"},
					{"name": "checksums.txt.sig", "browser_download_url": srv.URL + "/sig"},
				},
			})
		case "/a":
			_, _ = w.Write(archive)
		case "/sums":
			_, _ = w.Write(sums)
		case "/sig":
			_, _ = w.Write(sig)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func updater(srv *httptest.Server, current string, pub ed25519.PublicKey) *Updater {
	return &Updater{Repo: "o/r", Current: current, APIBase: srv.URL, Client: srv.Client(), PublicKey: pub, GOOS: "linux", GOARCH: "amd64"}
}

func TestCheckDownloadAndInstallABinary(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	name := ArchiveName("linux", "amd64")
	srv := server(t, "v1.3.0", name, tarGz(t, map[string]string{"mesa-desktop_1.3.0/mesa-desktop": "new build", "mesa-desktop_1.3.0/mesa.desktop": "x"}), priv)
	ctx := context.Background()

	if rel, err := updater(srv, "1.3.0", pub).Latest(ctx); err != nil || rel != nil {
		t.Fatalf("current build: Latest = %+v, %v; want nothing", rel, err)
	}
	if rel, err := updater(srv, "dev", pub).Latest(ctx); err != nil || rel != nil {
		t.Fatalf("dev build: Latest = %+v, %v; want nothing", rel, err)
	}
	u := updater(srv, "1.2.4", pub)
	rel, err := u.Latest(ctx)
	if err != nil || rel == nil || rel.Version != "1.3.0" || rel.Archive.Name != name {
		t.Fatalf("Latest = %+v, %v", rel, err)
	}

	dir := t.TempDir()
	exe := filepath.Join(dir, "mesa-desktop")
	if err := os.WriteFile(exe, []byte("old build"), 0o755); err != nil {
		t.Fatal(err)
	}
	archive, err := u.Download(ctx, rel, dir)
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	next, err := Install(archive, exe)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if b, _ := os.ReadFile(next); string(b) != "new build" {
		t.Errorf("installed %q", b)
	}
	if b, _ := os.ReadFile(exe + ".old"); string(b) != "old build" {
		t.Errorf("previous build kept as %q", b)
	}
	if info, _ := os.Stat(next); info.Mode().Perm()&0o100 == 0 {
		t.Error("installed build is not executable")
	}
}

func TestDownloadRefusesWhatIsNotSigned(t *testing.T) {
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	_, other, _ := ed25519.GenerateKey(rand.Reader)
	name := ArchiveName("linux", "amd64")
	srv := server(t, "v2.0.0", name, tarGz(t, map[string]string{"mesa-desktop": "x"}), other)
	ctx := context.Background()
	u := updater(srv, "1.0.0", pub)
	rel, err := u.Latest(ctx)
	if err != nil || rel == nil {
		t.Fatalf("Latest = %v, %v", rel, err)
	}
	if _, err := u.Download(ctx, rel, t.TempDir()); err == nil || !strings.Contains(err.Error(), "not signed") {
		t.Errorf("wrong key: error = %v", err)
	}
	u.PublicKey = nil
	if _, err := u.Download(ctx, rel, t.TempDir()); !errors.Is(err, ErrUnsigned) {
		t.Errorf("no key: error = %v, want ErrUnsigned", err)
	}
	// A tampered archive fails the checksum even with a valid signature.
	pub2, priv2, _ := ed25519.GenerateKey(rand.Reader)
	srv2 := server(t, "v2.0.0", name, tarGz(t, map[string]string{"mesa-desktop": "x"}), priv2)
	u2 := updater(srv2, "1.0.0", pub2)
	rel2, _ := u2.Latest(ctx)
	rel2.Archive.URL = srv2.URL + "/sums"
	if _, err := u2.Download(ctx, rel2, t.TempDir()); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Errorf("tampered archive: error = %v", err)
	}
}

func TestInstallSwapsAnAppBundle(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "Mesa.app", "Contents", "MacOS", "mesa-desktop")
	_ = os.MkdirAll(filepath.Dir(exe), 0o755)
	_ = os.WriteFile(exe, []byte("old"), 0o755)
	archive := filepath.Join(dir, "Mesa_darwin_universal.zip")
	_ = os.WriteFile(archive, zipOf(t, map[string]string{
		"Mesa.app/Contents/MacOS/mesa-desktop": "new",
		"Mesa.app/Contents/Info.plist":         "<plist/>",
	}), 0o644)
	next, err := Install(archive, exe)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if b, _ := os.ReadFile(next); string(b) != "new" {
		t.Errorf("installed %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "Mesa.app", "Contents", "Info.plist")); err != nil {
		t.Error("bundle was not replaced as a whole")
	}
	if _, err := os.Stat(filepath.Join(dir, "Mesa.app.old")); !os.IsNotExist(err) {
		t.Error("the old bundle was left behind")
	}
	evil := filepath.Join(dir, "evil.zip")
	_ = os.WriteFile(evil, zipOf(t, map[string]string{"../../escape": "x"}), 0o644)
	if err := unzip(evil, filepath.Join(dir, "x")); err == nil {
		t.Error("unzip accepted a path outside its directory")
	}
}

// TestOpenSSLSignaturesVerify checks the release workflow's signing command
// (openssl pkeyutl -sign -rawin with an Ed25519 key) against VerifySignature.
func TestOpenSSLSignaturesVerify(t *testing.T) {
	openssl, err := exec.LookPath("openssl")
	if err != nil {
		t.Skip("openssl is not installed")
	}
	dir := t.TempDir()
	key := filepath.Join(dir, "key.pem")
	msg := filepath.Join(dir, "checksums.txt")
	sig := filepath.Join(dir, "checksums.txt.sig")
	_ = os.WriteFile(msg, []byte("abc  file\n"), 0o644)
	for _, args := range [][]string{
		{"genpkey", "-algorithm", "ed25519", "-out", key},
		{"pkeyutl", "-sign", "-rawin", "-inkey", key, "-in", msg, "-out", sig},
	} {
		if out, err := exec.Command(openssl, args...).CombinedOutput(); err != nil {
			t.Skipf("openssl %v: %v %s", args[0], err, out)
		}
	}
	der, err := exec.Command(openssl, "pkey", "-in", key, "-pubout", "-outform", "DER").Output()
	if err != nil {
		t.Fatal(err)
	}
	pub := ed25519.PublicKey(der[len(der)-ed25519.PublicKeySize:])
	m, _ := os.ReadFile(msg)
	s, _ := os.ReadFile(sig)
	if !VerifySignature(pub, m, s) {
		t.Error("an openssl Ed25519 signature does not verify")
	}
}

func TestDue(t *testing.T) {
	f := filepath.Join(t.TempDir(), "state", "last-update-check")
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	if !Due(f, now, 24*time.Hour) {
		t.Error("first check is not due")
	}
	if Due(f, now.Add(time.Hour), 24*time.Hour) {
		t.Error("second check within a day is due")
	}
	if !Due(f, now.Add(25*time.Hour), 24*time.Hour) {
		t.Error("check after a day is not due")
	}
}

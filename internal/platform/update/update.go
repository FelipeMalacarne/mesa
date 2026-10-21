// Package update keeps the desktop app current: it finds the latest GitHub
// release, downloads the archive for this platform, verifies it against the
// release's checksums (themselves signed with the project's Ed25519 key,
// whose public half is built into the binary) and swaps it in place of the
// running executable or app bundle.
package update

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// PublicKey is the base64 Ed25519 key release checksums are signed with,
// set at build time: -ldflags "-X …/platform/update.PublicKey=…". A build
// without it can tell that an update exists but will not install one.
var PublicKey = ""

// DefaultRepo is where releases are published.
const DefaultRepo = "felipemalacarne/mesa"

const (
	checksumsName = "checksums.txt"
	signatureName = "checksums.txt.sig"
	maxDownload   = 512 << 20
)

// ErrUnsigned means this build cannot verify releases.
var ErrUnsigned = errors.New("this build has no release signing key, so updates must be installed by hand")

// Asset is one file of a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

// Release is a published version and the files this platform needs.
type Release struct {
	Version   string
	Notes     string
	Page      string
	Archive   Asset
	Checksums Asset
	Signature Asset
}

// Updater checks and downloads releases.
type Updater struct {
	Repo      string
	Current   string
	APIBase   string // https://api.github.com
	Client    *http.Client
	PublicKey ed25519.PublicKey
	GOOS      string
	GOARCH    string
}

// New returns an updater for the running build.
func New(current string) *Updater {
	u := &Updater{
		Repo:    DefaultRepo,
		Current: current,
		APIBase: "https://api.github.com",
		Client:  &http.Client{Timeout: 10 * time.Minute},
		GOOS:    runtime.GOOS,
		GOARCH:  runtime.GOARCH,
	}
	if raw, err := base64.StdEncoding.DecodeString(PublicKey); err == nil && len(raw) == ed25519.PublicKeySize {
		u.PublicKey = ed25519.PublicKey(raw)
	}
	return u
}

// ArchiveName is the release file that carries the desktop app for a
// platform; the release workflow publishes exactly these names.
func ArchiveName(goos, goarch string) string {
	switch goos {
	case "darwin":
		return "Mesa_darwin_universal.zip"
	case "windows":
		return "mesa-desktop_windows_" + goarch + ".zip"
	default:
		return "mesa-desktop_" + goos + "_" + goarch + ".tar.gz"
	}
}

// Development reports whether the running build is not a release.
func (u *Updater) Development() bool {
	_, ok := parseVersion(u.Current)
	return !ok
}

// Latest returns the newest release, or nil when the running build is
// already current or is a development build.
func (u *Updater) Latest(ctx context.Context) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.APIBase+"/repos/"+u.Repo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := u.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach the release server: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusNotFound {
		return nil, nil // no release yet
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release server answered %s", res.Status)
	}
	var body struct {
		Tag    string  `json:"tag_name"`
		Body   string  `json:"body"`
		Page   string  `json:"html_url"`
		Draft  bool    `json:"draft"`
		Pre    bool    `json:"prerelease"`
		Assets []Asset `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("cannot read the release: %w", err)
	}
	if body.Draft || body.Pre || !Newer(body.Tag, u.Current) {
		return nil, nil
	}
	rel := &Release{Version: strings.TrimPrefix(body.Tag, "v"), Notes: body.Body, Page: body.Page}
	want := ArchiveName(u.GOOS, u.GOARCH)
	for _, a := range body.Assets {
		switch a.Name {
		case want:
			rel.Archive = a
		case checksumsName:
			rel.Checksums = a
		case signatureName:
			rel.Signature = a
		}
	}
	if rel.Archive.URL == "" || rel.Checksums.URL == "" {
		return nil, fmt.Errorf("release %s has no %s for this platform", body.Tag, want)
	}
	return rel, nil
}

// Download fetches the release archive into dir after verifying the signed
// checksums, and returns its path.
func (u *Updater) Download(ctx context.Context, rel *Release, dir string) (string, error) {
	if len(u.PublicKey) == 0 {
		return "", ErrUnsigned
	}
	if rel.Signature.URL == "" {
		return "", errors.New("the release has no checksum signature")
	}
	sums, err := u.fetch(ctx, rel.Checksums.URL, 1<<20)
	if err != nil {
		return "", err
	}
	sig, err := u.fetch(ctx, rel.Signature.URL, 4<<10)
	if err != nil {
		return "", err
	}
	if !VerifySignature(u.PublicKey, sums, sig) {
		return "", errors.New("the release checksums are not signed by the Mesa release key")
	}
	want, err := checksumFor(sums, rel.Archive.Name)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rel.Archive.URL, nil)
	if err != nil {
		return "", err
	}
	res, err := u.Client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: %s", res.Status)
	}
	f, err := os.CreateTemp(dir, ".mesa-update-*-"+rel.Archive.Name)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, maxDownload+1))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil && n > maxDownload {
		err = errors.New("the download is larger than any Mesa release")
	}
	if err == nil && hex.EncodeToString(h.Sum(nil)) != want {
		err = errors.New("the download does not match the signed checksum")
	}
	if err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func (u *Updater) fetch(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := u.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed: %s", res.Status)
	}
	return io.ReadAll(io.LimitReader(res.Body, limit))
}

// VerifySignature checks an Ed25519 signature over msg. The signature file
// is the raw 64 bytes `openssl pkeyutl -sign -rawin` writes, or their
// base64 text.
func VerifySignature(pub ed25519.PublicKey, msg, sig []byte) bool {
	if len(sig) != ed25519.SignatureSize {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
		if err != nil {
			return false
		}
		sig = decoded
	}
	return len(sig) == ed25519.SignatureSize && ed25519.Verify(pub, msg, sig)
}

// checksumFor finds name in a sha256sum-style listing.
func checksumFor(sums []byte, name string) (string, error) {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name && len(fields[0]) == 64 {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("the checksums do not list %s", name)
}

// Newer reports whether version a is newer than b. Development builds
// ("dev", or anything that is not x.y.z) are never older than a release.
func Newer(a, b string) bool {
	va, ok := parseVersion(a)
	if !ok {
		return false
	}
	vb, ok := parseVersion(b)
	if !ok {
		return false
	}
	for i := range 3 {
		if va.nums[i] != vb.nums[i] {
			return va.nums[i] > vb.nums[i]
		}
	}
	// 1.2.0 is newer than 1.2.0-rc.1.
	if va.pre == "" || vb.pre == "" {
		return va.pre == "" && vb.pre != ""
	}
	return va.pre > vb.pre
}

type version struct {
	nums [3]int
	pre  string
}

func parseVersion(s string) (version, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	var v version
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		if s[i] == '-' {
			v.pre = strings.SplitN(s[i+1:], "+", 2)[0]
		}
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return v, false
		}
		v.nums[i] = n
	}
	return v, true
}

// Due reports whether an automatic check should run, given the file that
// records the last one, and records now when it does.
func Due(stateFile string, now time.Time, every time.Duration) bool {
	if b, err := os.ReadFile(stateFile); err == nil {
		if last, err := time.Parse(time.RFC3339, strings.TrimSpace(string(b))); err == nil && now.Sub(last) < every {
			return false
		}
	}
	_ = os.MkdirAll(filepath.Dir(stateFile), 0o700)
	_ = os.WriteFile(stateFile, []byte(now.UTC().Format(time.RFC3339)), 0o600)
	return true
}

package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Install replaces the running app with the one in archive and returns the
// executable to start. A plain executable (Linux, Windows) is swapped by
// renaming: the old one is kept as <exe>.old until the next update, which
// also lets Windows rename a running program. Inside a macOS app bundle the
// whole bundle is swapped.
func Install(archive, exe string) (string, error) {
	exe, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	if bundle, ok := appBundle(exe); ok {
		return installBundle(archive, bundle, exe)
	}
	return installBinary(archive, exe)
}

// Managed reports whether a package manager owns exe: Nix keeps it in the
// read-only /nix/store and updates it itself, so the app must not.
func Managed(exe string) bool {
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	return strings.HasPrefix(filepath.ToSlash(exe), "/nix/store/")
}

// appBundle returns /…/Mesa.app for /…/Mesa.app/Contents/MacOS/<exe>.
func appBundle(exe string) (string, bool) {
	macos := filepath.Dir(exe)
	contents := filepath.Dir(macos)
	bundle := filepath.Dir(contents)
	if filepath.Base(macos) == "MacOS" && filepath.Base(contents) == "Contents" && strings.HasSuffix(bundle, ".app") {
		return bundle, true
	}
	return "", false
}

func installBinary(archive, exe string) (string, error) {
	name := filepath.Base(exe)
	next := filepath.Join(filepath.Dir(exe), "."+name+".new")
	out, err := os.OpenFile(next, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return "", fmt.Errorf("cannot write next to %s: %w", exe, err)
	}
	found, err := extractFile(archive, name, out)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil && !found {
		err = fmt.Errorf("the update does not contain %s", name)
	}
	if err != nil {
		_ = os.Remove(next)
		return "", err
	}
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		_ = os.Remove(next)
		return "", err
	}
	if err := os.Rename(next, exe); err != nil {
		_ = os.Rename(old, exe)
		return "", err
	}
	return exe, nil
}

// extractFile copies the archive member whose base name is name into w.
func extractFile(archive, name string, w io.Writer) (bool, error) {
	if strings.HasSuffix(archive, ".zip") {
		zr, err := zip.OpenReader(archive)
		if err != nil {
			return false, err
		}
		defer func() { _ = zr.Close() }()
		for _, f := range zr.File {
			if path := filepath.ToSlash(f.Name); pathBase(path) == name && !f.FileInfo().IsDir() {
				rc, err := f.Open()
				if err != nil {
					return false, err
				}
				_, err = io.Copy(w, io.LimitReader(rc, maxDownload))
				_ = rc.Close()
				return err == nil, err
			}
		}
		return false, nil
	}
	f, err := os.Open(archive)
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return false, err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if h.Typeflag == tar.TypeReg && pathBase(h.Name) == name {
			_, err := io.Copy(w, io.LimitReader(tr, maxDownload))
			return err == nil, err
		}
	}
}

func pathBase(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// installBundle unpacks Mesa.app next to the running bundle and swaps the
// two directories.
func installBundle(archive, bundle, exe string) (string, error) {
	parent := filepath.Dir(bundle)
	tmp, err := os.MkdirTemp(parent, ".mesa-update-")
	if err != nil {
		return "", fmt.Errorf("cannot write next to %s: %w", bundle, err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := unzip(archive, tmp); err != nil {
		return "", err
	}
	fresh := filepath.Join(tmp, filepath.Base(bundle))
	if _, err := os.Stat(fresh); err != nil {
		matches, _ := filepath.Glob(filepath.Join(tmp, "*.app"))
		if len(matches) != 1 {
			return "", errors.New("the update does not contain an app bundle")
		}
		fresh = matches[0]
	}
	rel, _ := filepath.Rel(bundle, exe)
	if info, err := os.Stat(filepath.Join(fresh, rel)); err != nil || info.IsDir() {
		return "", fmt.Errorf("the update's bundle has no %s", rel)
	}
	old := bundle + ".old"
	_ = os.RemoveAll(old)
	if err := os.Rename(bundle, old); err != nil {
		return "", err
	}
	if err := os.Rename(fresh, bundle); err != nil {
		_ = os.Rename(old, bundle)
		return "", err
	}
	_ = os.RemoveAll(old)
	return filepath.Join(bundle, rel), nil
}

// unzip extracts archive into dir, refusing entries that escape it.
func unzip(archive, dir string) error {
	zr, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer func() { _ = zr.Close() }()
	root := filepath.Clean(dir) + string(os.PathSeparator)
	var total int64
	for _, f := range zr.File {
		target := filepath.Join(dir, filepath.FromSlash(f.Name))
		if !strings.HasPrefix(target, root) {
			return fmt.Errorf("unsafe path in update: %s", f.Name)
		}
		mode := f.Mode()
		switch {
		case mode.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		case mode&os.ModeSymlink != 0:
			rc, err := f.Open()
			if err != nil {
				return err
			}
			link, err := io.ReadAll(io.LimitReader(rc, 4096))
			_ = rc.Close()
			if err != nil {
				return err
			}
			dest := filepath.Join(filepath.Dir(target), string(link))
			if filepath.IsAbs(string(link)) || !strings.HasPrefix(dest, root) {
				return fmt.Errorf("unsafe link in update: %s", f.Name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(string(link), target); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm()|0o600)
		if err != nil {
			_ = rc.Close()
			return err
		}
		n, err := io.Copy(out, io.LimitReader(rc, maxDownload-total+1))
		_ = rc.Close()
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
		if total += n; total > maxDownload {
			return errors.New("the update unpacks to more than any Mesa release")
		}
	}
	return nil
}

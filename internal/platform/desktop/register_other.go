//go:build !linux && !windows

package desktop

// RegisterScheme does nothing here: on macOS the app bundle's Info.plist
// declares the scheme (packaging/darwin/Info.plist).
func RegisterScheme(string) error { return nil }

package desktop

import (
	"golang.org/x/sys/windows/registry"
)

// RegisterScheme makes exe the handler of mesa:// links for this user,
// under HKEY_CURRENT_USER\Software\Classes, which needs no elevation.
func RegisterScheme(exe string) error {
	root, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\`+Scheme, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	if err := root.SetStringValue("", "URL:Mesa"); err != nil {
		return err
	}
	if err := root.SetStringValue("URL Protocol", ""); err != nil {
		return err
	}
	cmd, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\`+Scheme+`\shell\open\command`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer func() { _ = cmd.Close() }()
	return cmd.SetStringValue("", `"`+exe+`" "%1"`)
}

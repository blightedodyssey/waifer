//go:build windows

package browsers

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// Chromium-family browsers tint their native window frame and toolbar from
// the BrowserThemeColor policy under HKCU\Software\Policies. Writing it on
// Apply syncs the frame live: no extension reload, no browser restart. One
// writer owns these values (this file); the winsys registry transaction and
// native_sync deliberately no longer touch them.
const framePolicyValue = "BrowserThemeColor"

// framePolicyTarget pairs a Chromium-family browser with its HKCU policy key.
// Only browsers that are actually installed are written.
type framePolicyTarget struct {
	name string
	key  string
}

var framePolicyTargets = []framePolicyTarget{
	{name: "chrome", key: `Software\Policies\Google\Chrome`},
	{name: "chromium", key: `Software\Policies\Chromium`},
	{name: "brave", key: `Software\Policies\BraveSoftware\Brave`},
}

// frameColor normalizes the palette background into the #RRGGBB form the
// BrowserThemeColor policy expects. ok is false when the variable does not
// hold a usable six-digit hex color.
func frameColor(vars map[string]string) (string, bool) {
	hex := strings.TrimPrefix(strings.TrimSpace(vars["background"]), "#")
	if len(hex) != 6 {
		return "", false
	}
	if _, err := strconv.ParseUint(hex, 16, 32); err != nil {
		return "", false
	}
	return "#" + strings.ToUpper(hex), true
}

// framePolicyKeys returns the policy keys for the browsers reported installed.
func framePolicyKeys(installed func(name string) bool) []string {
	keys := make([]string, 0, len(framePolicyTargets))
	for _, target := range framePolicyTargets {
		if installed(target.name) {
			keys = append(keys, target.key)
		}
	}
	return keys
}

// detectInstalledChromiumBrowsers reports which Chromium-family browsers have
// at least one profile on disk.
func detectInstalledChromiumBrowsers() map[string]bool {
	installed := make(map[string]bool, len(framePolicyTargets))
	for _, d := range chromiumDefinitions() {
		if detectChromiumFamily(d).HasData {
			installed[d.name] = true
		}
	}
	return installed
}

// applyBrowserFramePolicy writes BrowserThemeColor for every installed
// Chromium-family browser and returns the number of policy keys written.
// Failures are joined so the caller can log a single non-fatal warning.
func applyBrowserFramePolicy(vars map[string]string) (int, error) {
	color, ok := frameColor(vars)
	if !ok {
		return 0, fmt.Errorf("background color %q is not a usable #RRGGBB frame color", vars["background"])
	}
	installed := detectInstalledChromiumBrowsers()

	var errs []error
	written := 0
	for _, target := range framePolicyTargets {
		if !installed[target.name] {
			continue
		}
		if err := setFramePolicyValue(target.key, color); err != nil {
			errs = append(errs, err)
			continue
		}
		written++
	}
	return written, errors.Join(errs...)
}

// setFramePolicyValue creates (or opens) the browser's HKCU policy key and
// stores the frame color as a REG_SZ.
func setFramePolicyValue(keyPath, color string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open %s: %w", keyPath, err)
	}
	defer key.Close()
	if err := key.SetStringValue(framePolicyValue, color); err != nil {
		return fmt.Errorf("set %s\\%s: %w", keyPath, framePolicyValue, err)
	}
	return nil
}

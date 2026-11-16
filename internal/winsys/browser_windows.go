//go:build windows

package winsys

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

// ApplyFirefoxTheme discovers the active Firefox profile under %APPDATA% and
// writes its chrome/userChrome.css from the current palette.
func ApplyFirefoxTheme(vars map[string]string) error {
	profile, err := findFirefoxProfile(appDataDir())
	if err != nil {
		return err
	}
	return writeFirefoxUserChrome(profile, vars)
}

// applyFirefoxThemeIfPresent skips silently when no Firefox profile exists.
func applyFirefoxThemeIfPresent(vars map[string]string) (bool, error) {
	profile, err := findFirefoxProfile(appDataDir())
	if err != nil {
		return false, nil
	}
	return true, writeFirefoxUserChrome(profile, vars)
}

func restoreFirefoxTheme() error {
	profile, err := findFirefoxProfile(appDataDir())
	if err != nil {
		// Firefox may not be installed or may never have been launched.
		return nil
	}
	return restoreFirefoxUserChrome(profile)
}

// restoreChromeTheme removes Aether's browser theme policy so Chrome returns to
// the user's own theme after "Revert system theme".
func restoreChromeTheme() error {
	var errs []error
	for _, keyPath := range []string{chromePolicyKey, chromiumPolicyKey} {
		key, err := registry.OpenKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
		if err != nil {
			if errors.Is(err, registry.ErrNotExist) {
				continue
			}
			errs = append(errs, fmt.Errorf("open %s: %w", keyPath, err))
			continue
		}
		err = key.DeleteValue("BrowserThemeColor")
		key.Close()
		if err != nil && !errors.Is(err, registry.ErrNotExist) {
			errs = append(errs, fmt.Errorf("remove %s\\BrowserThemeColor: %w", keyPath, err))
		}
	}
	return errors.Join(errs...)
}

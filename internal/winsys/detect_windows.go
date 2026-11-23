//go:build windows

package winsys

import (
	"os"
	"path/filepath"

	"aether/internal/browsers"
	"aether/internal/harnesses"
)

// Detect reports which optional integrations are present on this machine.
// Vanilla Windows ("system") and Windows Terminal are always available; every
// other target is enabled only when its files, installation directory or
// Windhawk mod is actually found. Missing integrations are simply reported as
// false so callers can skip them without warnings or errors.
func Detect() map[string]bool {
	result := make(map[string]bool, len(detectTargetNames))
	for _, name := range detectTargetNames {
		result[name] = false
	}

	// Vanilla Windows native theming is always available.
	result["system"] = true
	// Windows Terminal ships with Windows 11; a missing settings file is not a
	// reason to skip it (best-effort detection).
	result["windows-terminal"] = true

	home := detectHomeDir()
	appData := os.Getenv("APPDATA")
	localAppData := os.Getenv("LOCALAPPDATA")

	// YASB: user config dir/styles.css or the %APPDATA% variant.
	yasbConfig := filepath.Join(home, ".config", "yasb")
	result["yasb"] = pathExists(filepath.Join(yasbConfig, "styles.css")) ||
		pathExists(yasbConfig) ||
		(appData != "" && pathExists(filepath.Join(appData, "yasb", "styles.css")))

	// GlazeWM: the YAML config in either the XDG-style or %APPDATA% location.
	glazeConfigs := []string{}
	if home != "" {
		glazeConfigs = append(glazeConfigs,
			filepath.Join(home, ".config", "glazewm", "config.yaml"),
			filepath.Join(home, ".config", "glazewm", "config.yml"),
		)
	}
	if appData != "" {
		glazeConfigs = append(glazeConfigs,
			filepath.Join(appData, "glazewm", "config.yaml"),
			filepath.Join(appData, "glazewm", "config.yml"),
		)
	}
	for _, config := range glazeConfigs {
		if pathExists(config) {
			result["glazewm"] = true
			break
		}
	}

	// Windhawk stylers: the mod registry keys drive all Windows 11 shell
	// targets, so one scan covers both the aggregate and per-target flags.
	for _, mod := range knownStylerMods {
		if IsWindhawkModInstalled(mod) {
			result["windhawk"] = true
		}
	}
	result["win11-start-menu"] = IsWindhawkModInstalled(WindhawkStartMenuStyler)
	result["win11-notifications"] = IsWindhawkModInstalled(WindhawkNotificationCenterStyler)
	result["win11-taskbar"] = IsWindhawkModInstalled(WindhawkTaskbarStyler)
	result["win11-file-explorer"] = IsWindhawkModInstalled(WindhawkFileExplorerStyler)

	result["browser"] = len(browsers.Detect().Browsers) > 0

	if appData != "" {
		result["discord"] = pathExists(filepath.Join(appData, "discord"))
		result["vencord"] = pathExists(filepath.Join(appData, "Vencord"))
	}
	if localAppData != "" && pathExists(filepath.Join(localAppData, "Vencord")) {
		result["vencord"] = true
	}

	// Harness names are data-driven so newly registered harnesses do not need
	// another entry here.
	for _, h := range harnesses.Detect().Harnesses {
		if h.HasData {
			result[h.Name] = true
		}
	}

	// Editor/vault targets: only claim them when a well-known config location
	// exists. When unsure, report false — legacy Include* flags still work.
	if home != "" {
		result["zed"] = pathExists(filepath.Join(home, ".config", "zed"))
		result["vscode"] = pathExists(filepath.Join(home, ".vscode")) ||
			(appData != "" && pathExists(filepath.Join(appData, "Code")))
		result["neovim"] = pathExists(filepath.Join(home, ".config", "nvim"))
		result["obsidian"] = pathExists(filepath.Join(home, ".config", "obsidian"))
	}
	if appData != "" {
		if pathExists(filepath.Join(appData, "Zed")) {
			result["zed"] = true
		}
		if pathExists(filepath.Join(appData, "obsidian")) {
			result["obsidian"] = true
		}
	}

	return result
}

// detectHomeDir keeps detection testable via HOME/USERPROFILE overrides while
// still falling back to the OS home directory on a normal Windows desktop.
func detectHomeDir() string {
	if home := os.Getenv("USERPROFILE"); home != "" {
		return home
	}
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func pathExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

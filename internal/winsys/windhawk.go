//go:build windows

package winsys

// WindhawkMod names retained for API compatibility with detection and options callers.
// Windhawk shell styler mods are intentionally unused; Waifer relies purely on native
// Windows system theming (system_windows.go / dwm_windows.go).
const (
WindhawkStartMenuStyler          = "windows-11-start-menu-styler"
WindhawkNotificationCenterStyler = "windows-11-notification-center-styler"
WindhawkTaskbarStyler            = "windows-11-taskbar-styler"
WindhawkFileExplorerStyler       = "windows-11-file-explorer-styler"
)

var knownStylerMods = []string{
WindhawkStartMenuStyler,
WindhawkNotificationCenterStyler,
WindhawkTaskbarStyler,
WindhawkFileExplorerStyler,
}

// IsWindhawkModInstalled reports false because Windhawk shell styler mods are removed
// and Waifer uses native Windows system theming exclusively.
func IsWindhawkModInstalled(_ string) bool {
return false
}

// ApplyWindhawkTheme is a no-op; native Windows theming is handled by system_windows.go
// and dwm_windows.go without probing HKLM Windhawk styler keys or spawning elevated .reg imports.
func ApplyWindhawkTheme(_ map[string]string, _ bool) ([]string, []string) {
return nil, nil
}

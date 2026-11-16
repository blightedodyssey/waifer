// Package winsys applies an Aether theme to a Windows desktop: wallpaper,
// system accent color and light/dark mode, Chrome/Firefox browser theming,
// Windows Terminal, Neovim, Zed and VS Code. On other operating systems every
// operation is a no-op so the rest of the app can call it unconditionally.
package winsys

import "errors"

// Options describes the parts of Aether's theme state that Windows needs.
type Options struct {
	// Variables holds the resolved template variables (hex colors plus the
	// "mode"/"theme_type" strings) for the active palette.
	Variables map[string]string
	LightMode bool
	// WallpaperPath is the (possibly blurred) wallpaper to set as the
	// desktop background. Empty leaves the desktop untouched.
	WallpaperPath string
	// ThemeDir is where Aether rendered the per-app files for this apply.
	ThemeDir string
	// Toggle targets. System theming (wallpaper + accent + light/dark mode)
	// follows IncludeSystem; the rest follow the user's app selections.
	IncludeSystem   bool
	IncludeTerminal bool
	IncludeNeovim   bool
	IncludeZed      bool
	IncludeDiscord  bool
	IncludeObsidian bool
	// Phase 4: Windows 11 shell Windhawk styler toggles.
	IncludeWin11StartMenu     bool
	IncludeWin11Notifications bool
	IncludeWin11Taskbar       bool
	IncludeWin11FileExplorer  bool
	// Phase 6: Browser theming (Chrome, Edge, Firefox, etc.).
	IncludeBrowser bool
	// YASB and GlazeWM bar / window-manager theming (opt-in).
	IncludeYasb    bool
	IncludeGlazeWM bool
	// Phase 7: AI Harness theming (opt-in).
	IncludeOpencode  bool
	IncludeHermes    bool
	IncludePi        bool
	IncludeClaudeCode bool
	IncludeJan       bool
	IncludeAider     bool
}

// Result reports what was applied and anything that was skipped.
type Result struct {
	Applied  []string `json:"applied"`
	Warnings []string `json:"warnings"`
}

// ErrUnsupported is returned by helpers that only exist on Windows.
var ErrUnsupported = errors.New("windows integration is only available on Windows")

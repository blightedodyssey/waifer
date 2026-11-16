package theme

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"aether/internal/color"
	"aether/internal/icontheme"
	"aether/internal/omarchy"
	"aether/internal/platform"
	"aether/internal/template"
	"aether/internal/winsys"
)

// Settings controls which editor integrations and applications the Writer
// emits files for.
type Settings struct {
	IncludeZed    bool            `json:"includeZed"`
	IncludeVscode bool            `json:"includeVscode"`
	IncludeNeovim bool            `json:"includeNeovim"`
	ExcludedApps  map[string]bool `json:"excludedApps,omitempty"`
}

// DefaultApplySettings matches the GUI defaults: every editor integration
// on, no apps excluded.
func DefaultApplySettings() Settings {
	return Settings{
		IncludeZed:    true,
		IncludeVscode: true,
		IncludeNeovim: true,
		ExcludedApps:  make(map[string]bool),
	}
}

// ApplyResult reports the outcome of a theme application.
type ApplyResult struct {
	Success   bool     `json:"success"`
	ThemePath string   `json:"themePath,omitempty"`
	Files     []string `json:"files,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// Writer renders the embedded templates into theme directories and applies
// results to the system (wallpaper, accent colors, editor integrations).
type Writer struct {
	templatesFS embed.FS
	root        string
}

// NewWriter builds a Writer over an embedded template filesystem.
func NewWriter(templatesFS embed.FS, root string) *Writer {
	return &Writer{templatesFS: templatesFS, root: root}
}

// templateAppNames maps template files to their application name.
var templateAppNames = map[string]string{
	"aether.zed.json":                                 "zed",
	"alacritty.toml":                                  "alacritty",
	"btop.theme":                                      "btop",
	"chromium.theme":                                  "chromium",
	"colors.toml":                                     "colors",
	"colors.v4.toml":                                  "colors",
	"foot.ini":                                        "foot",
	"ghostty.conf":                                    "ghostty",
	"hyprland.conf":                                   "hyprland",
	"hyprlock.conf":                                   "hyprlock",
	"icons.theme":                                     "icons",
	"kitty.conf":                                      "kitty",
	"mako.ini":                                        "mako",
	"neovim.lua":                                      "neovim",
	"swayosd.css":                                     "swayosd",
	"vencord.theme.css":                               "vencord",
	"vscode.json":                                     "vscode",
	"vscode-theme.json":                               "vscode",
	"vscode-extension/package.json":                   "vscode",
	"vscode-extension/themes/aether-color-theme.json": "vscode",
	"walker.css":            "walker",
	"warp.yaml":             "warp",
	"waybar.css":            "waybar",
	"windows-terminal.json": "windows-terminal",
	"zellij.kdl":            "zellij",
}

// GetAppNameFromFileName resolves the application a template file belongs to.
// Unknown files fall back to their base name without extension.
func GetAppNameFromFileName(name string) string {
	if app, ok := templateAppNames[name]; ok {
		return app
	}
	base := filepath.Base(name)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	return base
}

// validateTemplateInputs sanity-checks the state before any file is written.
func validateTemplateInputs(state *ThemeState) error {
	if state.ColorRoles.Background == "" || state.ColorRoles.Foreground == "" {
		return fmt.Errorf("palette has no background/foreground")
	}
	for i, c := range state.Palette {
		if c == "" {
			continue
		}
		if !color.IsHexColor(c) {
			return fmt.Errorf("palette color %d is not a valid hex color: %s", i, c)
		}
	}
	if state.WallpaperPath != "" && !IsImageFile(state.WallpaperPath) {
		return fmt.Errorf("unsupported wallpaper file: %s", state.WallpaperPath)
	}
	return validateIconTheme(state.IconTheme)
}

// buildVars assembles the template variable map for state, including native
// color overrides and the effective wallpaper path.
func buildVars(state *ThemeState, wallpaperPath string) map[string]string {
	vars := template.BuildVariables(state.ColorRoles, state.LightMode, state.ExtendedColors)
	appendNativeColors(vars, state.NativeColors)
	vars["wallpaper"] = wallpaperPath
	return vars
}

// appendNativeColors merges native (Omarchy-format) colors over the vars.
func appendNativeColors(vars map[string]string, native map[string]string) {
	for k, v := range native {
		if v != "" {
			vars[k] = v
		}
	}
}

// mergeTemplateVariables overlays per-app overrides onto a copy of base.
func mergeTemplateVariables(base map[string]string, override map[string]string) map[string]string {
	if len(override) == 0 {
		return base
	}
	merged := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	template.RecomputeDerived(merged)
	return merged
}

// deepCopy duplicates a nested override map.
func deepCopy(src map[string]map[string]string) map[string]map[string]string {
	out := make(map[string]map[string]string, len(src))
	for k, v := range src {
		inner := make(map[string]string, len(v))
		for ik, iv := range v {
			inner[ik] = iv
		}
		out[k] = inner
	}
	return out
}

// prepareThemeDir makes sure dir exists and is writable.
func prepareThemeDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.MkdirAll(dir, 0o755)
		}
		return fmt.Errorf("inspect theme dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("theme path is not a directory: %s", dir)
	}
	return ensureReplaceableTheme(dir)
}

// ensureReplaceableTheme verifies the theme dir accepts writes.
func ensureReplaceableTheme(dir string) error {
	probe := filepath.Join(dir, ".aether-write-test")
	if err := os.WriteFile(probe, []byte("ok"), 0o644); err != nil {
		return fmt.Errorf("theme dir is not writable: %w", err)
	}
	_ = os.Remove(probe)
	return nil
}

// generate renders every applicable template into dir and returns the list
// of written files.
func (w *Writer) generate(state *ThemeState, settings Settings, dir, wallpaperPath string) ([]string, error) {
	names, err := template.ListTemplates(w.templatesFS, w.root)
	if err != nil {
		return nil, err
	}
	vars := buildVars(state, wallpaperPath)

	files := make([]string, 0, len(names))
	for _, name := range names {
		switch {
		case name == "copy.json":
			continue
		case strings.HasPrefix(name, "vscode-extension/"):
			continue // handled by processVSCodeExtension
		}
		app := GetAppNameFromFileName(name)
		if settings.ExcludedApps[app] {
			continue
		}
		if app == "zed" && !settings.IncludeZed {
			continue
		}
		if app == "vscode" && !settings.IncludeVscode {
			continue
		}
		if app == "neovim" && !settings.IncludeNeovim {
			continue
		}

		content, ok := template.ReadCustomOverride(name)
		if !ok {
			content, err = template.ReadTemplate(w.templatesFS, w.root, name)
			if err != nil {
				return files, err
			}
		}
		rendered := template.ProcessTemplate(content, mergeTemplateVariables(vars, state.AppOverrides[app]))

		dest := filepath.Join(dir, filepath.Base(name))
		if err := platform.WriteText(dest, rendered); err != nil {
			return files, fmt.Errorf("write %s: %w", dest, err)
		}
		files = append(files, dest)
	}
	return files, nil
}

// ApplyTheme renders and applies the theme to the current system.
func (w *Writer) ApplyTheme(state *ThemeState, settings Settings) (*ApplyResult, error) {
	if err := validateTemplateInputs(state); err != nil {
		return nil, err
	}
	themeDir := platform.ThemeDir()
	if err := prepareThemeDir(themeDir); err != nil {
		return nil, err
	}
	effective, err := materializeWallpaper(state, themeDir)
	if err != nil {
		return nil, err
	}

	files, err := w.generate(state, settings, themeDir, effective)
	if err != nil {
		return nil, err
	}

	result := &ApplyResult{ThemePath: themeDir, Files: files}

	// Icon theme: explicit selections override the rendered Yaru mapping.
	if state.IconTheme.Mode == icontheme.SelectionExplicit && state.IconTheme.ID != "" {
		marker := filepath.Join(themeDir, "icons.theme")
		if err := platform.WriteText(marker, state.IconTheme.ID+"\n"); err != nil {
			return result, fmt.Errorf("write icons.theme: %w", err)
		}
		result.Files = append(result.Files, marker)
	}

	// Editor integrations and editor-specific destinations.
	if settings.IncludeVscode {
		if err := processVSCodeExtension(w, state, settings, themeDir); err != nil {
			return result, fmt.Errorf("vscode: %w", err)
		}
	}
	if settings.IncludeZed {
		if err := winsys.ApplyZed(themeDir); err != nil {
			return result, fmt.Errorf("zed: %w", err)
		}
	}
	if settings.IncludeNeovim {
		if err := winsys.ApplyNeovim(themeDir); err != nil {
			return result, fmt.Errorf("neovim: %w", err)
		}
	}
	if runtime.GOOS == "windows" {
		if err := winsys.ApplyWindowsTerminal(themeDir); err != nil {
			return result, fmt.Errorf("windows-terminal: %w", err)
		}
	}

	// Custom apps (standalone mode only — on Omarchy the native loader
	// consumes the theme and hooks handle extra apps).
	if !omarchy.IsInstalled() {
		apps, err := template.ProcessCustomApps(buildVars(state, effective), themeDir)
		if err != nil {
			return result, fmt.Errorf("custom apps: %w", err)
		}
		_ = apps
	}

	// System application.
	if effective != "" {
		if runtime.GOOS == "windows" {
			if err := winsys.SetWallpaper(effective); err != nil {
				return result, fmt.Errorf("wallpaper: %w", err)
			}
		} else if omarchy.IsInstalled() {
			if err := omarchy.SetBackground(effective); err != nil {
				return result, fmt.Errorf("wallpaper: %w", err)
			}
		}
	}
	if runtime.GOOS == "windows" {
		if err := winsys.Apply(state.Palette, state.LightMode); err != nil {
			return result, fmt.Errorf("system colors: %w", err)
		}
	}

	if err := HandleLightModeMarker(themeDir, state.LightMode); err != nil {
		return result, err
	}
	if err := writeThemePreview(themeDir, state.Palette); err != nil {
		return result, err
	}

	result.Success = true
	return result, nil
}

// GenerateOnly renders the theme into dir without touching the system.
func (w *Writer) GenerateOnly(state *ThemeState, settings Settings, dir string) error {
	if err := validateTemplateInputs(state); err != nil {
		return err
	}
	if dir == "" {
		dir = platform.ThemeDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create theme dir: %w", err)
	}
	effective, err := materializeWallpaper(state, dir)
	if err != nil {
		return err
	}
	if _, err := w.generate(state, settings, dir, effective); err != nil {
		return err
	}
	if state.IconTheme.Mode == icontheme.SelectionExplicit && state.IconTheme.ID != "" {
		if err := platform.WriteText(filepath.Join(dir, "icons.theme"), state.IconTheme.ID+"\n"); err != nil {
			return fmt.Errorf("write icons.theme: %w", err)
		}
	}
	if err := HandleLightModeMarker(dir, state.LightMode); err != nil {
		return err
	}
	return writeThemePreview(dir, state.Palette)
}

// GenerateOmarchyV4Only writes the native Omarchy theme format (colors.toml
// + colors.v4.toml) into dir, used for exports on Omarchy systems.
func (w *Writer) GenerateOmarchyV4Only(state *ThemeState, settings Settings, dir string) error {
	if err := validateTemplateInputs(state); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create theme dir: %w", err)
	}
	effective, err := materializeWallpaper(state, dir)
	if err != nil {
		return err
	}
	vars := buildVars(state, effective)
	for _, name := range []string{"colors.toml", "colors.v4.toml"} {
		content, err := template.ReadTemplate(w.templatesFS, w.root, name)
		if err != nil {
			return err
		}
		rendered := template.ProcessTemplate(content, mergeTemplateVariables(vars, state.AppOverrides["colors"]))
		if err := platform.WriteText(filepath.Join(dir, name), rendered); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	if err := HandleLightModeMarker(dir, state.LightMode); err != nil {
		return err
	}
	return writeThemePreview(dir, state.Palette)
}

// SaveAndApplyOmarchyTheme generates a named theme folder in Omarchy's theme
// directory and activates it through the Omarchy CLI.
func (w *Writer) SaveAndApplyOmarchyTheme(state *ThemeState, settings Settings, name string) (*ApplyResult, error) {
	if err := validateTemplateInputs(state); err != nil {
		return nil, err
	}
	if !ValidOmarchyThemeName(name) {
		return nil, fmt.Errorf("theme name must use lowercase letters, digits, and hyphens")
	}
	dir := filepath.Join(omarchy.UserThemesDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create theme dir: %w", err)
	}
	effective, err := materializeWallpaper(state, dir)
	if err != nil {
		return nil, err
	}
	files, err := w.generate(state, settings, dir, effective)
	if err != nil {
		return nil, err
	}
	if state.IconTheme.Mode == icontheme.SelectionExplicit && state.IconTheme.ID != "" {
		if err := platform.WriteText(filepath.Join(dir, "icons.theme"), state.IconTheme.ID+"\n"); err != nil {
			return nil, fmt.Errorf("write icons.theme: %w", err)
		}
		files = append(files, filepath.Join(dir, "icons.theme"))
	}
	if err := HandleLightModeMarker(dir, state.LightMode); err != nil {
		return nil, err
	}
	if err := writeThemePreview(dir, state.Palette); err != nil {
		return nil, err
	}

	// Activate: set the background first (blurred variant when requested),
	// then let Omarchy apply the theme with background handling skipped.
	if err := omarchy.ActivateTheme(name, effective); err != nil {
		return nil, fmt.Errorf("activate Omarchy theme: %w", err)
	}

	return &ApplyResult{Success: true, ThemePath: dir, Files: files}, nil
}

// InstallOmarchyTheme installs and activates a theme folder without
// returning a detailed result (silent URL-apply path).
func (w *Writer) InstallOmarchyTheme(state *ThemeState, settings Settings, name string) error {
	_, err := w.SaveAndApplyOmarchyTheme(state, settings, name)
	return err
}

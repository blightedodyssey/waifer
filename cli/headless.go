package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"

	"aether/internal/blueprint"
	"aether/internal/omarchy"
	"aether/internal/platform"
	"aether/internal/theme"
	"aether/internal/winsys"
)

// RunHeadless dispatches headless CLI commands that do not require a running
// GUI. They run the full Phase 5 pipeline (OS colors, Windhawk, Aether current
// state, templates, hooks) and exit.
func RunHeadless(args []string, templatesFS embed.FS) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Error: --theme, --wallpaper, or --list requires an argument")
		return 1
	}

	switch args[0] {
	case "--theme":
		return runHeadlessTheme(args[1:], templatesFS)
	case "--wallpaper":
		return runHeadlessWallpaper(args[1:])
	case "--list":
		return runHeadlessList()
	default:
		fmt.Fprintf(os.Stderr, "Unknown headless flag: %s\n", args[0])
		return 1
	}
}

func runHeadlessTheme(args []string, templatesFS embed.FS) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: aether --theme <name>")
		fmt.Fprintln(os.Stderr, "  <name> can be a saved theme folder name or a blueprint name")
		return 1
	}
	name := args[0]

	// Try blueprint first
	svc := blueprint.NewService()
	bp, err := svc.FindByName(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding theme: %v\n", err)
		return 1
	}
	if bp != nil {
		return applyHeadlessBlueprint(bp, templatesFS)
	}

	// Try saved theme folder
	themeDir := filepath.Join(platform.SavedThemesDir(), name)
	if _, err := os.Stat(themeDir); err == nil {
		return applyHeadlessThemeDir(themeDir, templatesFS)
	}

	// Try Omarchy theme
	if omarchy.IsInstalled() {
		omarchyDir := filepath.Join(omarchy.UserThemesDir(), name)
		if _, err := os.Stat(omarchyDir); err == nil {
			if err := omarchy.ActivateTheme(name, ""); err != nil {
				fmt.Fprintf(os.Stderr, "Error activating Omarchy theme: %v\n", err)
				return 1
			}
			fmt.Printf("Activated Omarchy theme: %s\n", name)
			return 0
		}
	}

	fmt.Fprintf(os.Stderr, "Error: Theme or blueprint %q not found\n", name)
	return 1
}

func applyHeadlessBlueprint(bp *blueprint.Blueprint, templatesFS embed.FS) int {
	var palette [16]string
	for i := 0; i < 16 && i < len(bp.Palette.Colors); i++ {
		palette[i] = bp.Palette.Colors[i]
	}

	roles := theme.MapColorsToRoles(palette)
	if v := bp.Palette.ExtendedColors["accent"]; v != "" {
		roles.Accent = v
	}
	if v := bp.Palette.ExtendedColors["cursor"]; v != "" {
		roles.Cursor = v
	}

	lightMode := bp.Palette.LightMode
	wallpaperPath := resolveWallpaperCLI(bp.Palette)

	writer := theme.NewWriter(templatesFS, "templates")
	iconTheme, err := bp.IconThemeSelection()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Blueprint iconTheme: %v\n", err)
		return 1
	}

	state := &theme.ThemeState{
		Palette:          palette,
		WallpaperPath:    wallpaperPath,
		WallpaperBlur:    bp.Palette.WallpaperBlur,
		LightMode:        lightMode,
		ColorRoles:       roles,
		ExtendedColors:   bp.Palette.ExtendedColors,
		NativeColors:     bp.Palette.NativeColors,
		AppOverrides:     bp.AppOverrides,
		AdditionalImages: bp.Palette.AdditionalImages,
		IconTheme:        iconTheme,
	}

	settings := theme.DefaultApplySettings()
	result, err := writer.ApplyTheme(state, settings)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error applying theme: %v\n", err)
		return 1
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}
	fmt.Printf("Applied blueprint: %s\n", bp.Name)
	return 0
}

func applyHeadlessThemeDir(themeDir string, templatesFS embed.FS) int {
	// Read colors.toml from the theme directory
	colorsPath := filepath.Join(themeDir, "colors.toml")
	vars, err := theme.ReadColorsToml(colorsPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading colors.toml: %v\n", err)
		return 1
	}

	// Build a minimal state from colors.toml
	palette := theme.PaletteFromVars(vars)
	roles := theme.MapColorsToRoles(palette)

	// Determine light mode from the theme directory
	lightMode := false
	if _, err := os.Stat(filepath.Join(themeDir, ".light-mode")); err == nil {
		lightMode = true
	}

	// Find wallpaper
	wallpaperPath := ""
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp"} {
		wp := filepath.Join(themeDir, "backgrounds", "wallpaper"+ext)
		if _, err := os.Stat(wp); err == nil {
			wallpaperPath = wp
			break
		}
	}
	// Fallback: check for any image in backgrounds dir
	if wallpaperPath == "" {
		bgDir := filepath.Join(themeDir, "backgrounds")
		if entries, err := os.ReadDir(bgDir); err == nil {
			for _, e := range entries {
				if !e.IsDir() && theme.IsImageFile(filepath.Join(bgDir, e.Name())) {
					wallpaperPath = filepath.Join(bgDir, e.Name())
					break
				}
			}
		}
	}

	writer := theme.NewWriter(templatesFS, "templates")
	state := &theme.ThemeState{
		Palette:        palette,
		WallpaperPath:  wallpaperPath,
		LightMode:      lightMode,
		ColorRoles:     roles,
		ExtendedColors: map[string]string{},
		NativeColors:   map[string]string{},
		AppOverrides:   map[string]map[string]string{},
	}

	settings := theme.DefaultApplySettings()
	result, err := writer.ApplyTheme(state, settings)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error applying theme: %v\n", err)
		return 1
	}
	for _, w := range result.Warnings {
		fmt.Fprintf(os.Stderr, "Warning: %s\n", w)
	}
	fmt.Printf("Applied theme: %s\n", filepath.Base(themeDir))
	return 0
}

func runHeadlessWallpaper(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: aether --wallpaper <path>")
		return 1
	}
	path := expandHome(args[0])
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a valid file\n", path)
		return 1
	}

	if runtime.GOOS == "windows" {
		if err := winsys.SetWallpaper(path); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting wallpaper: %v\n", err)
			return 1
		}
	} else {
		if err := omarchy.SetBackground(path); err != nil {
			fmt.Fprintf(os.Stderr, "Error setting wallpaper: %v\n", err)
			return 1
		}
	}
	fmt.Printf("Wallpaper set: %s\n", path)
	return 0
}

func runHeadlessList() int {
	var names []string

	// List saved themes
	if entries, err := os.ReadDir(platform.SavedThemesDir()); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				names = append(names, e.Name())
			}
		}
	}

	// List Omarchy themes
	if omarchy.IsInstalled() {
		if entries, err := os.ReadDir(omarchy.UserThemesDir()); err == nil {
			for _, e := range entries {
				if e.IsDir() {
					name := e.Name()
					// Deduplicate with saved themes
					found := false
					for _, n := range names {
						if n == name {
							found = true
							break
						}
					}
					if !found {
						names = append(names, name)
					}
				}
			}
		}
	}

	// List blueprints
	svc := blueprint.NewService()
	if bps, err := svc.LoadAll(); err == nil {
		for _, bp := range bps {
			found := false
			for _, n := range names {
				if n == bp.Name {
					found = true
					break
				}
			}
			if !found {
				names = append(names, bp.Name)
			}
		}
	}

	sort.Strings(names)
	for _, name := range names {
		fmt.Println(name)
	}
	return 0
}

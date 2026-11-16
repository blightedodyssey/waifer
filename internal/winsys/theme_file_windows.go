//go:build windows

package winsys

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// WriteThemeFile builds the .theme INI content from the active palette, writes
// Aether.theme to the Windows Themes directory, and registers it as the active
// theme so Personalization > Themes displays "Aether — <Theme Name>" and shows
// an Aether theme card in the native Windows theme grid.
func WriteThemeFile(vars map[string]string, wallpaperPath string, lightMode bool) error {
	content := BuildThemeContent(vars, wallpaperPath, lightMode)

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve user home for LOCALAPPDATA fallback: %w", err)
		}
		localAppData = filepath.Join(home, "AppData", "Local")
	}

	themePath := filepath.Join(localAppData, "Microsoft", "Windows", "Themes", "Aether.theme")
	if err := os.MkdirAll(filepath.Dir(themePath), 0o755); err != nil {
		return fmt.Errorf("create Themes directory: %w", err)
	}
	if err := os.WriteFile(themePath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write Aether.theme: %w", err)
	}

	// Register as the active theme in the registry so Windows picks it up.
	k, _, err := registry.CreateKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Themes`,
		registry.SET_VALUE,
	)
	if err != nil {
		return fmt.Errorf("open Themes registry: %w", err)
	}
	defer k.Close()
	if err := k.SetStringValue("CurrentTheme", themePath); err != nil {
		return fmt.Errorf("set CurrentTheme: %w", err)
	}

	// Broadcast WM_SETTINGCHANGE so the Personalization panel and Explorer
	// reload the theme list and display the Aether theme card.
	notifyThemeChange()

	return nil
}

// BuildThemeContent generates a valid Windows .theme INI file string without
// side effects. Exported for isolated testing.
func BuildThemeContent(vars map[string]string, wallpaperPath string, lightMode bool) string {
	if vars == nil {
		vars = map[string]string{}
	}

	var sb strings.Builder

	// Theme display name — "Aether - <name>" or "Aether - Live Palette".
	themeName := vars["name"]
	if themeName == "" {
		themeName = "Live Palette"
	}

	// [Theme]
	sb.WriteString("[Theme]\r\n")
	sb.WriteString(fmt.Sprintf("DisplayName=Aether - %s\r\n", themeName))
	sb.WriteString("\r\n")

	// [Control Panel\Desktop]
	sb.WriteString("[Control Panel\\Desktop]\r\n")
	if wallpaperPath != "" {
		sb.WriteString(fmt.Sprintf("Wallpaper=%s\r\n", wallpaperPath))
	}
	sb.WriteString("TileWallpaper=0\r\n")
	sb.WriteString("WallpaperStyle=10\r\n")
	sb.WriteString("\r\n")

	// [VisualStyles]
	sb.WriteString("[VisualStyles]\r\n")
	sb.WriteString("Path=%SystemRoot%\\resources\\Themes\\Aero\\Aero.msstyles\r\n")
	sb.WriteString("ColorStyle=NormalColor\r\n")
	sb.WriteString("Size=NormalSize\r\n")

	// ColorizationColor uses 0XFF<RRGGBB> (uppercase X, FF alpha, RRGGBB).
	accent := normalizeHex(vars["accent"])
	if accent != "" {
		hex := strings.ToUpper(strings.TrimPrefix(accent, "#"))
		sb.WriteString(fmt.Sprintf("ColorizationColor=0XFF%s\r\n", hex))
	}

	mode := "Dark"
	if lightMode {
		mode = "Light"
	}
	sb.WriteString(fmt.Sprintf("SystemMode=%s\r\n", mode))
	sb.WriteString(fmt.Sprintf("AppMode=%s\r\n", mode))

	return sb.String()
}

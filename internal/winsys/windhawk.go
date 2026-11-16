//go:build windows

package winsys

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

const (
	windhawkModsKey     = `SOFTWARE\Windhawk\Engine\Mods`
	windhawkSettingsSub = "Settings"
)

// WindhawkMod names for the Windows 11 shell stylers.
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

// Windhawk color-aware keys — we only touch values whose names contain these
// substrings.  Layout, geometry, sizing, margin, padding, radius, visibility,
// and opacity are never modified.
var windhawkColorKeyPatterns = []string{
	"background", "foreground", "borderbrush", "fill", "stroke",
	"bordercolor", "color", "brush", "themeresourcevariables",
	".styles[", // any style rule might contain a color
}

// IsWindhawkModInstalled checks whether a given Windhawk mod is present in the
// registry. It only verifies the key exists; it does not require the mod to be
// enabled.
func IsWindhawkModInstalled(modName string) bool {
	keyPath := filepath.Join(windhawkModsKey, modName)
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	k.Close()
	return true
}

// ApplyWindhawkTheme updates all installed Windhawk styler mods with colors
// from the active Aether palette. Non-color rules (layout, geometry, etc.) are
// preserved. If a mod is not installed or the registry is not writable, a
// warning is returned and the error is non-fatal.
func ApplyWindhawkTheme(vars map[string]string, lightMode bool) ([]string, []string) {
	var applied []string
	var warnings []string

	bg := normalizeHex(vars["background"])
	fg := normalizeHex(vars["foreground"])
	accent := normalizeHex(vars["accent"])
	if bg == "" {
		bg = "#1e1e2e"
	}
	if fg == "" {
		fg = "#cdd6f4"
	}
	if accent == "" {
		accent = "#cba6f7"
	}

	surface := blendHex(bg, lightModeFg(lightMode), 0.08)
	if lightMode {
		surface = blendHex(bg, "#000000", 0.05)
	}
	border := blendHex(accent, bg, 0.6)
	mutedFg := blendHex(fg, bg, 0.35)

	for _, mod := range knownStylerMods {
		if !IsWindhawkModInstalled(mod) {
			continue
		}
		if err := applyWindhawkModColors(mod, bg, fg, accent, surface, border, mutedFg, lightMode); err != nil {
			warnings = append(warnings, fmt.Sprintf("windhawk %s: %v", mod, err))
			continue
		}
		applied = append(applied, mod)
	}

	return applied, warnings
}

func lightModeFg(lightMode bool) string {
	if lightMode {
		return "#ffffff"
	}
	return "#000000"
}

func applyWindhawkModColors(modName, bg, fg, accent, surface, border, mutedFg string, lightMode bool) error {
	// Read existing live config.
	cfg, err := readWindhawkModConfig(modName)
	if err != nil {
		return err
	}

	// Inject Aether palette colors into the existing keys.
	injectAetherColors(cfg, bg, fg, accent, surface, border, mutedFg, lightMode)

	// Rewrite the settings back to the registry.
	if err := writeWindhawkModConfig(modName, cfg); err != nil {
		return err
	}
	return nil
}

// windhawkModConfig mirrors the flat string key-value map that Windhawk stores
// under its Settings registry key.
type windhawkModConfig struct {
	modName  string
	settings map[string]string // all REG_SZ values (empty strings preserved)
	dwords   map[string]uint32 // all REG_DWORD values (never touched)
}

// readWindhawkModConfig reads all string/dword values from a mod's Settings key.
func readWindhawkModConfig(modName string) (*windhawkModConfig, error) {
	keyPath := filepath.Join(windhawkModsKey, modName, windhawkSettingsSub)
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("open settings: %w", err)
	}
	defer k.Close()

	cfg := &windhawkModConfig{
		modName:  modName,
		settings: make(map[string]string),
		dwords:   make(map[string]uint32),
	}

	vals, err := k.ReadValueNames(-1)
	if err != nil {
		return nil, fmt.Errorf("enumerate values: %w", err)
	}

	for _, name := range vals {
		if s, _, err := k.GetStringValue(name); err == nil {
			cfg.settings[name] = s
			continue
		}
		if d, _, err := k.GetIntegerValue(name); err == nil {
			cfg.dwords[name] = uint32(d)
		}
	}

	return cfg, nil
}

// writeWindhawkModConfig attempts to write merged settings back via direct
// registry write.  If HKLM is not writable (ERROR_ACCESS_DENIED), it falls
// back to a temporary .reg file applied via ShellExecuteW with "runas" so
// Windows prompts once for elevation.
func writeWindhawkModConfig(modName string, cfg *windhawkModConfig) error {
	keyPath := filepath.Join(windhawkModsKey, modName, windhawkSettingsSub)

	// Try direct write first.
	err := writeWindhawkModConfigDirect(keyPath, cfg)
	if err == nil {
		return nil
	}

	// If access denied, fall back to elevated .reg import.
	if isAccessDenied(err) {
		return writeWindhawkModConfigElevated(modName, cfg)
	}
	return err
}

func writeWindhawkModConfigDirect(keyPath string, cfg *windhawkModConfig) error {
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, keyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	for name, value := range cfg.settings {
		if err := k.SetStringValue(name, value); err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
	}
	for name, value := range cfg.dwords {
		if err := k.SetDWordValue(name, value); err != nil {
			return fmt.Errorf("set dword %s: %w", name, err)
		}
	}
	return nil
}

func isAccessDenied(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Access is denied") ||
		strings.Contains(err.Error(), "ERROR_ACCESS_DENIED") ||
		strings.Contains(err.Error(), "0x5")
}

// writeWindhawkModConfigElevated generates a temporary .reg file and applies it
// via ShellExecuteW with the "runas" verb so Windows shows the UAC prompt once.
func writeWindhawkModConfigElevated(modName string, cfg *windhawkModConfig) error {
	tmpDir := os.TempDir()
	regPath := filepath.Join(tmpDir, fmt.Sprintf("aether-windhawk-%s.reg", modName))

	var b strings.Builder
	b.WriteString("Windows Registry Editor Version 5.00\r\n\r\n")
	b.WriteString(fmt.Sprintf("[HKEY_LOCAL_MACHINE\\SOFTWARE\\Windhawk\\Engine\\Mods\\%s\\Settings]\r\n", modName))

	for name, value := range cfg.settings {
		b.WriteString(fmt.Sprintf("%s=%s\r\n", quoteRegKey(name), quoteRegString(value)))
	}
	for name, value := range cfg.dwords {
		b.WriteString(fmt.Sprintf("%s=dword:%08x\r\n", quoteRegKey(name), value))
	}

	if err := os.WriteFile(regPath, []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("write .reg file: %w", err)
	}
	defer os.Remove(regPath)

	// Use ShellExecuteW with "runas" to prompt for elevation.
	shell32 := syscall.NewLazyDLL("shell32.dll")
	shellExecuteW := shell32.NewProc("ShellExecuteW")

	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString("reg.exe")
	params, _ := syscall.UTF16PtrFromString(fmt.Sprintf("import \"%s\"", regPath))

	r1, _, _ := shellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(params)),
		0,
		syscall.SW_HIDE,
	)
	// ShellExecuteW returns > 32 on success, or an error code <= 32 on failure.
	if r1 <= 32 {
		// Fall through to just running reg.exe directly as a subprocess.
		cmd := exec.Command("reg.exe", "import", regPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("elevated reg import failed (code %d): %w | %s", r1, err, string(out))
		}
	}
	return nil
}

func quoteRegKey(s string) string {
	// Keys with brackets need quoting in .reg files.
	if strings.ContainsAny(s, `[] `) {
		return `"` + s + `"`
	}
	return `"` + s + `"`
}

func quoteRegString(s string) string {
	if s == "" {
		return `""`
	}
	// Escape backslashes and quotes for .reg file format.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// injectAetherColors walks the existing config and replaces any color hex
// values in controlStyles / themeResourceVariables with the Aether palette.
// Empty keys are preserved.  Layout/sizing/margin keys are untouched.
func injectAetherColors(cfg *windhawkModConfig, bg, fg, accent, surface, border, mutedFg string, lightMode bool) {
	// First, build a map of existing controlStyle indices so we know
	// which targets exist and which are empty.
	type styleEntry struct {
		target string
		styles map[int]string
	}
	entries := make(map[int]*styleEntry)
	for k, v := range cfg.settings {
		if !strings.HasPrefix(k, "controlStyles[") {
			continue
		}
		// Parse controlStyles[N].target or controlStyles[N].styles[M]
		idx := parseControlStyleIndex(k)
		if idx < 0 {
			continue
		}
		if _, ok := entries[idx]; !ok {
			entries[idx] = &styleEntry{styles: make(map[int]string)}
		}
		if strings.HasSuffix(k, ".target") {
			entries[idx].target = v
		} else if strings.Contains(k, ".styles[") {
			sIdx := parseStyleIndex(k)
			if sIdx >= 0 {
				entries[idx].styles[sIdx] = v
			}
		}
	}

	// Replace color values in existing style entries.
	for k := range cfg.settings {
		if !isColorAwareKey(k) {
			continue
		}
		old := cfg.settings[k]
		if old == "" {
			continue // preserve empties
		}
		newVal := replaceColorInValue(old, bg, fg, accent, surface, border, mutedFg)
		if newVal != old {
			cfg.settings[k] = newVal
		}
	}

	// Inject themeResourceVariables.
	injectThemeResourceVariables(cfg, bg, fg, surface)

	// Set theme to "Aether" so Windhawk uses our injected custom colors.
	cfg.settings["theme"] = "Aether"
}

// isColorAwareKey returns true for keys that may contain color values.
func isColorAwareKey(name string) bool {
	lower := strings.ToLower(name)
	for _, pat := range windhawkColorKeyPatterns {
		if strings.Contains(lower, pat) {
			return true
		}
	}
	return false
}

// replaceColorInValue detects and replaces hex color values in a control style
// string (e.g. "Background=#1e1e2e" → "Background=#ff0000") while keeping the
// property name, semantics, and non-color parameters intact.
func replaceColorInValue(val, bg, fg, accent, surface, border, mutedFg string) string {
	parts := strings.SplitN(val, "=", 2)
	if len(parts) != 2 {
		return val
	}
	prop := strings.TrimSpace(parts[0])
	rhs := strings.TrimSpace(parts[1])

	var replacement string
	switch strings.ToLower(prop) {
	case "background":
		replacement = bg
	case "foreground":
		replacement = fg
	case "fill":
		replacement = surface
	case "stroke":
		replacement = border
	case "borderbrush", "bordercolor":
		replacement = border
	case "opacity":
		return val // don't touch opacity values
	case "visibility":
		return val // don't touch visibility
	default:
		// If the RHS contains a hex color, replace it based on where it appears.
		if hasHexColor(rhs) {
			replacement = smartColorReplacement(rhs, bg, fg, accent, surface, border, mutedFg)
		} else {
			return val
		}
	}

	if replacement == "" {
		return val
	}
	return fmt.Sprintf("%s=%s", prop, replacement)
}

// smartColorReplacement decides which palette color to use based on the
// semantic context of the old color value.
func smartColorReplacement(oldVal, bg, fg, accent, surface, border, mutedFg string) string {
	lower := strings.ToLower(oldVal)
	// Background-related → bg
	if strings.Contains(lower, "background") || strings.Contains(lower, "bg") {
		return bg
	}
	// Foreground/text-related → fg
	if strings.Contains(lower, "foreground") || strings.Contains(lower, "text") {
		return fg
	}
	// Accent/brand color → accent
	if strings.Contains(lower, "accent") || strings.Contains(lower, "brand") {
		return accent
	}
	// Border/edge → border
	if strings.Contains(lower, "border") || strings.Contains(lower, "edge") {
		return border
	}
	// Fill → surface
	if strings.Contains(lower, "fill") {
		return surface
	}
	// Menu-related → bg
	if strings.Contains(lower, "menu") {
		return bg
	}
	// Default: for any other color-bearing property, replace with surface.
	return surface
}

func hasHexColor(s string) bool {
	return strings.Contains(s, "#")
}

// injectThemeResourceVariables writes/overwrites the common WinUI brush
// overrides that Windhawk stylers expose.
func injectThemeResourceVariables(cfg *windhawkModConfig, bg, fg, surface string) {
	vars := map[string]string{
		"SystemControlTransientBackgroundBrush":    bg,
		"SystemControlAcrylicWindowBrush":          bg,
		"SystemControlBackgroundBaseLowBrush":      bg,
		"SystemControlHighlightBaseLowBrush":       surface,
		"SystemControlForegroundBaseHighBrush":     fg,
		"SystemControlHighlightListAccentLowBrush": surface,
	}

	idx := 0
	for name, color := range vars {
		key := fmt.Sprintf("themeResourceVariables[%d]", idx)
		cfg.settings[key] = fmt.Sprintf("%s=%s", name, color)
		idx++
	}
	// Remove any excess old themeResourceVariables entries.
	for {
		key := fmt.Sprintf("themeResourceVariables[%d]", idx)
		if _, ok := cfg.settings[key]; !ok {
			break
		}
		delete(cfg.settings, key)
		idx++
	}
}

// parseControlStyleIndex extracts the integer index from "controlStyles[N]...".
func parseControlStyleIndex(key string) int {
	start := strings.Index(key, "[")
	end := strings.Index(key, "]")
	if start < 0 || end < 0 || end <= start+1 {
		return -1
	}
	var idx int
	if _, err := fmt.Sscanf(key[start+1:end], "%d", &idx); err != nil {
		return -1
	}
	return idx
}

// parseStyleIndex extracts the integer style index from "...styles[M]".
func parseStyleIndex(key string) int {
	if !strings.Contains(key, ".styles[") {
		return -1
	}
	start := strings.LastIndex(key, "[")
	end := strings.LastIndex(key, "]")
	if start < 0 || end < 0 || end <= start+1 {
		return -1
	}
	var idx int
	if _, err := fmt.Sscanf(key[start+1:end], "%d", &idx); err != nil {
		return -1
	}
	return idx
}

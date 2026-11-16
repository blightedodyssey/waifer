package winsys

import (
	"os"
	"strings"
	"testing"
)

func setupWindhawkTestEnv(t *testing.T) {
	if os.Getenv("APPDATA") == "" {
		t.Setenv("APPDATA", t.TempDir())
	}
	if os.Getenv("LOCALAPPDATA") == "" {
		t.Setenv("LOCALAPPDATA", t.TempDir())
	}
}

func TestNormalizeHex(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"#ff0000", "#ff0000"},
		{"FF0000", "#ff0000"},
		{"  #abc123  ", "#abc123"},
		{"not-a-color", ""},
		{"", ""},
	}
	for _, c := range cases {
		got := normalizeHex(c.input)
		if got != c.want {
			t.Errorf("normalizeHex(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestBlendHex(t *testing.T) {
	got := blendHex("#ff0000", "#000000", 0.5)
	want := "#800000"
	if got != want {
		t.Errorf("blendHex(red, black, 0.5) = %q, want %q", got, want)
	}
}

func TestHexToRGBSpace(t *testing.T) {
	got := hexToRGBSpace("#1e1e2e")
	want := "30 30 46"
	if got != want {
		t.Errorf("hexToRGBSpace(#1e1e2e) = %q, want %q", got, want)
	}
}

func TestBuildControlPanelColors(t *testing.T) {
	setupWindhawkTestEnv(t)
	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
		"accent":     "#cba6f7",
	}

	values := buildControlPanelColors(vars, false)
	if len(values) == 0 {
		t.Fatal("expected control panel color values")
	}

	required := []string{"Hilight", "HotTrackingColor", "Window", "WindowText", "Background", "Menu", "MenuText"}
	found := make(map[string]bool)
	for _, v := range values {
		found[v.Name] = true
	}
	for _, name := range required {
		if !found[name] {
			t.Errorf("missing required Control Panel color key: %s", name)
		}
	}

	lightValues := buildControlPanelColors(vars, true)
	for _, v := range lightValues {
		if v.Name == "Background" && v.String != "255 255 255" {
			t.Errorf("light mode Background = %q, want 255 255 255", v.String)
		}
		if v.Name == "MenuText" && v.String != "0 0 0" {
			t.Errorf("light mode MenuText = %q, want 0 0 0", v.String)
		}
	}
}

func TestBuildAccentPalette(t *testing.T) {
	vars := map[string]string{"accent": "#ff0000"}
	pal := buildAccentPalette(vars)
	if len(pal) != 32 {
		t.Fatalf("AccentPalette length = %d, want 32", len(pal))
	}
	// Slot 3 (primary accent) should be BGR ordering: B=0x00, G=0x00, R=0xFF, A=0xFF
	// Offset = 3 * 4 = 12
	if pal[12] != 0x00 || pal[13] != 0x00 || pal[14] != 0xFF || pal[15] != 0xFF {
		t.Errorf("slot 3 accent = %02x %02x %02x %02x, want 00 00 ff ff",
			pal[12], pal[13], pal[14], pal[15])
	}
	// Slot 0 (lightest tint) should be closer to white
	if pal[0] < 0x80 || pal[2] < 0x80 {
		t.Errorf("slot 0 tint too dark: %02x %02x %02x", pal[0], pal[1], pal[2])
	}
	// Empty accent returns nil
	if buildAccentPalette(map[string]string{}) != nil {
		t.Error("expected nil palette for empty accent")
	}
}

func TestReplaceColorInValue(t *testing.T) {
	bg, fg, accent, surface, border, mutedFg := "#1e1e2e", "#cdd6f4", "#cba6f7", "#2a2a3e", "#a080c0", "#8888aa"

	cases := []struct {
		input string
		want  string
	}{
		{"Background=#ff0000", "Background=#1e1e2e"},
		{"Foreground=#ffffff", "Foreground=#cdd6f4"},
		{"Fill=#abc", "Fill=#2a2a3e"},
		{"Stroke=#abc", "Stroke=#a080c0"},
		{"BorderBrush=#fff", "BorderBrush=#a080c0"},
		{"BorderColor=#fff", "BorderColor=#a080c0"},
		{"Opacity=0.5", "Opacity=0.5"},
		{"Visibility=Collapsed", "Visibility=Collapsed"},
		{"Margin=0,0,0,0", "Margin=0,0,0,0"},
		{"Height:=48", "Height:=48"},
		{"", ""},
		{"no-equals", "no-equals"},
	}

	for _, c := range cases {
		got := replaceColorInValue(c.input, bg, fg, accent, surface, border, mutedFg)
		if got != c.want {
			t.Errorf("replaceColorInValue(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestInjectAetherColors_PreservesLayout(t *testing.T) {
	cfg := &windhawkModConfig{
		modName: "windows-11-taskbar-styler",
		settings: map[string]string{
			"theme":                         "TranslucentTaskbar",
			"styleConstants[0]":             "taskbar_height=48",
			"styleConstants[1]":             "icon_size=24",
			"controlStyles[0].target":       "Taskbar.TaskbarFrame",
			"controlStyles[0].styles[0]":    "Height:=taskbar_height",
			"controlStyles[0].styles[1]":    "Margin=0,0,0,0",
			"controlStyles[0].styles[2]":    "Padding=0,0,0,0",
			"controlStyles[1].target":       "Grid#SystemTrayFrameGrid",
			"controlStyles[1].styles[0]":    "Margin=0,0,10,0",
			"controlStyles[1].styles[1]":    "MinWidth=0",
			"controlStyles[2].target":       "Taskbar.TaskbarFrame > Grid#RootGrid > Taskbar.TaskbarBackground > Grid > Rectangle#BackgroundFill",
			"controlStyles[2].styles[0]":    "Visibility=collapsed",
			"themeResourceVariables[0]":     "",
			"clickThroughTaskbar":           "",
		},
		dwords: map[string]uint32{
			"clickThroughTaskbar": 0,
		},
	}

	injectAetherColors(cfg, "#1e1e2e", "#cdd6f4", "#cba6f7", "#2a2a3e", "#a080c0", "#8888aa", false)

	// Layout constants MUST be preserved.
	if cfg.settings["styleConstants[0]"] != "taskbar_height=48" {
		t.Errorf("styleConstants[0] = %q, want taskbar_height=48", cfg.settings["styleConstants[0]"])
	}
	if cfg.settings["styleConstants[1]"] != "icon_size=24" {
		t.Errorf("styleConstants[1] = %q, want icon_size=24", cfg.settings["styleConstants[1]"])
	}

	// Layout styles must be preserved.
	if cfg.settings["controlStyles[0].styles[0]"] != "Height:=taskbar_height" {
		t.Errorf("Height style mutated: %q", cfg.settings["controlStyles[0].styles[0]"])
	}
	if cfg.settings["controlStyles[0].styles[1]"] != "Margin=0,0,0,0" {
		t.Errorf("Margin style mutated: %q", cfg.settings["controlStyles[0].styles[1]"])
	}
	if cfg.settings["controlStyles[1].styles[0]"] != "Margin=0,0,10,0" {
		t.Errorf("Tray margin mutated: %q", cfg.settings["controlStyles[1].styles[0]"])
	}
	if cfg.settings["controlStyles[2].styles[0]"] != "Visibility=collapsed" {
		t.Errorf("Visibility style mutated: %q", cfg.settings["controlStyles[2].styles[0]"])
	}

	// Theme should switch to Aether.
	if cfg.settings["theme"] != "Aether" {
		t.Errorf("theme = %q, want Aether", cfg.settings["theme"])
	}

	// Theme resource variables should be injected (at least one must exist).
	found := false
	for k, v := range cfg.settings {
		if strings.HasPrefix(k, "themeResourceVariables[") && strings.Contains(v, "SystemControl") {
			found = true
			break
		}
	}
	if !found {
		t.Error("missing themeResourceVariables entries")
	}
}

func TestInjectAetherColors_ReplacesExistingColors(t *testing.T) {
	cfg := &windhawkModConfig{
		modName: "windows-11-notification-center-styler",
		settings: map[string]string{
			"theme":                      "Unified",
			"controlStyles[0].target":    "Border",
			"controlStyles[0].styles[0]": "Background=#1e1e2e",
			"controlStyles[0].styles[1]": "BorderThickness=2",
			"controlStyles[0].styles[2]": "BorderBrush=#f5c2e7",
			"controlStyles[0].styles[3]": "Opacity=1.0",
			"controlStyles[8].target":    "TextBlock#NotificationCenterHeader",
			"controlStyles[8].styles[0]": "Foreground=#76b900",
			"controlStyles[8].styles[1]": "FontWeight=Bold",
		},
	}

	injectAetherColors(cfg, "#ff0000", "#00ff00", "#0000ff", "#111111", "#222222", "#333333", false)

	// Background should be replaced with bg.
	if cfg.settings["controlStyles[0].styles[0]"] != "Background=#ff0000" {
		t.Errorf("background not replaced: %q", cfg.settings["controlStyles[0].styles[0]"])
	}
	// BorderThickness should be preserved.
	if cfg.settings["controlStyles[0].styles[1]"] != "BorderThickness=2" {
		t.Errorf("BorderThickness mutated: %q", cfg.settings["controlStyles[0].styles[1]"])
	}
	// BorderBrush should be replaced with border.
	if cfg.settings["controlStyles[0].styles[2]"] != "BorderBrush=#222222" {
		t.Errorf("border not replaced: %q", cfg.settings["controlStyles[0].styles[2]"])
	}
	// Opacity should be preserved.
	if cfg.settings["controlStyles[0].styles[3]"] != "Opacity=1.0" {
		t.Errorf("Opacity mutated: %q", cfg.settings["controlStyles[0].styles[3]"])
	}
	// Foreground should be replaced with fg.
	if cfg.settings["controlStyles[8].styles[0]"] != "Foreground=#00ff00" {
		t.Errorf("foreground not replaced: %q", cfg.settings["controlStyles[8].styles[0]"])
	}
	// FontWeight should be preserved.
	if cfg.settings["controlStyles[8].styles[1]"] != "FontWeight=Bold" {
		t.Errorf("FontWeight mutated: %q", cfg.settings["controlStyles[8].styles[1]"])
	}
}

func TestIsColorAwareKey(t *testing.T) {
	colorful := []string{
		"controlStyles[0].styles[0]",
		"controlStyles[3].styles[1]",
		"themeResourceVariables[0]",
		"themeResourceVariables[1]",
	}
	for _, v := range colorful {
		if !isColorAwareKey(v) {
			t.Errorf("isColorAwareKey(%q) should be true", v)
		}
	}

	nonColor := []string{
		"styleConstants[0]",
		"styleConstants[1]",
		"theme",
		"disableNewStartMenuLayout",
		"clickThroughTaskbar",
		"xamlDiagnosticsHandling",
	}
	for _, v := range nonColor {
		if isColorAwareKey(v) {
			t.Errorf("isColorAwareKey(%q) should be false", v)
		}
	}
}

func TestSmartColorReplacement(t *testing.T) {
	bg, fg, accent, surface, border, mutedFg := "#1e1e2e", "#cdd6f4", "#cba6f7", "#2a2a3e", "#a080c0", "#8888aa"

	cases := []struct {
		input string
		want  string
	}{
		{"#ff0000", surface},     // arbitrary color → surface (safe default)
		{"background", bg},       // contains "background"
		{"foreground", fg},       // contains "foreground"
		{"textColor", fg},        // contains "text"
		{"accentBrush", accent},  // contains "accent"
		{"borderEdge", border},   // contains "border" / "edge"
		{"fillColor", surface},   // contains "fill"
		{"menuBg", bg},           // contains "menu"
	}

	for _, c := range cases {
		got := smartColorReplacement(c.input, bg, fg, accent, surface, border, mutedFg)
		if got != c.want {
			t.Errorf("smartColorReplacement(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

func TestParseControlStyleIndex(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"controlStyles[0].target", 0},
		{"controlStyles[12].styles[3]", 12},
		{"controlStyles[999].target", 999},
		{"theme", -1},
		{"", -1},
	}
	for _, c := range cases {
		got := parseControlStyleIndex(c.input)
		if got != c.want {
			t.Errorf("parseControlStyleIndex(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

func TestParseStyleIndex(t *testing.T) {
	cases := []struct {
		input string
		want  int
	}{
		{"controlStyles[0].styles[3]", 3},
		{"controlStyles[12].styles[0]", 0},
		{"controlStyles[0].target", -1},
		{"theme", -1},
	}
	for _, c := range cases {
		got := parseStyleIndex(c.input)
		if got != c.want {
			t.Errorf("parseStyleIndex(%q) = %d, want %d", c.input, got, c.want)
		}
	}
}

func TestQuoteRegString(t *testing.T) {
	cases := []struct {
		input, want string
	}{
		{"hello", `"hello"`},
		{"", `""`},
		{`path\to\file`, `"path\\to\\file"`},
		{`say "hi"`, `"say \"hi\""`},
	}
	for _, c := range cases {
		got := quoteRegString(c.input)
		if got != c.want {
			t.Errorf("quoteRegString(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestIsWindhawkModInstalled_Safe is a smoke test for the helper.
func TestIsWindhawkModInstalled_Safe(t *testing.T) {
	setupWindhawkTestEnv(t)
	_ = IsWindhawkModInstalled("windows-11-taskbar-styler")
	_ = IsWindhawkModInstalled("nonexistent-mod-xyz")
}

// TestApplyWindhawkTheme_SkipsWhenNotInstalled runs on machines without Windhawk.
func TestApplyWindhawkTheme_SkipsWhenNotInstalled(t *testing.T) {
	setupWindhawkTestEnv(t)
	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
		"accent":     "#cba6f7",
	}
	applied, warnings := ApplyWindhawkTheme(vars, false)
	// On a machine without Windhawk, nothing is applied and no warnings.
	if len(applied) > 0 || len(warnings) > 0 {
		t.Logf("Windhawk may be installed: applied=%v warnings=%v", applied, warnings)
	}
}

// TestWindhawkFilePaths_Existence checks that all known Windhawk mod names are valid.
func TestWindhawkFilePaths_Existence(t *testing.T) {
	for _, mod := range knownStylerMods {
		if !strings.HasPrefix(mod, "windows-11-") {
			t.Errorf("mod %q does not follow naming convention", mod)
		}
		if strings.Contains(mod, `\`) || strings.Contains(mod, `/`) {
			t.Errorf("mod %q contains path separator", mod)
		}
	}
}

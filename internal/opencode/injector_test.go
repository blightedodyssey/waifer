package opencode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testVars(mode, bg, fg string) map[string]string {
	return map[string]string{
		"mode":       mode,
		"background": bg,
		"foreground": fg,
		"darker_bg":  "#080808",
		"dark_bg":    "#0c0c0c",
		"lighter_bg": "#1a1a1a",
		"muted":      "#777777",
		"light_fg":   "#cccccc",
		"bright_fg":  "#ffffff",
		"accent":     "#7aa2f7",
		"red":        "#f7768e",
		"green":      "#9ece6a",
		"yellow":     "#e0af68",
		"blue":       "#7aa2f7",
		"magenta":    "#bb9af7",
		"cyan":       "#7dcfff",
		"orange":     "#ff9e64",
		"selection":  "#334166",
	}
}

func parseTheme(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var theme map[string]any
	if err := json.Unmarshal(data, &theme); err != nil {
		t.Fatalf("invalid OpenCode V2 JSON: %v", err)
	}
	return theme
}

func TestBuildThemeV2DarkAndLight(t *testing.T) {
	for _, tc := range []struct {
		name string
		vars map[string]string
	}{
		{"dark", testVars("dark", "#111111", "#eeeeee")},
		{"light", testVars("light", "#ffffff", "#111111")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := BuildTheme(tc.vars)
			if err != nil {
				t.Fatalf("BuildTheme: %v", err)
			}
			if strings.Contains(string(data), "{{") {
				t.Fatal("generated theme still contains placeholders")
			}
			theme := parseTheme(t, data)
			if theme["$schema"] != TemplateSchema {
				t.Errorf("$schema = %v, want %s", theme["$schema"], TemplateSchema)
			}
			for _, group := range []string{"text", "background", "border", "scrollbar", "diff", "syntax", "markdown", "@dialog"} {
				if theme["base"].(map[string]any)[group] == nil {
					t.Errorf("base.%s is missing", group)
				}
			}
			mode, ok := theme[tc.name].(map[string]any)
			if !ok {
				t.Fatalf("%s section is missing", tc.name)
			}
			hues := mode["hue"].(map[string]any)
			for _, hue := range append([]string{"accent", "interactive", "neutral"}, baseHues...) {
				entry, ok := hues[hue]
				if !ok {
					t.Errorf("hue %s is missing", hue)
					continue
				}
				if ref, ok := entry.(string); ok {
					if !strings.HasPrefix(ref, "$hue.") {
						t.Errorf("hue %s alias = %q", hue, ref)
					}
					continue
				}
				ramp, ok := entry.(map[string]any)
				if !ok {
					t.Fatalf("hue %s has unexpected type %T", hue, entry)
				}
				for _, step := range hueSteps {
					color, ok := ramp[step].(string)
					if !ok || !isHex(color) {
						t.Errorf("hue %s step %s = %v, want hex color", hue, step, ramp[step])
					}
				}
				if ramp["500"] != tc.vars[map[string]string{"red": "red", "green": "green", "yellow": "yellow", "blue": "blue", "purple": "magenta", "cyan": "cyan", "orange": "orange"}[hue]] {
					// Only base syntax hues are checked here; accent/gray have separate anchors.
					if _, isSyntaxHue := map[string]bool{"red": true, "green": true, "yellow": true, "blue": true, "purple": true, "cyan": true, "orange": true}[hue]; isSyntaxHue {
						t.Errorf("hue %s step 500 = %v, want Aether palette color", hue, ramp["500"])
					}
				}
			}
			if mode["text"].(map[string]any)["base"] != tc.vars["foreground"] {
				t.Errorf("text.base = %v, want %s", mode["text"].(map[string]any)["base"], tc.vars["foreground"])
			}
			if mode["background"].(map[string]any)["base"] != tc.vars["background"] {
				t.Errorf("background.base = %v, want %s", mode["background"].(map[string]any)["base"], tc.vars["background"])
			}
			if mode["syntax"].(map[string]any)["keyword"] != tc.vars["magenta"] {
				t.Errorf("syntax.keyword = %v, want magenta", mode["syntax"].(map[string]any)["keyword"])
			}
		})
	}
}

func TestInjectorMergesCLIWithoutLosingSettings(t *testing.T) {
	dir := t.TempDir()
	cliPath := filepath.Join(dir, "cli.json")
	existing := `{
  "$schema": "https://opencode.ai/v2/cli.json",
  "theme": {"name": "tokyonight", "mode": "system", "font": "mono"},
  "mcpServers": {"github": {"command": "npx", "args": ["-y", "server"]}},
  "model": {"providerID": "anthropic", "modelID": "claude"}
}`
	if err := os.WriteFile(cliPath, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	injector := NewInjector(WithConfigDir(dir), WithDebounce(0), WithCleanupDelay(time.Hour))
	first, err := injector.Apply(testVars("dark", "#111111", "#eeeeee"))
	if err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	second, err := injector.Apply(testVars("light", "#ffffff", "#111111"))
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if first.Name == second.Name {
		t.Fatalf("slot did not rotate: %s -> %s", first.Name, second.Name)
	}

	data, err := os.ReadFile(cliPath)
	if err != nil {
		t.Fatal(err)
	}
	var cli map[string]any
	if err := json.Unmarshal(data, &cli); err != nil {
		t.Fatalf("cli.json is invalid: %v", err)
	}
	theme := cli["theme"].(map[string]any)
	if theme["name"] != second.Name || theme["mode"] != "light" {
		t.Errorf("theme = %+v, want %s/light", theme, second.Name)
	}
	if theme["font"] != "mono" {
		t.Error("existing theme settings were lost")
	}
	if cli["mcpServers"] == nil || cli["model"] == nil {
		t.Error("existing user settings were lost")
	}
	if cli["$schema"] != "https://opencode.ai/v2/cli.json" {
		t.Errorf("existing $schema was overwritten: %v", cli["$schema"])
	}
	if _, err := os.Stat(cliPath + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("temp file was left behind: %v", err)
	}
}

func TestInjectorRotatesSlotsAndCleansUpStaleFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "opencode")
	injector := NewInjector(WithConfigDir(dir), WithDebounce(0), WithCleanupDelay(10*time.Millisecond))
	config := testVars("dark", "#111111", "#eeeeee")

	first, err := injector.Apply(config)
	if err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	if first.Name != SlotA {
		t.Fatalf("first slot = %s, want %s", first.Name, SlotA)
	}
	second, err := injector.Apply(config)
	if err != nil {
		t.Fatalf("second Apply: %v", err)
	}
	if second.Name != SlotB {
		t.Fatalf("second slot = %s, want %s", second.Name, SlotB)
	}
	if _, err := os.Stat(filepath.Join(dir, "themes", SlotB+".json")); err != nil {
		t.Fatalf("active slot file is missing: %v", err)
	}
	deadline := time.Now().Add(500 * time.Millisecond)
	for {
		_, err := os.Stat(filepath.Join(dir, "themes", SlotA+".json"))
		if os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stale slot %s was not cleaned up", SlotA)
		}
		time.Sleep(5 * time.Millisecond)
	}

	third, err := injector.Apply(config)
	if err != nil {
		t.Fatalf("third Apply: %v", err)
	}
	if third.Name != SlotA {
		t.Fatalf("third slot = %s, want %s", third.Name, SlotA)
	}
	deadline = time.Now().Add(500 * time.Millisecond)
	for {
		_, err := os.Stat(filepath.Join(dir, "themes", SlotB+".json"))
		if os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("stale slot %s was not cleaned up", SlotB)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestConfigDirResolution(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	if got, want := ConfigDir(), filepath.Join(home, "xdg", "opencode"); got != want {
		t.Errorf("ConfigDir() = %q, want %q", got, want)
	}

	t.Setenv("XDG_CONFIG_HOME", "")
	if got, want := ConfigDir(), filepath.Join(home, ".config", "opencode"); got != want {
		t.Errorf("ConfigDir() = %q, want %q", got, want)
	}
}

func TestInjectorCreatesMissingConfig(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fresh", "opencode")
	injector := NewInjector(WithConfigDir(dir), WithDebounce(0))
	result, err := injector.Apply(testVars("dark", "#111111", "#eeeeee"))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if _, err := os.Stat(result.ThemePath); err != nil {
		t.Fatalf("theme file missing: %v", err)
	}
	data, err := os.ReadFile(result.ConfigPath)
	if err != nil {
		t.Fatal(err)
	}
	var cli map[string]any
	if err := json.Unmarshal(data, &cli); err != nil {
		t.Fatal(err)
	}
	if cli["$schema"] != TemplateSchema {
		t.Errorf("created cli.json $schema = %v, want %s", cli["$schema"], TemplateSchema)
	}
}

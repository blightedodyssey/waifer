//go:build windows

package winsys

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// writeGlazeWMConfig installs a config.yaml under a temp HOME and returns its
// path. It mirrors a real-world v3 config: v2-era gap keys, keybindings and
// window rules that must survive untouched.
func writeGlazeWMConfig(t *testing.T, body string) (string, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	// os.UserHomeDir uses USERPROFILE on Windows, but also honours HOME in some
	// builds; set both so the test is deterministic.
	t.Setenv("HOME", home)

	dir := filepath.Join(home, ".glzr", "glazewm")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir glazewm dir: %v", err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config.yaml: %v", err)
	}
	return dir, path
}

const glazeWMConfig = `# GlazeWM Configuration File

general:
  mod: alt
  cursor_follows_focus: false

gaps:
  inner_gaps: 4
  outer_gaps:
    top: 50
    bottom: 4
    left: 4
    right: 4

window_behavior:
  initial_state: 'tiling'

window_rules:
  - commands: ["ignore"]
    match:
      - window_process: { equals: "yasb" }

keybindings:
  - command: "focus left"
    binding: "Alt+H"
  - command: "reload-config"
    binding: "Alt+Shift+R"
`

func TestApplyGlazeWMThemeWritesV3WindowEffects(t *testing.T) {
	dir, path := writeGlazeWMConfig(t, glazeWMConfig)

	vars := map[string]string{"accent": "#ac1c3b", "dark_bg": "#050309", "background": "#07040c"}
	if err := ApplyGlazeWMTheme(vars); err != nil {
		t.Fatalf("ApplyGlazeWMTheme: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	got := string(raw)

	// v3 window_effects block with both borders enabled.
	for _, want := range []string{
		"window_effects:",
		"  focused_window:",
		"      color: \"#ac1c3b\"",
		"  other_windows:",
		"      color: \"#050309\"",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("config.yaml missing %q\n---\n%s", want, got)
		}
	}

	// The retired v2 key must be gone.
	if strings.Contains(got, "focused_border_color") {
		t.Errorf("config.yaml still contains the retired v2 border key")
	}

	// Keybindings and window rules are untouched.
	for _, want := range []string{
		`- command: "focus left"`,
		`binding: "Alt+H"`,
		`- command: "reload-config"`,
		`window_process: { equals: "yasb" }`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("config.yaml lost user content %q", want)
		}
	}

	// Gaps are migrated to the v3 schema and pinned to 3px; the YASB status
	// bar top offset (50px) is preserved from the existing config.
	if strings.Contains(got, "inner_gaps:") || strings.Contains(got, "outer_gaps:") {
		t.Errorf("v2 gap keys survived\n%s", got)
	}
	// GlazeWM v3.10+ rejects bare integers (LengthValueDe enum), so gaps are
	// emitted as quoted pixel strings.
	for _, want := range []string{`inner_gap: "3px"`, `top: "50px"`, `right: "3px"`, `bottom: "3px"`, `left: "3px"`} {
		if !strings.Contains(got, want) {
			t.Errorf("gaps missing %q\n%s", want, got)
		}
	}
	// No bare-integer gap value may survive anywhere in the file.
	if m := regexp.MustCompile(`(?m)^\s*(inner_gap|top|right|bottom|left):\s*[0-9]+\s*$`).FindString(got); m != "" {
		t.Errorf("bare-integer gap value survived: %q\n%s", m, got)
	}

	// Windows Terminal gets exactly one tiling rule.
	if n := strings.Count(got, "WindowsTerminal"); n != 1 {
		t.Errorf("WindowsTerminal rule count = %d, want 1\n%s", n, got)
	}
	if !strings.Contains(got, `set-title-bar-visibility hidden`) {
		t.Errorf("Windows Terminal rule does not hide the native title bar")
	}

	// The standalone drop-in mirrors the v3 shape.
	dropIn, err := os.ReadFile(filepath.Join(dir, "aether-colors.yaml"))
	if err != nil {
		t.Fatalf("read aether-colors.yaml: %v", err)
	}
	if !strings.Contains(string(dropIn), "window_effects:") {
		t.Errorf("aether-colors.yaml was not written in the v3 shape:\n%s", dropIn)
	}
}

func TestApplyGlazeWMThemeIsIdempotent(t *testing.T) {
	_, path := writeGlazeWMConfig(t, glazeWMConfig)
	vars := map[string]string{"accent": "#ac1c3b", "dark_bg": "#050309"}

	if err := ApplyGlazeWMTheme(vars); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	first, _ := os.ReadFile(path)
	if err := ApplyGlazeWMTheme(vars); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	second, _ := os.ReadFile(path)

	if string(first) != string(second) {
		t.Errorf("second apply changed config.yaml\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if n := strings.Count(string(second), "window_effects:"); n != 1 {
		t.Errorf("window_effects blocks = %d, want 1", n)
	}
	if n := strings.Count(string(second), "keybindings:"); n != 1 {
		t.Errorf("keybindings blocks = %d, want 1", n)
	}
}

func TestApplyGlazeWMThemeInsertsWhenSectionMissing(t *testing.T) {
	body := "general:\n  mod: alt\n\nkeybindings:\n  - command: \"focus left\"\n    binding: \"Alt+H\"\n"
	_, path := writeGlazeWMConfig(t, body)

	if err := ApplyGlazeWMTheme(map[string]string{"accent": "#7f5af0"}); err != nil {
		t.Fatalf("ApplyGlazeWMTheme: %v", err)
	}
	raw, _ := os.ReadFile(path)
	got := string(raw)
	if !strings.Contains(got, "window_effects:") {
		t.Errorf("window_effects was not inserted\n%s", got)
	}
	if !strings.Contains(got, `- command: "focus left"`) {
		t.Errorf("keybindings were lost\n%s", got)
	}
	// other_windows falls back to the background when dark_bg is absent.
	if !strings.Contains(got, `color: "#7f5af0"`) {
		t.Errorf("accent color missing\n%s", got)
	}
	// With no gaps block in the config, the top offset defaults to the 3px
	// tiling gap.
	if !strings.Contains(got, `top: "3px"`) {
		t.Errorf("default top gap missing\n%s", got)
	}
}

func TestApplyGlazeWMThemeMissingConfigIsNoop(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)

	if err := ApplyGlazeWMTheme(map[string]string{"accent": "#ac1c3b"}); err != nil {
		t.Fatalf("expected nil error when config.yaml is absent, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".glzr", "glazewm", "config.yaml")); !os.IsNotExist(err) {
		t.Errorf("config.yaml was created when it should have been left alone")
	}
}

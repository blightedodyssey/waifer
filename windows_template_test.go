package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"aether/internal/theme"
)

// TestWindowsTerminalTemplate checks that the Windows Terminal template is
// rendered with the active palette and is valid JSON.
func TestWindowsTerminalTemplate(t *testing.T) {
	state := theme.NewThemeState()
	w := theme.NewWriter(EmbeddedTemplates, "templates")
	out := t.TempDir()

	settings := theme.Settings{
		IncludedApps: map[string]bool{
			"windows-terminal": true,
			"colors":           true,
		},
	}
	if err := w.GenerateOnly(state, settings, out); err != nil {
		t.Fatalf("generate: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(out, "windows-terminal.json"))
	if err != nil {
		t.Fatalf("windows-terminal.json was not generated: %v", err)
	}
	var scheme map[string]string
	if err := json.Unmarshal(data, &scheme); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if scheme["name"] != "Aether" {
		t.Errorf("scheme name = %q, want Aether", scheme["name"])
	}
	for _, key := range []string{"background", "foreground", "black", "brightWhite", "purple"} {
		value, ok := scheme[key]
		if !ok {
			t.Errorf("missing %q", key)
			continue
		}
		if len(value) != 7 || value[0] != '#' {
			t.Errorf("%s = %q, want a #rrggbb color", key, value)
		}
	}
}

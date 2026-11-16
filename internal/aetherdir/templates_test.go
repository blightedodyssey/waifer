//go:build windows

package aetherdir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseUserTemplates(t *testing.T) {
	tmp := setupTestEnv(t)
	if err := os.MkdirAll(TemplatesDir(), 0755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}

	// Create a valid template
	tpl1 := filepath.Join(TemplatesDir(), "test1.tpl")
	content1 := `# target: %LOCALAPPDATA%\TestApp\colors.toml
background = "{{background}}"
foreground = "{{foreground}}"
accent = "{{accent_hex}}"
`
	if err := os.WriteFile(tpl1, []byte(content1), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	// Create a template without target directive (should be skipped)
	tpl2 := filepath.Join(TemplatesDir(), "notarget.tpl")
	if err := os.WriteFile(tpl2, []byte("just some content\n"), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	// Create a non-.tpl file (should be ignored)
	tpl3 := filepath.Join(TemplatesDir(), "readme.txt")
	if err := os.WriteFile(tpl3, []byte("# target: somewhere\n"), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
		"accent":     "#cba6f7",
	}

	templates, err := ParseUserTemplates(vars)
	if err != nil {
		t.Fatalf("ParseUserTemplates error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}

	tpl := templates[0]
	wantTarget := filepath.Join(tmp, "TestApp", "colors.toml")
	if tpl.TargetPath != wantTarget {
		t.Errorf("TargetPath = %q, want %q", tpl.TargetPath, wantTarget)
	}
	if tpl.SourcePath != tpl1 {
		t.Errorf("SourcePath = %q, want %q", tpl.SourcePath, tpl1)
	}
}

func TestUserTemplate_Render(t *testing.T) {
	tmp := setupTestEnv(t)
	if err := os.MkdirAll(TemplatesDir(), 0755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}

	tplPath := filepath.Join(TemplatesDir(), "render-test.tpl")
	content := `# target: %LOCALAPPDATA%\RenderTest\output.txt
background={{background}}
foreground={{foreground}}
accent_hex={{accent_hex}}
`
	if err := os.WriteFile(tplPath, []byte(content), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
		"accent":     "#cba6f7",
	}

	templates, err := ParseUserTemplates(vars)
	if err != nil {
		t.Fatalf("ParseUserTemplates error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}

	if err := templates[0].Render(); err != nil {
		t.Fatalf("Render error: %v", err)
	}

	outputPath := filepath.Join(tmp, "RenderTest", "output.txt")
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	out := string(data)
	if !strings.Contains(out, "background=#1e1e2e") {
		t.Errorf("missing background: %q", out)
	}
	if !strings.Contains(out, "foreground=#cdd6f4") {
		t.Errorf("missing foreground: %q", out)
	}
	if !strings.Contains(out, "accent_hex=cba6f7") {
		t.Errorf("missing stripped accent: %q", out)
	}
}

func TestParseSingleTemplate_BatchDirective(t *testing.T) {
	setupTestEnv(t)
	if err := os.MkdirAll(TemplatesDir(), 0755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}

	// Test // target: directive (batch/JSON style)
	tplPath := filepath.Join(TemplatesDir(), "batch.tpl")
	content := `// target: %APPDATA%\SomeApp\config.json
{"bg":"{{background}}"}`
	if err := os.WriteFile(tplPath, []byte(content), 0644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	vars := map[string]string{"background": "#000000"}
	templates, err := ParseUserTemplates(vars)
	if err != nil {
		t.Fatalf("ParseUserTemplates error: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}

	wantTarget := filepath.Join(os.Getenv("APPDATA"), "SomeApp", "config.json")
	if templates[0].TargetPath != wantTarget {
		t.Errorf("TargetPath = %q, want %q", templates[0].TargetPath, wantTarget)
	}
}

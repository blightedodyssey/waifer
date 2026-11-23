//go:build windows

package aetherdir

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRenderRefusesTerminalSettingsTarget is the regression guard for the bug
// that produced "All profiles were hidden in your settings": a drop-in template
// must never be allowed to replace Windows Terminal's settings.json wholesale.
func TestRenderRefusesTerminalSettingsTarget(t *testing.T) {
	tmp := setupTestEnv(t)
	if err := os.MkdirAll(TemplatesDir(), 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}

	target := filepath.Join(tmp, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir local state: %v", err)
	}
	const original = `{"profiles":{"list":[{"guid":"{1}","name":"PowerShell","hidden":false}]}}`
	if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
		t.Fatalf("write settings: %v", err)
	}

	tplPath := filepath.Join(TemplatesDir(), "rogue.tpl")
	tpl := "// target: " + target + "\n{\"name\": \"Aether\"}\n"
	if err := os.WriteFile(tplPath, []byte(tpl), 0o644); err != nil {
		t.Fatalf("write template: %v", err)
	}

	templates, err := ParseUserTemplates(map[string]string{"background": "#000000"})
	if err != nil {
		t.Fatalf("ParseUserTemplates: %v", err)
	}
	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}
	if err := templates[0].Render(); err == nil {
		t.Fatal("expected Render to refuse a Windows Terminal settings.json target")
	}

	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	if string(after) != original {
		t.Errorf("settings.json was overwritten by a template")
	}
}

func TestProtectedTemplateTarget(t *testing.T) {
	protected := []string{
		`C:\Users\me\AppData\Local\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json`,
		`C:\Users\me\AppData\Local\Packages\Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe\LocalState\settings.json`,
		`C:\Users\me\AppData\Local\Microsoft\Windows Terminal\settings.json`,
	}
	for _, path := range protected {
		if !protectedTemplateTarget(path) {
			t.Errorf("protectedTemplateTarget(%q) = false, want true", path)
		}
	}
	allowed := []string{
		`C:\Users\me\AppData\Local\Waifer\current\windows-terminal-scheme.json`,
		`C:\Users\me\AppData\Roaming\Code\User\settings.json`,
		`C:\Users\me\AppData\Local\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\state.json`,
	}
	for _, path := range allowed {
		if protectedTemplateTarget(path) {
			t.Errorf("protectedTemplateTarget(%q) = true, want false", path)
		}
	}
}

// TestRepairTerminalTemplate verifies the self-heal path: a template seeded by
// an older build (targeting the live settings.json) is rewritten to the safe
// standalone scheme target, while a user-authored template is left alone.
func TestRepairTerminalTemplate(t *testing.T) {
	setupTestEnv(t)
	if err := os.MkdirAll(TemplatesDir(), 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}

	legacy := filepath.Join(TemplatesDir(), terminalTemplateName)
	legacyBody := "// target: %LOCALAPPDATA%\\Packages\\Microsoft.WindowsTerminal_8wekyb3d8bbwe\\LocalState\\settings.json\n{\"name\":\"Aether\"}\n"
	if err := os.WriteFile(legacy, []byte(legacyBody), 0o644); err != nil {
		t.Fatalf("write legacy template: %v", err)
	}

	if err := repairTerminalTemplate(TemplatesDir()); err != nil {
		t.Fatalf("repairTerminalTemplate: %v", err)
	}

	healed, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatalf("read healed template: %v", err)
	}
	if target := firstTargetOf(string(healed)); protectedTemplateTarget(ExpandEnvVars(target)) {
		t.Errorf("legacy template still targets a protected file: %s", target)
	}
	if !strings.Contains(string(healed), starterTerminalSchemeTemplate[:40]) {
		t.Errorf("template was not replaced with the safe starter content")
	}
	// The safe template must never point at a live Terminal settings file.
	if protectedTemplateTarget(firstTargetOf(string(healed))) {
		t.Errorf("healed template still targets a protected file")
	}
}

func TestRepairTerminalTemplateLeavesUserFileAlone(t *testing.T) {
	setupTestEnv(t)
	if err := os.MkdirAll(TemplatesDir(), 0o755); err != nil {
		t.Fatalf("mkdir templates: %v", err)
	}

	custom := filepath.Join(TemplatesDir(), terminalTemplateName)
	body := "// target: %LOCALAPPDATA%\\Waifer\\current\\my-own-scheme.json\n{\"name\":\"Mine\"}\n"
	if err := os.WriteFile(custom, []byte(body), 0o644); err != nil {
		t.Fatalf("write custom template: %v", err)
	}

	if err := repairTerminalTemplate(TemplatesDir()); err != nil {
		t.Fatalf("repairTerminalTemplate: %v", err)
	}
	after, err := os.ReadFile(custom)
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	if string(after) != body {
		t.Errorf("user-authored template was modified")
	}
}

// TestSeedDoesNotShipDangerousTerminalTarget checks the shipped starter itself.
func TestSeedDoesNotShipDangerousTerminalTarget(t *testing.T) {
	setupTestEnv(t)
	if err := SeedStarterTemplates(); err != nil {
		t.Fatalf("SeedStarterTemplates: %v", err)
	}
	seeded, err := os.ReadFile(filepath.Join(TemplatesDir(), terminalTemplateName))
	if err != nil {
		t.Fatalf("read seeded template: %v", err)
	}
	target := firstTargetOf(string(seeded))
	if target == "" {
		t.Fatalf("seeded terminal template has no target directive")
	}
	if protectedTemplateTarget(ExpandEnvVars(target)) {
		t.Errorf("seeded terminal template targets a protected file: %s", target)
	}
}

// firstTargetOf extracts the target directive from a template document.
func firstTargetOf(doc string) string {
	firstLine := strings.SplitN(doc, "\n", 2)[0]
	idx := strings.Index(firstLine, targetDirectivePrefix)
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(firstLine[idx+len(targetDirectivePrefix):])
}

package harnesses

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- safeMergeJSON tests ---

func TestSafeMergeJSONCreatesNew(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	path := filepath.Join(tmpDir, "test.json")
	err := safeMergeJSON(path, func(data map[string]interface{}) error {
		data["theme"] = "aether"
		return nil
	})
	if err != nil {
		t.Fatalf("safeMergeJSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result["theme"] != "aether" {
		t.Errorf("theme = %v, want aether", result["theme"])
	}
}

func TestSafeMergeJSONPreservesOtherKeys(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	path := filepath.Join(tmpDir, "test.json")
	existing := map[string]interface{}{
		"mcpServers": map[string]interface{}{"test": true},
		"apiKey":     "secret123",
		"theme":      "default",
	}
	existingData, _ := json.Marshal(existing)
	os.WriteFile(path, existingData, 0o644)

	err := safeMergeJSON(path, func(data map[string]interface{}) error {
		data["theme"] = "aether"
		return nil
	})
	if err != nil {
		t.Fatalf("safeMergeJSON: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result["theme"] != "aether" {
		t.Errorf("theme = %v, want aether", result["theme"])
	}
	// Ensure sensitive keys are preserved
	if result["apiKey"] != "secret123" {
		t.Errorf("apiKey was overwritten, got %v", result["apiKey"])
	}
	mcpServers, ok := result["mcpServers"].(map[string]interface{})
	if !ok || mcpServers["test"] != true {
		t.Error("mcpServers was overwritten or lost")
	}
}

func TestSafeMergeJSONAbortsOnMalformed(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")
	os.WriteFile(path, []byte("this is not json"), 0o644)

	backupPath := path + ".aether.bak"
	os.WriteFile(backupPath, []byte("original"), 0o644)

	err := safeMergeJSON(path, func(data map[string]interface{}) error {
		data["theme"] = "aether"
		return nil
	})
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if !strings.Contains(err.Error(), "malformed") {
		t.Errorf("error should mention malformed, got: %v", err)
	}

	// Ensure original file was NOT modified
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "this is not json" {
		t.Error("original file was modified despite malformed JSON abort")
	}
}

func TestSafeMergeJSONCreatesBackup(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")
	os.WriteFile(path, []byte(`{"original": true}`), 0o644)

	err := safeMergeJSON(path, func(data map[string]interface{}) error {
		data["theme"] = "aether"
		return nil
	})
	if err != nil {
		t.Fatalf("safeMergeJSON: %v", err)
	}

	backupPath := path + ".aether.bak"
	data, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("backup file not created: %v", err)
	}
	if !strings.Contains(string(data), `"original": true`) {
		t.Errorf("backup does not contain original data: %s", string(data))
	}
}

// --- OpenCode tests ---

func TestApplyOpenCodeCreatesThemeAndUpdatesConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("LOCALAPPDATA", filepath.Join(tmpDir, "AppData", "Local"))

	configDir := filepath.Join(tmpDir, ".config", "opencode")
	os.MkdirAll(configDir, 0o755)
	cliData := map[string]interface{}{
		"mcpServers": map[string]interface{}{"github": true},
		"providers":  []string{"openai"},
	}
	cliRaw, _ := json.Marshal(cliData)
	os.WriteFile(filepath.Join(configDir, "cli.json"), cliRaw, 0o644)

	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
		"accent":     "#89b4fa",
		"cursor":     "#cdd6f4",
		"base00":     "#1e1e2e",
		"base0D":     "#89b4fa",
		"theme_type": "dark",
	}

	applied, warnings := applyOpenCode(vars)
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
	if len(applied) == 0 {
		t.Fatal("expected at least one opencode target applied")
	}

	// Verify the native V2 hot-reload theme file.
	themeFiles, err := filepath.Glob(filepath.Join(configDir, "themes", "aether-live-*.json"))
	if err != nil || len(themeFiles) == 0 {
		t.Fatalf("OpenCode V2 theme file not created: %v", err)
	}
	data, err := os.ReadFile(themeFiles[0])
	if err != nil {
		t.Fatalf("read theme file: %v", err)
	}
	var theme map[string]interface{}
	if err := json.Unmarshal(data, &theme); err != nil {
		t.Fatal(err)
	}
	if theme["$schema"] != "https://opencode.ai" {
		t.Errorf("theme $schema = %v", theme["$schema"])
	}
	if theme["base"] == nil || theme["dark"] == nil || theme["light"] == nil {
		t.Error("theme is missing base/dark/light sections")
	}

	// Verify cli.json preserved sensitive keys and points at a live slot.
	cliResult, err := os.ReadFile(filepath.Join(configDir, "cli.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cli map[string]interface{}
	if err := json.Unmarshal(cliResult, &cli); err != nil {
		t.Fatal(err)
	}
	name, _ := cli["theme"].(map[string]interface{})["name"].(string)
	if name != "aether-live-a" && name != "aether-live-b" {
		t.Errorf("cli theme name = %q, want an aether-live slot", name)
	}
	if _, ok := cli["mcpServers"]; !ok {
		t.Error("mcpServers was removed from cli.json")
	}

	// Verify Desktop theme JSON
	pluginDir := filepath.Join(configDir, "plugins", "aether-theme")
	desktopPath := filepath.Join(pluginDir, "desktop-theme.json")
	desktopRaw, err := os.ReadFile(desktopPath)
	if err != nil {
		t.Fatalf("desktop theme not created: %v", err)
	}
	var desktop map[string]interface{}
	if err := json.Unmarshal(desktopRaw, &desktop); err != nil {
		t.Fatal(err)
	}
	if desktop["name"] != "Aether" {
		t.Errorf("desktop theme name = %v, want Aether", desktop["name"])
	}
	if desktop["id"] != "aether" {
		t.Errorf("desktop theme id = %v, want aether", desktop["id"])
	}

	// Verify GUI plugin index.ts
	idxPath := filepath.Join(pluginDir, "index.ts")
	idxRaw, err := os.ReadFile(idxPath)
	if err != nil {
		t.Fatalf("plugin index not created: %v", err)
	}
	if !strings.Contains(string(idxRaw), "ctx.add(gui.Theme, themeData)") {
		t.Error("plugin index.ts missing ctx.add(gui.Theme, themeData)")
	}
}

func TestApplyOpenCodeSkipsMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// No .config/opencode directory
	vars := map[string]string{"accent": "#89b4fa"}
	applied, warnings := applyOpenCode(vars)
	if len(applied) > 0 {
		t.Error("expected no targets applied when opencode not installed")
	}
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
}

func TestApplyOpenCodeRegistersPluginInOpencodeJSON(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("LOCALAPPDATA", filepath.Join(tmpDir, "AppData", "Local"))

	configDir := filepath.Join(tmpDir, ".config", "opencode")
	os.MkdirAll(configDir, 0o755)

	// Existing opencode.json with plugins array and sensitive keys
	cfg := map[string]interface{}{
		"providers": []string{"openai"},
		"plugins":   []string{"file:///existing/plugin"},
		"model":     "gpt-4",
	}
	raw, _ := json.Marshal(cfg)
	os.WriteFile(filepath.Join(configDir, "opencode.json"), raw, 0o644)

	vars := map[string]string{
		"accent": "#89b4fa", "theme_type": "dark",
	}
	applied, warnings := applyOpenCode(vars)
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
	found := false
	for _, a := range applied {
		if strings.Contains(a, "opencode.json-plugin-registered") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected opencode.json plugin registration in applied list, got: %v", applied)
	}

	result, err := os.ReadFile(filepath.Join(configDir, "opencode.json"))
	if err != nil {
		t.Fatal(err)
	}
	var resultMap map[string]interface{}
	if err := json.Unmarshal(result, &resultMap); err != nil {
		t.Fatal(err)
	}

	// Verify providers still present
	if _, ok := resultMap["providers"]; !ok {
		t.Error("providers removed from opencode.json")
	}

	// Verify model still present
	if resultMap["model"] != "gpt-4" {
		t.Errorf("model = %v, want gpt-4", resultMap["model"])
	}

	// Verify plugin got added
	plugins, ok := resultMap["plugins"].([]interface{})
	if !ok {
		t.Fatalf("plugins type = %T, want []interface{}", resultMap["plugins"])
	}
	if len(plugins) != 2 {
		t.Errorf("plugins len = %d, want 2", len(plugins))
	}
	if plugins[0] != "file:///existing/plugin" {
		t.Errorf("plugins[0] = %v", plugins[0])
	}
	pluginDir := filepath.Join(configDir, "plugins", "aether-theme")
	expected := "file://" + filepath.ToSlash(pluginDir)
	if plugins[1] != expected {
		t.Errorf("plugin URI = %v, want %v", plugins[1], expected)
	}
}

func TestApplyOpenCodeRegistersPluginInOpencodeJSONC(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("LOCALAPPDATA", filepath.Join(tmpDir, "AppData", "Local"))

	configDir := filepath.Join(tmpDir, ".config", "opencode")
	os.MkdirAll(configDir, 0o755)

	// JSONC with comments and trailing comma
	jsonc := `{
  // My model settings
  "model": "gpt-4",
  "plugins": [
    "file:///existing/plugin",
  ],
}`
	os.WriteFile(filepath.Join(configDir, "opencode.jsonc"), []byte(jsonc), 0o644)

	vars := map[string]string{
		"accent": "#89b4fa", "theme_type": "dark",
	}
	applied, warnings := applyOpenCode(vars)
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
	found := false
	for _, a := range applied {
		if strings.Contains(a, "opencode.jsonc-plugin-registered") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected opencode.jsonc plugin registration in applied list, got: %v", applied)
	}

	result, err := os.ReadFile(filepath.Join(configDir, "opencode.jsonc"))
	if err != nil {
		t.Fatal(err)
	}
	var resultMap map[string]interface{}
	if err := json.Unmarshal(result, &resultMap); err != nil {
		t.Fatal(err)
	}

	// Verify model still present
	if resultMap["model"] != "gpt-4" {
		t.Errorf("model = %v, want gpt-4", resultMap["model"])
	}

	// Verify plugin got added
	plugins, ok := resultMap["plugins"].([]interface{})
	if !ok {
		t.Fatalf("plugins type = %T, want []interface{}", resultMap["plugins"])
	}
	if len(plugins) != 2 {
		t.Errorf("plugins len = %d, want 2", len(plugins))
	}
	pluginDir := filepath.Join(configDir, "plugins", "aether-theme")
	expected := "file://" + filepath.ToSlash(pluginDir)
	if plugins[1] != expected {
		t.Errorf("plugin URI = %v, want %v", plugins[1], expected)
	}
}

// --- Hermes tests ---

func TestApplyHermesCreatesDashboardAndSkin(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	hermesDir := filepath.Join(tmpDir, ".hermes")
	os.MkdirAll(hermesDir, 0o755)

	vars := map[string]string{
		"background":   "#1e1e2e",
		"foreground":   "#cdd6f4",
		"accent":       "#89b4fa",
		"dark_bg":      "#181825",
		"bright_black": "#45475a",
		"base0B":       "#a6e3a1",
		"base0A":       "#f9e2af",
		"base08":       "#f38ba8",
	}

	applied, warnings := applyHermes(vars)
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
	if len(applied) != 2 {
		t.Fatalf("expected 2 targets applied, got %d", len(applied))
	}

	// Verify dashboard theme
	dashPath := filepath.Join(hermesDir, "dashboard-themes", "aether.yaml")
	data, err := os.ReadFile(dashPath)
	if err != nil {
		t.Fatalf("dashboard theme not created: %v", err)
	}
	if !strings.Contains(string(data), "background: #1e1e2e") {
		t.Error("dashboard theme missing background color")
	}

	// Verify skin
	skinPath := filepath.Join(hermesDir, "skins", "aether.yaml")
	data, err = os.ReadFile(skinPath)
	if err != nil {
		t.Fatalf("skin not created: %v", err)
	}
	if !strings.Contains(string(data), "background: #1e1e2e") {
		t.Error("skin missing background color")
	}
}

// --- Pi tests ---

func TestApplyPiCreatesThemeAndUpdatesSettings(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	piDir := filepath.Join(tmpDir, ".pi", "agent")
	os.MkdirAll(piDir, 0o755)
	settings := map[string]interface{}{
		"model":  "gpt-4",
		"apiKey": "secret",
	}
	settingsRaw, _ := json.Marshal(settings)
	os.WriteFile(filepath.Join(piDir, "settings.json"), settingsRaw, 0o644)

	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
		"accent":     "#89b4fa",
		"dark_bg":    "#181825",
	}

	applied, warnings := applyPi(vars)
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
	if len(applied) == 0 {
		t.Fatal("expected at least one pi target applied")
	}

	// Verify settings.json preserved keys and added theme
	data, err := os.ReadFile(filepath.Join(piDir, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result["theme"] != "aether" {
		t.Errorf("theme = %v, want aether", result["theme"])
	}
	if result["apiKey"] != "secret" {
		t.Error("apiKey was overwritten")
	}
}

// --- Claude Code tests ---

func TestApplyClaudeCodeSetsThemeSafely(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	configData := map[string]interface{}{
		"apiKey":     "secret-key",
		"mcpServers": map[string]interface{}{"filesystem": true},
	}
	raw, _ := json.Marshal(configData)
	os.WriteFile(filepath.Join(tmpDir, ".claude.json"), raw, 0o644)

	vars := map[string]string{"theme_type": "dark"}
	applied, warnings := applyClaudeCode(vars)
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
	if len(applied) != 1 {
		t.Fatalf("expected 1 target applied, got %d", len(applied))
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".claude.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	if result["theme"] != "dark-ansi" {
		t.Errorf("theme = %v, want dark-ansi", result["theme"])
	}
	if result["apiKey"] != "secret-key" {
		t.Error("apiKey was overwritten")
	}
}

func TestApplyClaudeCodeSkipsMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	vars := map[string]string{"theme_type": "dark"}
	applied, warnings := applyClaudeCode(vars)
	if len(applied) > 0 {
		t.Error("expected no targets when claude not installed")
	}
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
}

// --- Jan tests ---

func TestApplyJanCreatesThemeFiles(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	janDir := filepath.Join(tmpDir, "jan")
	os.MkdirAll(janDir, 0o755)

	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
		"accent":     "#89b4fa",
		"dark_bg":    "#181825",
		"base00":     "#1e1e2e",
		"base0D":     "#89b4fa",
	}

	applied, warnings := applyJan(vars)
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
	if len(applied) != 2 {
		t.Fatalf("expected 2 targets applied, got %d", len(applied))
	}

	// Verify JSON theme
	themePath := filepath.Join(janDir, "themes", "aether.json")
	data, err := os.ReadFile(themePath)
	if err != nil {
		t.Fatalf("theme json not created: %v", err)
	}
	var theme map[string]interface{}
	if err := json.Unmarshal(data, &theme); err != nil {
		t.Fatal(err)
	}
	if theme["name"] != "aether" {
		t.Errorf("theme name = %v, want aether", theme["name"])
	}

	// Verify CSS
	cssPath := filepath.Join(janDir, "themes", "aether-theme.css")
	cssData, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("theme css not created: %v", err)
	}
	if !strings.Contains(string(cssData), "--bg: #1e1e2e") {
		t.Error("CSS missing --bg variable")
	}
}

// --- Aider tests ---

func TestApplyAiderUpdatesColorsPreservesKeys(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	configContent := `# Aider config
model: gpt-4
api_key: secret-key
# Theme colors
user-input-color: #ffffff
some-other-key: value
`
	os.WriteFile(filepath.Join(tmpDir, ".aider.conf.yml"), []byte(configContent), 0o644)

	vars := map[string]string{
		"background":   "#1e1e2e",
		"foreground":   "#cdd6f4",
		"accent":       "#89b4fa",
		"bright_black": "#45475a",
	}

	applied, warnings := applyAider(vars)
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
	if len(applied) != 1 {
		t.Fatalf("expected 1 target applied, got %d", len(applied))
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, ".aider.conf.yml"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Verify color was updated
	if !strings.Contains(content, "user-input-color: #89b4fa") {
		t.Error("user-input-color not updated to accent")
	}

	// Verify existing keys preserved
	if !strings.Contains(content, "model: gpt-4") {
		t.Error("model key was removed")
	}
	if !strings.Contains(content, "api_key: secret-key") {
		t.Error("api_key was removed")
	}
	if !strings.Contains(content, "some-other-key: value") {
		t.Error("some-other-key was removed")
	}
}

func TestApplyAiderSkipsMissing(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	vars := map[string]string{"accent": "#89b4fa"}
	applied, warnings := applyAider(vars)
	if len(applied) > 0 {
		t.Error("expected no targets when aider not installed")
	}
	if len(warnings) > 0 {
		t.Logf("warnings: %v", warnings)
	}
}

// --- Detect tests ---

func TestDetectFindsInstalledHarnesses(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)
	t.Setenv("APPDATA", tmpDir)
	t.Setenv("LOCALAPPDATA", tmpDir)

	// Setup opencode
	os.MkdirAll(filepath.Join(tmpDir, ".config", "opencode"), 0o755)
	// Setup hermes
	os.MkdirAll(filepath.Join(tmpDir, ".hermes"), 0o755)
	// Setup claude (using .claude.json)
	os.WriteFile(filepath.Join(tmpDir, ".claude.json"), []byte("{}"), 0o644)

	result := Detect()
	found := make(map[string]bool)
	for _, h := range result.Harnesses {
		found[h.Name] = h.HasData
	}

	if !found["opencode"] {
		t.Error("opencode not detected")
	}
	if !found["hermes"] {
		t.Error("hermes not detected")
	}
	if !found["claude-code"] {
		t.Error("claude-code not detected")
	}
	if found["pi"] {
		t.Error("pi should not be detected (not installed)")
	}
	if found["jan"] {
		t.Error("jan should not be detected (not installed)")
	}
}

// --- ApplyTheme (orchestrator) test ---

func TestApplyThemeOrchestrator(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("USERPROFILE", tmpDir)

	// Install opencode and pi
	os.MkdirAll(filepath.Join(tmpDir, ".config", "opencode"), 0o755)
	os.MkdirAll(filepath.Join(tmpDir, ".pi", "agent"), 0o755)
	os.WriteFile(filepath.Join(tmpDir, ".pi", "agent", "settings.json"), []byte("{}"), 0o644)

	vars := map[string]string{
		"background": "#1e1e2e",
		"foreground": "#cdd6f4",
		"accent":     "#89b4fa",
		"theme_type": "dark",
	}

	enabled := map[string]bool{
		"opencode": true,
		"pi":       true,
		"hermes":   false,
	}

	result := ApplyTheme(vars, enabled)
	if len(result.Warnings) > 0 {
		t.Logf("warnings: %v", result.Warnings)
	}

	// opencode should be applied
	opencodeFound := false
	piFound := false
	for _, a := range result.Applied {
		if strings.Contains(a, "opencode") {
			opencodeFound = true
		}
		if strings.Contains(a, "pi") {
			piFound = true
		}
	}
	if !opencodeFound {
		t.Error("opencode not in applied list")
	}
	if !piFound {
		t.Error("pi not in applied list")
	}
}

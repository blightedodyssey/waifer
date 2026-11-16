//go:build windows

package aetherdir

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// UserTemplate represents a single user drop-in .tpl file parsed from the
// templates directory.
type UserTemplate struct {
	SourcePath string            // path to the .tpl file
	TargetPath string            // expanded destination path from the directive comment
	Variables  map[string]string // variable substitutions
}

// targetDirectivePrefix is the expected start of a target directive line.
// Supported formats:
//   # target: <path>
//   // target: <path>
const targetDirectivePrefix = "target:"

// ParseUserTemplates scans %LOCALAPPDATA%\Aether\templates\*.tpl and parses
// each file for a target directive on line 1. It returns only templates that
// have a valid target directive. Windows environment variables (%APPDATA%,
// %LOCALAPPDATA%, %USERPROFILE%) in the target path are expanded.
func ParseUserTemplates(vars map[string]string) ([]UserTemplate, error) {
	tplDir := TemplatesDir()
	entries, err := os.ReadDir(tplDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read templates dir: %w", err)
	}

	var templates []UserTemplate
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(entry.Name()), ".tpl") {
			continue
		}
		srcPath := filepath.Join(tplDir, entry.Name())
		tpl, err := parseSingleTemplate(srcPath, vars)
		if err != nil {
			// Log but continue — individual bad templates shouldn't break the whole apply.
			fmt.Fprintf(os.Stderr, "aether: skip template %s: %v\n", entry.Name(), err)
			continue
		}
		if tpl != nil {
			templates = append(templates, *tpl)
		}
	}
	return templates, nil
}

// parseSingleTemplate reads a .tpl file and extracts the target directive.
func parseSingleTemplate(srcPath string, vars map[string]string) (*UserTemplate, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, err
	}
	content := string(data)

	// Extract target directive from line 1 (first non-empty line).
	lines := strings.Split(content, "\n")
	var targetRaw string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		idx := strings.Index(trimmed, targetDirectivePrefix)
		if idx >= 0 {
			targetRaw = strings.TrimSpace(trimmed[idx+len(targetDirectivePrefix):])
			break
		}
		// Only check the first non-empty line; if it doesn't contain the directive,
		// this template has no target.
		break
	}
	if targetRaw == "" {
		return nil, nil // No target directive — skip silently.
	}

	targetPath := ExpandEnvVars(targetRaw)
	return &UserTemplate{
		SourcePath: srcPath,
		TargetPath: targetPath,
		Variables:  vars,
	}, nil
}

// Render applies {{variable}} substitutions to the template content and writes
// the result to the target path. It supports both {{key}} (with # prefix) and
// {{key_hex}} (stripped # prefix) variable syntax.
func (t *UserTemplate) Render() error {
	data, err := os.ReadFile(t.SourcePath)
	if err != nil {
		return fmt.Errorf("read template %s: %w", t.SourcePath, err)
	}

	result := string(data)
	for key, value := range t.Variables {
		// {{key}} → full hex value (e.g. #1e1e2e)
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
		// {{key_hex}} → stripped hex value (e.g. 1e1e2e)
		stripped := strings.Replace(value, "#", "", 1)
		result = strings.ReplaceAll(result, "{{"+key+"_hex}}", stripped)
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(t.TargetPath), 0755); err != nil {
		return fmt.Errorf("create target dir for %s: %w", t.TargetPath, err)
	}

	if err := os.WriteFile(t.TargetPath, []byte(result), 0644); err != nil {
		return fmt.Errorf("write rendered template %s: %w", t.TargetPath, err)
	}
	return nil
}

// RenderAll applies all parsed user templates and returns any errors.
func RenderAll(templates []UserTemplate) []error {
	var errs []error
	for _, tpl := range templates {
		if err := tpl.Render(); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// SeedStarterTemplates creates 2–3 example .tpl files if the templates directory
// is empty. Existing user files are never overwritten.
func SeedStarterTemplates() error {
	if err := Ensure(); err != nil {
		return err
	}
	tplDir := TemplatesDir()
	entries, err := os.ReadDir(tplDir)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	// If directory has any files, skip seeding.
	for _, e := range entries {
		if !e.IsDir() {
			return nil
		}
	}

	starters := []struct {
		name    string
		content string
	}{
		{
			name: "alacritty-aether.toml.tpl",
			content: `# target: %APPDATA%\alacritty\aether-colors.toml
# Aether drop-in template for Alacritty terminal colors.
# Drop more .tpl files into %LOCALAPPDATA%\Aether\templates\ to target any app.

[colors.primary]
background = "{{background}}"
foreground = "{{foreground}}"

[colors.cursor]
text = "{{background}}"
cursor = "{{cursor}}"

[colors.normal]
black   = "{{color0}}"
red     = "{{color1}}"
green   = "{{color2}}"
yellow  = "{{color3}}"
blue    = "{{color4}}"
magenta = "{{color5}}"
cyan    = "{{color6}}"
white   = "{{color7}}"

[colors.bright]
black   = "{{color8}}"
red     = "{{color9}}"
green   = "{{color10}}"
yellow  = "{{color11}}"
blue    = "{{color12}}"
magenta = "{{color13}}"
cyan    = "{{color14}}"
white   = "{{color15}}"
`,
		},
		{
			name: "windows-terminal-aether.json.tpl",
			content: `// target: %LOCALAPPDATA%\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json
// Aether drop-in template for Windows Terminal color scheme.
// Uncomment and adjust the "schemes" array placement as needed.

{
  "name": "Aether",
  "background": "{{background_hex}}",
  "foreground": "{{foreground_hex}}",
  "cursorColor": "{{cursor_hex}}",
  "selectionBackground": "{{accent_hex}}",
  "black": "{{color0_hex}}",
  "red": "{{color1_hex}}",
  "green": "{{color2_hex}}",
  "yellow": "{{color3_hex}}",
  "blue": "{{color4_hex}}",
  "purple": "{{color5_hex}}",
  "cyan": "{{color6_hex}}",
  "white": "{{color7_hex}}",
  "brightBlack": "{{color8_hex}}",
  "brightRed": "{{color9_hex}}",
  "brightGreen": "{{color10_hex}}",
  "brightYellow": "{{color11_hex}}",
  "brightBlue": "{{color12_hex}}",
  "brightPurple": "{{color13_hex}}",
  "brightCyan": "{{color14_hex}}",
  "brightWhite": "{{color15_hex}}"
}
`,
		},
		{
			name: "vscode-settings-aether.json.tpl",
			content: `// target: %APPDATA%\Code\User\settings.json
// Aether drop-in template for VS Code workbench colors.
// This writes a standalone file; merge into your existing settings.json manually
// or use Aether's built-in VS Code export (Settings > Apps > VS Code).

{
  "workbench.colorCustomizations": {
    "editor.background": "{{background}}",
    "editor.foreground": "{{foreground}}",
    "editorCursor.foreground": "{{cursor}}",
    "editor.selectionBackground": "{{accent}}",
    "terminal.background": "{{background}}",
    "terminal.foreground": "{{foreground}}",
    "sideBar.background": "{{color0}}",
    "activityBar.background": "{{color0}}",
    "statusBar.background": "{{accent}}",
    "statusBar.foreground": "{{background}}"
  }
}
`,
		},
		{
			name: "yasb-aether-colors.css.tpl",
			content: `// target: %USERPROFILE%\.config\yasb\aether-colors.css
// Aether drop-in template for YASB status bar colors.
// The app target writes aether-colors.css directly; this template overrides it.

:root {
  --bg: {{background}};
  --fg: {{foreground}};
  --accent: {{accent}};
  --base00: {{color0}};
  --base01: {{color8}};
  --base02: {{dark_bg}};
  --base03: {{darker_bg}};
  --base04: {{lighter_bg}};
  --base05: {{foreground}};
  --base08: {{color1}};
  --base09: {{color2}};
  --base0A: {{color3}};
  --base0B: {{color4}};
  --base0C: {{color5}};
  --base0D: {{accent}};
  --base0E: {{color6}};
  --base0F: {{color7}};
}
`,
		},
		{
			name: "glazewm-aether-colors.yaml.tpl",
			content: `// target: %USERPROFILE%\.glzr\glazewm\aether-colors.yaml
// Aether drop-in template for GlazeWM border colors.
// Import this from your config.yaml with: $include aether-colors.yaml

focused_border_color: "{{accent_hex}}"
other_windows_border_color: "{{foreground_hex}}"
`,
		},
		{
			name: "opencode-aether-theme.json.tpl",
			content: `// target: %USERPROFILE%\.config\opencode\themes\aether.json
// Aether drop-in template for OpenCode theme.

{
  "name": "aether",
  "type": "dark",
  "colors": {
    "base00": "{{base00}}",
    "base01": "{{base01}}",
    "base02": "{{base02}}",
    "base03": "{{base03}}",
    "base04": "{{base04}}",
    "base05": "{{base05}}",
    "base06": "{{base06}}",
    "base07": "{{base07}}",
    "base08": "{{base08}}",
    "base09": "{{base09}}",
    "base0A": "{{base0A}}",
    "base0B": "{{base0B}}",
    "base0C": "{{base0C}}",
    "base0D": "{{base0D}}",
    "base0E": "{{base0E}}",
    "base0F": "{{base0F}}"
  }
}
`,
		},
		{
			name: "hermes-aether-dashboard.yaml.tpl",
			content: `# target: %USERPROFILE%\.hermes\dashboard-themes\aether.yaml
# Aether drop-in template for Hermes dashboard theme.

name: aether
type: dark
palette:
  background: "{{background}}"
  foreground: "{{foreground}}"
  accent: "{{accent}}"
  muted: "{{bright_black}}"
`,
		},
		{
			name: "pi-aether-theme.json.tpl",
			content: `// target: %USERPROFILE%\.pi\agent\themes\aether.json
// Aether drop-in template for Pi coding agent theme.

{
  "name": "aether",
  "type": "dark",
  "colors": {
    "accent": "{{accent}}",
    "text": "{{foreground}}",
    "background": "{{background}}",
    "muted": "{{bright_black}}",
    "success": "{{color2}}",
    "warning": "{{color3}}",
    "error": "{{color1}}"
  }
}
`,
		},
		{
			name: "jan-aether-theme.css.tpl",
			content: `/* target: %USERPROFILE%\jan\themes\aether-theme.css
   Aether drop-in template for Jan Desktop theme. */

:root {
  --bg: {{background}};
  --fg: {{foreground}};
  --accent: {{accent}};
  --surface: {{dark_bg}};
  --base00: {{color0}};
  --base0D: {{accent}};
}
`,
		},
		{
			name: "aider-aether-colors.yml.tpl",
			content: `# target: %USERPROFILE%\.aider.conf.yml
# Aether drop-in template for Aider CLI colors.
# Only color keys are updated — API keys and model settings are preserved.

user-input-color: "{{accent}}"
tool-output-color: "{{foreground}}"
assistant-output-color: "{{foreground}}"
completion-menu-color: "{{foreground}}"
completion-menu-bg-color: "{{background}}"
`,
		},
	}

	for _, s := range starters {
		path := filepath.Join(tplDir, s.name)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(s.content), 0644); err != nil {
				return fmt.Errorf("seed starter template %s: %w", s.name, err)
			}
		}
	}
	return nil
}

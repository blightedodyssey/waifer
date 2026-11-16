package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"aether/internal/platform"
	"aether/internal/template"
)

// vscodeExtensionID is the identifier written into the extension manifest.
const vscodeExtensionID = "local.theme-aether"

// vscodeRoots returns the user's VSCode extension and settings locations.
func vscodeRoots() (extensionsDir, settingsPath string) {
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			settingsPath = filepath.Join(appData, "Code", "User", "settings.json")
		} else {
			home, _ := os.UserHomeDir()
			settingsPath = filepath.Join(home, "AppData", "Roaming", "Code", "User", "settings.json")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".vscode", "extensions"), settingsPath
	}
	home, _ := os.UserHomeDir()
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(home, ".vscode", "extensions"), filepath.Join(configHome, "Code", "User", "settings.json")
}

// ApplyVSCodeTheme installs the generated color theme extension and selects
// it in VSCode's settings.json.
func ApplyVSCodeTheme(w *Writer, state *ThemeState, settings Settings) error {
	if !settings.IncludeVscode {
		return nil
	}
	themeDir := platform.ThemeDir()
	return processVSCodeExtension(w, state, settings, themeDir)
}

// processVSCodeExtension renders the vscode-extension templates into the
// user's VSCode extensions directory and points settings.json at the theme.
func processVSCodeExtension(w *Writer, state *ThemeState, settings Settings, themeDir string) error {
	extensionsDir, settingsPath := vscodeRoots()
	extRoot := filepath.Join(extensionsDir, vscodeExtensionID)
	themeOutDir := filepath.Join(extRoot, "themes")
	if err := os.MkdirAll(themeOutDir, 0o755); err != nil {
		return fmt.Errorf("create extension dir: %w", err)
	}

	vars := buildVars(state, state.WallpaperPath)

	// Render the extension theme file.
	themeTemplate := "vscode-extension/themes/aether-color-theme.json"
	content, err := template.ReadTemplate(w.templatesFS, w.root, themeTemplate)
	if err != nil {
		return err
	}
	rendered := template.ProcessTemplate(content, mergeTemplateVariables(vars, state.AppOverrides["vscode"]))
	if err := platform.WriteText(filepath.Join(themeOutDir, "aether-color-theme.json"), rendered); err != nil {
		return fmt.Errorf("write theme json: %w", err)
	}

	// Render the extension manifest (package.json).
	manifest, err := template.ReadTemplate(w.templatesFS, w.root, "vscode-extension/package.json")
	if err != nil {
		return err
	}
	if err := platform.WriteText(filepath.Join(extRoot, "package.json"), manifest); err != nil {
		return fmt.Errorf("write package.json: %w", err)
	}

	// Select the theme in settings.json.
	if err := upsertVSCodeSetting(settingsPath, "workbench.colorTheme", "Aether"); err != nil {
		return fmt.Errorf("update settings.json: %w", err)
	}
	return nil
}

// upsertVSCodeSetting replaces or appends a top-level string setting in a
// JSONC file, preserving comments and formatting as much as possible.
func upsertVSCodeSetting(path, key, value string) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	content := string(raw)

	pattern := regexp.MustCompile(`(?m)("` + regexp.QuoteMeta(key) + `"\s*:\s*)"[^"]*"`)
	if pattern.MatchString(content) {
		content = pattern.ReplaceAllString(content, `$1"`+value+`"`)
		return platform.WriteText(path, content)
	}

	trimmed := strings.TrimRight(strings.TrimSpace(content), "}")
	if trimmed == "" {
		content = "{\n    \"" + key + "\": \"" + value + "\"\n}"
	} else {
		content = trimmed + ",\n    \"" + key + "\": \"" + value + "\"\n}"
	}
	return platform.WriteText(path, content)
}

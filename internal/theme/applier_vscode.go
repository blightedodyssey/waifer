package theme

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"aether/internal/platform"
)

// ApplyVSCodeTheme installs the VSCode extension theme by copying the
// vscode-extension template directory from the embedded FS to
// ~/.vscode/extensions/local.theme-aether-1.0.0/ (and ~/.cursor/extensions/ on
// Windows), processing template variables along the way, and updating user
// settings.json when present.
func ApplyVSCodeTheme(fsys embed.FS, templatesDir string, variables map[string]string) error {
	home, err := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		if h := os.Getenv("HOME"); h != "" {
			home = h
			err = nil
		}
	}
	if err != nil {
		return err
	}

	extensionDir := filepath.Join(home, ".vscode", "extensions", "local.theme-aether-1.0.0")
	if err := platform.EnsureDir(extensionDir); err != nil {
		return err
	}

	if err := processVSCodeExtension(fsys, templatesDir, extensionDir, variables); err != nil {
		return err
	}
	log.Printf("VS Code theme extension installed to: %s", extensionDir)

	if runtime.GOOS == "windows" {
		cursorExtensionDir := filepath.Join(home, ".cursor", "extensions", "local.theme-aether-1.0.0")
		if err := platform.EnsureDir(cursorExtensionDir); err == nil {
			if err := processVSCodeExtension(fsys, templatesDir, cursorExtensionDir, variables); err == nil {
				log.Printf("Cursor theme extension installed to: %s", cursorExtensionDir)
			} else {
				log.Printf("Warning: could not install Cursor extension: %v", err)
			}
		} else {
			log.Printf("Warning: could not create Cursor extension dir: %v", err)
		}

		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		for _, appName := range []string{"Code", "Cursor"} {
			settingsPath := filepath.Join(appData, appName, "User", "settings.json")
			if err := patchVSCodeUserSettings(settingsPath); err != nil {
				log.Printf("Warning: could not update %s settings.json: %v", appName, err)
			}
		}
	}

	return nil
}

// patchVSCodeUserSettings sets "workbench.colorTheme": "Aether" in settingsPath
// if the User directory or settings.json exists.
func patchVSCodeUserSettings(settingsPath string) error {
	userDir := filepath.Dir(settingsPath)
	if info, err := os.Stat(userDir); err != nil || !info.IsDir() {
		return nil
	}

	root := map[string]any{}
	if raw, err := os.ReadFile(settingsPath); err == nil {
		if len(strings.TrimSpace(string(raw))) > 0 {
			parsed, parseErr := parseJSONCSimple(raw)
			if parseErr != nil {
				return fmt.Errorf("parse %s: %w", settingsPath, parseErr)
			}
			root = parsed
		}
		backup := settingsPath + ".aether-backup"
		if _, bErr := os.Stat(backup); os.IsNotExist(bErr) {
			_ = platform.CopyFile(settingsPath, backup)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	root["workbench.colorTheme"] = "Aether"
	encoded, err := json.MarshalIndent(root, "", "    ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	return platform.WriteText(settingsPath, string(encoded))
}

func parseJSONCSimple(data []byte) (map[string]any, error) {
	cleaned := removeTrailingCommasSimple(stripJSONCommentsSimple(data))
	var root map[string]any
	if err := json.Unmarshal(cleaned, &root); err != nil {
		return nil, err
	}
	return root, nil
}

func stripJSONCommentsSimple(data []byte) []byte {
	data = []byte(strings.TrimPrefix(string(data), "\ufeff"))
	out := make([]byte, 0, len(data))
	inString, escaped := false, false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				out = append(out, '\n')
			}
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			i++
		default:
			out = append(out, c)
		}
	}
	return out
}

func removeTrailingCommasSimple(data []byte) []byte {
	out := make([]byte, 0, len(data))
	inString, escaped := false, false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			out = append(out, c)
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		if c == '"' {
			inString = true
			out = append(out, c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(data) && strings.ContainsRune(" \t\r\n", rune(data[j])) {
				j++
			}
			if j < len(data) && (data[j] == '}' || data[j] == ']') {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

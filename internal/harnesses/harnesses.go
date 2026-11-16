// Package harnesses applies Aether themes to AI agent harnesses (OpenCode, Hermes,
// Pi, Claude Code, Jan, Aider) with a strict safe-merge contract: read existing
// config, skip on malformed data, backup before write, only touch theme keys.
package harnesses

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// HarnessInfo describes a detected AI harness installation.
type HarnessInfo struct {
	Name    string `json:"name"`
	Display string `json:"display"`
	// Path is the theme/config directory for that harness.
	Path    string `json:"path"`
	HasData bool   `json:"hasData"`
}

// DetectedHarnesses holds the full scan result for the UI.
type DetectedHarnesses struct {
	Harnesses []HarnessInfo `json:"harnesses"`
}

// ThemeResult reports what was applied and any warnings.
type ThemeResult struct {
	Applied  []string
	Warnings []string
}

var harnessRegistry = []struct {
	name     string
	display  string
	detect   func() (string, bool)
	apply    func(map[string]string) ([]string, []string)
}{
	{name: "opencode", display: "OpenCode", detect: detectOpenCode, apply: applyOpenCode},
	{name: "hermes", display: "Hermes", detect: detectHermes, apply: applyHermes},
	{name: "pi", display: "Pi", detect: detectPi, apply: applyPi},
	{name: "claude-code", display: "Claude Code", detect: detectClaudeCode, apply: applyClaudeCode},
	{name: "jan", display: "Jan", detect: detectJan, apply: applyJan},
	{name: "aider", display: "Aider", detect: detectAider, apply: applyAider},
}

// Detect scans the filesystem for installed harnesses.
func Detect() DetectedHarnesses {
	var harnesses []HarnessInfo
	for _, h := range harnessRegistry {
		path, ok := h.detect()
		info := HarnessInfo{Name: h.name, Display: h.display, Path: path, HasData: ok}
		harnesses = append(harnesses, info)
	}
	return DetectedHarnesses{Harnesses: harnesses}
}

// ApplyTheme applies the Aether palette to all enabled harnesses.
func ApplyTheme(vars map[string]string, enabled map[string]bool) ThemeResult {
	var applied []string
	var warnings []string
	for _, h := range harnessRegistry {
		if !enabled[h.name] {
			continue
		}
		a, w := h.apply(vars)
		applied = append(applied, a...)
		warnings = append(warnings, w...)
	}
	return ThemeResult{Applied: applied, Warnings: warnings}
}

// --- safe merge helpers ---

// safeMergeJSON reads an existing JSON file, unmarshals it, calls updater on
// the map, marshals back, backs up the original, then writes atomically.
// If the file is missing, updater runs on an empty map and a new file is created.
// If unparseable, returns an error without writing anything.
func safeMergeJSON(path string, updater func(map[string]interface{}) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	var data map[string]interface{}
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &data); err != nil {
			return fmt.Errorf("existing config is malformed: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read existing config: %w", err)
	}
	if data == nil {
		data = make(map[string]interface{})
	}

	if err := updater(data); err != nil {
		return err
	}

	out, err := jsonMarshal(data)
	if err != nil {
		return fmt.Errorf("marshal updated config: %w", err)
	}

	return backupAndWrite(path, out)
}

// safeMergeYAML reads existing YAML, calls updater for theme-specific keys,
// preserves all other keys, backs up, writes atomically.
func safeMergeYAML(path string, updater func(map[string]interface{}) error) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}

	var data map[string]interface{}
	if raw, err := os.ReadFile(path); err == nil {
		var err2 error
		data, err2 = parseYAML(string(raw))
		if err2 != nil {
			return fmt.Errorf("existing config is malformed: %w", err2)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read existing config: %w", err)
	}
	if data == nil {
		data = make(map[string]interface{})
	}

	if err := updater(data); err != nil {
		return err
	}

	out := []byte(marshalYAML(data))
	return backupAndWrite(path, out)
}

// backupAndWrite writes a .aether.bak backup then performs atomic write.
func backupAndWrite(path string, data []byte) error {
	if _, err := os.Stat(path); err == nil {
		backup := path + ".aether.bak"
		if raw, err := os.ReadFile(path); err == nil {
			if werr := os.WriteFile(backup, raw, 0o644); werr != nil {
				return fmt.Errorf("backup failed: %w", werr)
			}
		}
	}
	return writeFileAtomic(path, data)
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".aether-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func safeHex(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || !strings.HasPrefix(v, "#") {
		if v == "" {
			return "#000000"
		}
		return "#" + v
	}
	return v
}

func firstHex(value, fallback string) string {
	v := strings.TrimSpace(value)
	if v == "" || !strings.HasPrefix(v, "#") || len(v) != 7 {
		return fallback
	}
	return v
}

func jsonMarshal(v interface{}) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	if h == "" {
		h = "."
	}
	return h
}

func timestampBackupName(path string) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	return fmt.Sprintf("%s.aether.bak.%d%s", base, time.Now().Unix(), ext)
}

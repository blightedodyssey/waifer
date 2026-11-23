//go:build windows

package winsys

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// GlazeWM v3 (installed as .glzr/glazewm) moved border theming from
// `general.focused_border_color` (v2) into a `window_effects` block. Writing
// the old keys is a silent no-op, which is why waifer's borders never tracked
// the palette. This file edits only the blocks waifer owns and leaves every
// keybinding, gap and window rule otherwise intact.
const (
	glazeWMEffectsKey = "window_effects"
	glazeWMRulesKey   = "window_rules"
	glazeWMGapsKey    = "gaps"
	// GlazeWM renders a fixed-width DWM border; the visible "2px" separation
	// between tiled panes comes from gaps, so both are pinned to 2px.
	glazeWMGapPx = 2
)

// ApplyGlazeWMTheme updates the focused/unfocused border colors in
// %USERPROFILE%\.glzr\glazewm\config.yaml and mirrors them into
// aether-colors.yaml. If config.yaml does not exist, nothing is written.
func ApplyGlazeWMTheme(vars map[string]string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve user home: %w", err)
	}
	glazeDir := filepath.Join(home, ".glzr", "glazewm")
	configPath := filepath.Join(glazeDir, "config.yaml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read glazewm config.yaml: %w", err)
	}

	accent := normalizeHex(vars["accent"])
	if accent == "" {
		accent = "#cba6f7"
	}
	// Unfocused borders use the muted surface (omarchy's base01 / bright black).
	// `dark_bg` is a heavily darkened background and reads as invisible.
	other := normalizeHex(vars["bright_black"])
	if other == "" {
		other = normalizeHex(vars["dark_bg"])
	}
	if other == "" {
		other = normalizeHex(vars["background"])
	}
	if other == "" {
		other = "#1f1e20"
	}

	lines, ending, trailing := normalizedLines(string(data))
	lines = replaceTopLevelBlock(lines, glazeWMEffectsKey, windowEffectsBlock(accent, other))
	lines = replaceTopLevelBlock(lines, glazeWMGapsKey, gapsBlock(lines))
	lines = ensureTerminalWindowRule(lines)

	updated := strings.Join(lines, ending)
	if trailing {
		updated += ending
	}
	if updated != string(data) {
		if err := writeFileAtomic(configPath, []byte(updated)); err != nil {
			return fmt.Errorf("write glazewm config.yaml: %w", err)
		}
	}

	// Keep the standalone drop-in file in the current v3 shape so nothing that
	// copies it reintroduces the retired v2 keys.
	dropIn := "# Waifer-managed GlazeWM border colors (GlazeWM v3+).\n" +
		"# Merge these values into %USERPROFILE%\\\\.glzr\\\\glazewm\\\\config.yaml.\n" +
		"window_effects:\n" +
		"  focused_window:\n" +
		"    border:\n" +
		"      enabled: true\n" +
		"      color: " + strconv.Quote(accent) + "\n" +
		"  other_windows:\n" +
		"    border:\n" +
		"      enabled: true\n" +
		"      color: " + strconv.Quote(other) + "\n"
	if err := writeFileAtomic(filepath.Join(glazeDir, "aether-colors.yaml"), []byte(dropIn)); err != nil {
		return fmt.Errorf("write glazewm aether-colors.yaml: %w", err)
	}
	return nil
}

// normalizedLines splits the document, detects its line ending, and reports
// whether the source ended with a newline so the rewrite can preserve it.
func normalizedLines(doc string) (lines []string, ending string, trailing bool) {
	ending = "\n"
	if strings.Contains(doc, "\r\n") {
		ending = "\r\n"
	}
	trailing = strings.HasSuffix(doc, "\n")
	lines = strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}
	return lines, ending, trailing
}

func windowEffectsBlock(accent, other string) []string {
	return []string{
		"# waifer: focused/unfocused border colors are managed automatically.",
		glazeWMEffectsKey + ":",
		"  focused_window:",
		"    border:",
		"      enabled: true",
		"      color: " + strconv.Quote(accent),
		"  other_windows:",
		"    border:",
		"      enabled: true",
		"      color: " + strconv.Quote(other),
	}
}

// gapsBlock rewrites the v2 `inner_gaps`/`outer_gaps` keys (silently ignored by
// GlazeWM v3) into the v3 `inner_gap`/`outer_gap` schema and pins the tiling gap
// to 2px. The top outer gap is preserved because it holds the status-bar offset.
func gapsBlock(text []string) []string {
	top := 50
	if block := topLevelBlock(text, glazeWMGapsKey); block != nil {
		if v, ok := intValueInBlock(block, "top"); ok {
			top = v
		}
	}
	return []string{
		glazeWMGapsKey + ":",
		fmt.Sprintf("  inner_gap: %d", glazeWMGapPx),
		"  outer_gap:",
		fmt.Sprintf("    top: %d", top),
		fmt.Sprintf("    right: %d", glazeWMGapPx),
		fmt.Sprintf("    bottom: %d", glazeWMGapPx),
		fmt.Sprintf("    left: %d", glazeWMGapPx),
	}
}

func intValueInBlock(block []string, key string) (int, bool) {
	for _, line := range block {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, key+":") {
			continue
		}
		raw := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, key+":")), `"'`)
		raw = strings.TrimSuffix(raw, "px")
		if v, err := strconv.Atoi(raw); err == nil {
			return v, true
		}
	}
	return 0, false
}

// ensureTerminalWindowRule adds a tiling rule for Windows Terminal once. It is
// idempotent: if the process already appears in window_rules, nothing changes.
func ensureTerminalWindowRule(text []string) []string {
	rule := []string{
		"  # waifer: keep Windows Terminal tiled and hide its native DWM title bar.",
		`  - commands: ["set-tiling", "set-title-bar-visibility hidden"]`,
		"    match:",
		`      - window_process: { equals: "WindowsTerminal" }`,
	}
	if start, end, ok := findTopLevelBlock(text, glazeWMRulesKey); ok {
		for _, line := range text[start:end] {
			if strings.Contains(line, "WindowsTerminal") {
				return text
			}
		}
		merged := make([]string, 0, len(text)+len(rule))
		merged = append(merged, text[:end]...)
		merged = append(merged, rule...)
		merged = append(merged, text[end:]...)
		return merged
	}
	merged := append([]string{}, text...)
	if n := len(merged); n > 0 && strings.TrimSpace(merged[n-1]) != "" {
		merged = append(merged, "")
	}
	return append(merged, rule...)
}

// replaceTopLevelBlock replaces the whole top-level `key:` block with block. A
// missing key is inserted before `window_rules:` when possible so the file
// keeps its readable ordering, otherwise the block is appended.
func replaceTopLevelBlock(text []string, key string, block []string) []string {
	if start, end, ok := findTopLevelBlock(text, key); ok {
		merged := make([]string, 0, len(text)-(end-start)+len(block))
		merged = append(merged, text[:start]...)
		merged = append(merged, block...)
		merged = append(merged, text[end:]...)
		return merged
	}
	if start, _, ok := findTopLevelBlock(text, glazeWMRulesKey); ok {
		merged := make([]string, 0, len(text)+len(block)+1)
		merged = append(merged, text[:start]...)
		merged = append(merged, block...)
		merged = append(merged, "")
		merged = append(merged, text[start:]...)
		return merged
	}
	merged := append([]string{}, text...)
	if n := len(merged); n > 0 && strings.TrimSpace(merged[n-1]) != "" {
		merged = append(merged, "")
	}
	return append(merged, block...)
}

func topLevelBlock(text []string, key string) []string {
	start, end, ok := findTopLevelBlock(text, key)
	if !ok {
		return nil
	}
	return text[start:end]
}

// findTopLevelBlock returns the [start, end) line range of a top-level mapping
// key, where end is the first following non-empty line at column zero. Blank
// lines before the next key stay outside the range so separators survive, and a
// leading waifer-managed comment is pulled into the range so re-applying does
// not duplicate it.
func findTopLevelBlock(text []string, key string) (int, int, bool) {
	for i, line := range text {
		if !isTopLevelKey(line, key) {
			continue
		}
		start := i
		for start > 0 && isManagedComment(text[start-1]) {
			start--
		}
		end := len(text)
		for j := i + 1; j < len(text); j++ {
			next := text[j]
			if strings.TrimSpace(next) == "" {
				continue
			}
			if strings.HasPrefix(next, " ") || strings.HasPrefix(next, "\t") {
				continue
			}
			end = j
			break
		}
		// Keep trailing blank lines outside the replaced range.
		for end > i+1 && strings.TrimSpace(text[end-1]) == "" {
			end--
		}
		return start, end, true
	}
	return 0, 0, false
}

// isManagedComment reports whether a line is a waifer-owned marker comment that
// belongs to the block immediately below it.
func isManagedComment(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "# waifer:") || strings.HasPrefix(trimmed, "#waifer:")
}

func isTopLevelKey(line, key string) bool {
	if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
		return false
	}
	trimmed := strings.TrimSpace(line)
	return trimmed == key+":" || strings.HasPrefix(trimmed, key+": ")
}

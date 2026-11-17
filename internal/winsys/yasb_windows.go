//go:build windows

package winsys

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ApplyYasbTheme reads the user's live styles.css, replaces all color values
// (hex, rgba, and CSS var() calls) with the active Aether palette, creates a
// backup, performs an atomic write back to styles.css so YASB's file
// watcher hot-reloads the new colours immediately, and invokes yasbc reload
// if the yasbc CLI is available.
func ApplyYasbTheme(vars map[string]string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve user home: %w", err)
	}

	// Probe both possible YASB config locations in priority order.
	candidates := []string{
		filepath.Join(home, ".config", "yasb", "styles.css"),
		filepath.Join(os.Getenv("APPDATA"), "yasb", "styles.css"),
	}

	var stylesPath string
	var original string
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			stylesPath = p
			original = string(data)
			break
		}
	}

	// No existing file — fall back to primary path and create from template.
	if stylesPath == "" {
		stylesPath = candidates[0]
		original = defaultYasbStyles()
	}

	// Build the Aether-to-YASB semantic colour map.
	colours := yasbColourMap(vars)

	// ... rest continues as before
	// ------------------------------------------------------------------
	// 1. Backup on first touch
	// ------------------------------------------------------------------
	backupPath := stylesPath + ".aether.bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		if err := os.WriteFile(backupPath, []byte(original), 0o644); err != nil {
			return fmt.Errorf("create styles.css backup: %w", err)
		}
	}

	// ------------------------------------------------------------------
	// 2. Strip any stale @import for aether-colors.css
	// ------------------------------------------------------------------
	content := original
	content = regexp.MustCompile(`(?m)^\s*@import\s+["']aether-colors\.css["'];?\s*\n?`).ReplaceAllString(content, "")

	// ------------------------------------------------------------------
	// 3. Replace var(--name) calls with literal hex / rgba values.
	//    Qt QSS does not understand CSS custom properties, so we inline
	//    everything.
	// ------------------------------------------------------------------
	varReplacements := map[string]string{
		`var(--bg-color)`:     colours["bg"],
		`var(--text-color)`:   colours["text"],
		`var(--accent-color)`: colours["accent"],
		`var(--border-color)`: colours["border"],
	}
	for old, new := range varReplacements {
		content = strings.ReplaceAll(content, old, new)
	}

	// ------------------------------------------------------------------
	// 4. Overwrite the :root block (if present) with updated
	//    variable definitions.  Even though we inlined var() calls above,
	//    keeping :root updated avoids drift on the next apply cycle when
	//    the user might restore from backup.
	// ------------------------------------------------------------------
	rootPattern := regexp.MustCompile(`(?s):root\s*\{[^}]*\}`)
	newRoot := fmt.Sprintf(`:root {
    --bg-color: %s;
    --text-color: %s;
    --accent-color: %s;
    --border-color: %s;
}`, colours["bg"], colours["text"], colours["accent"], colours["border"])
	content = rootPattern.ReplaceAllString(content, newRoot)

	// ------------------------------------------------------------------
	// 5. Replace remaining hardcoded hex / rgba values that belong to
	//    the Aether semantic palette.
	// ------------------------------------------------------------------
	// 5a. Known accent colours that appear in the default / user template.
	for _, hex := range []string{"#cba6f7", "#f5c2e7", "#9ccfd8", "#89b4fa"} {
		content = strings.ReplaceAll(content, hex, colours["accent"])
		content = strings.ReplaceAll(content, strings.ToUpper(hex), colours["accent"])
	}

	// 5b. Subtle hover overlay (rgba white → semi-transparent text).
	hoverRGBA := fmt.Sprintf("rgba(%s, 0.05)", hexToRGB(colours["text"]))
	content = strings.ReplaceAll(content, "rgba(255, 255, 255, 0.05)", hoverRGBA)
	hoverRGBAMain := fmt.Sprintf("rgba(%s, 0.05)", hexToRGB(colours["text"]))
	content = strings.ReplaceAll(content, "rgba(255,255,255,0.05)", hoverRGBAMain)

	// ------------------------------------------------------------------
	// 6. Atomic write so YASB's watcher triggers a hot reload.
	// ------------------------------------------------------------------
	if err := writeFileAtomic(stylesPath, []byte(content)); err != nil {
		return fmt.Errorf("write styles.css: %w", err)
	}

	// ------------------------------------------------------------------
	// 7. Trigger yasbc reload if the CLI is available.
	// ------------------------------------------------------------------
	if _, err := exec.LookPath("yasbc"); err == nil {
		cmd := exec.Command("yasbc", "reload")
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		_ = cmd.Run() // best-effort; don't fail theme apply if reload errors
	}

	return nil
}

// yasbColourMap builds the semantic colour table for YASB from the active
// Aether palette.
func yasbColourMap(vars map[string]string) map[string]string {
	m := map[string]string{
		"bg":     firstHex(vars["base00"], "#1e1e2e"),
		"text":   firstHex(vars["base05"], "#cdd6f4"),
		"accent": firstHex(vars["base0D"], "#89b4fa"),
		"border": firstHex(vars["base0E"], "#cba6f7"),
	}
	return m
}

func firstHex(value, fallback string) string {
	v := strings.TrimSpace(value)
	if v == "" || !strings.HasPrefix(v, "#") || len(v) != 7 {
		return fallback
	}
	return v
}

// hexToRGB converts "#RRGGBB" to "R, G, B" (decimal) for use in rgba().
func hexToRGB(hex string) string {
	hex = strings.TrimPrefix(strings.TrimSpace(hex), "#")
	if len(hex) != 6 {
		return "205, 214, 244"
	}
	var r, g, b uint8
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return fmt.Sprintf("%d, %d, %d", r, g, b)
}

// defaultYasbStyles returns a safe minimal QSS template when the user does
// not yet have a styles.css.  It uses CSS custom properties so the same
// replacement pipeline works for both fresh and existing files.
func defaultYasbStyles() string {
	return `:root {
    --bg-color: #1e1e2e;
    --text-color: #cdd6f4;
    --accent-color: #cba6f7;
    --border-color: #f5c2e7;
}

* {
    font-family: "JetBrainsMono Nerd Font", monospace;
    font-size: 13px;
    font-weight: bold;
    color: var(--text-color);
}

.yasb-bar {
    background-color: var(--bg-color);
    border-bottom: 2px solid var(--border-color);
}

.widget {
    padding: 0px 10px;
    margin: 0px 2px;
}

.systray-widget {
    padding: 0px 6px;
    background-color: transparent;
}

.systray-widget QWidget {
    background-color: transparent;
}

.shortcuts-menu {
    color: var(--accent-color);
    padding: 0px 8px;
}

.shortcuts-menu:hover {
    color: #9ccfd8;
    background-color: rgba(255, 255, 255, 0.05);
}
`
}

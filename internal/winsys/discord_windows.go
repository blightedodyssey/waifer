//go:build windows

package winsys

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const discordThemeFileName = "aether.theme.css"

// ApplyDiscord writes the generated Vencord/Discord theme to Vencord, Vesktop,
// and BetterDiscord theme directories when present, and syncs Vencord's
// settings/quickCss.css if Vencord is installed.
func ApplyDiscord(vars map[string]string, themeDir string) error {
	css := renderDiscordCSS(themeDir, vars)
	appData := appDataDir()
	if strings.TrimSpace(appData) == "" {
		return fmt.Errorf("APPDATA is not set")
	}

	clientDirs := []struct {
		root      string
		themesDir string
		quickCSS  string
	}{
		{
			root:      filepath.Join(appData, "Vencord"),
			themesDir: filepath.Join(appData, "Vencord", "themes"),
			quickCSS:  filepath.Join(appData, "Vencord", "settings", "quickCss.css"),
		},
		{
			root:      filepath.Join(appData, "Vesktop"),
			themesDir: filepath.Join(appData, "Vesktop", "themes"),
			quickCSS:  filepath.Join(appData, "Vesktop", "settings", "quickCss.css"),
		},
		{
			root:      filepath.Join(appData, "BetterDiscord"),
			themesDir: filepath.Join(appData, "BetterDiscord", "themes"),
		},
	}

	installedCount := 0
	for _, client := range clientDirs {
		if info, err := os.Stat(client.root); err != nil || !info.IsDir() {
			continue
		}
		if err := writeDiscordClientTheme(client.themesDir, client.quickCSS, css); err != nil {
			return err
		}
		installedCount++
	}

	// Always ensure Vencord\themes\aether.theme.css and settings\quickCss.css
	// are written when Discord/Vencord is requested so users can enable it
	// immediately even on fresh Vencord installs.
	if installedCount == 0 {
		vencordThemes := filepath.Join(appData, "Vencord", "themes")
		vencordQuickCSS := filepath.Join(appData, "Vencord", "settings", "quickCss.css")
		if err := writeDiscordClientTheme(vencordThemes, vencordQuickCSS, css); err != nil {
			return err
		}
	}

	return nil
}

func writeDiscordClientTheme(themesDir, quickCSSPath string, css []byte) error {
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return fmt.Errorf("create Discord themes directory %s: %w", themesDir, err)
	}
	themePath := filepath.Join(themesDir, discordThemeFileName)
	if err := writeFileAtomic(themePath, css); err != nil {
		return fmt.Errorf("write Discord theme %s: %w", themePath, err)
	}

	if quickCSSPath != "" {
		settingsDir := filepath.Dir(quickCSSPath)
		if err := os.MkdirAll(settingsDir, 0o755); err != nil {
			return fmt.Errorf("create Vencord settings directory %s: %w", settingsDir, err)
		}
		backup := quickCSSPath + ".aether-backup"
		if _, err := os.Stat(backup); os.IsNotExist(err) {
			if _, statErr := os.Stat(quickCSSPath); statErr == nil {
				_ = copyFile(quickCSSPath, backup)
			}
		}
		if err := writeFileAtomic(quickCSSPath, css); err != nil {
			return fmt.Errorf("write Vencord quickCss.css: %w", err)
		}
	}
	return nil
}

func renderDiscordCSS(themeDir string, vars map[string]string) []byte {
	if themeDir != "" {
		if data, err := os.ReadFile(filepath.Join(themeDir, "vencord.theme.css")); err == nil && len(data) > 0 {
			return data
		}
	}
	c := func(key, fallback string) string {
		if v := normalizeHex(vars[key]); v != "" {
			return v
		}
		return fallback
	}
	content := fmt.Sprintf(`/**
* @name base16 Discord
* @author @bypass_
* @version 0.1.0
* @description Built using Aether and Pywal
* @source https://github.com/imbypass/base16-Discord
**/

/* import the parent */
@import url('https://imbypass.github.io/base16-discord/omarchy-discord.theme.css');

:root {
    --color00: %s;
    --color01: %s;
    --color02: %s;
    --color03: %s;
    --color04: %s;
    --color05: %s;
    --color06: %s;
    --color07: %s;
    --color08: %s;
    --color09: %s;
    --color10: %s;
    --color11: %s;
    --color12: %s;
    --color13: %s;
    --color14: %s;
    --color15: %s;
}
`,
		c("black", "#101014"),
		c("red", "#f38ba8"),
		c("green", "#a6e3a1"),
		c("yellow", "#f9e2af"),
		c("blue", "#89b4fa"),
		c("magenta", "#cba6f7"),
		c("cyan", "#94e2d5"),
		c("white", "#cdd6f4"),
		c("bright_black", "#45475a"),
		c("bright_red", "#f38ba8"),
		c("bright_green", "#a6e3a1"),
		c("bright_yellow", "#f9e2af"),
		c("bright_blue", "#89b4fa"),
		c("bright_magenta", "#cba6f7"),
		c("bright_cyan", "#94e2d5"),
		c("bright_white", "#ffffff"),
	)
	return []byte(content)
}

func restoreDiscordTheme() error {
	appData := appDataDir()
	if strings.TrimSpace(appData) == "" {
		return nil
	}
	for _, client := range []string{"Vencord", "Vesktop", "BetterDiscord"} {
		themePath := filepath.Join(appData, client, "themes", discordThemeFileName)
		_ = os.Remove(themePath)
		quickCSS := filepath.Join(appData, client, "settings", "quickCss.css")
		_ = restoreBackup(quickCSS)
	}
	return nil
}

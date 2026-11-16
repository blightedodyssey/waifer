//go:build windows

package winsys

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type applyTask struct {
	run func() ([]string, error)
}

// Apply runs every Windows integration for a theme. Targets are independent,
// so they run concurrently; the registry-based system/browser work is still
// committed as one atomic transaction. A missing app or locked file produces a
// warning instead of failing the whole theme application.
func Apply(opts Options) (Result, error) {
	tasks := make([]applyTask, 0, 6)

	if opts.WallpaperPath != "" {
		tasks = append(tasks, applyTask{run: func() ([]string, error) {
			if err := SetWallpaper(opts.WallpaperPath); err != nil {
				return nil, fmt.Errorf("desktop wallpaper: %w", err)
			}
			return []string{"wallpaper"}, nil
		}})
	}

	tasks = append(tasks, applyTask{run: func() ([]string, error) {
		if errs := applySystemColors(opts.Variables, opts.LightMode); len(errs) > 0 {
			return nil, errors.Join(errs...)
		}
		return []string{"accent-color", "light-dark-mode", "chrome-accent"}, nil
	}})

	// Firefox is optional: no profile simply means Firefox is not installed or
	// has never been launched, which is not a theme-application failure.
	tasks = append(tasks, applyTask{run: func() ([]string, error) {
		applied, err := applyFirefoxThemeIfPresent(opts.Variables)
		if err != nil {
			return nil, fmt.Errorf("firefox: %w", err)
		}
		if !applied {
			return nil, nil
		}
		return []string{"firefox"}, nil
	}})

	if opts.IncludeTerminal {
		tasks = append(tasks, applyTask{run: func() ([]string, error) {
			if err := ApplyWindowsTerminal(opts.Variables, opts.ThemeDir); err != nil {
				return nil, fmt.Errorf("windows terminal: %w", err)
			}
			return []string{"windows-terminal"}, nil
		}})
	}

	if opts.IncludeNeovim {
		tasks = append(tasks, applyTask{run: func() ([]string, error) {
			if err := ApplyNeovim(opts.Variables, opts.ThemeDir); err != nil {
				return nil, fmt.Errorf("neovim: %w", err)
			}
			return []string{"neovim"}, nil
		}})
	}

	if opts.IncludeZed {
		tasks = append(tasks, applyTask{run: func() ([]string, error) {
			if err := ApplyZed(opts.Variables, opts.ThemeDir, opts.LightMode); err != nil {
				return nil, fmt.Errorf("zed: %w", err)
			}
			return []string{"zed"}, nil
		}})
	}

	if opts.IncludeDiscord {
		tasks = append(tasks, applyTask{run: func() ([]string, error) {
			if err := ApplyDiscord(opts.Variables, opts.ThemeDir); err != nil {
				return nil, fmt.Errorf("discord/vencord: %w", err)
			}
			return []string{"vencord"}, nil
		}})
	}

	if opts.IncludeObsidian {
		tasks = append(tasks, applyTask{run: func() ([]string, error) {
			if err := ApplyObsidian(opts.Variables, opts.ThemeDir, opts.LightMode); err != nil {
				return nil, fmt.Errorf("obsidian: %w", err)
			}
			return []string{"obsidian"}, nil
		}})
	}

	applied := make([][]string, len(tasks))
	errs := make([]error, len(tasks))
	var wg sync.WaitGroup
	for i, task := range tasks {
		wg.Add(1)
		go func(i int, task applyTask) {
			defer wg.Done()
			applied[i], errs[i] = task.run()
		}(i, task)
	}
	wg.Wait()

	var res Result
	for i := range tasks {
		if errs[i] != nil {
			res.Warnings = append(res.Warnings, errs[i].Error())
			continue
		}
		res.Applied = append(res.Applied, applied[i]...)
	}
	return res, nil
}

// appDataDir returns %APPDATA%, falling back to the conventional path when the
// variable is missing (e.g. when launched from a service).
func appDataDir() string {
	if v := os.Getenv("APPDATA"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "Roaming")
}

// neovimConfigDir returns the directory Neovim reads its config from on
// Windows: $XDG_CONFIG_HOME/nvim or %LOCALAPPDATA%\nvim.
func neovimConfigDir() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, "nvim")
	}
	if v := os.Getenv("LOCALAPPDATA"); v != "" {
		return filepath.Join(v, "nvim")
	}
	return ""
}

// ApplyNeovim writes a self-contained colorscheme to
// <config>/colors/aether.lua so `:colorscheme aether` works without plugins.
// When the config has a LazyVim-style lua/plugins directory, the generated
// plugin spec is installed there too.
func ApplyNeovim(vars map[string]string, themeDir string) error {
	configDir := neovimConfigDir()
	if configDir == "" {
		return fmt.Errorf("could not locate the Neovim config directory")
	}

	colorsDir := filepath.Join(configDir, "colors")
	if err := os.MkdirAll(colorsDir, 0o755); err != nil {
		return fmt.Errorf("create Neovim colors directory: %w", err)
	}
	content := neovimColorscheme(vars)
	if err := writeFileAtomic(filepath.Join(colorsDir, "aether.lua"), []byte(content)); err != nil {
		return fmt.Errorf("write Neovim colorscheme: %w", err)
	}

	if themeDir != "" {
		pluginsDir := filepath.Join(configDir, "lua", "plugins")
		if info, err := os.Stat(pluginsDir); err == nil && info.IsDir() {
			if spec, err := os.ReadFile(filepath.Join(themeDir, "neovim.lua")); err == nil {
				if err := writeFileAtomic(filepath.Join(pluginsDir, "aether.lua"), spec); err != nil {
					return fmt.Errorf("write Neovim plugin spec: %w", err)
				}
			}
		}
	}
	return nil
}

// ApplyZed installs the generated Zed theme and, when Zed is configured,
// selects it for both light and dark appearances.
func ApplyZed(vars map[string]string, themeDir string, lightMode bool) error {
	if themeDir == "" {
		return fmt.Errorf("no theme directory to read from")
	}
	source := filepath.Join(themeDir, "aether.zed.json")
	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("Zed theme was not generated: %w", err)
	}

	zedDir := filepath.Join(appDataDir(), "Zed")
	themesDir := filepath.Join(zedDir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return fmt.Errorf("create Zed themes directory: %w", err)
	}
	if err := copyFile(source, filepath.Join(themesDir, "aether.json")); err != nil {
		return fmt.Errorf("install Zed theme: %w", err)
	}

	settingsPath := filepath.Join(zedDir, "settings.json")
	raw, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read Zed settings: %w", err)
	}
	root, err := parseJSONC(raw)
	if err != nil {
		return fmt.Errorf("parse Zed settings: %w", err)
	}
	mode := "dark"
	if lightMode {
		mode = "light"
	}
	root["theme"] = map[string]any{
		"mode":  mode,
		"dark":  "Aether",
		"light": "Aether",
	}
	encoded, err := json.MarshalIndent(root, "", "    ")
	if err != nil {
		return fmt.Errorf("encode Zed settings: %w", err)
	}
	encoded = append(encoded, '\n')
	backup := settingsPath + ".aether-backup"
	if _, err := os.Stat(backup); os.IsNotExist(err) {
		if err := copyFile(settingsPath, backup); err != nil {
			return fmt.Errorf("back up Zed settings: %w", err)
		}
	}
	if err := writeFileAtomic(settingsPath, encoded); err != nil {
		return fmt.Errorf("write Zed settings: %w", err)
	}
	return nil
}

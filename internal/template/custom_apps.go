package template

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"aether/internal/color"
	"aether/internal/platform"
)

// CustomAppConfig is the config.json found inside a custom app folder under
// the Aether custom directory.
type CustomAppConfig struct {
	Template    string `json:"template"`
	Destination string `json:"destination"`
}

// ProcessCustomApps renders every custom app folder under CustomDir():
// the folder's template is processed with vars, written to
// <themeDir>/<app>-<template> and symlinked to the configured destination.
// A post-apply.sh script in the folder is executed afterwards. Returns the
// processed app names.
func ProcessCustomApps(vars map[string]string, themeDir string) ([]string, error) {
	root := platform.CustomDir()
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read custom apps: %w", err)
	}

	apps := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // top-level files are template overrides, not apps
		}
		if err := processCustomApp(filepath.Join(root, entry.Name()), vars, themeDir); err != nil {
			return apps, fmt.Errorf("custom app %q: %w", entry.Name(), err)
		}
		apps = append(apps, entry.Name())
	}
	sort.Strings(apps)
	return apps, nil
}

// processCustomApp renders one custom app folder.
func processCustomApp(dir string, vars map[string]string, themeDir string) error {
	cfg, err := readCustomAppConfig(dir)
	if err != nil {
		return err
	}
	if cfg.Template == "" || cfg.Destination == "" {
		return fmt.Errorf("config.json must define template and destination")
	}

	content, err := os.ReadFile(filepath.Join(dir, cfg.Template))
	if err != nil {
		return fmt.Errorf("read template %s: %w", cfg.Template, err)
	}

	rendered := ProcessTemplate(string(content), vars)

	appName := filepath.Base(dir)
	themeFile := filepath.Join(themeDir, fmt.Sprintf("%s-%s", appName, cfg.Template))
	if err := os.MkdirAll(filepath.Dir(themeFile), 0o755); err != nil {
		return fmt.Errorf("prepare theme dir: %w", err)
	}
	if err := os.WriteFile(themeFile, []byte(rendered), 0o644); err != nil {
		return fmt.Errorf("write theme file: %w", err)
	}

	destination := expandHome(cfg.Destination)
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return fmt.Errorf("prepare destination: %w", err)
	}
	_ = os.Remove(destination)
	if err := platform.CreateSymlink(themeFile, destination); err != nil {
		// Symlinks may be unavailable (Windows without privileges): fall
		// back to a plain copy so the theme still reaches the app.
		if copyErr := platform.CopyFile(themeFile, destination); copyErr != nil {
			return err
		}
	}

	return runPostApply(dir)
}

// readCustomAppConfig decodes config.json in a custom app folder.
func readCustomAppConfig(dir string) (CustomAppConfig, error) {
	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		return CustomAppConfig{}, fmt.Errorf("read config.json: %w", err)
	}
	var cfg CustomAppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return CustomAppConfig{}, fmt.Errorf("parse config.json: %w", err)
	}
	return cfg, nil
}

// runPostApply executes post-apply.sh in dir when it exists and is
// executable. Non-Windows only (custom apps are a standalone/Linux flow).
func runPostApply(dir string) error {
	script := filepath.Join(dir, "post-apply.sh")
	if _, err := os.Stat(script); err != nil {
		return nil // no script: nothing to do
	}
	if runtime.GOOS == "windows" {
		return nil
	}
	cmd := exec.Command("/bin/bash", script)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("post-apply: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// ReadCustomOverride returns the user's replacement for a default template
// when one exists in CustomDir().
func ReadCustomOverride(name string) (string, bool) {
	if name == "" || strings.Contains(name, "..") {
		return "", false
	}
	data, err := os.ReadFile(filepath.Join(platform.CustomDir(), filepath.Base(name)))
	if err != nil {
		return "", false
	}
	return string(data), true
}

// selectionColor derives a selection background with readable contrast
// against both the background and foreground, biased toward the accent.
func selectionColor(background, foreground, accent string) string {
	if !color.IsHexColor(accent) {
		accent = foreground
	}
	if !color.IsHexColor(background) || !color.IsHexColor(foreground) {
		return accent
	}
	// Start from a 25% accent tint over the background, then push lightness
	// until the foreground stays readable on top of it.
	candidate := color.BlendHex(background, accent, 25)
	fgL := color.HexToOKLCH(foreground).L
	bgL := color.HexToOKLCH(background).L
	lch := color.HexToOKLCH(candidate)
	if fgL > bgL {
		// Light text on dark background: keep the selection darker than fg.
		if lch.L > fgL-0.15 {
			lch.L = fgL - 0.15
		}
	} else {
		if lch.L < fgL+0.15 {
			lch.L = fgL + 0.15
		}
	}
	return color.OKLCHToHex(lch)
}

// expandHome resolves a leading ~ to the user's home directory.
func expandHome(p string) string {
	if p == "~" {
		home, _ := os.UserHomeDir()
		return home
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[2:])
	}
	return p
}

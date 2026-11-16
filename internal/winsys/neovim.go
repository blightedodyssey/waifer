package winsys

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"aether/internal/platform"
)

// neovimConfigDir returns the user's Neovim configuration root.
func neovimConfigDir() string {
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "nvim")
		}
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "AppData", "Local", "nvim")
	}
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, _ := os.UserHomeDir()
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "nvim")
}

// neovimColorscheme is the destination path of the generated colorscheme.
func neovimColorscheme() string {
	return filepath.Join(neovimConfigDir(), "colors", "aether.lua")
}

// ApplyNeovim installs the rendered neovim.lua template as the aether
// colorscheme and hooks it into init.lua.
func ApplyNeovim(themeDir string) error {
	src := filepath.Join(themeDir, "neovim.lua")
	data, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // template not generated (excluded)
		}
		return fmt.Errorf("read %s: %w", src, err)
	}

	dest := neovimColorscheme()
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create colors dir: %w", err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("neovim: %w", err)
	}
	return ensureNeovimInit()
}

// ensureNeovimInit appends the colorscheme command to init.lua / init.vim
// when the aether colorscheme is not yet referenced.
func ensureNeovimInit() error {
	initLua := filepath.Join(neovimConfigDir(), "init.lua")
	line := "vim.cmd('colorscheme aether')"

	if data, err := os.ReadFile(initLua); err == nil {
		if containsLine(string(data), "colorscheme aether") {
			return nil
		}
		return writeFileAtomic(initLua, string(data)+"\n"+line+"\n")
	}
	if err := platform.EnsureDir(neovimConfigDir()); err != nil {
		return err
	}
	return writeFileAtomic(initLua, line+"\n")
}

// ClearNeovim removes the aether colorscheme and unregisters it from
// init.lua. Missing files are ignored.
func ClearNeovim() error {
	_ = os.Remove(neovimColorscheme())
	initLua := filepath.Join(neovimConfigDir(), "init.lua")
	data, err := os.ReadFile(initLua)
	if err != nil {
		return nil
	}
	var kept []string
	changed := false
	for _, line := range splitLinesN(string(data)) {
		if containsStr(line, "colorscheme aether") {
			changed = true
			continue
		}
		kept = append(kept, line)
	}
	if !changed {
		return nil
	}
	out := joinLines(kept)
	return writeFileAtomic(initLua, out)
}

// joinLines reassembles split lines with newlines.
func joinLines(lines []string) string {
	out := ""
	for _, line := range lines {
		out += line + "\n"
	}
	return out
}

// containsLine reports whether text contains a line containing needle.
func containsLine(text, needle string) bool {
	for _, line := range splitLinesN(text) {
		if containsStr(line, needle) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOfStr(s, sub) >= 0)
}

func indexOfStr(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func splitLinesN(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

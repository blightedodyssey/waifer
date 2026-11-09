package winsys

import (
	"strings"
	"testing"
)

func paletteVars() map[string]string {
	return map[string]string{
		"background":           "#101014",
		"foreground":           "#e6e6ef",
		"cursor":               "#89b4fa",
		"selection":            "#26263a",
		"selection_background": "#89b4fa",
		"black":                "#101014",
		"red":                  "#f38ba8",
		"green":                "#a6e3a1",
		"yellow":               "#f9e2af",
		"blue":                 "#89b4fa",
		"magenta":              "#cba6f7",
		"cyan":                 "#94e2d5",
		"white":                "#cdd6f4",
		"bright_black":         "#45475a",
		"bright_red":           "#f38ba8",
		"bright_green":         "#a6e3a1",
		"bright_yellow":        "#f9e2af",
		"bright_blue":          "#89b4fa",
		"bright_magenta":       "#cba6f7",
		"bright_cyan":          "#94e2d5",
		"bright_white":         "#ffffff",
		"accent":               "#89b4fa",
		"lighter_bg":           "#1a1a24",
		"muted":                "#7f849c",
		"orange":               "#fab387",
		"bright_fg":            "#ffffff",
		"mode":                 "dark",
	}
}

func TestNeovimColorscheme(t *testing.T) {
	lua := neovimColorscheme(paletteVars())
	for _, want := range []string{
		`vim.g.colors_name = "aether"`,
		`vim.o.background = "dark"`,
		`set(0, "Normal", { fg = "#e6e6ef", bg = "#101014" })`,
		`vim.g.terminal_color_0 = "#101014"`,
		`vim.g.terminal_color_15 = "#ffffff"`,
	} {
		if !strings.Contains(lua, want) {
			t.Errorf("colorscheme missing %q", want)
		}
	}
}

func TestNeovimColorschemeLight(t *testing.T) {
	vars := paletteVars()
	vars["mode"] = "light"
	if got := neovimColorscheme(vars); !strings.Contains(got, `vim.o.background = "light"`) {
		t.Errorf("light palette did not set light background")
	}
}

func TestNeovimColorschemeEmptyVars(t *testing.T) {
	lua := neovimColorscheme(map[string]string{})
	if !strings.Contains(lua, `fg = "#000000"`) {
		t.Errorf("empty palette should fall back to black, got:\n%s", lua)
	}
}

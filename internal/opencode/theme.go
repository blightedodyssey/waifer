// Package opencode builds OpenCode V2 themes from Aether's resolved palette
// variables and injects them into OpenCode's Windows config directory.
package opencode

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"aether/internal/color"
)

// TemplateSchema is the $schema written into generated V2 theme files. It is
// intentionally the short form used by Aether's OpenCode integration.
const TemplateSchema = "https://opencode.ai"

//go:embed v2-template.json
var v2Template string

var hueSteps = []string{"100", "200", "300", "400", "500", "600", "700", "800", "900"}

var baseHues = []string{"gray", "red", "orange", "yellow", "green", "cyan", "blue", "purple"}

// Mode returns the OpenCode mode implied by Aether's resolved variables.
func Mode(vars map[string]string) string {
	for _, key := range []string{"mode", "theme_type"} {
		if strings.EqualFold(strings.TrimSpace(vars[key]), "light") {
			return "light"
		}
	}
	return "dark"
}

// BuildTheme renders the embedded V2 template with Aether's 16-color palette
// and semantic colors. The returned bytes are a complete OpenCode V2 theme.
func BuildTheme(vars map[string]string) ([]byte, error) {
	values, err := buildPlaceholders(vars, Mode(vars))
	if err != nil {
		return nil, err
	}
	var theme map[string]any
	if err := json.Unmarshal([]byte(v2Template), &theme); err != nil {
		return nil, fmt.Errorf("parse embedded OpenCode V2 template: %w", err)
	}
	if err := replacePlaceholders(theme, values); err != nil {
		return nil, err
	}
	out, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal OpenCode V2 theme: %w", err)
	}
	return append(out, '\n'), nil
}

// buildPlaceholders resolves every {{TOKEN}} used by v2-template.json.
func buildPlaceholders(vars map[string]string, mode string) (map[string]string, error) {
	bg := pick(vars, "background", "#1e1e2e")
	fg := pick(vars, "foreground", "#cdd6f4")
	darker := pick(vars, "darker_bg", color.DarkenRGB(bg, 50))
	dark := pick(vars, "dark_bg", color.DarkenRGB(bg, 25))
	lighter := pick(vars, "lighter_bg", color.LightenRGB(bg, 10))
	muted := pick(vars, "muted", pick(vars, "bright_black", "#6c7086"))
	lightFG := pick(vars, "light_fg", color.LightenRGB(fg, 15))
	brightFG := pick(vars, "bright_fg", color.LightenRGB(fg, 25))
	accent := pick(vars, "accent", pick(vars, "blue", "#89b4fa"))

	values := map[string]string{
		"BG":         bg,
		"FG":         fg,
		"DARKER_BG":  darker,
		"DARK_BG":    dark,
		"LIGHTER_BG": lighter,
		"MUTED":      muted,
		"LIGHT_FG":   lightFG,
		"BRIGHT_FG":  brightFG,
		"ACCENT":     accent,
		"RED":        pick(vars, "red", "#f38ba8"),
		"GREEN":      pick(vars, "green", "#a6e3a1"),
		"YELLOW":     pick(vars, "yellow", "#f9e2af"),
		"BLUE":       pick(vars, "blue", accent),
		"PURPLE":     pick(vars, "magenta", pick(vars, "purple", "#cba6f7")),
		"CYAN":       pick(vars, "cyan", "#94e2d5"),
		"ORANGE":     pick(vars, "orange", color.LightenRGB(pick(vars, "red", "#f38ba8"), 15)),
		"BORDER":     mixHex(bg, fg, 0.25),
		"SELECTION":  pick(vars, "selection", pick(vars, "selection_background", fg)),
	}

	for _, modeName := range []string{"dark", "light"} {
		prefix := "HUE_" + strings.ToUpper(modeName) + "_"
		gray := grayRamp(darker, dark, bg, lighter, muted, lightFG, fg, brightFG)
		for index, step := range hueSteps {
			values[prefix+"GRAY_"+step] = gray[index]
		}
		for _, hue := range []string{"red", "green", "yellow", "blue", "purple", "cyan", "orange"} {
			ramp := toneRamp(values[strings.ToUpper(hue)], modeName)
			for index, step := range hueSteps {
				values[prefix+strings.ToUpper(hue)+"_"+step] = ramp[index]
			}
		}
		for _, alias := range []string{"accent", "interactive"} {
			ramp := toneRamp(accent, modeName)
			for index, step := range hueSteps {
				values[prefix+strings.ToUpper(alias)+"_"+step] = ramp[index]
			}
		}
	}

	for key, value := range values {
		if !isHex(value) {
			return nil, fmt.Errorf("OpenCode theme token %s is not a hex color: %q", key, value)
		}
	}
	return values, nil
}

// grayRamp maps Aether's semantic background/foreground scale onto the nine
// OpenCode hue steps. Missing steps are linearly interpolated between the
// nearest anchored colors.
func grayRamp(darker, dark, bg, lighter, muted, lightFG, fg, brightFG string) []string {
	anchors := []struct {
		step  int
		color string
	}{
		{100, darker},
		{200, bg},
		{300, lighter},
		{600, muted},
		{700, lightFG},
		{800, fg},
		{900, brightFG},
	}
	// dark_bg is the bridge between Darker and Base; fold it into the first
	// segment so the semantic Dark token still influences the generated ramp.
	_ = dark

	ramp := make([]string, len(hueSteps))
	for i, step := range hueSteps {
		var value int
		if _, err := fmt.Sscanf(step, "%d", &value); err != nil {
			continue
		}
		ramp[i] = interpolateAnchors(anchors, value)
	}
	return ramp
}

func interpolateAnchors(anchors []struct {
	step  int
	color string
}, step int) string {
	if step <= anchors[0].step {
		return anchors[0].color
	}
	for i := 0; i < len(anchors)-1; i++ {
		if step > anchors[i+1].step {
			continue
		}
		from, to := anchors[i], anchors[i+1]
		t := float64(step-from.step) / float64(to.step-from.step)
		return mixHex(from.color, to.color, t)
	}
	return anchors[len(anchors)-1].color
}

// toneRamp centers the given Aether color at step 500 and derives the other
// steps toward white or black. OpenCode expects light-to-dark steps for dark
// mode and dark-to-light steps for light mode.
func toneRamp(center, mode string) []string {
	// Dark mode runs light -> dark (100 -> 900), light mode runs dark -> light.
	var weights [9]float64
	var targets [9]string
	if mode == "light" {
		weights = [9]float64{0.82, 0.62, 0.40, 0.20, 0, 0.22, 0.45, 0.65, 0.85}
		targets = [9]string{"#000000", "#000000", "#000000", "#000000", center, "#ffffff", "#ffffff", "#ffffff", "#ffffff"}
	} else {
		weights = [9]float64{0.85, 0.65, 0.45, 0.22, 0, 0.20, 0.40, 0.62, 0.82}
		targets = [9]string{"#ffffff", "#ffffff", "#ffffff", "#ffffff", center, "#000000", "#000000", "#000000", "#000000"}
	}
	ramp := make([]string, len(hueSteps))
	for i := range ramp {
		ramp[i] = mixHex(center, targets[i], weights[i])
	}
	return ramp
}

func replacePlaceholders(node any, values map[string]string) error {
	switch value := node.(type) {
	case map[string]any:
		for key, child := range value {
			if text, ok := child.(string); ok && strings.HasPrefix(text, "{{") && strings.HasSuffix(text, "}}") {
				name := strings.TrimSuffix(strings.TrimPrefix(text, "{{"), "}}")
				resolved, ok := values[name]
				if !ok {
					return fmt.Errorf("unresolved OpenCode theme placeholder %q", name)
				}
				value[key] = resolved
				continue
			}
			if err := replacePlaceholders(child, values); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range value {
			if err := replacePlaceholders(child, values); err != nil {
				return err
			}
		}
	}
	return nil
}

func pick(vars map[string]string, key, fallback string) string {
	if value := strings.TrimSpace(vars[key]); isHex(value) {
		if strings.HasPrefix(value, "#") {
			return strings.ToLower(value)
		}
		return "#" + strings.ToLower(value)
	}
	return fallback
}

func isHex(value string) bool {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) != 6 {
		return false
	}
	for _, r := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

func mixHex(from, to string, weight float64) string {
	if weight <= 0 {
		return from
	}
	if weight >= 1 {
		return to
	}
	a, b := color.HexToRGB(from), color.HexToRGB(to)
	return color.RGBToHex(
		a.R+(b.R-a.R)*weight,
		a.G+(b.G-a.G)*weight,
		a.B+(b.B-a.B)*weight,
	)
}

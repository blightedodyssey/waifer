package template

import (
	"regexp"
	"strconv"
	"strings"

	"aether/internal/color"
)

// placeholderRe matches {name}, {name.mod} and {name.mod:alpha} where mod is
// one of strip/rgb/rgba/yaru and alpha is a float.
var placeholderRe = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)(\.[A-Za-z]+(?::[0-9]*\.?[0-9]+)?)?\}`)

// ProcessTemplate replaces every {variable} and {variable.modifier} in
// content with its value from vars. Placeholders without a matching variable
// are left untouched so partial renders remain debuggable.
func ProcessTemplate(content string, vars map[string]string) string {
	return placeholderRe.ReplaceAllStringFunc(content, func(match string) string {
		trimmed := strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")
		name, modifier, hasMod := strings.Cut(trimmed, ".")
		value, ok := vars[name]
		if !ok {
			return match // unknown variable: keep as-is
		}
		if !hasMod {
			return value
		}
		if !isKnownModifier(modifier) {
			return match // unknown modifier: keep the placeholder intact
		}
		return applyModifier(value, modifier)
	})
}

// applyModifier formats value according to the modifier suffix
// (".strip", ".rgb", ".rgba", ".rgba:0.5", ".yaru"). Non-color values pass
// through unchanged.
func applyModifier(value, modifier string) string {
	verb, arg, hasArg := strings.Cut(modifier, ":")

	switch verb {
	case "strip":
		return strings.TrimPrefix(value, "#")
	case "rgb":
		if !color.IsHexColor(value) {
			return value
		}
		return color.HexToRGBString(value)
	case "rgba":
		if !color.IsHexColor(value) {
			return value
		}
		alpha := 1.0
		if hasArg {
			parsed, err := strconv.ParseFloat(arg, 64)
			if err == nil {
				alpha = parsed
			}
		}
		return color.HexToRGBA(value, alpha)
	case "yaru":
		if !color.IsHexColor(value) {
			return value
		}
		return color.HexToYaruTheme(value)
	default:
		// Unknown modifier: callers pre-filter these; pass value through.
		return value
	}
}

// isKnownModifier reports whether the modifier suffix is supported.
func isKnownModifier(modifier string) bool {
	verb, _, _ := strings.Cut(modifier, ":")
	switch verb {
	case "strip", "rgb", "rgba", "yaru":
		return true
	}
	return false
}

// ReplaceVariable substitutes every occurrence of {name} and its modifier
// forms with value.
func ReplaceVariable(content, name, value string) string {
	pattern := `\{` + regexp.QuoteMeta(name) + `(\.[A-Za-z]+(?::[0-9]*\.?[0-9]+)?)?\}`
	re := regexp.MustCompile(pattern)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		trimmed := strings.TrimSuffix(strings.TrimPrefix(match, "{"), "}")
		_, modifier, _ := strings.Cut(trimmed, ".")
		if modifier == "" {
			return value
		}
		if !isKnownModifier(modifier) {
			return match
		}
		return applyModifier(value, modifier)
	})
}

// ExtractVariableNames returns the sorted, de-duplicated variable names used
// by a template (modifier suffixes stripped).
func ExtractVariableNames(content string) []string {
	matches := placeholderRe.FindAllStringSubmatch(content, -1)
	seen := make(map[string]bool, len(matches))
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			names = append(names, m[1])
		}
	}
	sortStrings(names)
	return names
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

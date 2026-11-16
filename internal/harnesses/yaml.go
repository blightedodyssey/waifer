package harnesses

import (
	"fmt"
	"strconv"
	"strings"
)

// parseYAML performs a very minimal YAML parse sufficient for Aider and Hermes.
// It handles top-level key: value pairs, simple nested maps (one level), and
// ignores comments and blank lines.  It does NOT support lists, multi-line
// strings, or complex flow styles — those are preserved verbatim as raw strings.
func parseYAML(input string) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	lines := strings.Split(input, "\n")
	var currentNestedKey string
	var currentNestedMap map[string]interface{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Check indentation (simple heuristic: 2+ spaces = nested)
		indent := len(line) - len(strings.TrimLeft(line, " "))

		if indent >= 2 && currentNestedKey != "" && currentNestedMap != nil {
			// Nested key under current block
			key, val, ok := parseYAMLLine(trimmed)
			if ok {
				currentNestedMap[key] = val
			}
			continue
		}

		// Top-level key
		key, val, ok := parseYAMLLine(trimmed)
		if !ok {
			continue
		}

		if val == "" {
			// This might be the start of a nested block — create nested map
			currentNestedKey = key
			currentNestedMap = make(map[string]interface{})
			result[key] = currentNestedMap
		} else {
			currentNestedKey = ""
			currentNestedMap = nil
			result[key] = val
		}
	}

	return result, nil
}

func parseYAMLLine(line string) (string, string, bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key := strings.TrimSpace(line[:idx])
	val := strings.TrimSpace(line[idx+1:])
	return key, val, key != ""
}

// marshalYAML serializes a map[string]interface{} to a minimal YAML string.
// Nested maps are indented with 2 spaces.  Values are output verbatim.
func marshalYAML(data map[string]interface{}) string {
	var b strings.Builder
	for key, val := range data {
		switch v := val.(type) {
		case map[string]interface{}:
			b.WriteString(fmt.Sprintf("%s:\n", key))
			for k2, v2 := range v {
				b.WriteString(fmt.Sprintf("  %s: %v\n", k2, v2))
			}
		case string:
			b.WriteString(fmt.Sprintf("%s: %s\n", key, v))
		case int:
			b.WriteString(fmt.Sprintf("%s: %d\n", key, v))
		case float64:
			b.WriteString(fmt.Sprintf("%s: %g\n", key, v))
		case bool:
			b.WriteString(fmt.Sprintf("%s: %t\n", key, v))
		default:
			b.WriteString(fmt.Sprintf("%s: %v\n", key, v))
		}
	}
	return b.String()
}

// parseYAMLNumber attempts to parse a string value as int or float.
func parseYAMLNumber(s string) interface{} {
	if i, err := strconv.Atoi(s); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}
	return s
}

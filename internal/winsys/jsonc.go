// Package winsys applies themes to the Windows desktop: wallpaper, accent
// color, light/dark mode, Windows Terminal, Zed and Neovim.
package winsys

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// stripJSONComments removes // and /* */ comments from JSONC content while
// respecting string literals.
func stripJSONComments(content string) string {
	var out strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(content); i++ {
		c := content[i]
		if inString {
			out.WriteByte(c)
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
			out.WriteByte(c)
		case c == '/' && i+1 < len(content) && content[i+1] == '/':
			// Line comment: skip to end of line.
			for i < len(content) && content[i] != '\n' {
				i++
			}
			if i < len(content) {
				out.WriteByte('\n')
			}
		case c == '/' && i+1 < len(content) && content[i+1] == '*':
			// Block comment: skip to closing marker.
			i += 2
			for i+1 < len(content) && !(content[i] == '*' && content[i+1] == '/') {
				i++
			}
			i++
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// removeTrailingCommas drops commas that precede a } or ].
func removeTrailingCommas(content string) string {
	var out strings.Builder
	inString := false
	escaped := false
	for i := 0; i < len(content); i++ {
		c := content[i]
		if inString {
			out.WriteByte(c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
			out.WriteByte(c)
		case ',':
			rest := strings.TrimLeftFunc(content[i+1:], func(r rune) bool {
				return unicode.IsSpace(r)
			})
			if strings.HasPrefix(rest, "}") || strings.HasPrefix(rest, "]") {
				continue // drop the comma
			}
			out.WriteByte(c)
		default:
			out.WriteByte(c)
		}
	}
	return out.String()
}

// parseJSONC decodes JSON with comments and trailing commas tolerated.
func parseJSONC(content string) (map[string]interface{}, error) {
	cleaned := removeTrailingCommas(stripJSONComments(content))
	dec := json.NewDecoder(bytes.NewReader([]byte(cleaned)))
	dec.UseNumber()
	var out map[string]interface{}
	if err := dec.Decode(&out); err != nil {
		return nil, fmt.Errorf("parse JSONC: %w", err)
	}
	return out, nil
}

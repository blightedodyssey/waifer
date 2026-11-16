package color

import "fmt"

// ValidateHex normalizes a hex color or reports a descriptive error.
func ValidateHex(s string) (string, error) {
	if !IsHexColor(s) {
		return "", fmt.Errorf("invalid hex color %q (expected #rgb or #rrggbb)", s)
	}
	return expandHex(s), nil
}

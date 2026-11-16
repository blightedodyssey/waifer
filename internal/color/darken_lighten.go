package color

// DarkenRGB darkens a hex color by the given percentage (0-100). Invalid
// input yields an empty string so callers can detect failure without a
// second return value.
func DarkenRGB(hex string, percent float64) string {
	if !IsHexColor(hex) {
		return ""
	}
	h := HexToHSL(hex)
	h.L = clampL(h.L * (1 - percent/100))
	return HSLToHex(h)
}

// LightenRGB lightens a hex color by the given percentage (0-100). Invalid
// input yields an empty string.
func LightenRGB(hex string, percent float64) string {
	if !IsHexColor(hex) {
		return ""
	}
	h := HexToHSL(hex)
	h.L = clampL(h.L + (1-h.L)*(percent/100))
	return HSLToHex(h)
}

package color

import "math"

// DarkenHex darkens a hex color by percent (0-100), returning a normalized
// hex string. Invalid input yields "#000000" so template rendering never
// fails on a bad variable.
func DarkenHex(hex string, percent float64) string {
	out := DarkenRGB(hex, percent)
	if out == "" {
		return "#000000"
	}
	return out
}

// LightenHex lightens a hex color by percent (0-100).
func LightenHex(hex string, percent float64) string {
	out := LightenRGB(hex, percent)
	if out == "" {
		return "#ffffff"
	}
	return out
}

// BlendHex mixes two hex colors; amount is the weight of the second color
// (0-100). Blending happens in OKLab so midpoints stay perceptually even.
func BlendHex(hexA, hexB string, amount float64) string {
	if !IsHexColor(hexA) {
		return hexB
	}
	if !IsHexColor(hexB) {
		return hexA
	}
	t := amount / 100
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	a := HexToOKLab(hexA)
	b := HexToOKLab(hexB)
	return OKLabToHex(OKLab{
		L: a.L + (b.L-a.L)*t,
		A: a.A + (b.A-a.A)*t,
		B: a.B + (b.B-a.B)*t,
	})
}

// HueRotateHex rotates a hex color's hue by the given degrees.
func HueRotateHex(hex string, degrees float64) string {
	if !IsHexColor(hex) {
		return hex
	}
	c := HexToOKLCH(hex)
	c.H = math.Mod(c.H+degrees+360, 360)
	return OKLCHToHex(c)
}

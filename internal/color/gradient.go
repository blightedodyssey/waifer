package color

import "math"

// GenerateGradient produces a 16-step gradient between two hex colors by
// linear interpolation in OKLab space (perceptually even steps).
func GenerateGradient(startHex, endHex string) [16]string {
	var out [16]string
	if !IsHexColor(startHex) || !IsHexColor(endHex) {
		for i := range out {
			out[i] = startHex
		}
		return out
	}
	start := HexToOKLab(startHex)
	end := HexToOKLab(endHex)
	for i := 0; i < 16; i++ {
		t := float64(i) / 15
		out[i] = OKLabToHex(OKLab{
			L: start.L + (end.L-start.L)*t,
			A: start.A + (end.A-start.A)*t,
			B: start.B + (end.B-start.B)*t,
		})
	}
	return out
}

// GeneratePaletteFromColor builds a full 16-color ANSI palette from one base
// color: background and foreground from lightness variants of the hue, the
// six chromatic slots at hue rotations, and their bright counterparts.
func GeneratePaletteFromColor(baseHex string) [16]string {
	var out [16]string
	if !IsHexColor(baseHex) {
		return out
	}
	base := HexToOKLCH(baseHex)

	bgL := base.L
	if bgL > 0.35 {
		bgL = 0.13 // dark background for light-ish base colors
	} else {
		bgL = base.L * 0.35
	}
	fgL := 0.92
	if base.L > 0.5 {
		fgL = 0.18
	}

	// Hue offsets for red..cyan.
	offsets := [6]float64{0, 60, 120, 180, 240, 300}
	for i, off := range offsets {
		c := base
		c.H = normalizeHue(base.H + off)
		c.C = base.C
		// Slot lightness ladder, dark palette assumption.
		slotL := 0.55 + 0.06*float64(i%3)
		if bgL > 0.5 {
			slotL = 0.45 - 0.04*float64(i%3)
		}
		c.L = slotL
		out[i+1] = OKLCHToHex(c)
		bright := c
		bright.L = clampL(c.L + 0.18)
		bright.C = c.C * 1.25
		out[i+9] = OKLCHToHex(bright)
	}

	bg := base
	bg.L = bgL
	bg.C = base.C * 0.15
	out[0] = OKLCHToHex(bg)

	fg := base
	fg.L = fgL
	fg.C = base.C * 0.25
	out[7] = OKLCHToHex(fg)
	out[8] = OKLCHToHex(OKLCH{L: clampL(fgL - 0.25), C: base.C * 0.2, H: base.H})
	out[15] = OKLCHToHex(OKLCH{L: clampL(fgL + 0.05), C: base.C * 0.3, H: base.H})

	return out
}

// normalizeHue keeps a hue in [0,360).
func normalizeHue(h float64) float64 {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}
	return h
}

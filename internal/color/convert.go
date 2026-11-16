package color

import "math"

// hue2rgb is the standard HSL hue-to-channel helper.
func hue2rgb(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6.0:
		return p + (q-p)*6*t
	case t < 0.5:
		return q
	case t < 2.0/3.0:
		return p + (q-p)*(2.0/3.0-t)*6
	default:
		return p
	}
}

// RGBToHSL converts 8-bit sRGB to HSL.
func RGBToHSL(r, g, b float64) HSL {
	rf := r / 255
	gf := g / 255
	bf := b / 255

	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	l := (max + min) / 2

	if max == min {
		return HSL{H: 0, S: 0, L: l}
	}

	d := max - min
	var s float64
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}

	var h float64
	switch max {
	case rf:
		h = (gf - bf) / d
		if g < b {
			h += 6
		}
	case gf:
		h = (bf-rf)/d + 2
	default:
		h = (rf-gf)/d + 4
	}
	return HSL{H: h * 60, S: s, L: l}
}

// HSLToRGB converts HSL back to 8-bit sRGB.
func HSLToRGB(h HSL) RGB {
	hn := math.Mod(h.H, 360) / 360
	if hn < 0 {
		hn++
	}

	if h.S == 0 {
		v := float64(clampByte(int(math.Round(h.L * 255))))
		return RGB{R: v, G: v, B: v}
	}

	var q float64
	if h.L < 0.5 {
		q = h.L * (1 + h.S)
	} else {
		q = h.L + h.S - h.L*h.S
	}
	p := 2*h.L - q

	return RGB{
		R: float64(clampByte(int(math.Round(hue2rgb(p, q, hn+1.0/3.0) * 255)))),
		G: float64(clampByte(int(math.Round(hue2rgb(p, q, hn) * 255)))),
		B: float64(clampByte(int(math.Round(hue2rgb(p, q, hn-1.0/3.0) * 255)))),
	}
}

// HexToHSL parses hex into HSL.
func HexToHSL(hex string) HSL {
	c := HexToRGB(hex)
	return RGBToHSL(c.R, c.G, c.B)
}

// HSLToHex formats HSL as #rrggbb.
func HSLToHex(h HSL) string {
	rgb := HSLToRGB(h)
	return RGBToHex(rgb.R, rgb.G, rgb.B)
}

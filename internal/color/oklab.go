package color

import "math"

// srgbToLinear converts an sRGB channel value (0-1) to linear light.
func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

// linearToSrgb converts a linear-light value (0-1) back to sRGB.
func linearToSrgb(c float64) float64 {
	if c <= 0.0031308 {
		return c * 12.92
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

// RGBToOKLab converts 8-bit sRGB channels to OKLab.
func RGBToOKLab(r, g, b float64) OKLab {
	rl := srgbToLinear(r / 255)
	gl := srgbToLinear(g / 255)
	bl := srgbToLinear(b / 255)

	l := math.Cbrt(0.4122214708*rl + 0.5363325363*gl + 0.0514459929*bl)
	m := math.Cbrt(0.2119034982*rl + 0.6806995451*gl + 0.1073969566*bl)
	s := math.Cbrt(0.0883024619*rl + 0.2817188376*gl + 0.6299787005*bl)

	return OKLab{
		L: 0.2104542553*l + 0.7936177850*m - 0.0040720468*s,
		A: 1.9779984951*l - 2.4285922050*m + 0.4505937099*s,
		B: 0.0259040371*l + 0.7827717662*m - 0.8086757660*s,
	}
}

// OKLabToRGB converts OKLab back to 8-bit sRGB.
func OKLabToRGB(c OKLab) RGB {
	l_ := c.L + 0.3963377774*c.A + 0.2158037573*c.B
	m_ := c.L - 0.1055613458*c.A - 0.0638541728*c.B
	s_ := c.L - 0.0894841775*c.A - 1.2914855480*c.B

	l := l_ * l_ * l_
	m := m_ * m_ * m_
	s := s_ * s_ * s_

	rl := 4.0767416621*l - 3.3077115913*m + 0.2309699292*s
	gl := -1.2684380046*l + 2.6097574011*m - 0.3413193965*s
	bl := -0.0041960863*l - 0.7034186147*m + 1.7076147010*s

	return RGB{
		R: float64(clampByte(int(math.Round(linearToSrgb(rl) * 255)))),
		G: float64(clampByte(int(math.Round(linearToSrgb(gl) * 255)))),
		B: float64(clampByte(int(math.Round(linearToSrgb(bl) * 255)))),
	}
}

// HexToOKLab parses hex into OKLab.
func HexToOKLab(hex string) OKLab {
	c := HexToRGB(hex)
	return RGBToOKLab(c.R, c.G, c.B)
}

// OKLabToHex formats OKLab as #rrggbb.
func OKLabToHex(c OKLab) string {
	rgb := OKLabToRGB(c)
	return RGBToHex(rgb.R, rgb.G, rgb.B)
}

// OKLabToOKLCH converts OKLab to cylindrical OKLCH.
func OKLabToOKLCH(c OKLab) OKLCH {
	chroma := math.Hypot(c.A, c.B)
	hue := math.Atan2(c.B, c.A) * 180 / math.Pi
	if hue < 0 {
		hue += 360
	}
	return OKLCH{L: c.L, C: chroma, H: hue}
}

// OKLCHToOKLab converts OKLCH back to OKLab.
func OKLCHToOKLab(c OKLCH) OKLab {
	rad := c.H * math.Pi / 180
	return OKLab{L: c.L, A: c.C * math.Cos(rad), B: c.C * math.Sin(rad)}
}

// RGBToOKLCH converts 8-bit sRGB to OKLCH.
func RGBToOKLCH(r, g, b int) OKLCH {
	return OKLabToOKLCH(RGBToOKLab(float64(r), float64(g), float64(b)))
}

// OKLCHToRGB converts OKLCH to 8-bit sRGB.
func OKLCHToRGB(c OKLCH) RGB {
	return OKLabToRGB(OKLCHToOKLab(c))
}

// HexToOKLCH parses hex into OKLCH.
func HexToOKLCH(hex string) OKLCH {
	return OKLabToOKLCH(HexToOKLab(hex))
}

// OKLCHToHex formats OKLCH as #rrggbb.
func OKLCHToHex(c OKLCH) string {
	return OKLabToHex(OKLCHToOKLab(c))
}

// OKLabDistance is the Euclidean distance between two OKLab colors.
func OKLabDistance(a, b OKLab) float64 {
	dl := a.L - b.L
	da := a.A - b.A
	db := a.B - b.B
	return math.Sqrt(dl*dl + da*da + db*db)
}

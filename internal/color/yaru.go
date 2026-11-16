package color

import "math"

// yaruVariants are the standard Ubuntu Yaru accent colors used for icon
// theme selection (ID suffix after "Yaru-").
var yaruVariants = []struct {
	Suffix string
	Hue    float64
	Chroma float64
}{
	{"", 230, 0.10},     // plain Yaru (orange accent, neutral)
	{"blue", 250, 0.16},
	{"green", 145, 0.14},
	{"olive", 105, 0.12},
	{"orange", 60, 0.16},
	{"purple", 300, 0.16},
	{"red", 25, 0.18},
	{"teal", 195, 0.14},
	{"yellow", 90, 0.13},
	{"magenta", 330, 0.18},
	{"viridian", 170, 0.13},
	{"darkblue", 265, 0.15},
	{"magentapink", 345, 0.18},
}

// HexToYaruTheme returns the Yaru icon-theme variant whose accent hue best
// matches the given hex color. The result is the full theme name, e.g.
// "Yaru-olive" (plain "Yaru" when the accent match is the default).
func HexToYaruTheme(hex string) string {
	if !IsHexColor(hex) {
		return "Yaru"
	}
	c := HexToOKLCH(hex)
	best := yaruVariants[0]
	bestDist := math.MaxFloat64
	for _, v := range yaruVariants[1:] {
		dh := hueDistance(c.H, v.Hue)
		dc := math.Abs(c.C-v.Chroma) * 40
		d := dh*dh + dc*dc
		if d < bestDist {
			bestDist = d
			best = v
		}
	}
	if best.Suffix == "" {
		return "Yaru"
	}
	return "Yaru-" + best.Suffix
}

// hueDistance returns the distance between two hues on the color wheel.
func hueDistance(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 360)
	if d > 180 {
		d = 360 - d
	}
	return d
}

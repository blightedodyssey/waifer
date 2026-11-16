package extraction

import (
	"math"

	"aether/internal/color"
)

// ANSI base hues (degrees) approximating the classic terminal wheel.
var ansiHues = [6]float64{25, 135, 95, 250, 320, 195}

// modeParams tunes the generic palette generator for one extraction mode.
type modeParams struct {
	// Background
	bgDarkL, bgLightL float64 // background lightness in dark / light mode
	bgChromaDark      float64 // chroma for dark backgrounds
	bgHue             func(stats ImageStats) float64
	// Foreground
	fgDarkL, fgLightL float64
	// ANSI colors
	ansiChroma        float64                    // normal-color chroma
	ansiChromaBright  float64                    // bright-color chroma
	ansiL             func(slot int) float64     // normal-color lightness
	ansiLBright       func(slot int) float64     // bright-color lightness
	hueFor            func(slot int, stats ImageStats) float64
	// Staggering
	lStagger float64 // extra lightness offsets applied per slot
}

// slotHuesFor returns the 8 ANSI hues (normal + bright share hues).
func slotHuesFor(p modeParams, stats ImageStats) [8]float64 {
	var hues [8]float64
	for i := 0; i < 8; i++ {
		hues[i] = p.hueFor(i, stats)
	}
	return hues
}

// generate builds a 16-color palette with the given parameters.
func generate(stats ImageStats, lightMode bool, p modeParams) [16]string {
	hues := slotHuesFor(p, stats)

	bgH := p.bgHue(stats)
	bgL := p.bgDarkL
	bgC := p.bgChromaDark
	fgL := p.fgDarkL
	if lightMode {
		bgL = p.bgLightL
		bgC = p.bgChromaDark * 0.3
		fgL = p.fgLightL
	}

	var palette [16]string
	for i := 0; i < 16; i++ {
		var c color.OKLCH
		switch {
		case i == 0: // black / background
			c = color.OKLCH{L: bgL, C: bgC, H: bgH}
		case i == 8: // bright black
			if lightMode {
				c = color.OKLCH{L: clampL(bgL - 0.22), C: bgC * 2, H: bgH}
			} else {
				c = color.OKLCH{L: clampL(bgL + 0.22), C: bgC * 2, H: bgH}
			}
		case i < 8: // normal ANSI (1-6) + white (7)
			if i == 7 {
				c = color.OKLCH{L: fgL, C: 0.005, H: bgH}
			} else {
				l := p.ansiL(i)
				c = color.OKLCH{L: l, C: p.ansiChroma, H: hues[i]}
			}
		case i == 15: // bright white
			if lightMode {
				c = color.OKLCH{L: clampL(bgL - 0.15), C: 0.005, H: bgH}
			} else {
				c = color.OKLCH{L: clampL(fgL + 0.15), C: 0.005, H: bgH}
			}
		default: // bright ANSI (9-14)
			idx := i - 8
			l := p.ansiLBright(idx)
			c = color.OKLCH{L: l, C: p.ansiChromaBright, H: hues[idx]}
		}
		if p.lStagger > 0 && i >= 1 && i <= 6 {
			c.L = clampL(c.L + float64(i%3)*p.lStagger)
		}
		palette[i] = color.OKLCHToHex(c)
	}
	return palette
}

// clampL constrains OKLab lightness.
func clampL(v float64) float64 {
	if v < 0.02 {
		return 0.02
	}
	if v > 0.98 {
		return 0.98
	}
	return v
}

// wrapHue keeps a hue in 0-360.
func wrapHue(h float64) float64 {
	return math.Mod(math.Mod(h, 360)+360, 360)
}

// standardANSIHues is the neutral hue mapping (normal colors 1-6 map to
// ansiHues; bright colors reuse the same hue family).
func standardANSIHues(p modeParams) modeParams {
	p.hueFor = func(slot int, stats ImageStats) float64 {
		if slot >= 8 {
			slot -= 8
		}
		if slot <= 0 {
			return ansiHues[0]
		}
		return ansiHues[(slot-1)%6]
	}
	return p
}

// dominantHueWheel rotates the ANSI wheel toward the image's dominant hue so
// palettes feel derived from the wallpaper.
func dominantHueWheel(weight float64) func(modeParams) modeParams {
	return func(p modeParams) modeParams {
		p.hueFor = func(slot int, stats ImageStats) float64 {
			base := ansiHues[slot%6]
			if stats.IsMonochrome {
				return base
			}
			return wrapHue(base + (stats.DominantH-base)*weight)
		}
		return p
	}
}

// modeGenerators maps mode names to palette builders.
var modeGenerators = map[string]func(stats ImageStats, lightMode bool) [16]string{
	"normal":              normalMode,
	"monochromatic":       monochromaticMode,
	"analogous":           analogousMode,
	"pastel":              pastelMode,
	"material":            materialMode,
	"colorful":            colorfulMode,
	"muted":               mutedMode,
	"bright":              brightMode,
	"complementary":       wheelMode(180, 0.9, 0.09),
	"triadic":             wheelMode(120, 0.9, 0.11),
	"split-complementary": wheelMode(150, 0.9, 0.10),
	"tetradic":            wheelMode(90, 0.9, 0.11),
	"fire":                moodMode(fireSpec),
	"ocean":               moodMode(oceanSpec),
	"forest":              moodMode(forestSpec),
	"earthtone":           moodMode(earthSpec),
	"neon":                neonMode,
	"sunset":              moodMode(sunsetSpec),
	"vaporwave":           moodMode(vaporwaveSpec),
	"midnight":            moodMode(midnightSpec),
	"aurora":              moodMode(auroraSpec),
	"high-contrast":       highContrastMode,
	"duotone":             duotoneMode,
}

// normalMode auto-detects monochrome vs chromatic content.
func normalMode(stats ImageStats, lightMode bool) [16]string {
	p := modeParams{
		bgDarkL:         math.Max(0.16, math.Min(stats.DarkL, 0.28)),
		bgLightL:        0.93,
		bgChromaDark:    0.02,
		fgDarkL:         0.86,
		fgLightL:        0.16,
		ansiChroma:      0.11,
		ansiChromaBright: 0.14,
		ansiL:           func(slot int) float64 { return 0.62 },
		ansiLBright:     func(slot int) float64 { return 0.74 },
	}
	if stats.IsMonochrome {
		// Monochrome wallpapers: neutral greys with one quiet accent hue.
		p = dominantHueWheel(1)(p)
		p.ansiChroma = 0.04
		p.ansiChromaBright = 0.05
		return generate(stats, lightMode, p)
	}
	p = dominantHueWheel(0.35)(p)
	return generate(stats, lightMode, p)
}

// monochromaticMode uses a single hue across all ANSI slots.
func monochromaticMode(stats ImageStats, lightMode bool) [16]string {
	hue := stats.DominantH
	p := modeParams{
		bgDarkL: 0.17, bgLightL: 0.93, bgChromaDark: 0.015,
		fgDarkL: 0.85, fgLightL: 0.17,
		ansiChroma: 0.055, ansiChromaBright: 0.075,
		ansiL:       func(slot int) float64 { return 0.55 + float64(slot)*0.03 },
		ansiLBright: func(slot int) float64 { return 0.68 + float64(slot)*0.02 },
	}
	if stats.IsMonochrome {
		hue = 250 // neutral fallback hue for grayscale wallpapers
	}
	p.hueFor = func(slot int, stats ImageStats) float64 { return wrapHue(hue + float64(slot)*4) }
	return generate(stats, lightMode, p)
}

// analogousMode keeps hues within ±30° of the dominant hue.
func analogousMode(stats ImageStats, lightMode bool) [16]string {
	base := stats.DominantH
	p := modeParams{
		bgDarkL: 0.17, bgLightL: 0.93, bgChromaDark: 0.02,
		fgDarkL: 0.86, fgLightL: 0.16,
		ansiChroma: 0.09, ansiChromaBright: 0.12,
		ansiL:       func(slot int) float64 { return 0.6 },
		ansiLBright: func(slot int) float64 { return 0.73 },
	}
	p.hueFor = func(slot int, stats ImageStats) float64 {
		offsets := [6]float64{-30, 0, 30, -15, 15, 0}
		return wrapHue(base + offsets[slot%6])
	}
	return generate(stats, lightMode, p)
}

// pastelMode produces soft, high-lightness colors.
func pastelMode(stats ImageStats, lightMode bool) [16]string {
	p := modeParams{
		bgDarkL: 0.22, bgLightL: 0.95, bgChromaDark: 0.015,
		fgDarkL: 0.88, fgLightL: 0.18,
		ansiChroma: 0.07, ansiChromaBright: 0.09,
		ansiL:       func(slot int) float64 { return 0.78 },
		ansiLBright: func(slot int) float64 { return 0.85 },
	}
	if lightMode {
		p.fgLightL = 0.22
	}
	return generate(stats, lightMode, standardANSIHues(p))
}

// materialMode uses balanced, saturated hues.
func materialMode(stats ImageStats, lightMode bool) [16]string {
	p := modeParams{
		bgDarkL: 0.18, bgLightL: 0.94, bgChromaDark: 0.02,
		fgDarkL: 0.87, fgLightL: 0.17,
		ansiChroma: 0.13, ansiChromaBright: 0.15,
		ansiL:       func(slot int) float64 { return 0.62 },
		ansiLBright: func(slot int) float64 { return 0.75 },
	}
	return generate(stats, lightMode, dominantHueWheel(0.25)(p))
}

// colorfulMode maximizes chroma.
func colorfulMode(stats ImageStats, lightMode bool) [16]string {
	p := modeParams{
		bgDarkL: 0.16, bgLightL: 0.94, bgChromaDark: 0.03,
		fgDarkL: 0.88, fgLightL: 0.16,
		ansiChroma: 0.17, ansiChromaBright: 0.20,
		ansiL:       func(slot int) float64 { return 0.63 },
		ansiLBright: func(slot int) float64 { return 0.76 },
	}
	return generate(stats, lightMode, standardANSIHues(p))
}

// mutedMode lowers chroma and staggers lightness.
func mutedMode(stats ImageStats, lightMode bool) [16]string {
	p := modeParams{
		bgDarkL: 0.20, bgLightL: 0.93, bgChromaDark: 0.012,
		fgDarkL: 0.84, fgLightL: 0.18,
		ansiChroma: 0.035, ansiChromaBright: 0.05,
		ansiL:       func(slot int) float64 { return 0.58 },
		ansiLBright: func(slot int) float64 { return 0.72 },
		lStagger:    0.045,
	}
	return generate(stats, lightMode, dominantHueWheel(0.4)(p))
}

// brightMode raises lightness across the palette.
func brightMode(stats ImageStats, lightMode bool) [16]string {
	p := modeParams{
		bgDarkL: 0.18, bgLightL: 0.95, bgChromaDark: 0.02,
		fgDarkL: 0.9, fgLightL: 0.16,
		ansiChroma: 0.12, ansiChromaBright: 0.14,
		ansiL:       func(slot int) float64 { return 0.7 },
		ansiLBright: func(slot int) float64 { return 0.82 },
	}
	return generate(stats, lightMode, standardANSIHues(p))
}

// wheelMode builds hue-wheel palettes: hues alternate between the dominant
// hue and hue+degrees. spread scales chroma, base chroma is fixed.
func wheelMode(degrees, spread float64, chroma float64) func(stats ImageStats, lightMode bool) [16]string {
	return func(stats ImageStats, lightMode bool) [16]string {
		base := stats.DominantH
		p := modeParams{
			bgDarkL: 0.17, bgLightL: 0.94, bgChromaDark: 0.02,
			fgDarkL: 0.87, fgLightL: 0.16,
			ansiChroma: chroma, ansiChromaBright: chroma * 1.2,
			ansiL:       func(slot int) float64 { return 0.62 },
			ansiLBright: func(slot int) float64 { return 0.75 },
		}
		p.hueFor = func(slot int, stats ImageStats) float64 {
			return wrapHue(base + float64(slot%6)*degrees*spread)
		}
		return generate(stats, lightMode, p)
	}
}

// moodSpec defines a fixed-hue mood palette.
type moodSpec struct {
	bgHue      float64
	bgL, bgLl  float64
	bgC        float64
	fgL, fgLl  float64
	hues       [6]float64 // normal ANSI hues
	chroma     float64
	brightCh   float64
	ansiL      float64
	ansiLB     float64
}

// moodMode builds a palette from a fixed mood specification.
func moodMode(spec moodSpec) func(stats ImageStats, lightMode bool) [16]string {
	return func(stats ImageStats, lightMode bool) [16]string {
		p := modeParams{
			bgDarkL: spec.bgL, bgLightL: spec.bgLl, bgChromaDark: spec.bgC,
			fgDarkL: spec.fgL, fgLightL: spec.fgLl,
			ansiChroma: spec.chroma, ansiChromaBright: spec.brightCh,
			ansiL:       func(slot int) float64 { return spec.ansiL + float64(slot)*0.015 },
			ansiLBright: func(slot int) float64 { return spec.ansiLB + float64(slot)*0.012 },
		}
		p.hueFor = func(slot int, stats ImageStats) float64 { return spec.hues[slot%6] }
		return generate(stats, lightMode, p)
	}
}

var (
	fireSpec = moodSpec{
		bgHue: 30, bgL: 0.15, bgLl: 0.93, bgC: 0.02, fgL: 0.85, fgLl: 0.16,
		hues: [6]float64{20, 80, 55, 35, 5, 65}, chroma: 0.13, brightCh: 0.15, ansiL: 0.62, ansiLB: 0.75,
	}
	oceanSpec = moodSpec{
		bgHue: 230, bgL: 0.16, bgLl: 0.92, bgC: 0.03, fgL: 0.85, fgLl: 0.16,
		hues: [6]float64{210, 160, 185, 245, 285, 195}, chroma: 0.11, brightCh: 0.13, ansiL: 0.62, ansiLB: 0.75,
	}
	forestSpec = moodSpec{
		bgHue: 130, bgL: 0.16, bgLl: 0.93, bgC: 0.025, fgL: 0.85, fgLl: 0.16,
		hues: [6]float64{25, 120, 95, 150, 180, 165}, chroma: 0.1, brightCh: 0.13, ansiL: 0.62, ansiLB: 0.75,
	}
	earthSpec = moodSpec{
		bgHue: 55, bgL: 0.18, bgLl: 0.92, bgC: 0.015, fgL: 0.83, fgLl: 0.18,
		hues: [6]float64{35, 70, 55, 90, 25, 50}, chroma: 0.04, brightCh: 0.055, ansiL: 0.6, ansiLB: 0.73,
	}
	sunsetSpec = moodSpec{
		bgHue: 330, bgL: 0.17, bgLl: 0.93, bgC: 0.03, fgL: 0.87, fgLl: 0.16,
		hues: [6]float64{15, 60, 45, 5, 320, 200}, chroma: 0.12, brightCh: 0.14, ansiL: 0.63, ansiLB: 0.76,
	}
	vaporwaveSpec = moodSpec{
		bgHue: 290, bgL: 0.19, bgLl: 0.93, bgC: 0.03, fgL: 0.87, fgLl: 0.16,
		hues: [6]float64{340, 160, 200, 280, 310, 190}, chroma: 0.14, brightCh: 0.16, ansiL: 0.64, ansiLB: 0.77,
	}
	midnightSpec = moodSpec{
		bgHue: 265, bgL: 0.14, bgLl: 0.92, bgC: 0.03, fgL: 0.83, fgLl: 0.16,
		hues: [6]float64{340, 140, 90, 250, 300, 200}, chroma: 0.08, brightCh: 0.1, ansiL: 0.6, ansiLB: 0.73,
	}
	auroraSpec = moodSpec{
		bgHue: 235, bgL: 0.15, bgLl: 0.92, bgC: 0.035, fgL: 0.85, fgLl: 0.16,
		hues: [6]float64{320, 140, 120, 210, 165, 180}, chroma: 0.12, brightCh: 0.15, ansiL: 0.63, ansiLB: 0.76,
	}
)

// neonMode is max-chroma on near-black.
func neonMode(stats ImageStats, lightMode bool) [16]string {
	p := modeParams{
		bgDarkL: 0.12, bgLightL: 0.95, bgChromaDark: 0.02,
		fgDarkL: 0.9, fgLightL: 0.14,
		ansiChroma: 0.24, ansiChromaBright: 0.28,
		ansiL:       func(slot int) float64 { return 0.66 },
		ansiLBright: func(slot int) float64 { return 0.72 },
	}
	if lightMode {
		p.fgLightL = 0.14
	}
	return generate(stats, lightMode, standardANSIHues(p))
}

// highContrastMode guarantees WCAG AAA (7:1) contrast between fg and bg.
func highContrastMode(stats ImageStats, lightMode bool) [16]string {
	p := modeParams{
		bgDarkL: 0.12, bgLightL: 0.97, bgChromaDark: 0.01,
		fgDarkL: 0.95, fgLightL: 0.12,
		ansiChroma: 0.12, ansiChromaBright: 0.14,
		ansiL:       func(slot int) float64 { return 0.7 },
		ansiLBright: func(slot int) float64 { return 0.88 },
	}
	if lightMode {
		p.fgLightL = 0.08
	}
	return generate(stats, lightMode, standardANSIHues(p))
}

// duotoneMode uses only the dominant hue and its complement.
func duotoneMode(stats ImageStats, lightMode bool) [16]string {
	base := stats.DominantH
	p := modeParams{
		bgDarkL: 0.16, bgLightL: 0.94, bgChromaDark: 0.02,
		fgDarkL: 0.86, fgLightL: 0.16,
		ansiChroma: 0.1, ansiChromaBright: 0.13,
		ansiL:       func(slot int) float64 { return 0.62 },
		ansiLBright: func(slot int) float64 { return 0.75 },
	}
	p.hueFor = func(slot int, stats ImageStats) float64 {
		if slot%2 == 0 {
			return wrapHue(base)
		}
		return wrapHue(base + 180)
	}
	return generate(stats, lightMode, p)
}

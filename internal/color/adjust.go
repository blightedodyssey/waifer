package color

import (
	"encoding/json"
	"math"
)

// Adjustments is the image-adjustment pipeline applied to every palette
// color. Field values are percentages where relevant (e.g. Saturation 20
// means +20%); HueShift/Temperature/Tint are degrees.
type Adjustments struct {
	Vibrance   float64 `json:"vibrance,omitempty"`
	Saturation float64 `json:"saturation,omitempty"`
	Contrast   float64 `json:"contrast,omitempty"`
	Brightness float64 `json:"brightness,omitempty"`
	Shadows    float64 `json:"shadows,omitempty"`
	Highlights float64 `json:"highlights,omitempty"`
	HueShift   float64 `json:"hueShift,omitempty"`
	HueTarget  float64 `json:"hueTarget,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	Tint       float64 `json:"tint,omitempty"`
	Gamma      float64 `json:"gamma,omitempty"`
	BlackPoint float64 `json:"blackPoint,omitempty"`
	WhitePoint float64 `json:"whitePoint,omitempty"`
}

// DefaultAdjustments returns the neutral pipeline (no visible change).
func DefaultAdjustments() Adjustments {
	return Adjustments{}
}

// clampL constrains an OKLab lightness to the valid range.
func clampL(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// applyVibrance raises saturation more for low-chroma colors (protects skin
// tones and near-neutrals from oversaturation).
func applyVibrance(c OKLCH, amount float64) OKLCH {
	if amount == 0 {
		return c
	}
	// 0-1 fraction of how saturated the color already is.
	factor := math.Min(c.C/0.2, 1)
	boost := amount / 100 * (1 - factor)
	c.C += boost * 0.2
	if c.C < 0 {
		c.C = 0
	}
	return c
}

// applySaturation scales chroma linearly.
func applySaturation(c OKLCH, amount float64) OKLCH {
	if amount == 0 {
		return c
	}
	c.C *= 1 + amount/100
	if c.C < 0 {
		c.C = 0
	}
	return c
}

// applyContrast pivots lightness around 0.5.
func applyContrast(c OKLCH, amount float64) OKLCH {
	if amount == 0 {
		return c
	}
	k := 1 + amount/100
	c.L = clampL(0.5 + (c.L-0.5)*k)
	return c
}

// applyBrightness lifts or lowers lightness.
func applyBrightness(c OKLCH, amount float64) OKLCH {
	if amount == 0 {
		return c
	}
	c.L = clampL(c.L + amount/100)
	return c
}

// applyShadows affects only the darker part of the range.
func applyShadows(c OKLCH, amount float64) OKLCH {
	if amount == 0 {
		return c
	}
	weight := 1 - c.L // 1 in deepest shadows, 0 in highlights
	c.L = clampL(c.L + weight*(amount/100))
	return c
}

// applyHighlights affects only the brighter part of the range.
func applyHighlights(c OKLCH, amount float64) OKLCH {
	if amount == 0 {
		return c
	}
	weight := c.L
	c.L = clampL(c.L + weight*(amount/100))
	return c
}

// applyHueShift rotates the hue.
func applyHueShift(c OKLCH, degrees float64) OKLCH {
	if degrees == 0 {
		return c
	}
	c.H = math.Mod(c.H+degrees+360, 360)
	return c
}

// applyHueTarget pulls chromatic colors toward a specific hue (degrees). The
// closer to the target, the less it moves.
func applyHueTarget(c OKLCH, target float64) OKLCH {
	if target <= 0 || c.C < 0.01 {
		return c
	}
	// weight by chroma so neutrals keep their hue character
	weight := math.Min(c.C/0.15, 1) * 0.5
	delta := math.Mod(target-c.H+540, 360) - 180
	c.H = math.Mod(c.H+delta*weight+360, 360)
	return c
}

// applyTemperature warms (+) or cools (-) by shifting a/b in OKLab.
func applyTemperature(c OKLCH, amount float64) OKLCH {
	if amount == 0 {
		return c
	}
	lab := OKLCHToOKLab(c)
	lab.B += amount / 100 * 0.05
	return OKLabToOKLCH(lab)
}

// applyTint shifts green/magenta balance.
func applyTint(c OKLCH, amount float64) OKLCH {
	if amount == 0 {
		return c
	}
	lab := OKLCHToOKLab(c)
	lab.A += amount / 100 * 0.05
	return OKLabToOKLCH(lab)
}

// applyGamma applies a power curve to lightness.
func applyGamma(c OKLCH, gamma float64) OKLCH {
	if gamma == 0 || gamma == 1 {
		return c
	}
	c.L = clampL(math.Pow(c.L, 1/gamma))
	return c
}

// applyBlackPoint deepens near-black values.
func applyBlackPoint(c OKLCH, amount float64) OKLCH {
	if amount == 0 {
		return c
	}
	// remap [bp..1] -> [0..1] in lightness
	bp := amount / 100 * 0.15
	if c.L <= bp {
		c.L = 0
	} else {
		c.L = clampL((c.L - bp) / (1 - bp))
	}
	return c
}

// applyWhitePoint compresses near-white values.
func applyWhitePoint(c OKLCH, amount float64) OKLCH {
	if amount == 0 {
		return c
	}
	wp := 1 - amount/100*0.15
	if c.L >= wp {
		c.L = 1
	} else {
		c.L = clampL(c.L / wp)
	}
	return c
}

// AdjustColor runs the full adjustment pipeline on one hex color.
func AdjustColor(hex string, adj Adjustments) string {
	if !IsHexColor(hex) {
		return hex
	}
	c := HexToOKLCH(hex)
	c = applyVibrance(c, adj.Vibrance)
	c = applySaturation(c, adj.Saturation)
	c = applyHueShift(c, adj.HueShift)
	c = applyHueTarget(c, adj.HueTarget)
	c = applyTemperature(c, adj.Temperature)
	c = applyTint(c, adj.Tint)
	c = applyContrast(c, adj.Contrast)
	c = applyBrightness(c, adj.Brightness)
	c = applyShadows(c, adj.Shadows)
	c = applyHighlights(c, adj.Highlights)
	c = applyGamma(c, adj.Gamma)
	c = applyBlackPoint(c, adj.BlackPoint)
	c = applyWhitePoint(c, adj.WhitePoint)
	return OKLCHToHex(c)
}

// ParseAdjustments decodes an adjustment map (as stored in blueprints) into
// the typed struct, preserving unknown-shape tolerance via JSON round-trip.
func ParseAdjustments(data map[string]float64) (Adjustments, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return DefaultAdjustments(), err
	}
	adj := DefaultAdjustments()
	if err := json.Unmarshal(raw, &adj); err != nil {
		return DefaultAdjustments(), err
	}
	return adj, nil
}

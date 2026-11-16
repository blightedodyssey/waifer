// Package extraction derives 16-color ANSI palettes from images through a
// set of named extraction modes. Results are content-hashed and cached.
package extraction

import (
	"fmt"
	"math"

	"aether/internal/color"
	"aether/internal/wallpaper"
)

// Sample is one analyzed pixel in OKLCH space.
type Sample struct {
	L, C, H float64
}

// ImageStats summarizes a sampled image.
type ImageStats struct {
	Samples    []Sample
	MeanL      float64
	DarkL      float64 // 5th percentile lightness
	LightL     float64 // 95th percentile lightness
	MeanC      float64
	DominantH  float64 // circular mean hue of chromatic pixels
	HueSpread  float64 // circular std deviation of hue
	IsMonochrome bool  // perceptually near-grayscale
}

// sampleImage decodes path and samples at most maxPixels pixels.
func sampleImage(path string, maxPixels int) ([]Sample, error) {
	if err := wallpaper.ValidateImageFile(path); err != nil {
		return nil, err
	}
	img, err := wallpaper.DecodeImage(path)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	dx, dy := b.Dx(), b.Dy()
	if dx == 0 || dy == 0 {
		return nil, fmt.Errorf("image has no pixels: %s", path)
	}

	step := 1
	for dx*dy/step > maxPixels {
		step++
	}

	samples := make([]Sample, 0, (dx*dy)/(step*step)+1)
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			r, g, bl, _ := img.At(x, y).RGBA()
			lch := color.RGBToOKLCH(int(r>>8), int(g>>8), int(bl>>8))
			samples = append(samples, Sample{L: lch.L, C: lch.C, H: lch.H})
		}
	}
	return samples, nil
}

// analyzeSamples computes percentiles and hue statistics.
func analyzeSamples(samples []Sample) ImageStats {
	stats := ImageStats{Samples: samples}
	if len(samples) == 0 {
		return stats
	}

	lightness := make([]float64, len(samples))
	var chromaSum float64
	for i, s := range samples {
		lightness[i] = s.L
		chromaSum += s.C
	}
	sortFloats(lightness)
	stats.DarkL = lightness[len(lightness)*5/100]
	stats.LightL = lightness[len(lightness)*95/100]

	for _, l := range lightness {
		stats.MeanL += l
	}
	stats.MeanL /= float64(len(samples))
	stats.MeanC = chromaSum / float64(len(samples))

	// Circular hue statistics over chromatic pixels only.
	chromaticThreshold := 0.04
	var sinSum, cosSum float64
	var chromatic int
	for _, s := range samples {
		if s.C < chromaticThreshold {
			continue
		}
		rad := s.H * math.Pi / 180
		sinSum += math.Sin(rad)
		cosSum += math.Cos(rad)
		chromatic++
	}
	if chromatic > 0 {
		meanRad := math.Atan2(sinSum/float64(chromatic), cosSum/float64(chromatic))
		if meanRad < 0 {
			meanRad += 2 * math.Pi
		}
		stats.DominantH = meanRad * 180 / math.Pi
		stats.HueSpread = hueSpread(samples, stats.DominantH, chromaticThreshold)
	}
	ratio := 1.0
	if stats.MeanL > 0 {
		ratio = stats.MeanC / (stats.MeanC + 0.08)
	}
	stats.IsMonochrome = chromatic == 0 || stats.MeanC < 0.045 || ratio < 0.35
	return stats
}

// hueSpread computes the circular standard deviation of hue.
func hueSpread(samples []Sample, dominant float64, threshold float64) float64 {
	var sum float64
	var n int
	for _, s := range samples {
		if s.C < threshold {
			continue
		}
		d := s.H - dominant
		if d > 180 {
			d -= 360
		} else if d < -180 {
			d += 360
		}
		sum += d * d
		n++
	}
	if n == 0 {
		return 0
	}
	variance := sum / float64(n)
	if variance > 32400 {
		variance = 32400
	}
	return math.Sqrt(variance)
}

// sortFloats sorts ascending (insertion sort; sample sizes are small).
func sortFloats(values []float64) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// IsDarkColor reports whether a hex color is perceptually dark.
func IsDarkColor(hex string) bool {
	if !color.IsHexColor(hex) {
		return false
	}
	return color.HexToOKLCH(hex).L < 0.5
}

// IsMonochromeImage reports whether a palette is perceptually grayscale
// (every color within a narrow hue band at low chroma).
func IsMonochromeImage(colors []string) bool {
	if len(colors) == 0 {
		return true
	}
	var chromaSum float64
	var count int
	for _, c := range colors {
		if !color.IsHexColor(c) {
			continue
		}
		chromaSum += color.HexToOKLCH(c).C
		count++
	}
	if count == 0 {
		return true
	}
	return chromaSum/float64(count) < 0.05
}

package extraction

import (
	"fmt"

	"aether/internal/color"
	"aether/internal/wallpaper"
)

// maxSamplePixels bounds per-image sampling work.
const maxSamplePixels = 240000

// ExtractColors extracts a 16-color palette from an image using the named
// mode. Results are content-hashed and cached, so repeated calls for the
// same (path, lightMode, mode) tuple are cheap.
func ExtractColors(path string, lightMode bool, mode string) ([16]string, error) {
	if !wallpaper.IsImageFile(path) {
		return [16]string{}, fmt.Errorf("unsupported image file: %s", path)
	}
	generator, ok := modeGenerators[mode]
	if !ok {
		return [16]string{}, fmt.Errorf("unknown extraction mode: %s", mode)
	}

	key, err := cacheKey(path, lightMode, mode)
	if err == nil {
		if palette, ok := cacheLookup(key); ok {
			return palette, nil
		}
	}

	samples, err := sampleImage(path, maxSamplePixels)
	if err != nil {
		return [16]string{}, err
	}
	stats := analyzeSamples(samples)
	palette := generatePalette(stats, lightMode, generator)

	if key != "" {
		cacheStore(key, palette)
	}
	return palette, nil
}

// generatePalette guards against nil generators.
func generatePalette(stats ImageStats, lightMode bool, generator func(ImageStats, bool) [16]string) [16]string {
	if generator == nil {
		generator = normalMode
	}
	palette := generator(stats, lightMode)
	// Ensure a strict background/foreground relationship even for odd
	// samples: swap the extremes when a light palette was produced in dark
	// mode or vice versa.
	if !lightMode && color.HexToOKLCH(palette[0]).L > color.HexToOKLCH(palette[7]).L {
		palette[0], palette[7] = palette[7], palette[0]
	}
	if lightMode && color.HexToOKLCH(palette[0]).L < color.HexToOKLCH(palette[7]).L {
		palette[0], palette[7] = palette[7], palette[0]
	}
	return palette
}

// ExtractColorsFromImages blends palettes extracted from multiple images
// (the main wallpaper plus additional images). Unreadable or unsupported
// files are skipped and counted. Requires at least one readable image.
func ExtractColorsFromImages(paths []string, lightMode bool, mode string) ([16]string, int, error) {
	generator, ok := modeGenerators[mode]
	if !ok {
		return [16]string{}, 0, fmt.Errorf("unknown extraction mode: %s", mode)
	}

	skipped := 0
	var samples []Sample
	for _, path := range paths {
		if path == "" || !wallpaper.IsImageFile(path) {
			skipped++
			continue
		}
		these, err := sampleImage(path, maxSamplePixels/2)
		if err != nil {
			skipped++
			continue
		}
		samples = append(samples, these...)
	}
	if len(samples) == 0 {
		return [16]string{}, skipped, fmt.Errorf("no readable images in %d file(s)", len(paths))
	}

	stats := analyzeSamples(samples)
	palette := generatePalette(stats, lightMode, generator)
	return palette, skipped, nil
}

// validExtractionModes lists the supported mode names.
func validExtractionModes() []string {
	return []string{
		"normal", "monochromatic", "analogous", "pastel", "material", "colorful",
		"muted", "bright", "complementary", "triadic", "split-complementary",
		"tetradic", "fire", "ocean", "forest", "earthtone", "neon", "sunset",
		"vaporwave", "midnight", "aurora", "high-contrast", "duotone",
	}
}

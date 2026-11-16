package wallpaper

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"aether/internal/platform"
)

// WritePreview renders a horizontal strip of palette colors as PNG at path.
func WritePreview(path string, palette []string) error {
	const (
		swatchW = 64
		swatchH = 128
	)
	img := image.NewRGBA(image.Rect(0, 0, swatchW*len(palette), swatchH))
	for i, hex := range palette {
		r, g, b, ok := parseHexChannels(hex)
		if !ok {
			continue
		}
		swatch := image.NewRGBA(image.Rect(0, 0, swatchW, swatchH))
		draw.Draw(swatch, swatch.Bounds(), &image.Uniform{C: color.RGBA{R: r, G: g, B: b, A: 0xff}}, image.Point{}, draw.Src)
		draw.Draw(img, swatch.Bounds().Add(image.Pt(i*swatchW, 0)), swatch, image.Point{}, draw.Src)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create preview: %w", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("encode preview: %w", err)
	}
	return nil
}

// parseHexChannels decodes #rgb/#rrggbb into 8-bit channels.
func parseHexChannels(hex string) (r, g, b byte, ok bool) {
	h := strings.TrimPrefix(hex, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 {
		return 0, 0, 0, false
	}
	var v uint32
	for i := 0; i < 6; i++ {
		c := h[i]
		var d uint32
		switch {
		case c >= '0' && c <= '9':
			d = uint32(c - '0')
		case c >= 'a' && c <= 'f':
			d = uint32(c-'a') + 10
		case c >= 'A' && c <= 'F':
			d = uint32(c-'A') + 10
		default:
			return 0, 0, 0, false
		}
		v = v<<4 | d
	}
	return byte(v >> 16), byte(v >> 8), byte(v), true
}

// cacheKey builds a stable cache filename for a source image and size.
func cacheKey(path string, width int) string {
	info, err := os.Stat(path)
	suffix := ""
	if err == nil {
		suffix = fmt.Sprintf("%d-%d", info.Size(), info.ModTime().Unix())
	}
	sum := sha256.Sum256([]byte(path + suffix))
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return fmt.Sprintf("%s-%s-%d", sanitizeName(base), fmt.Sprintf("%x", sum[:])[:10], width)
}

// resizeToFit scales img to at most width wide (preserving aspect).
func resizeToFit(img image.Image, width int) image.Image {
	b := img.Bounds()
	if b.Dx() <= width || b.Dx() == 0 {
		return img
	}
	h := b.Dy() * width / b.Dx()
	if h < 1 {
		h = 1
	}
	out := image.NewRGBA(image.Rect(0, 0, width, h))
	for y := 0; y < h; y++ {
		sy := b.Min.Y + y*b.Dy()/h
		for x := 0; x < width; x++ {
			sx := b.Min.X + x*b.Dx()/width
			out.Set(x, y, img.At(sx, sy))
		}
	}
	return out
}

// GetThumbnail returns a small cached JPEG preview of the image.
func GetThumbnail(path string) (string, error) {
	return generateScaled(path, 256, false)
}

// GetPreview returns an 800px-wide cached preview of the image.
func GetPreview(path string) (string, error) {
	return generateScaled(path, 800, false)
}

// IsPreviewCached reports whether an 800px preview already exists and is
// fresh for the source image.
func IsPreviewCached(path string) bool {
	target := filepath.Join(platform.ThumbnailDir(), cacheKey(path, 800)+".jpg")
	if info, err := os.Stat(target); err != nil || info.IsDir() {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	cached, err := os.Stat(target)
	if err != nil {
		return false
	}
	return cached.ModTime().After(info.ModTime())
}

func generateScaled(path string, width int, checkOnly bool) (string, error) {
	if !isImageFile(path) {
		return "", fmt.Errorf("unsupported image file: %s", path)
	}
	if err := os.MkdirAll(platform.ThumbnailDir(), 0o755); err != nil {
		return "", fmt.Errorf("create thumbnail dir: %w", err)
	}
	target := filepath.Join(platform.ThumbnailDir(), cacheKey(path, width)+".jpg")
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		if src, serr := os.Stat(path); serr != nil || info.ModTime().After(src.ModTime()) {
			return target, nil
		}
	}

	img, err := DecodeImage(path)
	if err != nil {
		return "", err
	}
	scaled := resizeToFit(img, width)
	if err := WriteJPEG(target, scaled, 80); err != nil {
		return "", err
	}
	return target, nil
}

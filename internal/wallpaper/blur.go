package wallpaper

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"os"
	"path/filepath"
	"strings"

	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// CreateBlurredVariant renders a heavily blurred JPEG copy of path inside
// dir and returns its path. The result is cached: repeated calls for the
// same input reuse the previously generated file. The cache name embeds a
// content hash so edited images produce fresh variants.
func CreateBlurredVariant(path, dir string) (string, error) {
	if !isImageFile(path) {
		return "", fmt.Errorf("unsupported image file: %s", path)
	}
	if err := ValidateImageFile(path); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create blur dir: %w", err)
	}

	img, err := DecodeImage(path)
	if err != nil {
		return "", err
	}

	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	key := fmt.Sprintf("%x", sha256.Sum256([]byte(path)))[:8]
	out := filepath.Join(dir, fmt.Sprintf("%s-blurred-%s.jpg", base, key))
	if info, err := os.Stat(out); err == nil && !info.IsDir() {
		return out, nil
	}

	blurred := BlurImage(img, 24)
	if err := WriteJPEG(out, blurred, 85); err != nil {
		return "", err
	}
	return out, nil
}

// BlurImage produces a downscaled, box-blurred copy of img approximating a
// strong gaussian blur, suitable for wallpaper backdrops.
func BlurImage(img image.Image, radius int) image.Image {
	b := img.Bounds()
	// Downscale to ~10% for a cheap but convincing blur, then upscale.
	smallW := b.Dx() / 10
	smallH := b.Dy() / 10
	if smallW < 8 {
		smallW = 8
	}
	if smallH < 8 {
		smallH = 8
	}
	if smallW > b.Dx() {
		smallW = b.Dx()
	}
	if smallH > b.Dy() {
		smallH = b.Dy()
	}

	small := image.NewRGBA(image.Rect(0, 0, smallW, smallH))
	for y := 0; y < smallH; y++ {
		for x := 0; x < smallW; x++ {
			sx := b.Min.X + x*b.Dx()/smallW
			sy := b.Min.Y + y*b.Dy()/smallH
			r, g, bl, a := img.At(sx, sy).RGBA()
			small.SetRGBA(x, y, clampRGBA(r, g, bl, a))
		}
	}

	// Box blur passes on the small image.
	blurred := boxBlurRGBA(small, 2)
	blurred = boxBlurRGBA(blurred, 2)

	// Upscale back with bilinear-ish sampling.
	upscaled := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		fy := float64(y) * float64(smallH) / float64(b.Dy())
		y0 := int(fy)
		y1 := y0 + 1
		if y1 >= smallH {
			y1 = smallH - 1
		}
		wy := fy - float64(y0)
		for x := 0; x < b.Dx(); x++ {
			fx := float64(x) * float64(smallW) / float64(b.Dx())
			x0 := int(fx)
			x1 := x0 + 1
			if x1 >= smallW {
				x1 = smallW - 1
			}
			wx := fx - float64(x0)

			c00 := blurred.RGBAAt(x0, y0)
			c10 := blurred.RGBAAt(x1, y0)
			c01 := blurred.RGBAAt(x0, y1)
			c11 := blurred.RGBAAt(x1, y1)
			upscaled.SetRGBA(x, y, color.RGBA{
				R: lerpByte(lerpByte(c00.R, c10.R, wx), lerpByte(c01.R, c11.R, wx), wy),
				G: lerpByte(lerpByte(c00.G, c10.G, wx), lerpByte(c01.G, c11.G, wx), wy),
				B: lerpByte(lerpByte(c00.B, c10.B, wx), lerpByte(c01.B, c11.B, wx), wy),
				A: 0xff,
			})
		}
	}
	return upscaled
}

// boxBlurRGBA applies one separable box blur pass with the given radius.
func boxBlurRGBA(img *image.RGBA, radius int) *image.RGBA {
	b := img.Bounds()
	out := image.NewRGBA(b)
	w := b.Dx()
	h := b.Dy()
	if w == 0 || h == 0 {
		return out
	}

	// Horizontal pass.
	tmp := image.NewRGBA(b)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, bl, n int
			for dx := -radius; dx <= radius; dx++ {
				sx := x + dx
				if sx < 0 {
					sx = 0
				}
				if sx >= w {
					sx = w - 1
				}
				c := img.RGBAAt(sx, y)
				r += int(c.R)
				g += int(c.G)
				bl += int(c.B)
				n++
			}
			tmp.SetRGBA(x, y, color.RGBA{
				R: byte(r / n), G: byte(g / n), B: byte(bl / n), A: 0xff,
			})
		}
	}

	// Vertical pass.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var r, g, bl, n int
			for dy := -radius; dy <= radius; dy++ {
				sy := y + dy
				if sy < 0 {
					sy = 0
				}
				if sy >= h {
					sy = h - 1
				}
				c := tmp.RGBAAt(x, sy)
				r += int(c.R)
				g += int(c.G)
				bl += int(c.B)
				n++
			}
			out.SetRGBA(x, y, color.RGBA{
				R: byte(r / n), G: byte(g / n), B: byte(bl / n), A: 0xff,
			})
		}
	}
	return out
}

func lerpByte(a, b byte, t float64) byte {
	av := float64(a)
	bv := float64(b)
	v := av + (bv-av)*t
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return byte(v + 0.5)
}

func clampRGBA(r, g, b, a uint32) (out color.RGBA) {
	out.R = byte(r >> 8)
	out.G = byte(g >> 8)
	out.B = byte(b >> 8)
	out.A = byte(a >> 8)
	return
}

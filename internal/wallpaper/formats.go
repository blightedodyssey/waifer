// Package wallpaper handles image validation, local scanning, thumbnails,
// blurred variants and remote downloads.
package wallpaper

import (
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"

	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"
)

// MaxImageBytes bounds wallpaper downloads (32 MiB).
const MaxImageBytes int64 = 32 << 20

// MaxDocumentBytes bounds theme document downloads (2 MiB).
const MaxDocumentBytes int64 = 2 << 20

// ValidateImageFile decodes the file header to prove it is a readable image.
func ValidateImageFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open image: %w", err)
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return fmt.Errorf("not a readable image: %w", err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("image has invalid dimensions")
	}
	return nil
}

// DecodeImage loads a file into a decoded image.
func DecodeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image: %w", err)
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	return img, nil
}

// WriteJPEG encodes img as JPEG with the given quality (1-100).
func WriteJPEG(path string, img image.Image, quality int) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: quality}); err != nil {
		return fmt.Errorf("encode JPEG: %w", err)
	}
	return nil
}

// imageExtensions is the accepted set of image file extensions.
var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
	".gif": true, ".bmp": true,
}

// isImageFile reports whether path has a supported image extension.
func isImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return imageExtensions[ext]
}

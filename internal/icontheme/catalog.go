package icontheme

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// maxPreviewBytes bounds a single decoded preview image (4 MiB).
const maxPreviewBytes = 4 << 20

// Catalog scans icon roots for installed themes and rasterizes previews.
type Catalog struct {
	mu      sync.RWMutex
	roots   []Root
	summary []ThemeSummary
}

// NewCatalog builds a catalog over the default roots.
func NewCatalog() *Catalog {
	return NewCatalogWithRoots(BuildRoots())
}

// NewCatalogWithRoots builds a catalog over explicit roots (tests).
func NewCatalogWithRoots(roots []Root) *Catalog {
	return &Catalog{roots: roots}
}

// List returns the cached theme list, scanning first if needed.
func (c *Catalog) List(ctx context.Context) ([]ThemeSummary, error) {
	c.mu.RLock()
	cached := c.summary
	c.mu.RUnlock()
	if cached != nil {
		return cached, nil
	}
	return c.Refresh(ctx)
}

// Refresh rescans the roots and returns the fresh list.
func (c *Catalog) Refresh(ctx context.Context) ([]ThemeSummary, error) {
	summary, err := c.scanCatalog(ctx)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.summary = summary
	c.mu.Unlock()
	return summary, nil
}

// Preview rasterizes sample icons for themeID into PNG data URLs.
func (c *Catalog) Preview(ctx context.Context, themeID string) (ThemePreview, error) {
	if !ValidateID(themeID) {
		return ThemePreview{}, fmt.Errorf("invalid icon theme ID %q", themeID)
	}
	samples := make([]string, 0, 4)
	for _, root := range c.roots {
		themeDir := filepath.Join(root.Path, themeID)
		if info, err := os.Stat(themeDir); err != nil || !info.IsDir() {
			continue
		}
		for _, sub := range []string{"48x48/apps", "48x48", "scalable/apps", "48x48/devices", "48x48/places"} {
			icons, err := iconSamples(filepath.Join(themeDir, sub), 4)
			if err != nil {
				continue
			}
			samples = append(samples, icons...)
			if len(samples) >= 4 {
				break
			}
		}
		if len(samples) >= 4 {
			break
		}
	}
	if len(samples) == 0 {
		return ThemePreview{ID: themeID, ThemeID: themeID, Samples: []string{}}, nil
	}
	if len(samples) > 4 {
		samples = samples[:4]
	}
	return ThemePreview{ID: themeID, ThemeID: themeID, Samples: samples}, nil
}

// scanCatalog walks the roots and summarizes every theme directory.
func (c *Catalog) scanCatalog(ctx context.Context) ([]ThemeSummary, error) {
	seen := make(map[string]bool)
	out := []ThemeSummary{}
	for _, root := range c.roots {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entries, err := os.ReadDir(root.Path)
		if err != nil {
			continue // root missing
		}
		for _, entry := range entries {
			if !entry.IsDir() || entry.Name() == "default" || seen[entry.Name()] {
				continue
			}
			dir := filepath.Join(root.Path, entry.Name())
			if _, err := os.Stat(filepath.Join(dir, "index.theme")); err != nil {
				continue // not a theme
			}
			meta := parseThemeMetadata(filepath.Join(dir, "index.theme"))
			seen[entry.Name()] = true
			out = append(out, ThemeSummary{
				ID:         entry.Name(),
				Name:       safeDisplayName(entry.Name(), meta),
				HasPreview: findConceptImage(dir) != "",
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// safeDisplayName returns the theme's Name field, falling back to the ID.
func safeDisplayName(id string, meta map[string]string) string {
	if name := meta["Name"]; name != "" {
		return name
	}
	return id
}

// findConceptImage locates a theme preview image (preview.png style).
func findConceptImage(dir string) string {
	candidates := []string{"preview.png", "preview.jpg", "preview.svg"}
	for _, name := range candidates {
		p := filepath.Join(dir, name)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

// parseThemeMetadata extracts key=value pairs from the [Icon Theme] section
// of an index.theme file.
func parseThemeMetadata(path string) map[string]string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	meta := make(map[string]string)
	scanner := bufio.NewScanner(f)
	inSection := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			inSection = line == "[Icon Theme]"
			continue
		}
		if !inSection {
			continue
		}
		eq := strings.Index(line, "=")
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		meta[key] = strings.TrimSpace(line[eq+1:])
	}
	return meta
}

// iconSamples rasterizes up to maxCount regular icons in dir as PNG data URLs.
func iconSamples(dir string, maxCount int) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	out := make([]string, 0, maxCount)
	for _, entry := range entries {
		if entry.IsDir() || len(out) >= maxCount {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" {
			continue
		}
		data, err := readBoundedRegularFile(filepath.Join(dir, entry.Name()), maxPreviewBytes)
		if err != nil {
			continue
		}
		if dataURL, err := encodePNGDataURL(data); err == nil {
			out = append(out, dataURL)
		}
	}
	return out, nil
}

// readBoundedRegularFile reads at most limit bytes of a regular file.
func readBoundedRegularFile(path string, limit int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() || info.Size() > limit {
		return nil, fmt.Errorf("file too large or not regular: %s", path)
	}
	return os.ReadFile(path)
}

// encodePNGDataURL re-encodes image bytes as a PNG data URL, normalizing
// formats and bounding the decoded size.
func encodePNGDataURL(data []byte) (string, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	b := img.Bounds()
	if b.Dx() > 256 || b.Dy() > 256 {
		img = downscaleIcon(img, 128)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64Encode(buf.Bytes()), nil
}

// downscaleIcon halves an image until it fits within the target size.
func downscaleIcon(img image.Image, target int) image.Image {
	b := img.Bounds()
	for b.Dx() > target || b.Dy() > target {
		small := image.NewRGBA(image.Rect(0, 0, b.Dx()/2, b.Dy()/2))
		for y := 0; y < small.Bounds().Dy(); y++ {
			for x := 0; x < small.Bounds().Dx(); x++ {
				small.Set(x, y, img.At(b.Min.X+x*2, b.Min.Y+y*2))
			}
		}
		img = small
		b = img.Bounds()
	}
	return img
}

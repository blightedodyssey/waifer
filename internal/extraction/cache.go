package extraction

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"aether/internal/platform"
)

// cachedPalette is a memoized extraction result.
type cachedPalette struct {
	Palette [16]string
}

// cacheTTL caps cache freshness by wall clock (10 minutes).
const cacheTTLSeconds = 10 * 60

// memoryCache is the in-process memoization layer.
var memoryCache = struct {
	sync.Mutex
	entries map[string]cachedPalette
}{entries: map[string]cachedPalette{}}

// cacheKey hashes the tuple (content, lightMode, mode).
func cacheKey(path string, lightMode bool, mode string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintf(h, "%d:%d:%v:%s", info.Size(), info.ModTime().Unix(), lightMode, mode)
	return fmt.Sprintf("%s-%s.json", filepath.Base(path), fmt.Sprintf("%x", h.Sum(nil))[:16]), nil
}

// diskCacheDir is the palette cache directory.
func diskCacheDir() string {
	return platform.ColorCacheDir()
}

// cacheLookup consults the memory cache then the disk cache.
func cacheLookup(key string) ([16]string, bool) {
	memoryCache.Lock()
	if entry, ok := memoryCache.entries[key]; ok {
		memoryCache.Unlock()
		return entry.Palette, true
	}
	memoryCache.Unlock()

	data, err := os.ReadFile(filepath.Join(diskCacheDir(), key))
	if err != nil {
		return [16]string{}, false
	}
	if stale(data) {
		_ = os.Remove(filepath.Join(diskCacheDir(), key))
		return [16]string{}, false
	}
	var palette [16]string
	if err := unmarshalPalette(data, &palette); err != nil {
		return [16]string{}, false
	}
	memoryCache.Lock()
	memoryCache.entries[key] = cachedPalette{Palette: palette}
	memoryCache.Unlock()
	return palette, true
}

// cacheStore writes the palette to both cache layers.
func cacheStore(key string, palette [16]string) {
	memoryCache.Lock()
	memoryCache.entries[key] = cachedPalette{Palette: palette}
	memoryCache.Unlock()

	if err := platform.EnsureDir(diskCacheDir()); err != nil {
		return
	}
	data, err := marshalPalette(palette)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(diskCacheDir(), key), data, 0o644)
}

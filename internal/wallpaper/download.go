package wallpaper

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aether/internal/platform"
)

// httpClient is the shared download client with a sane timeout.
var httpClient = &http.Client{Timeout: 2 * time.Minute}

// DownloadToCache fetches url into the cache download directory and returns
// the local path. The transfer is bounded by maxBytes; the file is staged in
// a .part-* temp name and renamed into place only after a complete,
// verified download.
func DownloadToCache(rawURL string, maxBytes int64) (string, error) {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return "", fmt.Errorf("only http(s) URLs are supported: %s", rawURL)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse URL: %w", err)
	}

	dir := platform.DownloadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create download dir: %w", err)
	}

	name := sanitizeName(parsed.Path)
	// A content-addressed prefix keeps distinct files apart while letting
	// identical URLs reuse the cached copy.
	sum := sha256.Sum256([]byte(rawURL))
	local := filepath.Join(dir, fmt.Sprintf("%s-%s", fmt.Sprintf("%x", sum[:])[:10], name))

	if info, err := os.Stat(local); err == nil && !info.IsDir() {
		return local, nil // already cached
	}

	resp, err := httpClient.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download %s: HTTP %d", rawURL, resp.StatusCode)
	}

	tmp, err := os.CreateTemp(dir, ".part-*")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}

	written, err := io.Copy(tmp, io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		cleanup()
		return "", fmt.Errorf("download %s: %w", rawURL, err)
	}
	if written > maxBytes {
		cleanup()
		return "", fmt.Errorf("download %s: exceeds %d byte limit", rawURL, maxBytes)
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", fmt.Errorf("finalize download: %w", err)
	}
	if err := os.Rename(tmpName, local); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("finalize download: %w", err)
	}
	return local, nil
}

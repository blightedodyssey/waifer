// Package githubsource lists public GitHub repository directories as
// wallpaper sources and fetches bounded cached previews.
package githubsource

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"aether/internal/wallpaper"
)

// apiBase is the GitHub REST root.
const apiBase = "https://api.github.com"

// httpClient is the shared API client.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// ghEntry is one row of the contents API response.
type ghEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	DownloadURL string `json:"download_url"`
	Size        int64  `json:"size"`
}

// ListContentsResult is the Wails-friendly listing.
type ListContentsResult struct {
	URL    string             `json:"url"`
	Images []ListContentsItem `json:"images"`
}

// ListContentsItem is one downloadable image in a repository directory.
type ListContentsItem struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	DownloadURL string `json:"downloadURL"`
	Size        int64  `json:"size"`
}

// ThumbnailResult carries a cached local preview path.
type ThumbnailResult struct {
	Path string `json:"path"`
}

// Client lists repository directories.
type Client struct{}

// NewClient returns a GitHub source client.
func NewClient() *Client {
	return &Client{}
}

// parseRepoDir reduces a GitHub URL (repo root, /tree/branch/path or raw API
// URL) to an owner/repo pair plus subdirectory and branch.
func parseRepoDir(raw string) (repo, dir, ref string, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", "", fmt.Errorf("empty URL")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", "", fmt.Errorf("parse URL: %w", err)
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("URL is not a GitHub repository: %s", raw)
	}
	repo = parts[0] + "/" + parts[1]
	rest := parts[2:]
	if len(rest) > 0 && rest[0] == "tree" {
		if len(rest) >= 2 {
			ref = rest[1]
		}
		if len(rest) >= 3 {
			dir = strings.Join(rest[2:], "/")
		}
	}
	return repo, dir, ref, nil
}

// ListImages lists the image files in a public repository directory.
// Results are memoized per URL so repeated refreshes stay cheap.
func (c *Client) ListImages(rawURL string) (*ListContentsResult, error) {
	if cached, ok := cachedListImages(rawURL); ok {
		return cached, nil
	}
	repo, dir, ref, err := parseRepoDir(rawURL)
	if err != nil {
		return nil, err
	}

	api := fmt.Sprintf("%s/repos/%s/contents/%s", apiBase, repo, dir)
	if ref != "" {
		api += "?ref=" + url.QueryEscape(ref)
	}
	req, err := http.NewRequest(http.MethodGet, api, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "aether")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("list GitHub contents: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("GitHub directory not found: %s", rawURL)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list GitHub contents: HTTP %d", resp.StatusCode)
	}
	var entries []ghEntry
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&entries); err != nil {
		return nil, fmt.Errorf("list GitHub contents: %w", err)
	}

	result := &ListContentsResult{URL: rawURL, Images: []ListContentsItem{}}
	for _, entry := range entries {
		if entry.Type != "file" || !isWallpaperFile(entry.Name) {
			continue
		}
		result.Images = append(result.Images, ListContentsItem{
			Name:        entry.Name,
			Path:        entry.Path,
			DownloadURL: entry.DownloadURL,
			Size:        entry.Size,
		})
	}
	storeListing(rawURL, result)
	return result, nil
}

// DownloadThumbnail downloads a remote image (typically one of the entries
// above) into the thumbnail cache as a bounded preview.
func DownloadThumbnail(rawURL string) (*ThumbnailResult, error) {
	if rawURL == "" {
		return nil, fmt.Errorf("empty image URL")
	}
	local, err := wallpaper.DownloadToCache(rawURL, wallpaper.MaxImageBytes)
	if err != nil {
		return nil, err
	}
	if err := wallpaper.ValidateImageFile(local); err != nil {
		return nil, err
	}
	return &ThumbnailResult{Path: local}, nil
}

// isWallpaperFile matches supported image extensions.
func isWallpaperFile(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".webp", ".bmp"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

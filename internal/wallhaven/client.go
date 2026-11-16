// Package wallhaven is a thin client for the wallhaven.cc search API plus
// bounded image downloads into Aether's cache.
package wallhaven

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"aether/internal/platform"
	"aether/internal/wallpaper"
)

// apiBase is the wallhaven REST root.
const apiBase = "https://wallhaven.cc/api/v1"

// pageURLPrefix identifies wallhaven detail pages.
const pageURLPrefix = "wallhaven.cc/w/"

// Client talks to wallhaven.cc.
type Client struct {
	apiKey string
	http   *http.Client
}

// NewClient returns a client that picks up a saved API key from
// ~/.config/aether/wallhaven.json when present.
func NewClient() *Client {
	c := &Client{http: &http.Client{Timeout: 2 * time.Minute}}
	if cfg := readSavedConfig(); cfg["apiKey"] != "" {
		c.apiKey, _ = cfg["apiKey"].(string)
	}
	return c
}

// readSavedConfig loads the app's wallhaven settings.
func readSavedConfig() map[string]interface{} {
	v, err := platform.ReadJSON[map[string]interface{}](filepath.Join(platform.ConfigDir(), "wallhaven.json"))
	if err != nil {
		return nil
	}
	return v
}

// SetAPIKey stores a key for this client's lifetime.
func (c *Client) SetAPIKey(key string) {
	c.apiKey = strings.TrimSpace(key)
}

// SearchParams are the supported wallhaven query parameters.
type SearchParams struct {
	Query      string `json:"q,omitempty"`
	Categories string `json:"categories,omitempty"`
	Purity     string `json:"purity,omitempty"`
	Sorting    string `json:"sorting,omitempty"`
	Order      string `json:"order,omitempty"`
	Page       int    `json:"page,omitempty"`
	AtLeast    string `json:"atLeast,omitempty"`
	Colors     string `json:"colors,omitempty"`
}

// Thumb holds one thumbnail URL variant. The JSON shape is
// {"large": "...", "original": "...", "small": "..."}.
type Thumbs map[string]string

// Wallpaper is one search result.
type Wallpaper struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	Path       string `json:"path"`
	Resolution string `json:"resolution"`
	Category   string `json:"category"`
	Purity     string `json:"purity"`
	Thumbs     Thumbs `json:"thumbs,omitempty"`
}

// SearchMeta carries pagination info.
type SearchMeta struct {
	CurrentPage int `json:"current_page"`
	LastPage    int `json:"last_page"`
	Total       int `json:"total"`
}

// SearchResult is the API envelope.
type SearchResult struct {
	Data []Wallpaper `json:"data"`
	Meta SearchMeta  `json:"meta"`
}

// InfoResult is the single-wallpaper metadata response.
type InfoResult struct {
	ID         string `json:"id"`
	URL        string `json:"url"`
	Path       string `json:"path"`
	Resolution string `json:"resolution"`
	Category   string `json:"category"`
	Purity     string `json:"purity"`
	Thumbs     Thumbs `json:"thumbs,omitempty"`
}

// Search executes one search page.
func (c *Client) Search(params SearchParams) (*SearchResult, error) {
	return c.SearchMultiPage(params, 1)
}

// SearchMultiPage fetches up to pages consecutive pages starting at
// params.Page (or 1) and merges the results.
func (c *Client) SearchMultiPage(params SearchParams, pages int) (*SearchResult, error) {
	if pages < 1 {
		pages = 1
	}
	start := params.Page
	if start < 1 {
		start = 1
	}

	merged := &SearchResult{Data: []Wallpaper{}}
	for page := start; page < start+pages; page++ {
		single := params
		single.Page = page
		result, err := c.searchPage(single)
		if err != nil {
			return merged, err
		}
		merged.Data = append(merged.Data, result.Data...)
		merged.Meta = result.Meta
		if page >= result.Meta.LastPage {
			break
		}
	}
	return merged, nil
}

// searchPage performs a single API call.
func (c *Client) searchPage(params SearchParams) (*SearchResult, error) {
	query := url.Values{}
	if params.Query != "" {
		query.Set("q", params.Query)
	}
	if params.Categories != "" {
		query.Set("categories", params.Categories)
	}
	if params.Purity != "" {
		query.Set("purity", params.Purity)
	}
	if params.Sorting != "" {
		query.Set("sorting", params.Sorting)
	}
	if params.Order != "" {
		query.Set("order", params.Order)
	}
	if params.AtLeast != "" {
		query.Set("atleast", params.AtLeast)
	}
	if params.Colors != "" {
		query.Set("colors", params.Colors)
	}
	page := params.Page
	if page < 1 {
		page = 1
	}
	query.Set("page", strconv.Itoa(page))

	endpoint := apiBase + "/search?" + query.Encode()
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "aether")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wallhaven search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wallhaven search: HTTP %d", resp.StatusCode)
	}
	var result SearchResult
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("wallhaven search: %w", err)
	}
	return &result, nil
}

// Info fetches metadata for a single wallpaper ID.
func (c *Client) Info(id string) (*InfoResult, error) {
	id = extractID(id)
	if id == "" {
		return nil, fmt.Errorf("wallhaven: missing wallpaper id")
	}
	endpoint := apiBase + "/w/" + id
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "aether")
	if c.apiKey != "" {
		req.Header.Set("X-API-Key", c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wallhaven info: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("wallhaven info: HTTP %d", resp.StatusCode)
	}
	var wrapped struct {
		Data InfoResult `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&wrapped); err != nil {
		return nil, fmt.Errorf("wallhaven info: %w", err)
	}
	return &wrapped.Data, nil
}

// Download fetches a wallpaper image URL into the cache and returns the
// local path.
func (c *Client) Download(imageURL string) (string, error) {
	return c.DownloadFromURL(imageURL)
}

// pageURLPattern matches a wallhaven detail page URL.
var pageURLPattern = regexp.MustCompile(`wallhaven\.cc/w/([A-Za-z0-9]+)`)

// directImagePattern matches a full-size image URL carrying the wallpaper id.
var directImagePattern = regexp.MustCompile(`(?i)wallhaven\.cc/full/([a-z0-9]{2})/wallhaven-([a-z0-9]+)`)

// extractID reduces bare ids, page URLs and full-size image URLs to the
// canonical wallpaper ID.
func extractID(target string) string {
	target = strings.TrimSpace(target)
	if m := pageURLPattern.FindStringSubmatch(target); m != nil {
		return m[1]
	}
	if m := directImagePattern.FindStringSubmatch(target); m != nil {
		return m[2]
	}
	if !strings.ContainsAny(target, "/.") {
		return target // bare ID
	}
	return ""
}

// DownloadFromURL resolves a wallhaven reference (bare ID, page URL or image
// URL) to a full-size image and downloads it into the cache.
func (c *Client) DownloadFromURL(target string) (string, error) {
	target = strings.TrimSpace(target)
	imageURL := target
	if strings.Contains(target, pageURLPrefix) || !strings.HasPrefix(target, "http") {
		id := extractID(target)
		if id == "" {
			return "", fmt.Errorf("wallhaven: cannot resolve %q", target)
		}
		info, err := c.Info(id)
		if err != nil {
			return "", err
		}
		if info.Path == "" {
			return "", fmt.Errorf("wallhaven: no image path for %s", id)
		}
		imageURL = info.Path
	}

	local, err := wallpaper.DownloadToCache(imageURL, wallpaper.MaxImageBytes)
	if err != nil {
		return "", err
	}
	if err := wallpaper.ValidateImageFile(local); err != nil {
		_ = os.Remove(local)
		return "", fmt.Errorf("wallhaven: %w", err)
	}
	return local, nil
}

// DownloadThumb downloads a thumbnail image into the thumbnail cache.
func (c *Client) DownloadThumb(thumbURL string) (string, error) {
	if thumbURL == "" {
		return "", fmt.Errorf("wallhaven: empty thumbnail URL")
	}
	local, err := wallpaper.DownloadToCache(thumbURL, 4<<20)
	if err != nil {
		return "", err
	}
	if err := wallpaper.ValidateImageFile(local); err != nil {
		_ = os.Remove(local)
		return "", fmt.Errorf("wallhaven thumb: %w", err)
	}
	return local, nil
}

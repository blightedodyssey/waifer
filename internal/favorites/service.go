// Package favorites stores the user's favorited wallpapers (local paths and
// wallhaven URLs) in a single JSON file.
package favorites

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"aether/internal/platform"
)

// Favorite is one favorited wallpaper. Data carries free-form metadata
// (wallhaven id, resolution, name, ...) that the export flow re-attaches.
type Favorite struct {
	Path string                 `json:"path"`
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// favoritesFile is the backing store.
func favoritesFile() string {
	return platform.FavoritesFile()
}

// Service reads and writes the favorites store.
type Service struct{}

// NewService returns a favorites service.
func NewService() *Service {
	return &Service{}
}

// GetAll returns all favorites sorted by path.
func (s *Service) GetAll() []Favorite {
	raw, err := load()
	if err != nil {
		return []Favorite{}
	}
	out := append([]Favorite(nil), raw...)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Toggle adds a favorite when missing and removes it when present.
// Returns the new favorite state.
func (s *Service) Toggle(path, favType string, data map[string]interface{}) bool {
	all, err := load()
	if err != nil {
		all = []Favorite{}
	}
	for i, fav := range all {
		if fav.Path == path {
			all = append(all[:i], all[i+1:]...)
			if err := save(all); err != nil {
				fmt.Fprintf(os.Stderr, "favorites: %v\n", err)
			}
			return false
		}
	}
	if data == nil {
		data = map[string]interface{}{}
	}
	all = append(all, Favorite{Path: path, Type: favType, Data: data})
	if err := save(all); err != nil {
		fmt.Fprintf(os.Stderr, "favorites: %v\n", err)
	}
	return true
}

// IsFavorite reports whether path is favorited.
func (s *Service) IsFavorite(path string) bool {
	all, err := load()
	if err != nil {
		return false
	}
	for _, fav := range all {
		if fav.Path == path {
			return true
		}
	}
	return false
}

// load reads the store file.
func load() ([]Favorite, error) {
	data, err := os.ReadFile(favoritesFile())
	if err != nil {
		if os.IsNotExist(err) {
			return []Favorite{}, nil
		}
		return nil, fmt.Errorf("read favorites: %w", err)
	}
	var all []Favorite
	if err := json.Unmarshal(data, &all); err != nil {
		return []Favorite{}, nil // corrupt store behaves as empty
	}
	return all, nil
}

// save writes the store atomically.
func save(all []Favorite) error {
	if err := platform.EnsureDir(filepath.Dir(favoritesFile())); err != nil {
		return fmt.Errorf("prepare favorites dir: %w", err)
	}
	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("encode favorites: %w", err)
	}
	return os.WriteFile(favoritesFile(), data, 0o600)
}

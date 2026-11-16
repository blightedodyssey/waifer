package blueprint

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"aether/internal/icontheme"
	"aether/internal/platform"
)

// nowUnix is a test seam for timestamps.
var nowUnix = time.Now().Unix

// Service reads and writes blueprints in the blueprint data directory.
type Service struct{}

// NewService returns a blueprint service.
func NewService() *Service {
	return &Service{}
}

// LoadAll returns every saved blueprint sorted by timestamp (newest first).
func (s *Service) LoadAll() ([]Blueprint, error) {
	dir := platform.BlueprintDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []Blueprint{}, nil
		}
		return nil, fmt.Errorf("read blueprints: %w", err)
	}

	out := make([]Blueprint, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // unreadable entries are skipped
		}
		var bp Blueprint
		if err := json.Unmarshal(data, &bp); err != nil {
			continue
		}
		if bp.Name == "" {
			bp.Name = strings.TrimSuffix(entry.Name(), ".json")
		}
		out = append(out, bp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Timestamp != out[j].Timestamp {
			return out[i].Timestamp > out[j].Timestamp
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// FindByName locates a blueprint by name (case-insensitive). Returns
// (nil, nil) when not found.
func (s *Service) FindByName(name string) (*Blueprint, error) {
	all, err := s.LoadAll()
	if err != nil {
		return nil, err
	}
	want := strings.ToLower(strings.TrimSpace(name))
	for i := range all {
		if strings.ToLower(all[i].Name) == want {
			return &all[i], nil
		}
	}
	return nil, nil
}

// Save persists a blueprint under name, replacing an existing entry.
func (s *Service) Save(name string, bp Blueprint) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("blueprint name is required")
	}
	if !ValidName(name) {
		return fmt.Errorf("blueprint name contains unsafe characters: %s", name)
	}
	bp.Name = name
	if bp.Timestamp == 0 {
		bp.Timestamp = nowUnix()
	}
	path := filepath.Join(platform.BlueprintDir(), slugify(name)+".json")
	if err := platform.WriteJSON(path, bp); err != nil {
		return fmt.Errorf("save blueprint: %w", err)
	}
	return nil
}

// Delete removes a blueprint by name.
func (s *Service) Delete(name string) error {
	bp, err := s.FindByName(name)
	if err != nil {
		return err
	}
	if bp == nil {
		return fmt.Errorf("blueprint %q not found", name)
	}
	// Re-derive the file path from the found blueprint's name.
	slug := slugify(bp.Name)
	if slug == "" {
		slug = "imported"
	}
	path := filepath.Join(platform.BlueprintDir(), slug+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete blueprint: %w", err)
	}
	return nil
}

// validNameRe rejects path separators and control characters in names.
var validNameRe = regexp.MustCompile(`^[^/\\<>:"|?*\x00-\x1f]+$`)

// ValidName reports whether name is safe for a blueprint title.
func ValidName(name string) bool {
	return name != "" && validNameRe.MatchString(name)
}

// Validate reports whether bp has the minimum valid structure: a name and at
// least one color.
func (s *Service) Validate(bp *Blueprint) bool {
	return validateBlueprint(bp)
}

// validateBlueprint is the shared structural check.
func validateBlueprint(bp *Blueprint) bool {
	if bp == nil || strings.TrimSpace(bp.Name) == "" {
		return false
	}
	if len(bp.Palette.Colors) == 0 {
		return false
	}
	// Any non-empty color values must be hex-shaped.
	for _, c := range bp.Palette.Colors {
		if c == "" {
			continue
		}
		if !strings.HasPrefix(c, "#") || len(c) != 7 {
			return false
		}
	}
	// An explicit icon theme selection must parse.
	if bp.IconTheme != nil {
		if _, err := icontheme.NormalizeSelection(*bp.IconTheme); err != nil {
			return false
		}
	}
	return true
}

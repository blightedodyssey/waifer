// Package icontheme scans installed desktop icon themes and manages the
// explicit-vs-automatic selection carried in theme state.
package icontheme

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Selection modes.
const (
	SelectionAutomatic = "automatic"
	SelectionExplicit  = "explicit"
)

// Selection is the icon theme choice stored in theme state and blueprints.
// Mode is "automatic" (Aether derives a Yaru variant) or "explicit" with a
// theme ID.
type Selection struct {
	Mode string `json:"mode"`
	ID   string `json:"id,omitempty"`
}

// Automatic returns the default selection.
func Automatic() Selection {
	return Selection{Mode: SelectionAutomatic}
}

// ValidateID reports whether themeID is a safe directory name component.
func ValidateID(themeID string) bool {
	if themeID == "" || themeID == "." || themeID == ".." {
		return false
	}
	if strings.ContainsAny(themeID, "/\\") {
		return false
	}
	return !strings.ContainsAny(themeID, "\x00")
}

// NormalizeSelection validates and normalizes a selection coming from the
// frontend or the CLI.
func NormalizeSelection(sel Selection) (Selection, error) {
	switch sel.Mode {
	case SelectionAutomatic:
		return Selection{Mode: SelectionAutomatic}, nil
	case SelectionExplicit:
		if !ValidateID(sel.ID) {
			return Selection{}, fmt.Errorf("invalid icon theme ID %q", sel.ID)
		}
		return Selection{Mode: SelectionExplicit, ID: sel.ID}, nil
	case "":
		return Selection{Mode: SelectionAutomatic}, nil
	default:
		return Selection{}, fmt.Errorf("unknown icon theme mode %q", sel.Mode)
	}
}

// Origin labels where a root came from.
type Origin string

const (
	OriginUser   Origin = "user"
	OriginSystem Origin = "system"
)

// Root is one icon-theme search root.
type Root struct {
	Path   string `json:"path"`
	Origin Origin `json:"origin"`
}

// ThemeSummary describes one installed icon theme.
type ThemeSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	HasPreview bool   `json:"hasPreview"`
}

// ThemePreview carries backend-rasterized sample icons for a theme, encoded
// as PNG data URLs.
type ThemePreview struct {
	ID      string   `json:"id"`
	ThemeID string   `json:"themeId"`
	Samples []string `json:"samples"`
}

// BuildRoots returns the icon theme search roots.
func BuildRoots() []Root {
	var roots []Root
	if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
		roots = append(roots, Root{Path: filepath.Join(dataHome, "icons"), Origin: OriginUser})
	} else if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots,
			Root{Path: filepath.Join(home, ".local", "share", "icons"), Origin: OriginUser},
			Root{Path: filepath.Join(home, ".icons"), Origin: OriginUser},
		)
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		roots = append(roots, Root{Path: filepath.Join(local, "icons"), Origin: OriginSystem})
	}
	return dedupeRoots(roots)
}

// dedupeRoots removes duplicate roots while keeping order.
func dedupeRoots(roots []Root) []Root {
	seen := make(map[string]bool, len(roots))
	out := make([]Root, 0, len(roots))
	for _, r := range roots {
		r.Path = filepath.Clean(r.Path)
		if r.Path == "" || seen[r.Path] {
			continue
		}
		seen[r.Path] = true
		out = append(out, r)
	}
	return out
}

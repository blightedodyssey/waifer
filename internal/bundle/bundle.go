// Package bundle exports and imports portable theme bundles: a small zip
// containing colors.toml, an optional wallpaper, an optional custom.css and a
// versioned manifest.
package bundle

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aether/internal/platform"
)

const (
	manifestVersion = 1
	manifestName    = "manifest.json"
	colorsName      = "colors.toml"
	cssName         = "custom.css"
)

type manifest struct {
	Version   int       `json:"version"`
	Files     []string  `json:"files"`
	CreatedAt time.Time `json:"created_at"`
}

// Result reports the extracted bundle paths.
type Result struct {
	Dir        string `json:"dir"`
	ColorsTOML string `json:"colors_toml"`
	Wallpaper  string `json:"wallpaper,omitempty"`
	CustomCSS  string `json:"custom_css,omitempty"`
}

// Export writes a zip archive at dest. colors.toml is required; wallpaper and
// CSS entries are included only when their paths are non-empty. The wallpaper
// keeps its extension (sanitized) as wallpaper.<ext>.
func Export(dest, colorsTomlPath, wallpaperPath, customCSSPath string) (err error) {
	if strings.TrimSpace(colorsTomlPath) == "" {
		return fmt.Errorf("colors.toml path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create bundle destination: %w", err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create bundle: %w", err)
	}

	zw := zip.NewWriter(f)
	defer func() {
		if closeErr := zw.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if closeErr := f.Close(); err == nil && closeErr != nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(dest) // never leave a half-written archive behind
		}
	}()

	manifestData := manifest{Version: manifestVersion, CreatedAt: time.Now()}
	addFile := func(name, src string) error {
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", src, err)
		}
		w, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("add %s: %w", name, err)
		}
		if _, err := w.Write(data); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		manifestData.Files = append(manifestData.Files, name)
		return nil
	}

	if err := addFile(colorsName, colorsTomlPath); err != nil {
		return err
	}
	if wallpaperPath != "" {
		if err := addFile(wallpaperEntryName(wallpaperPath), wallpaperPath); err != nil {
			return err
		}
	}
	if customCSSPath != "" {
		if err := addFile(cssName, customCSSPath); err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(manifestData, "", "  ")
	if err != nil {
		return fmt.Errorf("encode manifest: %w", err)
	}
	w, err := zw.Create(manifestName)
	if err != nil {
		return fmt.Errorf("add manifest: %w", err)
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// Import validates src and extracts its allowed entries into a fresh directory
// under %LOCALAPPDATA%\Waifer\bundles\<name> (or an XDG/home fallback).
// Unknown entries, directory traversal and symlinks are rejected before any
// file is written.
func Import(src string) (*Result, error) {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return nil, fmt.Errorf("open bundle: %w", err)
	}
	defer zr.Close()

	files := make(map[string]*zip.File)
	var names []string
	for _, zf := range zr.File {
		name, err := classifyEntry(zf.Name)
		if err != nil {
			return nil, err
		}
		if zf.FileInfo().IsDir() || zf.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("unsupported bundle entry %q", zf.Name)
		}
		if _, exists := files[name]; exists {
			return nil, fmt.Errorf("duplicate bundle entry %q", name)
		}
		files[name] = zf
		names = append(names, name)
	}
	if _, ok := files[manifestName]; !ok {
		return nil, fmt.Errorf("bundle is missing %s", manifestName)
	}
	if _, ok := files[colorsName]; !ok {
		return nil, fmt.Errorf("bundle is missing %s", colorsName)
	}

	data, err := readEntry(files[manifestName], 1<<20)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	var mf manifest
	if err := json.Unmarshal(data, &mf); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if mf.Version != manifestVersion {
		return nil, fmt.Errorf("unsupported bundle manifest version %d", mf.Version)
	}
	for _, name := range mf.Files {
		if _, err := classifyEntry(name); err != nil {
			return nil, fmt.Errorf("manifest declares invalid entry: %w", err)
		}
		if _, ok := files[name]; !ok {
			return nil, fmt.Errorf("manifest declares missing entry %q", name)
		}
	}

	dir := importDir(filepath.Base(src))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create bundle directory: %w", err)
	}
	result := &Result{Dir: dir}

	sort.Strings(names) // deterministic extraction order
	for _, name := range names {
		out := filepath.Join(dir, name)
		// Defense in depth: classifyEntry only accepts flat names, but verify
		// the joined path still resolves inside dir before writing.
		rel, err := filepath.Rel(dir, out)
		if err != nil || rel != name {
			_ = os.RemoveAll(dir)
			return nil, fmt.Errorf("unsafe bundle entry %q", name)
		}
		data, err := readEntry(files[name], 512<<20)
		if err != nil {
			_ = os.RemoveAll(dir)
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		if err := os.WriteFile(out, data, 0o644); err != nil {
			_ = os.RemoveAll(dir)
			return nil, fmt.Errorf("write %s: %w", name, err)
		}
		switch {
		case name == colorsName:
			result.ColorsTOML = out
		case name == cssName:
			result.CustomCSS = out
		case strings.HasPrefix(name, "wallpaper."):
			result.Wallpaper = out
		}
	}
	return result, nil
}

// classifyEntry enforces a flat, allow-listed entry set. Zip paths always use
// '/', but backslashes are rejected too so Windows cannot reinterpret a name.
func classifyEntry(name string) (string, error) {
	if name == "" || strings.ContainsAny(name, "/\\\x00:") || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid bundle entry %q", name)
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("invalid bundle entry %q", name)
		}
	}
	switch name {
	case manifestName, colorsName, cssName:
		return name, nil
	}
	if strings.HasPrefix(name, "wallpaper.") {
		ext := strings.TrimPrefix(name, "wallpaper.")
		if ext != "" && len(ext) <= 10 {
			valid := true
			for _, r := range ext {
				if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
					valid = false
					break
				}
			}
			if valid {
				return name, nil
			}
		}
	}
	return "", fmt.Errorf("unexpected bundle entry %q", name)
}

func readEntry(zf *zip.File, limit int64) ([]byte, error) {
	r, err := zf.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(io.LimitReader(r, limit))
}

func wallpaperEntryName(path string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	var b strings.Builder
	for _, r := range ext {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
		if b.Len() == 10 {
			break
		}
	}
	if b.Len() == 0 {
		b.WriteString("png")
	}
	return "wallpaper." + b.String()
}

// bundleRoot mirrors the Aether data-root convention: LOCALAPPDATA on Windows
// (and as an explicit override), XDG/home fallback elsewhere.
func bundleRoot() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "Waifer", "bundles")
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "aether", "bundles")
	}
	return filepath.Join(platform.HomeDir(), ".local", "share", "aether", "bundles")
}

func importDir(sourceBase string) string {
	name := strings.TrimSuffix(sourceBase, filepath.Ext(sourceBase))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	name = strings.Trim(strings.Trim(b.String(), "-_"), " ")
	if name == "" {
		name = "bundle"
	}
	dir := filepath.Join(bundleRoot(), name)
	if _, err := os.Stat(dir); err == nil {
		dir = fmt.Sprintf("%s-%d", dir, time.Now().UnixNano())
	}
	return dir
}

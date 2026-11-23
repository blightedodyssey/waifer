// Package snapshot captures and restores the small set of user configuration
// files that Aether themes (YASB, GlazeWM and Windhawk styler exports).
//
// Snapshots are read-only copies of user config: Capture never modifies the
// original files. Rollback writes back only paths that appear in the manifest
// and that are members of the known target list, so a tampered manifest cannot
// be used to overwrite arbitrary files.
package snapshot

import (
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
	// maxSnapshots is the retention window: newest five snapshots are kept.
	maxSnapshots = 5
	manifestName = "manifest.json"
)

// windhawkStylerMods mirrors the Windhawk styler IDs used by internal/winsys.
// Their settings live in the registry and cannot be copied by a file snapshot;
// the names are recorded in the manifest so a rollback has an audit trail.
var windhawkStylerMods = []string{
	"windows-11-start-menu-styler",
	"windows-11-notification-center-styler",
	"windows-11-taskbar-styler",
	"windows-11-file-explorer-styler",
}

// ManifestFile links a snapshot copy to the absolute path it came from.
type ManifestFile struct {
	Path string `json:"path"` // original absolute path
	Rel  string `json:"rel"`  // slash-separated path inside the snapshot dir
}

// Manifest is the on-disk description of one snapshot.
type Manifest struct {
	Version      int            `json:"version"`
	ID           string         `json:"id"`
	Label        string         `json:"label"`
	CreatedAt    time.Time      `json:"created_at"`
	Files        []ManifestFile `json:"files"`
	WindhawkMods []string       `json:"windhawk_mods,omitempty"`
}

// Info is the user-facing summary returned by List.
type Info struct {
	ID           string    `json:"id"`
	Label        string    `json:"label"`
	CreatedAt    time.Time `json:"created_at"`
	Files        []string  `json:"files"`
	WindhawkMods []string  `json:"windhawk_mods,omitempty"`
}

// target pairs a known user config path with its stable relative name in a
// snapshot. Rel is deliberately derived from the source category, not from the
// absolute path, so snapshots stay relocatable across machines.
type target struct {
	Path string
	Rel  string
}

// Root returns the snapshot root. On Windows this is %LOCALAPPDATA%\Waifer\snapshots.
// LOCALAPPDATA is honored as an explicit override on other platforms too (this
// keeps tests isolated); otherwise ~/.local/share/aether/snapshots is used.
func Root() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "Waifer", "snapshots")
	}
	return filepath.Join(platform.HomeDir(), ".local", "share", "aether", "snapshots")
}

// TargetFiles returns the subset of known config paths that currently exist on
// disk. Directories and duplicate paths are ignored.
func TargetFiles() []string {
	seen := make(map[string]bool)
	var out []string
	for _, t := range allTargets() {
		st, err := os.Stat(t.Path)
		if err != nil || st.IsDir() {
			continue
		}
		key := canonical(t.Path)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, t.Path)
	}
	return out
}

// Capture copies all existing targets into a new timestamped snapshot and
// returns its ID (the directory name under Root). A manifest-only snapshot is
// still created when no target files exist, so a rollback action is available.
func Capture(label string) (string, error) {
	root := Root()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create snapshot root: %w", err)
	}

	now := time.Now()
	id := fmt.Sprintf("%s-%s-%d", now.Format("20060102T150405"), safeLabel(label), now.UnixNano())
	dir := filepath.Join(root, id)
	if err := os.Mkdir(dir, 0o755); err != nil {
		return "", fmt.Errorf("create snapshot %s: %w", id, err)
	}

	manifest := Manifest{
		Version:      1,
		ID:           id,
		Label:        strings.TrimSpace(label),
		CreatedAt:    now,
		WindhawkMods: append([]string(nil), windhawkStylerMods...),
	}
	for _, t := range allTargets() {
		st, err := os.Stat(t.Path)
		if err != nil || st.IsDir() {
			continue
		}
		dst := filepath.Join(dir, filepath.FromSlash(t.Rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("create snapshot subdirectory: %w", err)
		}
		if err := copyFile(t.Path, dst, st.Mode().Perm()); err != nil {
			_ = os.RemoveAll(dir)
			return "", fmt.Errorf("copy %s: %w", t.Path, err)
		}
		manifest.Files = append(manifest.Files, ManifestFile{Path: t.Path, Rel: t.Rel})
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("encode manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, manifestName), append(data, '\n'), 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("write manifest: %w", err)
	}

	if err := prune(root); err != nil {
		return id, fmt.Errorf("prune old snapshots: %w", err)
	}
	return id, nil
}

// List returns snapshots oldest-first. Directories without a readable manifest
// are ignored so a partially written or foreign directory cannot break the UI.
func List() ([]Info, error) {
	root := Root()
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read snapshot root: %w", err)
	}

	var out []Info
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if _, err := os.Stat(filepath.Join(dir, manifestName)); err != nil {
			continue
		}
		manifest, err := readManifest(dir)
		if err != nil {
			continue
		}
		info := Info{
			ID:           entry.Name(),
			Label:        manifest.Label,
			CreatedAt:    manifest.CreatedAt,
			WindhawkMods: manifest.WindhawkMods,
		}
		for _, f := range manifest.Files {
			info.Files = append(info.Files, f.Path)
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Rollback restores every file listed in the snapshot manifest. id may be a
// snapshot ID or an absolute path to a snapshot directory under Root.
// Only paths from the fixed known-target list are ever written.
func Rollback(id string) error {
	dir, err := snapshotDir(id)
	if err != nil {
		return err
	}
	manifest, err := readManifest(dir)
	if err != nil {
		return err
	}
	if manifest.Version != 1 {
		return fmt.Errorf("unsupported snapshot manifest version %d", manifest.Version)
	}

	for _, f := range manifest.Files {
		expected, ok := relForPath(f.Path)
		if !ok {
			return fmt.Errorf("refusing to restore unknown path %q", f.Path)
		}
		if !safeRel(f.Rel) || canonicalRel(f.Rel) != canonicalRel(expected) {
			return fmt.Errorf("refusing to restore unsafe manifest entry %q -> %q", f.Rel, f.Path)
		}
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.Rel)))
		if err != nil {
			return fmt.Errorf("read snapshot file %s: %w", f.Rel, err)
		}
		if err := atomicWrite(f.Path, data, 0o644); err != nil {
			return fmt.Errorf("restore %s: %w", f.Path, err)
		}
	}
	return nil
}

// allTargets is the complete, deterministic set of paths Rollback is allowed
// to write. TargetFiles is just this set filtered by existence.
func allTargets() []target {
	profile := userProfile()
	appdata := appData()

	targets := []target{
		{filepath.Join(profile, ".config", "yasb", "config.yaml"), "userprofile/.config/yasb/config.yaml"},
		{filepath.Join(appdata, "yasb", "config.yaml"), "appdata/yasb/config.yaml"},
		{filepath.Join(appdata, "glazewm", "config.yaml"), "appdata/glazewm/config.yaml"},
		{filepath.Join(profile, ".config", "glazewm", "config.yaml"), "userprofile/.config/glazewm/config.yaml"},
	}

	// Windhawk settings are registry-backed; only user-exported .reg files can
	// be snapshotted. These are optional and skipped when absent.
	exportRoots := []struct {
		dir string
		rel string
	}{
		{filepath.Join(localAppData(), "Windhawk", "Exports"), "windhawk/local"},
		{filepath.Join(profile, "Documents", "Windhawk", "Exports"), "windhawk/documents"},
	}
	for _, root := range exportRoots {
		for _, mod := range windhawkStylerMods {
			targets = append(targets, target{
				Path: filepath.Join(root.dir, mod, "Settings.reg"),
				Rel:  filepath.ToSlash(filepath.Join(root.rel, mod, "Settings.reg")),
			})
		}
	}
	return targets
}

func relForPath(path string) (string, bool) {
	key := canonical(path)
	for _, t := range allTargets() {
		if canonical(t.Path) == key {
			return t.Rel, true
		}
	}
	return "", false
}

func readManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return nil, fmt.Errorf("read snapshot manifest: %w", err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse snapshot manifest: %w", err)
	}
	return &manifest, nil
}

func snapshotDir(id string) (string, error) {
	root := Root()
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("snapshot id is empty")
	}
	if filepath.IsAbs(id) || strings.ContainsAny(id, `/\`) {
		abs, err := filepath.Abs(id)
		if err != nil {
			return "", fmt.Errorf("invalid snapshot id: %w", err)
		}
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(rootAbs, abs)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("snapshot path outside %s", root)
		}
		id = filepath.Base(abs)
	}
	if id == "." || id == ".." || !filepath.IsLocal(id) {
		return "", fmt.Errorf("invalid snapshot id %q", id)
	}
	return filepath.Join(root, id), nil
}

// prune keeps only the maxSnapshots newest snapshot directories, comparing
// names lexically (their timestamp prefix makes that chronological order).
func prune(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	var dirs []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, entry.Name(), manifestName)); err == nil {
			dirs = append(dirs, entry.Name())
		}
	}
	sort.Strings(dirs)
	if len(dirs) <= maxSnapshots {
		return nil
	}
	for _, name := range dirs[:len(dirs)-maxSnapshots] {
		if err := os.RemoveAll(filepath.Join(root, name)); err != nil {
			return err
		}
	}
	return nil
}

func safeRel(rel string) bool {
	if rel == "" || strings.ContainsAny(rel, "\x00\\") || strings.HasPrefix(rel, "/") {
		return false
	}
	local := filepath.FromSlash(rel)
	return filepath.IsLocal(local) && !filepath.IsAbs(local) && filepath.Clean(local) == local
}

func canonicalRel(rel string) string {
	return canonical(filepath.Clean(filepath.FromSlash(rel)))
}

func canonical(path string) string {
	clean := filepath.Clean(path)
	if isWindows() {
		return strings.ToLower(clean) // Windows paths are case-insensitive
	}
	return clean
}

func isWindows() bool {
	return os.PathSeparator == '\\'
}

func safeLabel(label string) string {
	label = strings.TrimSpace(label)
	if label == "" {
		return "manual"
	}
	var b strings.Builder
	for _, r := range label {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('-')
		}
	}
	out := b.String()
	if len(out) > 48 {
		out = out[:48]
	}
	return strings.Trim(out, "-_")
}

func userProfile() string {
	if p := os.Getenv("USERPROFILE"); p != "" {
		return p
	}
	return platform.HomeDir()
}

func appData() string {
	if p := os.Getenv("APPDATA"); p != "" {
		return p
	}
	if p := os.Getenv("XDG_CONFIG_HOME"); p != "" {
		return p
	}
	return filepath.Join(platform.HomeDir(), ".config")
}

func localAppData() string {
	if p := os.Getenv("LOCALAPPDATA"); p != "" {
		return p
	}
	if p := os.Getenv("XDG_DATA_HOME"); p != "" {
		return p
	}
	return filepath.Join(platform.HomeDir(), ".local", "share")
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm.Perm())
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// atomicWrite writes a sibling temp file first so a crashed rollback never
// leaves a truncated user config behind.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".aether-snapshot-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm.Perm()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

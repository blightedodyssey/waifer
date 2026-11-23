package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupSnapshotEnv(t *testing.T) (profile, appdata, local string) {
	t.Helper()
	base := t.TempDir()
	home := filepath.Join(base, "home")
	profile = filepath.Join(base, "profile")
	appdata = filepath.Join(base, "appdata")
	local = filepath.Join(base, "local")
	for _, dir := range []string{home, profile, appdata, local} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Mirror the project's test-isolation rule: never touch real user config.
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", profile)
	t.Setenv("APPDATA", appdata)
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	return profile, appdata, local
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCaptureCopiesTargetsAndWritesManifest(t *testing.T) {
	profile, appdata, local := setupSnapshotEnv(t)
	yasb := filepath.Join(profile, ".config", "yasb", "config.yaml")
	glaze := filepath.Join(appdata, "glazewm", "config.yaml")
	writeTestFile(t, yasb, "yasb: original\n")
	writeTestFile(t, glaze, "glaze: original\n")
	writeTestFile(t, filepath.Join(appdata, "not-a-target.txt"), "ignore me\n")

	if got, want := Root(), filepath.Join(local, "Waifer", "snapshots"); got != want {
		t.Fatalf("Root() = %q, want %q", got, want)
	}
	targets := TargetFiles()
	if len(targets) != 2 {
		t.Fatalf("TargetFiles() = %v, want 2 paths", targets)
	}

	id, err := Capture("pre change")
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(Root(), id)
	copiedYasb, err := os.ReadFile(filepath.Join(dir, "userprofile", ".config", "yasb", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(copiedYasb) != "yasb: original\n" {
		t.Fatalf("copied YASB config = %q", copiedYasb)
	}
	copiedGlaze, err := os.ReadFile(filepath.Join(dir, "appdata", "glazewm", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(copiedGlaze) != "glaze: original\n" {
		t.Fatalf("copied GlazeWM config = %q", copiedGlaze)
	}
	if _, err := os.Stat(filepath.Join(dir, manifestName)); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	err = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(entry.Name(), "not-a-target") {
			t.Fatalf("unexpected non-target file in snapshot: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	infos, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 1 || infos[0].ID != id || infos[0].Label != "pre change" {
		t.Fatalf("List() = %+v, want one snapshot %q", infos, id)
	}
	if len(infos[0].Files) != 2 || len(infos[0].WindhawkMods) != 4 {
		t.Fatalf("unexpected manifest summary: %+v", infos[0])
	}

	// Capture is strictly read+copy: originals must be untouched.
	for path, want := range map[string]string{yasb: "yasb: original\n", glaze: "glaze: original\n"} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("original %s changed: %q (%v)", path, data, err)
		}
	}
}

func TestCapturePrunesToLastFive(t *testing.T) {
	setupSnapshotEnv(t)
	var ids []string
	for i := 0; i < 7; i++ {
		id, err := Capture("prune")
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	infos, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if len(infos) != 5 {
		t.Fatalf("List() length = %d, want 5", len(infos))
	}
	for i, want := range ids[2:] {
		if infos[i].ID != want {
			t.Fatalf("snapshot[%d] = %q, want %q", i, infos[i].ID, want)
		}
	}
	// Manifest-only snapshots are valid when no targets exist.
	if len(infos[0].Files) != 0 {
		t.Fatalf("expected manifest-only snapshot, got %v", infos[0].Files)
	}
}

func TestRollbackRestoresOnlyManifestFiles(t *testing.T) {
	profile, _, _ := setupSnapshotEnv(t)
	target := filepath.Join(profile, ".config", "yasb", "config.yaml")
	writeTestFile(t, target, "saved value\n")

	id, err := Capture("before edit")
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, target, "edited value\n")
	if err := Rollback(id); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "saved value\n" {
		t.Fatalf("rollback result = %q (%v)", data, err)
	}

	// A manifest that names an unknown path must be refused, and that path
	// must remain untouched.
	unknown := filepath.Join(t.TempDir(), "evil.txt")
	writeTestFile(t, unknown, "do not touch\n")
	writeManifest(t, id, Manifest{
		Version: 1,
		ID:      id,
		Files: []ManifestFile{
			{Path: unknown, Rel: "userprofile/.config/yasb/config.yaml"},
		},
	})
	if err := Rollback(id); err == nil {
		t.Fatal("Rollback accepted a manifest with an unknown path")
	}
	data, err = os.ReadFile(unknown)
	if err != nil || string(data) != "do not touch\n" {
		t.Fatalf("unknown path was modified: %q (%v)", data, err)
	}

	// A known path with a traversal Rel must also be refused.
	writeManifest(t, id, Manifest{
		Version: 1,
		ID:      id,
		Files:   []ManifestFile{{Path: target, Rel: "../evil.txt"}},
	})
	if err := Rollback(id); err == nil {
		t.Fatal("Rollback accepted a traversal Rel")
	}
}

func writeManifest(t *testing.T, id string, manifest Manifest) {
	t.Helper()
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(Root(), id, manifestName), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

package bundle

import (
	"archive/zip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupBundleEnv(t *testing.T) string {
	t.Helper()
	base := t.TempDir()
	home := filepath.Join(base, "home")
	local := filepath.Join(base, "local")
	for _, dir := range []string{home, local} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("HOME", home)
	t.Setenv("LOCALAPPDATA", local)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	return local
}

func writeBundleFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func makeZip(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, data := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func validManifest(t *testing.T, names ...string) []byte {
	t.Helper()
	data, err := json.Marshal(manifest{Version: manifestVersion, Files: names})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestExportImportRoundTrip(t *testing.T) {
	local := setupBundleEnv(t)
	base := t.TempDir()
	colors := filepath.Join(base, "colors.toml")
	wallpaper := filepath.Join(base, "wall.PNG")
	css := filepath.Join(base, "custom.css")
	writeBundleFile(t, colors, "base00 = '#000000'\n")
	writeBundleFile(t, wallpaper, "fake-png-bytes")
	writeBundleFile(t, css, "body { color: red; }\n")

	archivePath := filepath.Join(base, "out", "my-theme.zip")
	if err := Export(archivePath, colors, wallpaper, css); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	names := make(map[string]bool)
	for _, zf := range zr.File {
		names[zf.Name] = true
	}
	if err := zr.Close(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"colors.toml", "wallpaper.png", "custom.css", manifestName} {
		if !names[want] {
			t.Fatalf("zip missing %q (entries: %v)", want, names)
		}
	}
	if len(names) != 4 {
		t.Fatalf("unexpected zip entries: %v", names)
	}

	result, err := Import(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(local, "Waifer", "bundles", "my-theme"); result.Dir != want {
		t.Fatalf("result.Dir = %q, want %q", result.Dir, want)
	}
	checks := map[string]string{
		result.ColorsTOML: "base00 = '#000000'\n",
		result.Wallpaper:  "fake-png-bytes",
		result.CustomCSS:  "body { color: red; }\n",
	}
	for path, want := range checks {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("imported %s = %q (%v)", path, data, err)
		}
	}
}

func TestExportImportWithoutOptionalFiles(t *testing.T) {
	setupBundleEnv(t)
	base := t.TempDir()
	colors := filepath.Join(base, "colors.toml")
	writeBundleFile(t, colors, "base05 = '#ffffff'\n")
	archivePath := filepath.Join(base, "minimal.zip")
	if err := Export(archivePath, colors, "", ""); err != nil {
		t.Fatal(err)
	}
	result, err := Import(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if result.Wallpaper != "" || result.CustomCSS != "" {
		t.Fatalf("unexpected optional paths: %+v", result)
	}
	data, err := os.ReadFile(result.ColorsTOML)
	if err != nil || string(data) != "base05 = '#ffffff'\n" {
		t.Fatalf("imported colors = %q (%v)", data, err)
	}
}

func TestImportRejectsZipSlipAndUnknownEntries(t *testing.T) {
	local := setupBundleEnv(t)
	base := t.TempDir()

	cases := []struct {
		name    string
		entries map[string][]byte
	}{
		{
			name: "zip slip",
			entries: map[string][]byte{
				manifestName:  validManifest(t, colorsName, "../evil.txt"),
				colorsName:    []byte("base00 = '#000000'\n"),
				"../evil.txt": []byte("pwned"),
			},
		},
		{
			name: "nested path",
			entries: map[string][]byte{
				manifestName:      validManifest(t, colorsName),
				colorsName:        []byte("base00 = '#000000'\n"),
				"sub/colors.toml": []byte("nested"),
			},
		},
		{
			name: "unknown entry",
			entries: map[string][]byte{
				manifestName: validManifest(t, colorsName),
				colorsName:   []byte("base00 = '#000000'\n"),
				"notes.txt":  []byte("unexpected"),
			},
		},
		{
			name: "bad manifest version",
			entries: map[string][]byte{
				manifestName: []byte(`{"version":2,"files":["colors.toml"]}`),
				colorsName:   []byte("base00 = '#000000'\n"),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(base, strings.ReplaceAll(tc.name, " ", "-")+".zip")
			makeZip(t, path, tc.entries)
			if _, err := Import(path); err == nil {
				t.Fatal("Import accepted a malicious bundle")
			}
		})
	}

	err := filepath.WalkDir(local, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(entry.Name(), "evil") || strings.Contains(entry.Name(), "notes") {
			t.Fatalf("malicious entry was extracted: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

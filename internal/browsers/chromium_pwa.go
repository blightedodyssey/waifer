package browsers

// PWATheme patches Chrome PWAs and WebCatalog apps.
// This file is intentionally thin so the rest of the package doesn't change.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PatchPWAsAndWebCatalog scans Chrome's Web Applications and WebCatalog for
// installed apps and updates their manifest.json theme_color / background_color.
// It also returns a list of every patched manifest path for DWM refresh.
func PatchPWAsAndWebCatalog(chromeUserData string, vars map[string]string) (manifests []string, warnings []string) {
	bg := safeHex(vars["background"])
	accent := safeHex(vars["accent"])

	// Chrome PWAs
	if m, w := patchPWADir(filepath.Join(chromeUserData, "Web Applications"), bg, accent); len(m) > 0 {
		manifests = append(manifests, m...)
		warnings = append(warnings, w...)
	}

	// WebCatalog
	local := os.Getenv("LOCALAPPDATA")
	if local != "" {
		if m, w := patchPWADir(filepath.Join(local, "WebCatalog"), bg, accent); len(m) > 0 {
			manifests = append(manifests, m...)
			warnings = append(warnings, w...)
		}
	}

	return manifests, warnings
}

func patchPWADir(dir, bg, accent string) ([]string, []string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil
	}

	var manifests []string
	var warnings []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "manifest.json")
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			continue
		}

		var manifest map[string]interface{}
		if err := json.Unmarshal(data, &manifest); err != nil {
			warnings = append(warnings, fmt.Sprintf("parse PWA manifest %s: %v", entry.Name(), err))
			continue
		}

		manifest["theme_color"] = accent
		manifest["background_color"] = bg

		encoded, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("encode PWA manifest %s: %v", entry.Name(), err))
			continue
		}
		encoded = append(encoded, '\n')

		bak := manifestPath + ".aether.bak"
		if _, err := os.Stat(bak); os.IsNotExist(err) {
			if err := os.WriteFile(bak, data, 0o644); err != nil {
				warnings = append(warnings, fmt.Sprintf("backup PWA manifest %s: %v", entry.Name(), err))
			}
		}

		if err := writeFileAtomic(manifestPath, encoded); err != nil {
			warnings = append(warnings, fmt.Sprintf("write PWA manifest %s: %v", entry.Name(), err))
			continue
		}
		manifests = append(manifests, manifestPath)
	}
	return manifests, warnings
}

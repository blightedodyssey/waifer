package template

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// ListTemplates walks the embedded template filesystem under root and returns
// every regular file path relative to root (e.g. "kitty.conf",
// "vscode-extension/themes/aether-color-theme.json").
func ListTemplates(templatesFS embed.FS, root string) ([]string, error) {
	var names []string
	err := fs.WalkDir(templatesFS, root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel := strings.TrimPrefix(p, root+"/")
		names = append(names, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	sort.Strings(names)
	return names, nil
}

// ReadTemplate returns the raw template content for name under root.
func ReadTemplate(templatesFS embed.FS, root, name string) (string, error) {
	if strings.Contains(name, "..") || path.IsAbs(name) {
		return "", fmt.Errorf("invalid template name %q", name)
	}
	data, err := templatesFS.ReadFile(path.Join(root, name))
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", name, err)
	}
	return string(data), nil
}

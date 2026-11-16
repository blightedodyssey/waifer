// Package favexport archives the user's favorite wallpapers into a .zip.
// Remote (wallhaven) favorites are downloaded first, which is slow enough
// that the work runs in the background with progress events.
package favexport

import (
	"archive/zip"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"aether/internal/wallhaven"
	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Item is one favorite to export.
type Item struct {
	Path string                 `json:"path"`
	Name string                 `json:"name,omitempty"`
	Meta map[string]interface{} `json:"meta,omitempty"`
}

// Exporter zips favorites into a user-chosen directory.
type Exporter struct {
	mu      sync.Mutex
	running bool
	cancel  context.CancelFunc
	client  *wallhaven.Client
}

// New returns an exporter bound to a wallhaven client (used for remote
// favorites).
func New(client *wallhaven.Client) *Exporter {
	return &Exporter{client: client}
}

// IsRunning reports whether an export is in flight.
func (e *Exporter) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// Start launches the export. Returns the archive path the export will write.
// Events:
//
//	favorites-export-started   {total, path}
//	favorites-export-progress  {index, total, name}
//	favorites-export-complete  {path, count}
//	favorites-export-error     {error}
func (e *Exporter) Start(ctx context.Context, items []Item, dir string) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("no context")
	}
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return "", fmt.Errorf("an export is already running")
	}
	target := archivePath(dir)
	runCtx, cancel := context.WithCancel(ctx)
	e.running = true
	e.cancel = cancel
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			e.running = false
			e.cancel = nil
			e.mu.Unlock()
		}()

		if err := e.run(runCtx, items, target); err != nil {
			if runCtx.Err() == nil {
				wailsrt.EventsEmit(ctx, "favorites-export-error", map[string]interface{}{"error": err.Error()})
			}
			return
		}
		wailsrt.EventsEmit(ctx, "favorites-export-complete", map[string]interface{}{
			"path":  target,
			"count": len(items),
		})
	}()
	return target, nil
}

// Cancel stops the running export.
func (e *Exporter) Cancel() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
	}
}

// archivePath builds a non-clobbering target filename in dir.
func archivePath(dir string) string {
	base := "aether-favorites.zip"
	target := filepath.Join(dir, base)
	for i := 2; ; i++ {
		if _, err := os.Stat(target); err != nil {
			return target
		}
		target = filepath.Join(dir, fmt.Sprintf("aether-favorites-%d.zip", i))
	}
}

// run performs the export synchronously.
func (e *Exporter) run(ctx context.Context, items []Item, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("prepare export dir: %w", err)
	}
	f, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer f.Close()

	wailsrt.EventsEmit(context.Background(), "favorites-export-started", map[string]interface{}{
		"total": len(items),
		"path":  target,
	})

	zw := zip.NewWriter(f)
	defer zw.Close()

	for i, item := range items {
		if ctx.Err() != nil {
			return fmt.Errorf("cancelled")
		}
		wailsrt.EventsEmit(context.Background(), "favorites-export-progress", map[string]interface{}{
			"index": i + 1,
			"total": len(items),
			"name":  itemName(item),
		})

		localPath := item.Path
		if !isLocalFile(localPath) && isHTTP(localPath) && e.client != nil {
			// Remote favorite: resolve to a full-size image first.
			downloaded, derr := e.client.DownloadFromURL(item.Path)
			if derr != nil {
				continue // skip unavailable favorites
			}
			localPath = downloaded
		} else if !isLocalFile(localPath) {
			continue
		}

		data, merr := os.ReadFile(localPath)
		if merr != nil {
			continue
		}
		name := itemName(item)
		if ext := filepath.Ext(localPath); ext != "" && filepath.Ext(name) != ext {
			name += ext
		}
		entry, werr := zw.Create("favorites/" + name)
		if werr != nil {
			return fmt.Errorf("archive entry: %w", werr)
		}
		if _, werr := entry.Write(data); werr != nil {
			return fmt.Errorf("archive write: %w", werr)
		}
	}
	return nil
}

// itemName sanitizes an archive entry name.
func itemName(item Item) string {
	name := strings.TrimSpace(item.Name)
	if name == "" {
		name = filepath.Base(strings.ReplaceAll(item.Path, "\\", "/"))
	}
	name = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '.', r == '-', r == '_', r == ' ':
			return r
		default:
			return '-'
		}
	}, name)
	name = strings.Trim(name, "-.")
	if name == "" {
		name = "favorite"
	}
	return name
}

// isLocalFile reports whether path exists as a regular file.
func isLocalFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// isHTTP reports whether s is an http(s) URL.
func isHTTP(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

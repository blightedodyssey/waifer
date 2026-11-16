package theme

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"

	"aether/internal/platform"
)

// ThemeWatcher keeps the parsed contents of the active colors.toml available
// to the frontend without a Wails event-emit race: the UI polls it on mount.
type ThemeWatcher struct {
	mu     sync.RWMutex
	colors map[string]string
	stop   chan struct{}
	once   sync.Once
}

// NewThemeWatcher returns an idle watcher.
func NewThemeWatcher() *ThemeWatcher {
	return &ThemeWatcher{colors: make(map[string]string)}
}

// Start launches the background polling loop. Safe to call once per watcher.
func (w *ThemeWatcher) Start(ctx context.Context) {
	w.once.Do(func() {
		w.stop = make(chan struct{})
		go w.loop(ctx)
	})
}

// Stop terminates the polling loop.
func (w *ThemeWatcher) Stop() {
	if w.stop != nil {
		select {
		case <-w.stop:
		default:
			close(w.stop)
		}
	}
}

func (w *ThemeWatcher) loop(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	w.refresh() // initial read
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stop:
			return
		case <-ticker.C:
			w.refresh()
		}
	}
}

func (w *ThemeWatcher) refresh() {
	path := filepath.Join(platform.ThemeDir(), "colors.toml")
	data, err := os.ReadFile(path)
	if err != nil {
		return // no theme applied yet
	}
	colors, err := parseColorsTOML(string(data))
	if err != nil {
		return
	}
	w.mu.Lock()
	w.colors = colors
	w.mu.Unlock()
}

// CurrentColors returns the latest parsed colors.toml map.
func (w *ThemeWatcher) CurrentColors() map[string]string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make(map[string]string, len(w.colors))
	for k, v := range w.colors {
		out[k] = v
	}
	return out
}

// parseColorsTOML extracts `key = "value"` pairs from a colors.toml file.
// Only quoted string values are recognized; comments are ignored.
func parseColorsTOML(content string) (map[string]string, error) {
	colors := make(map[string]string)
	for _, line := range splitLines(content) {
		line = trimComment(line)
		if line == "" || line[0] == '[' {
			continue
		}
		eq := indexOf(line, '=')
		if eq < 0 {
			continue
		}
		key := trimSpace(line[:eq])
		value := trimSpace(line[eq+1:])
		value = unquote(value)
		if key != "" && value != "" {
			colors[key] = value
		}
	}
	return colors, nil
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func trimComment(line string) string {
	if idx := indexOf(line, '#'); idx >= 0 {
		return line[:idx]
	}
	return line
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

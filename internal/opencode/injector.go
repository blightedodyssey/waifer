package opencode

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// SlotA and SlotB are the alternating theme names that force OpenCode's
	// TUI file watcher to repaint on every apply.
	SlotA = "aether-live-a"
	SlotB = "aether-live-b"

	DefaultDebounce     = 150 * time.Millisecond
	DefaultCleanupDelay = 600 * time.Millisecond
)

// Result reports the files written by an injection.
type Result struct {
	Name       string   `json:"name"`
	Mode       string   `json:"mode"`
	ThemePath  string   `json:"themePath"`
	ConfigPath string   `json:"configPath"`
	Applied    []string `json:"applied,omitempty"`
}

// ConfigDir resolves OpenCode's config directory. XDG_CONFIG_HOME wins when
// set; otherwise the path is <home>/.config/opencode.
func ConfigDir() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "opencode")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		home = "."
	}
	return filepath.Join(home, ".config", "opencode")
}

// Injector writes V2 themes, rotates hot-reload slots and debounces requests.
type Injector struct {
	mu           sync.Mutex
	writeMu      sync.Mutex
	configDir    string
	debounce     time.Duration
	cleanupDelay time.Duration

	pending  map[string]string
	waiters  []chan applyOutcome
	timer    *time.Timer
	lastSlot string
}

type applyOutcome struct {
	result Result
	err    error
}

type Option func(*Injector)

// WithConfigDir overrides path resolution (used by tests and custom installs).
func WithConfigDir(dir string) Option {
	return func(i *Injector) { i.configDir = dir }
}

// WithDebounce overrides the 150ms live-apply debounce.
func WithDebounce(d time.Duration) Option {
	return func(i *Injector) { i.debounce = d }
}

// WithCleanupDelay overrides the 600ms stale-slot cleanup delay.
func WithCleanupDelay(d time.Duration) Option {
	return func(i *Injector) { i.cleanupDelay = d }
}

// NewInjector creates an injector with production defaults.
func NewInjector(opts ...Option) *Injector {
	i := &Injector{
		debounce:     DefaultDebounce,
		cleanupDelay: DefaultCleanupDelay,
	}
	for _, opt := range opts {
		opt(i)
	}
	if i.debounce < 0 {
		i.debounce = 0
	}
	if i.cleanupDelay < 0 {
		i.cleanupDelay = 0
	}
	return i
}

var defaultInjector = NewInjector()

// Default returns the process-wide injector shared by all theme applies.
func Default() *Injector { return defaultInjector }

// Apply debounces and performs a native injection. Concurrent calls coalesce
// onto the latest palette and all callers receive the result of that flush.
func (i *Injector) Apply(vars map[string]string) (Result, error) {
	clone := make(map[string]string, len(vars))
	for key, value := range vars {
		clone[key] = value
	}

	i.mu.Lock()
	waiter := make(chan applyOutcome, 1)
	i.waiters = append(i.waiters, waiter)
	i.pending = clone
	if i.timer == nil {
		i.timer = time.AfterFunc(i.debounce, i.flush)
	}
	i.mu.Unlock()

	outcome := <-waiter
	return outcome.result, outcome.err
}

func (i *Injector) flush() {
	i.mu.Lock()
	vars := i.pending
	waiters := i.waiters
	i.pending = nil
	i.waiters = nil
	i.timer = nil
	i.mu.Unlock()

	result, err := i.injectNow(vars)
	for _, waiter := range waiters {
		waiter <- applyOutcome{result: result, err: err}
		close(waiter)
	}
}

func (i *Injector) dir() string {
	if i.configDir != "" {
		return i.configDir
	}
	return ConfigDir()
}

// injectNow serializes filesystem work so two debounce bursts cannot rotate
// the same slot or interleave partial writes.
func (i *Injector) injectNow(vars map[string]string) (Result, error) {
	i.writeMu.Lock()
	defer i.writeMu.Unlock()

	themeData, err := BuildTheme(vars)
	if err != nil {
		return Result{}, err
	}

	configDir := i.dir()
	themesDir := filepath.Join(configDir, "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create OpenCode themes directory: %w", err)
	}

	cliPath := filepath.Join(configDir, "cli.json")
	slot := i.nextSlot(cliPath)
	mode := Mode(vars)
	themePath := filepath.Join(themesDir, slot+".json")
	if err := atomicWrite(themePath, themeData); err != nil {
		return Result{}, fmt.Errorf("write OpenCode V2 theme: %w", err)
	}
	if err := updateCLI(cliPath, slot, mode); err != nil {
		return Result{}, err
	}

	i.mu.Lock()
	i.lastSlot = slot
	i.mu.Unlock()

	inactive := SlotA
	if slot == SlotA {
		inactive = SlotB
	}
	i.scheduleCleanup(filepath.Join(themesDir, inactive+".json"), inactive)

	return Result{
		Name:       slot,
		Mode:       mode,
		ThemePath:  themePath,
		ConfigPath: cliPath,
		Applied:    []string{"opencode-v2-theme", "opencode-hot-reload"},
	}, nil
}

// nextSlot alternates slot names. Existing cli.json state is used on the
// first call after a restart so rotation continues across app sessions.
func (i *Injector) nextSlot(cliPath string) string {
	i.mu.Lock()
	last := i.lastSlot
	i.mu.Unlock()

	if last == "" {
		if raw, err := os.ReadFile(cliPath); err == nil {
			var config map[string]any
			if json.Unmarshal(raw, &config) == nil {
				if theme, ok := config["theme"].(map[string]any); ok {
					switch name, _ := theme["name"].(string); name {
					case SlotA, SlotB:
						last = name
					}
				}
			}
		}
	}
	if last == SlotA {
		return SlotB
	}
	return SlotA
}

func (i *Injector) scheduleCleanup(path, slot string) {
	time.AfterFunc(i.cleanupDelay, func() {
		// Take the write lock so this cannot interleave with injectNow. Without
		// it, a timer scheduled by the previous apply can land between the new
		// theme write and the lastSlot assignment, see a stale active slot, and
		// delete the file that just became active.
		i.writeMu.Lock()
		defer i.writeMu.Unlock()

		i.mu.Lock()
		active := i.lastSlot
		i.mu.Unlock()
		if active == slot {
			return
		}
		_ = os.Remove(path)
	})
}

// updateCLI deep-merges only the theme keys into cli.json. All other user
// settings, including nested MCP and provider config, are preserved.
func updateCLI(path, slot, mode string) error {
	data := make(map[string]any)
	if raw, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(raw, &data); err != nil {
			return fmt.Errorf("OpenCode cli.json is malformed: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read OpenCode cli.json: %w", err)
	}
	if data == nil {
		data = make(map[string]any)
	}
	if _, ok := data["$schema"]; !ok {
		data["$schema"] = TemplateSchema
	}
	themeBlock, ok := data["theme"].(map[string]any)
	if !ok || themeBlock == nil {
		themeBlock = make(map[string]any)
	}
	themeBlock["name"] = slot
	themeBlock["mode"] = mode
	data["theme"] = themeBlock

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal OpenCode cli.json: %w", err)
	}
	if err := atomicWrite(path, append(out, '\n')); err != nil {
		return fmt.Errorf("write OpenCode cli.json: %w", err)
	}
	return nil
}

// atomicWrite writes to a sibling .tmp file and renames it into place so
// OpenCode's watcher never observes a partially written JSON file.
func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Package scheduler selects which Aether blueprint to apply, either from the
// Windows app light/dark preference or from a simple local-time day/night
// schedule. All state is local; there is no telemetry or background service.
package scheduler

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aether/internal/platform"
)

const (
	ModeSystem   = "system"
	ModeSchedule = "schedule"
)

// Settings controls automatic day/night blueprint switching.
type Settings struct {
	Enabled        bool    `json:"enabled"`
	LightBlueprint string  `json:"light_blueprint"`
	DarkBlueprint  string  `json:"dark_blueprint"`
	Mode           string  `json:"mode"` // "system" or "schedule"
	Latitude       float64 `json:"latitude,omitempty"`
	Longitude      float64 `json:"longitude,omitempty"`
	// Latitude/Longitude are accepted for future astronomical scheduling but
	// are intentionally unused today: the fixed 06:00-18:00 local-time window
	// stays deterministic and works offline without a solar-position library.
}

// Load reads scheduler settings. A missing file returns safe defaults.
func Load() (Settings, error) {
	settings := Settings{Mode: ModeSystem}
	path := settingsPath()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return settings, nil
	}
	if err != nil {
		return settings, fmt.Errorf("read scheduler settings: %w", err)
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, fmt.Errorf("parse scheduler settings: %w", err)
	}
	if strings.TrimSpace(settings.Mode) == "" {
		settings.Mode = ModeSystem
	}
	return settings, nil
}

// Save persists settings to %LOCALAPPDATA%\Waifer\scheduler.json using an
// atomic temp-file rename.
func Save(settings Settings) error {
	path := settingsPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create scheduler directory: %w", err)
	}
	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode scheduler settings: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".scheduler-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	return nil
}

// DesiredMode returns the mode implied by the stored settings at now.
// Schedule mode: 06:00-18:00 local time is light, otherwise dark (lat/lon are
// documented above as reserved). System mode: read the Windows Personalize
// registry value AppsUseLightTheme; non-Windows defaults to dark.
func (s Settings) DesiredMode(now time.Time) string {
	switch strings.ToLower(strings.TrimSpace(s.Mode)) {
	case ModeSchedule:
		if hour := now.Hour(); hour >= 6 && hour < 18 {
			return "light"
		}
		return "dark"
	case ModeSystem, "":
		if systemPrefersLight() {
			return "light"
		}
		return "dark"
	default:
		return "dark"
	}
}

// DesiredMode loads settings from disk and evaluates them at now.
func DesiredMode(now time.Time) string {
	settings, _ := Load()
	return settings.DesiredMode(now)
}

// ShouldSwitchAt reports whether enabled settings would change the current
// mode at the given instant. Splitting out the time makes this testable.
func (s Settings) ShouldSwitchAt(current string, now time.Time) bool {
	if !s.Enabled {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(current), s.DesiredMode(now))
}

// ShouldSwitch evaluates ShouldSwitchAt at the current time.
func (s Settings) ShouldSwitch(current string) bool {
	return s.ShouldSwitchAt(current, time.Now())
}

// ShouldSwitch loads settings from disk and evaluates them now.
func ShouldSwitch(current string) bool {
	settings, _ := Load()
	return settings.ShouldSwitch(current)
}

func settingsPath() string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return filepath.Join(local, "Waifer", "scheduler.json")
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "aether", "scheduler.json")
	}
	return filepath.Join(platform.HomeDir(), ".local", "share", "aether", "scheduler.json")
}

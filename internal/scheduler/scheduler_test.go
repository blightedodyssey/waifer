package scheduler

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func setupSchedulerEnv(t *testing.T) string {
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

func TestDesiredModeScheduleBoundaries(t *testing.T) {
	settings := Settings{Mode: ModeSchedule, Latitude: 51.5, Longitude: -0.12}
	cases := []struct {
		hour, minute int
		want         string
	}{
		{5, 59, "dark"},
		{6, 0, "light"},
		{12, 0, "light"},
		{17, 59, "light"},
		{18, 0, "dark"},
		{23, 30, "dark"},
	}
	for _, tc := range cases {
		now := time.Date(2026, 11, 21, tc.hour, tc.minute, 0, 0, time.Local)
		if got := settings.DesiredMode(now); got != tc.want {
			t.Errorf("DesiredMode(%02d:%02d) = %q, want %q", tc.hour, tc.minute, got, tc.want)
		}
	}
}

func TestDesiredModeSystem(t *testing.T) {
	settings := Settings{Mode: ModeSystem}
	got := settings.DesiredMode(time.Now())
	if runtime.GOOS != "windows" && got != "dark" {
		t.Fatalf("non-Windows system mode = %q, want dark", got)
	}
	if got != "light" && got != "dark" {
		t.Fatalf("system mode = %q, want light or dark", got)
	}
}

func TestShouldSwitchAt(t *testing.T) {
	noon := time.Date(2026, 11, 21, 12, 0, 0, 0, time.Local)
	night := time.Date(2026, 11, 21, 22, 0, 0, 0, time.Local)

	disabled := Settings{Enabled: false, Mode: ModeSchedule}
	if disabled.ShouldSwitchAt("dark", noon) {
		t.Fatal("disabled settings must never request a switch")
	}

	enabled := Settings{Enabled: true, Mode: ModeSchedule}
	if !enabled.ShouldSwitchAt("dark", noon) {
		t.Fatal("expected switch from dark to light at noon")
	}
	if enabled.ShouldSwitchAt("LIGHT", noon) {
		t.Fatal("mode comparison should be case-insensitive")
	}
	if !enabled.ShouldSwitchAt("light", night) {
		t.Fatal("expected switch from light to dark at night")
	}
	if enabled.ShouldSwitchAt("dark", night) {
		t.Fatal("unexpected switch when already dark at night")
	}
}

func TestLoadSaveRoundTrip(t *testing.T) {
	local := setupSchedulerEnv(t)
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Enabled || loaded.Mode != ModeSystem {
		t.Fatalf("default settings = %+v", loaded)
	}

	want := Settings{
		Enabled:        true,
		LightBlueprint: "aether-light",
		DarkBlueprint:  "aether-dark",
		Mode:           ModeSchedule,
		Latitude:       40.7,
		Longitude:      -74.0,
	}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(local, "Waifer", "scheduler.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("settings file not written: %v", err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() = %+v, want %+v", got, want)
	}
}

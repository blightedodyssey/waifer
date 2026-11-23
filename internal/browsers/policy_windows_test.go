//go:build windows

package browsers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFrameColorFormatting(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{in: "#1e1e2e", want: "#1E1E2E", ok: true},
		{in: "1e1e2e", want: "#1E1E2E", ok: true},
		{in: "  #c09ef5 ", want: "#C09EF5", ok: true},
		{in: "#ABCDEF", want: "#ABCDEF", ok: true},
		{in: "red", ok: false},
		{in: "", ok: false},
		{in: "#12345", ok: false},
		{in: "#1234567", ok: false},
		{in: "#12345G", ok: false},
	}
	for _, tc := range cases {
		got, ok := frameColor(map[string]string{"background": tc.in})
		if ok != tc.ok {
			t.Errorf("frameColor(%q) ok = %v, want %v", tc.in, ok, tc.ok)
			continue
		}
		if ok && got != tc.want {
			t.Errorf("frameColor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFramePolicyKeySelection(t *testing.T) {
	keys := framePolicyKeys(func(name string) bool {
		return name == "chrome" || name == "brave"
	})
	if len(keys) != 2 ||
		!strings.Contains(keys[0], `Google\Chrome`) ||
		!strings.Contains(keys[1], `BraveSoftware`) {
		t.Errorf("framePolicyKeys(chrome+brave) = %v, want the Chrome and Brave policy keys", keys)
	}
	if got := framePolicyKeys(func(string) bool { return false }); len(got) != 0 {
		t.Errorf("framePolicyKeys(none) = %v, want empty", got)
	}
	if got := framePolicyKeys(func(string) bool { return true }); len(got) != len(framePolicyTargets) {
		t.Errorf("framePolicyKeys(all) = %v, want %d keys", got, len(framePolicyTargets))
	}
}

func TestFramePolicyDetection(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("LOCALAPPDATA", tempDir)

	mkProfile := func(parts ...string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(append(parts, "Default")...), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mkProfile(tempDir, "Google", "Chrome", "User Data")
	mkProfile(tempDir, "BraveSoftware", "Brave-Browser", "User Data")

	installed := detectInstalledChromiumBrowsers()
	if !installed["chrome"] || !installed["brave"] {
		t.Errorf("chrome and brave should be detected, got %v", installed)
	}
	if installed["chromium"] {
		t.Error("chromium should not be detected without a profile")
	}
}

func TestApplyBrowserFramePolicyRejectsBadColor(t *testing.T) {
	// An unusable background must fail fast, before any registry access.
	if _, err := applyBrowserFramePolicy(map[string]string{"background": "nope"}); err == nil {
		t.Fatal("expected an error for an unusable background color")
	}
}

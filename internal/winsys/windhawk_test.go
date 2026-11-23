//go:build windows

package winsys

import "testing"

func TestWindhawkNoOpStub(t *testing.T) {
for _, mod := range knownStylerMods {
if IsWindhawkModInstalled(mod) {
t.Fatalf("expected IsWindhawkModInstalled(%q) to be false", mod)
}
}
applied, warnings := ApplyWindhawkTheme(map[string]string{"accent": "#f38b83"}, false)
if len(applied) != 0 || len(warnings) != 0 {
t.Fatalf("expected no applied mods or warnings, got applied=%v warnings=%v", applied, warnings)
}
}

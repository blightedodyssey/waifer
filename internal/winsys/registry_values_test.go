package winsys

import (
	"strings"
	"testing"
)

func registryMap(values []registryValue) map[string]registryValue {
	out := make(map[string]registryValue, len(values))
	for _, value := range values {
		out[value.Path+`\`+value.Name] = value
	}
	return out
}

func TestSystemThemeValuesDark(t *testing.T) {
	values, err := systemThemeValues(map[string]string{
		"accent":     "#c09ef5",
		"background": "#141114",
	}, false)
	if err != nil {
		t.Fatalf("systemThemeValues: %v", err)
	}
	got := registryMap(values)

	checks := []struct {
		path string
		name string
		want uint32
	}{
		{personalizeKey, "ColorPrevalence", 1},
		{personalizeKey, "AppsUseLightTheme", 0},
		{personalizeKey, "SystemUsesLightTheme", 0},
		{dwmKey, "ColorPrevalence", 1},
		{dwmKey, "AccentColor", 0xFFF59EC0},
		{dwmKey, "ColorizationColor", 0xC4C09EF5},
		{dwmKey, "ColorizationAfterglow", 0xC4C09EF5},
		{explorerAccentKey, "AccentColorMenu", 0xFFF59EC0},
		{explorerAccentKey, "StartColorMenu", 0xFFF59EC0},
	}
	for _, check := range checks {
		value, ok := got[check.path+`\`+check.name]
		if !ok {
			t.Fatalf("missing %s\\%s", check.path, check.name)
		}
		if value.Type != registryDWORD {
			t.Errorf("%s type = %d, want DWORD", check.name, value.Type)
		}
		if value.DWord != check.want {
			t.Errorf("%s = %#x, want %#x", check.name, value.DWord, check.want)
		}
	}

	inactive, ok := got[dwmKey+`\AccentColorInactive`]
	if !ok {
		t.Fatal("missing AccentColorInactive")
	}
	if inactive.DWord == got[dwmKey+`\AccentColor`].DWord {
		t.Error("AccentColorInactive should differ from AccentColor")
	}
}

func TestSystemThemeValuesLightMode(t *testing.T) {
	values, err := systemThemeValues(map[string]string{
		"accent":     "#c09ef5",
		"background": "#fdf6e3",
	}, true)
	if err != nil {
		t.Fatalf("systemThemeValues: %v", err)
	}
	got := registryMap(values)
	if got[personalizeKey+`\AppsUseLightTheme`].DWord != 1 {
		t.Error("AppsUseLightTheme should be 1 in light mode")
	}
	if got[personalizeKey+`\ColorPrevalence`].DWord != 1 {
		t.Error("ColorPrevalence should stay enabled in light mode")
	}
}

func TestSystemThemeValuesWithoutAccent(t *testing.T) {
	values, err := systemThemeValues(map[string]string{"background": "#141114"}, false)
	if err != nil {
		t.Fatalf("systemThemeValues without accent: %v", err)
	}
	got := registryMap(values)
	if got[personalizeKey+`\AppsUseLightTheme`].DWord != 0 {
		t.Error("personalization values should still be written without an accent")
	}
	if _, ok := got[dwmKey+`\AccentColor`]; ok {
		t.Error("DWM accent should be skipped when no accent color is available")
	}

	if _, err := systemThemeValues(map[string]string{"accent": "not-a-color"}, false); err == nil {
		t.Error("expected invalid accent color to fail")
	}
}

func TestColorConversion(t *testing.T) {
	abgr, err := abgrColor("#c09ef5")
	if err != nil {
		t.Fatal(err)
	}
	if abgr != 0xFFF59EC0 {
		t.Errorf("abgr = %#x, want 0xFFF59EC0", abgr)
	}
	argb, err := argbColor("#c09ef5", 0xC4)
	if err != nil {
		t.Fatal(err)
	}
	if argb != 0xC4C09EF5 {
		t.Errorf("argb = %#x, want 0xC4C09EF5", argb)
	}
	if got := blendHex("#ffffff", "#000000", 0.5); got != "#808080" {
		t.Errorf("blend = %s, want #808080", got)
	}
	if got := normalizeHex("#ABCDEF"); got != "#abcdef" {
		t.Errorf("normalize = %s, want #abcdef", got)
	}
}

func TestRegistryTransactionCoversSystem(t *testing.T) {
	values, err := systemThemeValues(map[string]string{
		"accent":     "#c09ef5",
		"background": "#141114",
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	joined := make([]string, 0, len(values))
	for _, value := range values {
		joined = append(joined, value.Path+`\`+value.Name)
	}
	all := strings.Join(joined, "\n")
	for _, want := range []string{
		`Themes\Personalize\ColorPrevalence`,
		`Themes\Personalize\AppsUseLightTheme`,
		`DWM\AccentColor`,
		`DWM\AccentColorInactive`,
	} {
		if !strings.Contains(all, want) {
			t.Errorf("transaction is missing %s", want)
		}
	}
	// Frame colors are owned by internal/browsers, not the system transaction.
	if strings.Contains(all, "BrowserThemeColor") {
		t.Error("BrowserThemeColor should not be part of the system transaction")
	}
}

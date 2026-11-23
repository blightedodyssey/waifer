package blueprint

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeImportFixture(t *testing.T, ext, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "theme"+ext)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func pywalColorJSON(t *testing.T, values [16]string) string {
	t.Helper()
	var out strings.Builder
	out.WriteString(`{"colors":{`)
	for i, value := range values {
		if i > 0 {
			out.WriteByte(',')
		}
		fmt.Fprintf(&out, "%q:%q", fmt.Sprintf("color%d", i), value)
	}
	out.WriteString(`},"special":{"background":"#000000","foreground":"#ffffff","cursor":"#abcdef","accent":"#fedcba"}}`)
	return out.String()
}

func TestImportBase24MapsStandardAndExtendedColors(t *testing.T) {
	base := func(i int) string { return fmt.Sprintf("#%02X%02X%02X", i, i, i) }
	var content strings.Builder
	fmt.Fprintln(&content, "scheme: Waifu24")
	fmt.Fprintln(&content, "author: Test Author")
	for i := 0; i < 24; i++ {
		key := fmt.Sprintf("base%02X", i)
		switch {
		case i >= 0x10 && i%2 == 0:
			fmt.Fprintf(&content, "%s: %q # extended slot\n", key, base(i)[1:])
		case i >= 0x10:
			fmt.Fprintf(&content, "%s: %s\n", key, base(i))
		default:
			fmt.Fprintf(&content, "%s: %s\n", key, base(i)[1:])
		}
	}

	bp, err := ImportBase24(writeImportFixture(t, ".yaml", content.String()))
	if err != nil {
		t.Fatal(err)
	}
	if bp.Name != "Waifu24" {
		t.Errorf("name = %q, want Waifu24", bp.Name)
	}
	want := []string{
		base(0x00), base(0x08), base(0x0B), base(0x0A),
		base(0x0D), base(0x0E), base(0x0C), base(0x05),
		base(0x03), base(0x08), base(0x0B), base(0x0A),
		base(0x0D), base(0x0E), base(0x0C), base(0x07),
	}
	if len(bp.Palette.Colors) != len(want) {
		t.Fatalf("got %d ANSI colors, want %d", len(bp.Palette.Colors), len(want))
	}
	for i := range want {
		if bp.Palette.Colors[i] != want[i] {
			t.Errorf("color%d = %q, want %q", i, bp.Palette.Colors[i], want[i])
		}
	}
	if len(bp.Palette.ExtendedColors) != 8 {
		t.Fatalf("got %d extended colors, want 8: %v", len(bp.Palette.ExtendedColors), bp.Palette.ExtendedColors)
	}
	for i := 0x10; i <= 0x17; i++ {
		key := fmt.Sprintf("base%02X", i)
		if got := bp.Palette.ExtendedColors[key]; got != base(i) {
			t.Errorf("extended %s = %q, want %q", key, got, base(i))
		}
	}
}

func TestImportBase24HandlesEmptyAndQuotedExtendedValues(t *testing.T) {
	var content strings.Builder
	fmt.Fprintln(&content, "# generated fixture")
	for i := 0; i < 16; i++ {
		fmt.Fprintf(&content, "base%02X: %06X\n", i, i)
	}
	fmt.Fprintln(&content, "base10:")
	fmt.Fprintln(&content, "base11: 454545 # inline comment")
	fmt.Fprintln(&content, `base12: "#464646" # quoted value`)

	bp, err := ImportBase24(writeImportFixture(t, ".yml", content.String()))
	if err != nil {
		t.Fatal(err)
	}
	if bp.Name != "theme" {
		t.Errorf("name = %q, want filename fallback theme", bp.Name)
	}
	checks := map[string]string{
		"base10": "",
		"base11": "#454545",
		"base12": "#464646",
	}
	for key, want := range checks {
		got, ok := bp.Palette.ExtendedColors[key]
		if !ok {
			t.Errorf("extended color %s is missing", key)
			continue
		}
		if got != want {
			t.Errorf("extended %s = %q, want %q", key, got, want)
		}
	}
	if bp.Palette.Colors[0] != "#000000" || bp.Palette.Colors[7] != "#000005" {
		t.Errorf("unexpected ANSI endpoints: %v", bp.Palette.Colors)
	}
}

func TestImportBase24RequiresEightBaseColors(t *testing.T) {
	content := "scheme: Sparse\nbase00: 000000\nbase01: 111111\nbase02: 222222\nbase10: 101010\n"
	_, err := ImportBase24(writeImportFixture(t, ".yaml", content))
	if err == nil || !strings.Contains(err.Error(), "not enough colors") {
		t.Fatalf("ImportBase24() error = %v, want not-enough-colors error", err)
	}
}

func TestImportPywalNestedWallustShape(t *testing.T) {
	var values [16]string
	for i := range values {
		value := fmt.Sprintf("%06x", i)
		if i%2 == 0 {
			value = "#" + value
		}
		values[i] = value
	}
	content := strings.Replace(pywalColorJSON(t, values), `"colors":{`, `"wallust":{"mode":"dark"},"colors":{`, 1)

	bp, err := ImportPywal(writeImportFixture(t, ".json", content))
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range values {
		expected := "#" + strings.TrimPrefix(want, "#")
		if bp.Palette.Colors[i] != expected {
			t.Errorf("color%d = %q, want %q", i, bp.Palette.Colors[i], expected)
		}
	}
	for key, want := range map[string]string{
		"background": "#000000",
		"foreground": "#ffffff",
		"cursor":     "#abcdef",
		"accent":     "#fedcba",
	} {
		if got := bp.Palette.ExtendedColors[key]; got != want {
			t.Errorf("extended %s = %q, want %q", key, got, want)
		}
	}
	if bp.Palette.Mode != "dark" || bp.Palette.LightMode {
		t.Errorf("mode = %q, lightMode = %v; want dark/false", bp.Palette.Mode, bp.Palette.LightMode)
	}
}

func TestImportPywalFlatShape(t *testing.T) {
	var doc strings.Builder
	doc.WriteString(`{"pywal":{"mode":"LIGHT"},`)
	for i := 0; i < 16; i++ {
		fmt.Fprintf(&doc, "%q:%q,", fmt.Sprintf("color%d", i), fmt.Sprintf("#%06X", i))
	}
	doc.WriteString(`"background":"#000000","foreground":"#ffffff","cursor":"#ffffff","accent":"#123456"}`)

	bp, err := ImportPywal(writeImportFixture(t, ".json", doc.String()))
	if err != nil {
		t.Fatal(err)
	}
	if bp.Palette.Colors[0] != "#000000" || bp.Palette.Colors[15] != "#00000F" {
		t.Errorf("unexpected flat palette endpoints: %v", bp.Palette.Colors)
	}
	if got := bp.Palette.ExtendedColors["accent"]; got != "#123456" {
		t.Errorf("accent = %q, want #123456", got)
	}
	if bp.Palette.Mode != "light" || !bp.Palette.LightMode {
		t.Errorf("mode = %q, lightMode = %v; want light/true", bp.Palette.Mode, bp.Palette.LightMode)
	}
}

func TestImportPywalRejectsInvalidHex(t *testing.T) {
	var values [16]string
	for i := range values {
		values[i] = fmt.Sprintf("#%06X", i)
	}
	values[1] = "zzzzzz"
	content := pywalColorJSON(t, values)

	_, err := ImportPywal(writeImportFixture(t, ".json", content))
	if err == nil || !strings.Contains(err.Error(), "palette color 1 is not a hex color") {
		t.Fatalf("ImportPywal() error = %v, want hex validation error", err)
	}
}

func TestImportAutoDispatch(t *testing.T) {
	var values [16]string
	for i := range values {
		values[i] = fmt.Sprintf("#%06X", i)
	}
	bp, err := ImportAuto(writeImportFixture(t, ".json", pywalColorJSON(t, values)))
	if err != nil || len(bp.Palette.Colors) != 16 {
		t.Fatalf("ImportAuto(JSON) = %v, %v", bp, err)
	}

	var yamlContent strings.Builder
	for i := 0; i < 16; i++ {
		fmt.Fprintf(&yamlContent, "base%02X: %06X\n", i, i)
	}
	bp, err = ImportAuto(writeImportFixture(t, ".yml", yamlContent.String()))
	if err != nil || bp.Palette.Colors[0] != "#000000" {
		t.Fatalf("ImportAuto(Base16 YAML) = %v, %v", bp, err)
	}

	fmt.Fprintln(&yamlContent, "base10: 101010")
	bp, err = ImportAuto(writeImportFixture(t, ".yaml", yamlContent.String()))
	if err != nil || bp.Palette.ExtendedColors["base10"] != "#101010" {
		t.Fatalf("ImportAuto(Base24 YAML) = %v, %v", bp, err)
	}

	var tomlContent strings.Builder
	for i := 0; i < 16; i++ {
		fmt.Fprintf(&tomlContent, "color%d = %q\n", i, fmt.Sprintf("#%06X", i))
	}
	bp, err = ImportAuto(writeImportFixture(t, ".toml", tomlContent.String()))
	if err != nil || bp.Palette.Colors[1] != "#000001" {
		t.Fatalf("ImportAuto(TOML) = %v, %v", bp, err)
	}
}

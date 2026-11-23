package cli

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"aether/internal/platform"
)

// Version is set at build time via ldflags.
var Version = "dev"

// Run dispatches CLI commands. Returns exit code.
func Run(args []string, templatesFS embed.FS) int {
	if len(args) == 0 {
		printUsage()
		return 1
	}

	cmd := args[0]
	switch cmd {
	// --- Existing commands ---
	case "--generate":
		return runGenerate(args[1:], templatesFS)
	case "--list-blueprints":
		return runListBlueprints(args[1:])
	case "--apply-blueprint":
		return runApplyBlueprint(args[1:], templatesFS)
	case "--import-blueprint":
		return runImportBlueprint(args[1:], templatesFS)
	case "--import-base16":
		return runImportBase16(args[1:], templatesFS)
	case "--import-colors-toml":
		return runImportColorsToml(args[1:], templatesFS)
	case "--handle-url":
		return runHandleURL(args[1:], templatesFS)

	// --- Color utilities ---
	case "--color-convert":
		return runColorConvert(args[1:])
	case "--contrast":
		return runContrast(args[1:])
	case "--adjust-color":
		return runAdjustColor(args[1:])
	case "--darken":
		return runDarken(args[1:])
	case "--lighten":
		return runLighten(args[1:])
	case "--color-info":
		return runColorInfo(args[1:])

	// --- Palette operations ---
	case "--extract-palette":
		return runExtractPalette(args[1:])
	case "--palette-from-color":
		return runPaletteFromColor(args[1:])
	case "--gradient":
		return runGradient(args[1:])
	case "--adjust-palette":
		return runAdjustPalette(args[1:])
	case "--palette-info":
		return runPaletteInfo(args[1:])

	// --- Blueprint extensions ---
	case "--show-blueprint":
		return runShowBlueprint(args[1:])
	case "--delete-blueprint":
		return runDeleteBlueprint(args[1:])
	case "--export-blueprint":
		return runExportBlueprint(args[1:])

	// --- Inspection ---
	case "--list-apps":
		return runListApps(args[1:], templatesFS)
	case "--list-modes":
		return runListModes(args[1:])
	case "--show-variables":
		return runShowVariables(args[1:], templatesFS)
	case "--current-theme":
		return runCurrentTheme(args[1:])
	case "--preview-template":
		return runPreviewTemplate(args[1:], templatesFS)

	// --- Wallpapers ---
	case "--search-wallhaven":
		return runSearchWallhaven(args[1:])
	case "--wallhaven-thumbs":
		return runWallhavenThumbs(args[1:])
	case "--wallhaven-download":
		return runDownloadWallhaven(args[1:])
	case "--wallhaven-info":
		return runWallhavenInfo(args[1:])
	case "--list-wallpapers":
		return runListWallpapers(args[1:])
	case "--random-wallpaper":
		return runRandomWallpaper(args[1:])

	// --- Favorites ---
	case "--list-favorites":
		return runListFavorites(args[1:])
	case "--toggle-favorite":
		return runToggleFavorite(args[1:])
	case "--is-favorite":
		return runIsFavorite(args[1:])

	// --- Meta ---
	case "upgrade":
		return runUpgrade(args[1:])
	case "--help", "-h":
		printUsage()
		return 0
	case "--version", "-v":
		fmt.Printf("waifer %s\n", Version)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		return 1
	}
}

func printUsage() {
	fmt.Println(`Waifer - Windows Theme Engine

Usage:
  waifer                                    Launch GUI
  waifer upgrade                            Upgrade to the latest release
  waifer --help                             Show this help
  waifer --version                          Show version

Headless mode (no GUI):
  waifer --theme <name>                     Apply a saved theme or blueprint by name
  waifer --wallpaper <path>                 Set desktop wallpaper only
  waifer --list                             List available theme names

Theme generation:
  waifer --generate <wallpaper> [options]   Extract colors and apply theme
    --light-mode                            Generate light variant
    --no-apply                              Render templates without activating
    --output <path>                         Output dir (defaults to %LOCALAPPDATA%\Waifer\current)
    --icon-theme automatic|<ID>              Use color-matched Yaru or an explicit installed theme ID
    --no-zed                                Skip Zed extension (default on)
    --no-vscode                             Skip VSCode integration (default on)
    --no-neovim                             Skip Neovim template (default on)
  waifer --extract-palette <wallpaper>      Extract palette (read-only, no apply)

Blueprint management:
  waifer --list-blueprints                  List saved themes
  waifer --show-blueprint <name>            Show blueprint details
  waifer --apply-blueprint <name>           Apply a saved theme
  waifer --delete-blueprint <name>          Delete a blueprint
  waifer --export-blueprint <name>          Export blueprint to JSON file

Import commands:
  waifer --import-blueprint <url|path>      Import blueprint from URL or file
  waifer --import-base16 <url|file.yaml>    Import Base16 color scheme (URL or file)
    --wallpaper <url|path>                  Set wallpaper from URL or local path
    --light-mode                            Force light mode
  waifer --import-colors-toml <url|file>    Import colors.toml color scheme (URL or file)
    --wallpaper <url|path>                  Set wallpaper from URL or local path
    --light-mode                            Force light mode
  waifer --handle-url <waifer://...>        Handle a web link (confirms unless silent=true)
                                              waifer://apply?external_theme=URL
                                              waifer://apply?colors=URL
                                              waifer://apply?wallpaper=URL
                                              waifer://apply?mode=light|dark
                                              waifer://apply?as_omarchy_theme=NAME

Color utilities:
  waifer --color-info <hex>                 Show all color representations
  waifer --color-convert <hex> [--to fmt]   Convert color (rgb|hsl|oklab|oklch)
  waifer --contrast <color1> <color2>       WCAG contrast ratio + grading
  waifer --adjust-color <hex> [adjustments] Apply adjustment pipeline
  waifer --darken <hex> <percent>           Darken a color
  waifer --lighten <hex> <percent>          Lighten a color

Palette operations:
  waifer --palette-from-color <hex>         Generate 16-color palette from one color
  waifer --gradient <start> <end>           Generate color gradient
  waifer --adjust-palette <colors> [adj]    Adjust entire palette
  waifer --palette-info <colors|blueprint>  Analyze palette

Inspection:
  waifer --list-apps                        List supported applications
  waifer --list-modes                       List extraction modes
  waifer --show-variables <source>          Show template variables
  waifer --current-theme                    Show currently applied theme
  waifer --preview-template <app> <source>  Preview rendered template

Wallpapers:
  waifer --search-wallhaven <query>         Search wallhaven.cc (metadata only)
  waifer --wallhaven-thumbs <query>         Search + download small thumbnails to cache
  waifer --wallhaven-download <id|url>      Download full wallpaper by id or page URL
  waifer --wallhaven-info <id|url>          Show metadata for a single wallpaper
  waifer --list-wallpapers [--with-previews] List local wallpapers (cached 800px previews when flag set)
  waifer --random-wallpaper                 Pick random local wallpaper

  Wallhaven search flags (apply to --search-wallhaven and --wallhaven-thumbs):
    --page N                                Starting page number (default 1)
    --pages N                               (--wallhaven-thumbs only) Fetch N consecutive pages concurrently
    --categories 111                        Bitmask: General/Anime/People
    --purity 100                            Bitmask: SFW/Sketchy/NSFW
    --sorting date_added|relevance|random|views|favorites|toplist
    --order desc|asc
    --at-least 1920x1080                    Minimum resolution
    --colors HEX                            Filter by wallhaven palette color

Favorites:
  waifer --list-favorites                   List all favorites
  waifer --toggle-favorite <path>           Add/remove favorite
  waifer --is-favorite <path>               Check if favorited

Remote control (requires running Waifer GUI):
  waifer status                             Show editor state
  waifer extract <wallpaper>                Load wallpaper into editor
  waifer set-color <index> <hex>            Change palette color
  waifer adjust [--vibrance X] ...          Adjust palette sliders
  waifer set-mode <mode>                    Change extraction mode
  waifer apply                              Apply current theme
  waifer toggle-light-mode                  Toggle light/dark mode
  waifer load-blueprint <name>              Load blueprint into editor
  waifer apply-blueprint <name>             Apply blueprint directly
  waifer set-wallpaper <path>               Set wallpaper path

GUI options:
  waifer --tab <name>                       Open GUI with specific tab

Global options:
  --json                                    Machine-readable JSON output
  --light-mode                              Use light mode
  --extract-mode <mode>                     Extraction mode (see --list-modes)`)
}

func parseFlag(args []string, flag string) (string, []string) {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			remaining := make([]string, 0, len(args)-2)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+2:]...)
			return args[i+1], remaining
		}
	}
	return "", args
}

func hasFlag(args []string, flag string) (bool, []string) {
	for i, arg := range args {
		if arg == flag {
			remaining := make([]string, 0, len(args)-1)
			remaining = append(remaining, args[:i]...)
			remaining = append(remaining, args[i+1:]...)
			return true, remaining
		}
	}
	return false, args
}

// validModes are the allowed extraction modes.
var validModes = map[string]bool{
	"normal": true, "monochromatic": true, "analogous": true,
	"pastel": true, "material": true, "colorful": true,
	"muted": true, "bright": true,
	"complementary": true, "triadic": true,
	"split-complementary": true, "tetradic": true,
	"fire": true, "ocean": true, "forest": true,
	"earthtone": true, "neon": true, "sunset": true, "vaporwave": true,
	"midnight": true, "aurora": true,
	"high-contrast": true, "duotone": true,
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home := platform.HomeDir()
		if home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

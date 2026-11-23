<p align="center">
  <img src="icon.png" alt="Waifer icon" width="256" height="256">
</p>

# Waifer

Waifer is a Windows-native theming engine. It extracts a 16-color palette from a wallpaper and applies one cohesive theme across Windows itself and the applications you actually run.

Waifer is a fork of [Aether](https://github.com/omacom/aether), the Linux/Omarchy theming app. The Go color engine and the Svelte UI come from Aether; the desktop integration layer is rewritten for Windows. Upstream Aether stays the original project — its Linux/Omarchy guides are kept under [`docs/`](docs/) for reference.

## What's new in Waifer 1.0

- **Vanilla Windows native theming, always on** — accent color, title bars, app/system light and dark mode, and the desktop wallpaper, with no helper app required.
- **Presence-first auto-detection** — YASB, GlazeWM, Windhawk stylers, browsers, Discord/Vencord, and AI harnesses are themed only when they are actually installed.
- **Base16/Base24 and Pywal/Wallust imports** — start from an existing palette instead of extracting a new one.
- **Live exports** — `theme.css`, `theme.json`, `colors.ini`, and `colors.ps1` are rewritten on every apply for scripts and external tools.
- **5-slot snapshots with rollback** — the last five applies are captured, so a theme change can be undone.
- **Light/dark scheduler** — switch between light and dark palettes on a daily schedule.
- **`.waifer` bundles** — portable theme packages that carry colors, wallpaper, and custom CSS between machines.

## Features

### Color Extraction

- Pure Go median-cut algorithm that generates a full 16-color ANSI palette from any wallpaper
- 8 extraction modes: Normal, Monochromatic, Analogous, Pastel, Material, Colorful, Muted, and Bright
- 12 fine-tuning sliders (vibrance, contrast, temperature, shadows, highlights, and more)
- Light and dark mode toggle with automatic color anchor swapping

### Wallpaper Tools

- Search and download wallpapers from wallhaven.cc directly in the app
- Browse public GitHub repositories for wallpapers
- Export favorite wallpapers as a ZIP archive with source metadata
- Apply a blurred wallpaper variant while extraction uses the original image
- Full wallpaper editor with blur, exposure, sharpen, vignette, grain, and color toning
- 12 one-click image presets: Cinematic, Vintage, Film, Dramatic, and more

### Theme Library

- 24 built-in color presets including Dracula, Nord, Gruvbox, Catppuccin, and Sakura
- Import 250+ community Base16 color schemes, plus Base24, Pywal, and Wallust palettes
- Save and restore complete themes as blueprint files
- Export themes as shareable `.waifer` bundles with selective app inclusion

### Windows Targets

- **Windows system**: accent color, active/inactive title bars, color prevalence, and app/system light/dark mode, committed in one transaction and broadcast with `WM_SETTINGCHANGE`
- **Windows Terminal**: generated color scheme selected for every profile
- **Editors**: VS Code, Zed, Neovim, and Obsidian
- **Browsers**: Chrome/Chromium (policy-backed accent) and Firefox (`userChrome.css`)
- **Shell replacements**: YASB bars, GlazeWM window manager, and the Windhawk Start Menu, Notification Center, Taskbar, and File Explorer stylers
- **Chat**: Discord and Vencord
- **AI harnesses**: OpenCode, Hermes, Pi, Claude Code, Jan, and Aider
- Template system with hex, RGB, RGBA, and stripped format modifiers, per-app overrides, and post-apply hooks

### Extras

- WCAG contrast ratio checker with AAA/AA accessibility grading
- Gradient generator and single-color palette builder
- 50-step undo/redo history
- Headless CLI for scripting, scheduled tasks, and automation

## Quick Start

### Build from Source

Requirements: Go 1.25+, Node 22+, and the [Wails v2](https://wails.io) CLI.

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0

git clone https://github.com/blightedodyssey/aether-win-clone.git
cd aether-win-clone
.\scripts\build-windows.sh
```

The build produces `build/bin/waifer.exe`. It needs the WebView2 runtime, which ships with Windows 11 and current Windows 10.

```powershell
.\build\bin\waifer.exe
```

See [README-WINDOWS.md](README-WINDOWS.md) for the Windows port details, the files Waifer writes, and known limitations.

### Basic Usage

1. Select a wallpaper (drag & drop, file picker, or wallhaven browser)
2. Click **Extract** to generate a color palette
3. Adjust colors as needed
4. Click **Apply Theme**

## CLI

```powershell
waifer --generate .\wallpaper.jpg
waifer --apply-blueprint "My Theme"
waifer --list-blueprints
```

See `waifer --help` for all options.

## Local Development

```powershell
# Install frontend dependencies
cd frontend; npm install; cd ..

# Run in development mode (hot reload)
wails dev

# Build production binary
wails build
```

### Verification

Run `make test` for the Go package tests. From `frontend/`, run `npm ci`, `npm run check`, `npm test`, and `npm run build`. The frontend regression suite uses mocked Wails calls, so it does not change your desktop. Use `wails dev` for manual verification of native dialogs, rendering, and theme application.

## Documentation

| Guide | Description |
|-------|-------------|
| [Windows Port](README-WINDOWS.md) | Windows targets, written files, build, limitations |
| [Installation](docs/installation.md) | Build and run on Windows |
| [File System](docs/filesystem.md) | Where Waifer stores files |
| [CLI Reference](docs/cli.md) | Command-line options |
| [Color Extraction](docs/color-extraction.md) | How the algorithm works |
| [Base16 Schemes](docs/base16.md) | Import community color schemes |
| [Wallpaper Editor](docs/wallpaper-editor.md) | Image filters and presets |
| [Wallhaven](docs/wallhaven.md) | Browse online wallpapers |
| [GitHub Source](docs/github-source.md) | Browse repository wallpapers |
| [Favorites](docs/favorites.md) | Save wallpapers and export a collection |
| [Blueprints](docs/blueprints.md) | Save and restore themes |
| [Custom Templates](docs/custom-templates.md) | Add support for your apps |
| [Custom Apps](docs/custom-apps.md) | Per-app template system |
| [Remote Control](docs/remote-control.md) | IPC commands and AI integration |
| [Web Handler](docs/web-handler.md) | Handle `waifer://` links |
| [Troubleshooting](docs/troubleshooting.md) | Common issues |

Upstream Aether pages that describe Linux/Omarchy integration are still in `docs/` and are marked as such: [Standalone](docs/standalone.md), [Omarchy Shell Plugins](docs/quickshell.md), and [Protocol Handler](docs/protocol-handler.md).

## Upstream

Waifer tracks [omacom/aether](https://github.com/omacom/aether). Bug reports about the Linux/Omarchy build belong upstream; Waifer issues belong in this repository.

## Contributing

See [CLAUDE.md](CLAUDE.md) for architecture details.

## License

MIT — Created by [Bjarne Overli](https://x.com/iamdothash)

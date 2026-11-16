# Aether for Windows

A Windows port of [Aether](https://github.com/omacom/aether), the wallpaper
color extractor and desktop theming app. It keeps Aether's Go color engine and
Svelte UI, and replaces the Linux/Omarchy integration with a native Windows
one.

> Upstream Aether is MIT licensed; see `LICENSE`. This tree is a fork of the
> upstream snapshot, with the platform layer rewritten for Windows.

## What works

- Wallpaper picking, drag and drop, local scanning, wallhaven search, blur and
  image editing, GitHub wallpaper sources.
- The full 16-color extraction pipeline (all 8 modes, all 12 fine-tuning
  sliders, light/dark palette anchoring), presets, blueprints, favorites,
  gradient/palette generators, WCAG contrast tools.
- **System theme**: sets the Windows accent color, active/inactive title-bar
  colors, color prevalence, and app/system light/dark mode. All theme and
  browser registry values commit in one asynchronous transaction, then a
  `WM_SETTINGCHANGE` broadcast repaints Explorer, Start, and the notification
  center without restarting `explorer.exe`.
- **Chrome / Chromium**: writes the policy-backed `BrowserThemeColor` value so
  the browser follows Aether's accent.
- **Firefox**: finds the active profile in `%APPDATA%\Mozilla\Firefox` and
  generates `chrome/userChrome.css` with the dark palette and lavender accent.
- **Wallpaper**: copies the selected image into `%LOCALAPPDATA%\aether` and sets
  it as the desktop background with fill scaling.
- **Windows Terminal**: upserts an `Aether` color scheme into `settings.json`
  and makes it the default for every profile.
- **VS Code**: installs the generated theme extension into
  `%USERPROFILE%\.vscode\extensions` (same mechanism as upstream).
- **Neovim**: writes a standalone `colors/aether.lua` colorscheme to
  `%LOCALAPPDATA%\nvim`; when a LazyVim `lua/plugins` directory exists the
  generated plugin spec is installed there too.
- **Zed**: installs the generated theme into `%APPDATA%\Zed\themes` and selects
  it in `%APPDATA%\Zed\settings.json`.
- Exports theme bundles (all generated config files) to a folder.

## Files Aether writes

| Target | Path |
| --- | --- |
| Config / settings | `%APPDATA%\aether\` |
| Generated theme files | `%APPDATA%\aether\theme\` |
| Cache, downloads | `%LOCALAPPDATA%\aether\` |
| Managed wallpaper | `%LOCALAPPDATA%\aether\wallpapers\current.<ext>` |
| Windows Terminal | `%LOCALAPPDATA%\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json` (or Preview/Canary/unpackaged) |
| Zed | `%APPDATA%\Zed\settings.json`, `%APPDATA%\Zed\themes\aether.json` |
| Neovim | `%LOCALAPPDATA%\nvim\colors\aether.lua` |
| Firefox | `%APPDATA%\Mozilla\Firefox\Profiles\<profile>\chrome\userChrome.css` (plus a generated `user.js` only when the profile has none) |
| Chrome / Chromium | `HKCU\Software\Policies\Google\Chrome\BrowserThemeColor`, `HKCU\Software\Policies\Chromium\BrowserThemeColor` |

Before the first edit, `settings.json` is backed up next to the original as
`settings.json.aether-backup`. **Revert system theme** restores those backups,
removes the Aether-managed browser files and Chrome policy value, and deletes
the generated editor integrations. It does not restore the previous accent
color, which Windows does not expose.

## Build

Requirements on the build machine: Go 1.25+, Node 22+, and the Wails CLI.

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
./scripts/build-windows.sh
```

The script works from Linux, macOS, or Windows and produces
`build/bin/aether.exe`. The Windows build does not need CGO or mingw; WebView2
is loaded at runtime. The target machine needs the WebView2 runtime, which is
preinstalled on Windows 11 and current Windows 10.

Run it directly:

```powershell
.\aether.exe
```

or build a development version with live reload:

```bash
wails dev
```

## Known limitations

- The binary is unsigned, so SmartScreen warns on first launch.
- The shell refresh is a `WM_SETTINGCHANGE` broadcast. Some third-party shell
  replacements may still ignore it and need their own restart.
- Firefox loads `userChrome.css` at startup, so a running browser must be
  restarted to see the generated theme. If the profile already has a `user.js`,
  Aether leaves it untouched; the `toolkit.legacyUserProfileCustomizations.stylesheets`
  preference must then already be enabled.
- Chrome applies the theme policy dynamically, but an already-running browser
  may keep cached window frames until its next launch.
- Linux-only app targets (Hyprland, Waybar, Wofi, Mako, SwayOSD, Walkers,
  Kitty, Foot, Ghostty, Btop, the Chromium config template, icon themes) are
  hidden on Windows. Chrome accent sync uses the policy-backed path above.
- The `aether://` URL handler and the updater are Linux-only upstream features
  and are not wired up on Windows.
- Set Wallpaper Only applies the background; it does not re-extract colors.

## Layout of the port

- `internal/winsys/` — the Windows integration (transactional registry,
  wallpaper, browser sync, Windows Terminal, Zed, Neovim). Every operation is a
  no-op on other platforms, so the upstream Linux build still works.
- `internal/platform/paths.go` — Windows config/cache/data directories.
- `internal/theme/writer.go` — calls `winsys.Apply` after generating theme
  files, and exposes per-target warnings to the UI.
- `templates/windows-terminal.json` — the Windows Terminal color scheme
  template.
- `frontend/src/lib/stores/platform.svelte.ts` — platform detection used to
  hide Linux-only targets in the UI.

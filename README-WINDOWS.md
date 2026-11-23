# Waifer for Windows

Waifer is a Windows-native fork of [Aether](https://github.com/omacom/aether), the
wallpaper color extractor and desktop theming app. It keeps Aether's Go color
engine and Svelte UI, and replaces the Linux/Omarchy integration with a native
Windows one.

> Upstream Aether is MIT licensed; see `LICENSE`. This tree is a fork of the
> upstream snapshot, with the platform layer rewritten for Windows. Waifer is
> the fork's name; Aether is the original project.

## What's new in Waifer 1.0

- Vanilla Windows native theming, always on: accent color, title bars, app and
  system light/dark mode, desktop wallpaper.
- Presence-first auto-detection: YASB, GlazeWM, Windhawk stylers, browsers,
  Discord/Vencord, and AI harnesses are themed only when installed.
- Base16/Base24 and Pywal/Wallust palette imports.
- Live exports: `theme.css`, `theme.json`, `colors.ini`, `colors.ps1`.
- 5-slot snapshots with rollback.
- Light/dark scheduler.
- `.waifer` bundles for portable themes.

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
  the browser follows Waifer's accent.
- **Firefox**: finds the active profile in `%APPDATA%\Mozilla\Firefox` and
  generates `chrome/userChrome.css` with the dark palette and lavender accent.
- **Wallpaper**: copies the selected image into `%LOCALAPPDATA%\Waifer` and sets
  it as the desktop background with fill scaling.
- **Windows Terminal**: upserts a `Waifer` color scheme into `settings.json`
  and makes it the default for every profile.
- **VS Code**: installs the generated theme extension into
  `%USERPROFILE%\.vscode\extensions` (same mechanism as upstream).
- **Neovim**: writes a standalone `colors/waifer.lua` colorscheme to
  `%LOCALAPPDATA%\nvim`; when a LazyVim `lua/plugins` directory exists the
  generated plugin spec is installed there too.
- **Zed**: installs the generated theme into `%APPDATA%\Zed\themes` and selects
  it in `%APPDATA%\Zed\settings.json`.
- **Shell and desktop**: YASB bars, GlazeWM window manager, the Windhawk Start
  Menu / Notification Center / Taskbar / File Explorer stylers, Discord and
  Vencord, and the AI harnesses OpenCode, Hermes, Pi, Claude Code, Jan, and
  Aider.
- **Live exports**: `%LOCALAPPDATA%\Waifer\current` holds `colors.toml`,
  `theme.css`, `theme.json`, `colors.ini`, `colors.ps1`, `theme.md`, and the
  managed wallpaper, rewritten on every apply.
- **Snapshots and bundles**: the five most recent applies are captured for
  rollback, and themes can be exported and imported as `.waifer` bundles.
- Exports theme bundles (all generated config files) to a folder.

## Files Waifer writes

| Target | Path |
| --- | --- |
| Config / settings | `%APPDATA%\Waifer\` |
| Generated theme files | `%APPDATA%\Waifer\theme\` |
| Live exports, templates, hooks, snapshots | `%LOCALAPPDATA%\Waifer\` |
| Cache, downloads | `%LOCALAPPDATA%\Waifer\cache\`, `%LOCALAPPDATA%\Waifer\data\` |
| Managed wallpaper | `%LOCALAPPDATA%\Waifer\current\wallpaper.<ext>` |
| Windows Terminal | `%LOCALAPPDATA%\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json` (or Preview/Canary/unpackaged) |
| Zed | `%APPDATA%\Zed\settings.json`, `%APPDATA%\Zed\themes\waifer.json` |
| Neovim | `%LOCALAPPDATA%\nvim\colors\waifer.lua` |
| Firefox | `%APPDATA%\Mozilla\Firefox\Profiles\<profile>\chrome\userChrome.css` (plus a generated `user.js` only when the profile has none) |
| Chrome / Chromium | `HKCU\Software\Policies\Google\Chrome\BrowserThemeColor`, `HKCU\Software\Policies\Chromium\BrowserThemeColor` |

Before the first edit, `settings.json` is backed up next to the original as
`settings.json.waifer-backup`. **Revert system theme** restores those backups,
removes the Waifer-managed browser files and Chrome policy value, and deletes
the generated editor integrations. It does not restore the previous accent
color, which Windows does not expose.

## Build

Requirements on the build machine: Go 1.25+, Node 22+, and the Wails CLI.

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@v2.16.0
./scripts/build-windows.sh
```

The script works from Linux, macOS, or Windows and produces
`build/bin/waifer.exe`. The Windows build does not need CGO or mingw; WebView2
is loaded at runtime. The target machine needs the WebView2 runtime, which is
preinstalled on Windows 11 and current Windows 10.

Run it directly:

```powershell
.\waifer.exe
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
  Waifer leaves it untouched; the `toolkit.legacyUserProfileCustomizations.stylesheets`
  preference must then already be enabled.
- Chrome applies the theme policy dynamically, but an already-running browser
  may keep cached window frames until its next launch.
- Upstream Linux targets (Hyprland, Waybar, Wofi, Mako, SwayOSD, Walkers,
  Kitty, Foot, Ghostty, btop, the Chromium config template, icon themes) are not
  part of the Windows build. Chrome accent sync uses the policy-backed path
  above.
- Web links use the `waifer://` scheme; upstream `aether://` links are not
  handled.
- Set Wallpaper Only applies the background; it does not re-extract colors.

## Layout of the port

- `internal/winsys/` — the Windows integration (transactional registry,
  wallpaper, browser sync, Windows Terminal, Zed, Neovim, YASB, GlazeWM,
  Windhawk stylers, Discord/Vencord). Every operation is a no-op on other
  platforms.
- `internal/aetherdir/` — the Waifer ecosystem directories and live exports
  under `%LOCALAPPDATA%\Waifer`.
- `internal/snapshot/` — the five-slot config snapshots and rollback.
- `internal/bundle/` — `.waifer` bundle export and import.
- `internal/platform/paths.go` — config/cache/data directories.
- `internal/theme/writer.go` — calls `winsys.Apply` after generating theme
  files, and exposes per-target warnings to the UI.
- `templates/windows-terminal.json` — the Windows Terminal color scheme
  template.
- `frontend/src/lib/stores/platform.svelte.ts` — platform detection used to
  hide unsupported targets in the UI.

# Aether — Windows Port Audit

**Date:** 2026-11-15
**Scope:** `I:\Downloads\aether-win-clone` (source snapshot) plus the compiled `build/bin/aether.exe` (22,781,952 bytes, built 2026-11-09).
**Method:** Full read of the visible source tree, docs, CI and packaging; symbol/string forensics on the compiled Windows binary (PE symbol extraction and `rg -a` string recovery) to reconstruct the parts of the codebase that are absent from this snapshot.
**Nature:** Read-only audit. No source files were modified; this document is the only addition.

---

## 0. Context: the snapshot is incomplete

The attached clone cannot build — several load-bearing parts of the project referenced by the remaining code are missing:

| Missing from this clone | Referenced by |
|---|---|
| `internal/` (entire engine: `platform`, `theme`, `template`, `winsys`, `omarchy`, `icontheme`, `extraction`, `color`, `blueprint`, `favorites`, `favexport`, `wallhaven`, `wallpaper`, `githubsource`, `batch`, `update`, `ipc`, `pending`) | every root Go file |
| `main.go`, `go.mod`, `go.sum`, `wails.json` | Wails build |
| `templates/` (all app templates incl. `windows-terminal.json`, `warp.yaml`, `vencord.theme.css`) | `theme.NewWriter(EmbeddedTemplates, "templates")` — `app.go:107` |
| `frontend/wailsjs/`, `frontend/src/lib/stores/`, `lib/utils/`, `lib/types/`, `frontend/tests/` | every frontend component |
| `li.oever.aether.url-handler.desktop`, `icon.png` | `.github/workflows/release.yml:75-76` |

Because `internal/` is absent, all statements below about the Windows implementation are sourced from binary forensics on `aether.exe` (function symbols, string constants, registry keys, and the embedded template manifest recovered from the image). Where something is only visible in the binary, it is marked **[binary]**.

**The port is real and further along than the source tree suggests.** The binary contains a genuine `internal/winsys` package implementing wallpaper, accent-color, light/dark-mode, Windows Terminal, Zed, and Neovim integration (details in §3).

### What is already ported correctly (verified in source)

- `App.IsWindows()` / `App.IsMacOS()` bindings (`app.go:56-60`).
- GOOS-branched wallpaper apply: `winsys.SetWallpaper` on Windows, `omarchy.SetBackground` elsewhere (`app.go:516-519`).
- Linux-only targets filtered from the UI in **both** Go (`windowsHiddenApps`, `app.go:1183-1187`) and the frontend (`ActionBar.svelte:135-149`) — duplicated lists that must be kept in sync (see fix plan).
- xdg-desktop-portal file dialogs gated to Linux with Wails-dialog fallback (`dialogs.go:33-36`).
- Icon-theme picker hidden on Windows (`SettingsSidebar.svelte:55`); Omarchy "system" tab gated by capability detection (`App.svelte:310`).
- Config storage routed through `platform.*` helpers; binary evidence (`%AppData% is not defined` — Go `os.UserConfigDir` error text) shows `platform.ConfigDir()` resolves to `%APPDATA%\aether` on Windows, honoring `XDG_CONFIG_HOME` on Linux.
- Windows toast notifications via `git.sr.ht/~jackmordaunt/go-toast/v2` **[binary]**.
- `aether://` protocol registration via registry (CLSID key + GUID + activation executable) **[binary]**.
- Self-upgrade via a generated PowerShell script with `-ExecutionPolicy` handling **[binary]**.

---

## 1. Hardcoded Linux paths & commands

### 1a. Code that runs but does nothing (or logs errors) on Windows

| Location | Issue |
|---|---|
| `app.go:145` | `theme.RetireLegacyGTKStylesheets()` runs unconditionally at startup. GTK-specific (binary contains `inspectLegacyGTKStylesheet`/`verifyLegacyGTKPath`/`removeLegacyGTKStylesheet`); at best a no-op, at worst a logged error on every Windows launch. |
| `app.go:157` | IPC server on a **Unix domain socket** (`ipc.DefaultSocketPath()` → `aether.sock` + `.pid` file; documented at `docs/remote-control.md:5,287-292`). Go's AF_UNIX works on Windows 10 1803+, but the PID-file stale-socket cleanup is Unix-oriented and untested on Windows; no named-pipe fallback exists. |
| `app.go:1173-1199` | `allExportableApps` still enumerates 13 Linux-only apps; exports always land in a folder named **`omarchy-<slug>-theme`** (`app.go:1262`) even on Windows. |
| `app.go:1277` | `os.Symlink` — **fails on Windows without Developer Mode/admin**. Dormant in `ExportTheme` (requires Omarchy), but `platform.CreateSymlink` is used actively by the custom-app pipeline. |
| theme.Writer custom-app pipeline **[binary]**: `start post-apply.sh`, `chmod`, `process custom apps` | The documented flow is render template → **symlink** to destination → run **`post-apply.sh`** (`docs/custom-apps.md:151-158`). On Windows there is no `#!/bin/bash`, `sed -i`, `pgrep`/`pkill` (see `examples/custom/apps/cava/post-apply.sh`), and symlinks need privileges. |
| `cli/cli.go:135` | Help text tells Windows users output "defaults to `~/.config/aether/theme`" — on Windows it is actually `%APPDATA%\aether\theme`. Misleading, not fatal. |
| Embedded templates | All templates ship in the exe regardless of platform. Hidden Linux apps (`hyprland.conf`, `hyprlock.conf`, `kitty.conf`, `waybar.css`, `foot.ini`, `mako.ini`, `wofi.css`, `swayosd.css`, `walker.css`, `ghostty.conf`, `btop.theme`, `chromium.theme`, `icons.theme`) are still rendered into the theme output dir on Windows unless excluded — disk clutter with no consumer. `icons.theme` (GTK icon theme) has no meaning on Windows. |
| Stale comments (`app.go:868-961`) | Doc comments describe storage as `~/.config/aether/...`; the code routes through `platform.*` and resolves correctly. Cosmetic. |

### 1b. Linux-only commands / IPC-reload machinery (source + docs)

There are **no direct `hyprctl`/`systemctl`/`pkill` invocations in the visible Go source** — reloads are delegated to Omarchy (`omarchy theme set`) or handled inside the writer. The Linux command surface lives in:

- `docs/cli.md:505-513` — instructs users to add a Hyprland keybind running `aether --generate $(hyprctl hyprpaper listactive …)`.
- `docs/standalone.md:35-97` — integration instructions are 100% Linux: `source = ~/.config/aether/theme/hyprland.conf`, kitty `include`, `@import url("file:///home/…/waybar.css")`, rofi `@theme`, **`swaybg -i … -m fill`**, `hyprctl reload`.
- `docs/remote-control.md` — Unix-socket semantics, `chmod 0600` permissions, `/home/user/...` example paths.
- `docs/protocol-handler.md`, `Makefile:14-21`, `release.yml` — `xdg-mime`, `.desktop` files, `update-desktop-database`, `gtk-update-icon-cache` — all inert on Windows (the Windows protocol handler itself **is** implemented via registry **[binary]**).
- `contrib/quickshell/` — bash installer + QML plugins targeting `~/.config/omarchy/plugins` and `/usr/share/omarchy/shell`. Entirely Omarchy/Linux.
- `examples/custom/apps/cava/` — destination `~/.config/cava/themes/aether` + bash post-apply using `sed -i`, `pgrep -x cava && pkill -USR2 cava`.
- A `notify-send`-style string (`mode=do-not-disturb app-name=notify-send`) exists inside the Windows exe **[binary]** — a shared notification adapter that should be GOOS-split (the Windows path correctly links `go-toast`/wintoast).

### 1c. Build, CI, packaging — all Linux-only

- `Makefile:3` — `pkg-config webkit2gtk` tag detection; `install:` copies to `/usr/bin`, installs `.desktop` files, runs `xdg-mime` (lines 8-22). **No Windows build/install target at all.**
- `.github/workflows/release.yml` — Linux amd64/arm64 only (`.deb`, AUR). **No Windows artifact is ever produced.** The checked-in `aether.exe` was clearly built by hand on a Windows box.
- `.github/workflows/aur.yml` — AUR packaging.
- `README.md` / `CLAUDE.md` — Arch/Debian instructions, `/usr/bin` installs, webkit2gtk prerequisites; CLAUDE.md points agents to a maintainer-local docs path (`~/Documents/bjarne/projects/Aether`).
- `docs/filesystem.md`, `docs/installation.md`, `docs/troubleshooting.md`, `docs/quickshell.md`, `docs/web-handler.md` — XDG layout, pacman/omarchy commands, Linux paths throughout.

---

## 2. Target mapping gaps

The Targets bar on Windows shows (after `windowsHiddenApps` filtering): **Neovim, Zed, VS Code, Alacritty, Warp, Windows Terminal, Vencord, Zellij**, plus the always-on `colors.toml`. Binary symbol analysis (`aether/internal/winsys.*`, `aether/internal/theme.*`) shows only some of these have real Windows appliers:

| Target | Windows status (verified) | Correct Windows path | Gap |
|---|---|---|---|
| **Neovim** | ✅ **[binary]** `winsys.ApplyNeovim`, `neovimConfigDir`, `neovimColorscheme` — writes an `aether.lua` colorscheme + a lazy.nvim plugin spec (`omacom/aether.nvim`, `lazy = false`) | Config: `%LOCALAPPDATA%\nvim` (stdpath config); colors: `%LOCALAPPDATA%\nvim\colors\aether.lua`; spec: `%LOCALAPPDATA%\nvim\lua\plugins\` | Docs still teach `dofile("~/.config/aether/theme/neovim.lua")`. Verify the config dir is not hardcoded to `~/.config/nvim`. |
| **Zed** | ✅ **[binary]** `winsys.ApplyZed` — creates themes dir, installs theme, **backs up + patches `settings.json`** to select it | Theme: `%APPDATA%\Zed\themes\aether.json`; settings: `%APPDATA%\Zed\settings.json` (Roaming — confirmed by Zed docs) | Solid. |
| **VS Code** | ✅ **[binary]** `theme.ApplyVSCodeTheme` + `processVSCodeExtension` — exports extension `aether-1.0.0` (with `themes/aether-color-theme.json`, `package.json`) into `.vscode\extensions`, sets the color theme, removes stale versions | Extension: `%USERPROFILE%\.vscode\extensions\aether-1.0.0\`; activation: `%APPDATA%\Code\User\settings.json` | No support for **Insiders** (`%APPDATA%\Code - Insiders\User`), **VSCodium** (`%APPDATA%\VSCodium\User`), or profiles. |
| **Windows Terminal** | ✅ **[binary]** `winsys.ApplyWindowsTerminal` — JSONC-aware (`parseJSONC`, `removeTrailingCommas`), `terminalSettingsCandidates` covers Stable/Preview/Canary package families, `upsertScheme` + backup/`Restore` | `%LOCALAPPDATA%\Packages\Microsoft.WindowsTerminal_8wekyb3d8bbwe\LocalState\settings.json` (+ `_Preview_`, `_Canary_` variants) | ❌ **Unpackaged installs** (`%LOCALAPPDATA%\Microsoft\Windows Terminal\settings.json`) are absent from the candidate strings. Verify `upsertScheme` *activates* the scheme (`profiles.defaults.colorScheme`), not merely registers it. |
| **Alacritty** | ❌ **No applier.** `templates/alacritty.toml` is rendered into `%APPDATA%\aether\theme\` only | Config: `%APPDATA%\alacritty\alacritty.toml`; recommended: write theme file + idempotent `import = ["…"]` line (with backup, same pattern as `ApplyZed`) | Full gap — target is togglable in the UI but nothing reaches Alacritty. |
| **Vencord** | ❌ **No applier.** `vencord.theme.css` rendered to theme dir only | Themes: `%APPDATA%\Vencord\themes\aether.theme.css` (needs the `/** @name … */` metadata header Vencord parses) | Full gap — Discord/Vencord never sees it. |
| **Warp** | ❌ **No applier.** `warp.yaml` rendered to theme dir only | Themes: `%APPDATA%\warp\Warp\data\themes\aether.yaml` (Roaming — per Warp docs); config: `%LOCALAPPDATA%\warp\Warp\config\settings.toml` | Full gap. Warp auto-detects themes in that folder; activation is via its theme picker (no documented settings.toml key — document as a manual step). |
| **Zellij** | ❌ **No applier.** `zellij.kdl` rendered to theme dir only | Windows config dir (Zellij ≥ 0.44): `%APPDATA%\Zellij\config`; themes: `%APPDATA%\Zellij\config\themes\aether.kdl` (older builds read `%USERPROFILE%\.config\zellij`) | Full gap — and native-Windows Zellij is brand-new (support merged ~2026). Consider hiding the target until its Windows builds stabilize, or support both paths. |
| `colors` (TOML) | ✅ always written to `%APPDATA%\aether\theme\colors.toml` (+ `colors.v4.toml` for Omarchy v4) | — | Fine. |
| `btop`, `chromium`, `foot`, `ghostty`, `kitty`, `hyprland`, `hyprlock`, `icons`, `mako`, `swayosd`, `walker`, `waybar`, `wofi` | Hidden from UI (correct) but templates still embedded and renderable | e.g. btop4win would use `%USERPROFILE%\.config\btop\themes`; Chrome would need `User Data\Default\Preferences` surgery | Optional future targets (btop4win, Chrome/Edge); keep hidden for now. |

**Summary:** 4 of the 8 visible targets (Alacritty, Vencord, Warp, Zellij) are advertised in the Windows Targets bar but currently only produce orphaned files in `%APPDATA%\aether\theme\`. This is the single biggest functional gap in the port.

---

## 3. Windows system hooks: present vs missing

### Implemented in `internal/winsys` (recovered from the binary)

- **Wallpaper** — `SetWallpaper` via `SystemParametersInfoW` (SPI_SETDESKWALLPAPER), with `Control Panel\Desktop` registry writes for style/tiling ("store wallpaper style/tiling") and a `managedWallpaperPath` copy so the source image cannot vanish.
- **Accent color** — `SetAccentColor`: registry `ColorizationColor` (HKCU\Software\Microsoft\Windows\DWM) + `AccentColorMenu`/`ColorPrevalence` (HKCU\…\Explorer\Accent).
- **Dark/light mode** — `SetLightMode`: `AppsUseLightTheme`, `SystemUsesLightTheme`, `ImmersiveColorSet` under `SOFTWARE\Microsoft\Windows\CurrentVersion\Themes\Personalize`, plus `theme.HandleLightModeMarker`, `applySystemColors` and `notifyColorChange` (settings-change broadcast).
- **Windows Terminal / Zed / Neovim appliers** with atomic writes (`writeFileAtomic`), backups and `Restore`/`restoreBackup`.
- **Toasts** — `go-toast/v2` (wintoast).
- **Protocol handler** — registry-based `aether://` registration (CLSID key, GUID write, "setting activation executable").
- **Self-upgrade** — generated PowerShell script with `-ExecutionPolicy` handling.

### Missing or partial

1. **Windows Terminal activation & unpackaged installs** — likely registers the "Aether" scheme but there is no evidence it sets `profiles.defaults.colorScheme`, and the unpackaged (`%LOCALAPPDATA%\Microsoft\Windows Terminal`) location is absent from the candidates.
2. **Taskbar / Start menu / Explorer theming** — no hooks at all. Windows has no GTK equivalent; realistic integration points:
   - **YASB** (the de-facto "Waybar for Windows") — config at `%USERPROFILE%\.yasb\` (Jinja-templated `config.yaml` + CSS). A YASB theme target is the natural port of the Waybar template.
   - **Windhawk** — mods live in its own app; Aether cannot practically auto-configure it, but could ship a documented "Aether accent CSS" or export theme JSON for mods that accept color config.
   - Native registry niceties (optional): `TaskbarAl`, `EnableTransparency`, taskbar size.
3. **Wallpaper fit control** — style/tiling registry values are written, but there is no user-facing Fill/Fit/Tile/Span setting, and `SystemParametersInfoW` is single-monitor — the modern `IDesktopWallpaper` COM API would enable per-monitor wallpapers and slideshows.
4. **System light/dark sync** — `SetLightMode` exists, but the visible `app.go` apply path never references it; verify Apply/`toggle-light-mode` actually calls it, or wire it.
5. **Revert semantics** — `ClearTheme` (`app.go:523`) needs to restore WT/Zed settings from backups *and* reset accent/dark-mode registry values; only file backups are evident.
6. **IPC transport** — Unix socket only; no `\\.\pipe\aether` named-pipe fallback.

---

## 4. Prioritized fix plan

### P0 — Make the port buildable and reproducible
1. **Restore the missing source** (`internal/`, `main.go`, `go.mod`, `wails.json`, `templates/`, `frontend/wailsjs` + `stores`/`utils`/`types`, the two `.desktop` files, `icon.png`). Everything else is unverifiable guesswork until the tree builds. (The embedded `templates/` set has already been recovered from the exe into this repo — see Appendix A.)
2. **Add Windows build plumbing**: a `wails build -platform windows/amd64` Makefile target; a `build-windows` job in `release.yml` on `windows-latest` producing `aether-windows-amd64.zip` (later: winget manifest / signed installer).
3. **Single source of truth for `windowsHiddenApps`** — duplicated in `app.go:1183` and `ActionBar.svelte:135`; expose it from the backend (or generate the TS constant) and add a sync test.

### P1 — Close the four target mapping gaps
4. **`winsys.ApplyVencord`** → write `%APPDATA%\Vencord\themes\aether.theme.css` (with the Vencord `@name/@author` header).
5. **`winsys.ApplyWarp`** → write `%APPDATA%\warp\Warp\data\themes\aether.yaml`; document the one-time theme-picker activation.
6. **`winsys.ApplyAlacritty`** → write theme + idempotently patch `import = [...]` into `%APPDATA%\alacritty\alacritty.toml` (backup first, same pattern as `ApplyZed`).
7. **`winsys.ApplyZellij`** → write `%APPDATA%\Zellij\config\themes\aether.kdl` with a `%USERPROFILE%\.config\zellij` fallback; or hide the target on Windows until Zellij's Windows support stabilizes.
8. **Custom apps on Windows**: support `post-apply.ps1` (PowerShell) alongside `.sh`, expand `%APPDATA%`/`%LOCALAPPDATA%` in `destination`, and replace mandatory symlinks with copy/hardlink fallback when Developer Mode is off.
9. **VS Code variants**: detect `Code - Insiders` / `VSCodium` user-settings dirs (auto-detect or a setting).

### P2 — Deepen and harden system integration
10. **Windows Terminal**: set `profiles.defaults.colorScheme = "Aether"` (opt-in toggle, backup/restore); add the unpackaged `%LOCALAPPDATA%\Microsoft\Windows Terminal` candidate.
11. **Wallpaper**: expose fit (Fill/Fit/Tile/Center/Span → `WallpaperStyle`/`TileWallpaper` values); optionally move to `IDesktopWallpaper` for per-monitor support.
12. **Wire dark mode to the OS**: ensure Apply/`toggle-light-mode` calls `winsys.SetLightMode` and broadcasts the change; make `ClearTheme` restore registry + file backups.
13. **IPC**: add a named-pipe transport (`\\.\pipe\aether`) with the Unix socket as fallback; verify PID-file stale detection on Windows.
14. **YASB target** (Waybar analog): render a `config.yaml` fragment + CSS into `%USERPROFILE%\.yasb\`; optionally emit a Windhawk-friendly theme doc. This fills the "desktop shell" hole the Linux port covers with Waybar/Wofi.

### P3 — Docs, tests, polish
15. **Docs**: add `docs/windows.md` (AppData layout, supported-targets matrix, limitations — lock screen not themable, Explorer follows accent); make `README.md`, `filesystem.md`, `standalone.md`, `installation.md`, `troubleshooting.md` and the CLI help text (`cli/cli.go:135`) platform-aware; gate Hyprland/swaybg/xdg content to Linux.
16. **Tests**: make the XDG-env-var tests (`app_*_test.go`) platform-aware; add Windows CI (`go test ./...` on `windows-latest`), a path-mapping table test (app → expected Windows destination), a JSONC round-trip test for WT settings patching, and registry set/restore tests using temp keys.
17. **Cleanup**: skip rendering hidden Linux templates into the Windows theme dir (incl. `icons.theme`); stop calling `RetireLegacyGTKStylesheets()` on Windows (GOOS gate at `app.go:145`); GOOS-split the notify-send adapter; drop the `omarchy-` prefix from Windows export folder names.

**Quick wins if only three things are done:** items 4-7 (the four broken targets), 10 (WT scheme activation), and 15 (honest Windows docs) — they convert the port from "wallpaper + accent toy" into a functional theme engine for the apps Windows users actually run.

---

## Appendix A — Embedded template manifest (recovered from `aether.exe`)

24 template files are embedded (not 23 — the earlier estimate missed the second `vscode-extension` file):

```
templates/aether.zed.json
templates/alacritty.toml
templates/btop.theme
templates/chromium.theme
templates/colors.toml
templates/colors.v4.toml
templates/foot.ini
templates/ghostty.conf
templates/hyprland.conf
templates/hyprlock.conf
templates/icons.theme
templates/kitty.conf
templates/mako.ini
templates/neovim.lua
templates/swayosd.css
templates/vencord.theme.css
templates/vscode-extension/package.json
templates/vscode-extension/themes/aether-color-theme.json
templates/vscode.json
templates/walker.css
templates/warp.yaml
templates/waybar.css
templates/windows-terminal.json
templates/zellij.kdl
```

`copy.json`, referenced by `cli/inspect.go:56`, is **not** in the embedded set. All 24 files have been extracted verbatim from the exe into `templates/` in this repository (see § "Extraction log" at the end of this document).

## Appendix B — Recovered `winsys` symbols

```
winsys.Apply                winsys.Restore              winsys.applySystemColors
winsys.SetWallpaper         winsys.SetAccentColor       winsys.SetLightMode
winsys.ApplyWindowsTerminal winsys.ApplyZed             winsys.ApplyNeovim
winsys.findTerminalSettings winsys.terminalSettingsCandidates  winsys.terminalScheme
winsys.upsertScheme         winsys.parseJSONC           winsys.stripJSONComments
winsys.removeTrailingCommas winsys.neovimConfigDir      winsys.neovimColorscheme
winsys.managedWallpaperPath winsys.notifyColorChange    winsys.parseHexColor
winsys.setDWord             winsys.appDataDir           winsys.copyFile
winsys.writeFileAtomic      winsys.restoreBackup
```

Notable absences: `ApplyVencord`, `ApplyWarp`, `ApplyAlacritty`, `ApplyZellij`, `ApplyVSCode` (VS Code is handled in `internal/theme`).

## Appendix C — Correct Windows destination paths (reference)

| App | Config root | Theme destination |
|---|---|---|
| Neovim | `%LOCALAPPDATA%\nvim` | `colors\aether.lua` + `lua\plugins\` spec |
| Zed | `%APPDATA%\Zed` | `themes\aether.json` + `settings.json` patch |
| VS Code | `%APPDATA%\Code` / `%USERPROFILE%\.vscode` | `User\settings.json` + `extensions\aether-1.0.0\` |
| Windows Terminal (packaged) | `%LOCALAPPDATA%\Packages\<PFN>\LocalState` | `settings.json` scheme upsert |
| Windows Terminal (unpackaged) | `%LOCALAPPDATA%\Microsoft\Windows Terminal` | `settings.json` (currently missing) |
| Alacritty | `%APPDATA%\alacritty` | `alacritty.toml` + `import` patch |
| Vencord | `%APPDATA%\Vencord` | `themes\aether.theme.css` |
| Warp | `%APPDATA%\warp\Warp\data` | `themes\aether.yaml` |
| Zellij (≥0.44 Win) | `%APPDATA%\Zellij\config` | `themes\aether.kdl` |
| YASB (proposed) | `%USERPROFILE%\.yasb` | theme `config.yaml` + CSS |
| Aether itself | `%APPDATA%\aether` | settings, blueprints, `theme\` output |

## Extraction log

- **2026-11-15** — all 24 embedded templates recovered byte-for-byte from `build/bin/aether.exe` into `templates/` (`templates/vscode-extension/themes/` subdirectory recreated). Extraction scripts kept in `docs/recovery/` (`extract_templates.py`, `verify_extract.py`, `compare_wallpapers.py`); no source files were touched.

### Extraction validation (all passing)

1. **Independent byte comparison** — the 8 bundled wallpapers (`assets/default-wallpapers/*`), embedded in the same `embed.FS` mechanism, were extracted from the exe and are **byte-identical** to the originals still present in this repo (8/8).
2. **Embedded hash verification** — every `embed.FS` entry stores a 16-byte hash. The binary was built with **Go 1.27.3**, whose compiler (`cmd/compile/internal/staticdata` + `cmd/internal/hash`) computes it as a *decorated* SHA-256, not a plain one:
   - file ≤ 1024 bytes: `sha256(data)[:16]` with **byte 0 XOR 0xFF** (`hash.Sum32`: `sum[0] ^= 0xff`)
   - file > 1024 bytes: streamed via `hash.New32()`, which prefixes `0x01` → `sha256(0x01 || data)[:16]`
   
   With this formula, **32/32** embedded entries (24 templates + 8 wallpapers) verify. `templates/copy.json` remains absent from the embedded set, as noted in Appendix A.

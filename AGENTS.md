# AGENTS.md

Repository layout for the Aether Windows port.

## Top-level
- `app.go` — Wails backend (App struct, bindings to frontend).
- `main.go` — entry point; dispatches CLI vs GUI; Wails run loop.
- `second_instance.go` — single-instance handling.
- `external_import.go` — pending aether:// import flow.
- `default_wallpapers.go` — bundled wallpaper installer.
- `dialogs.go` — native dialogs.
- `cli/` — CLI subcommands and IPC client.
- `ipc/` — IPC protocol, server, client (named pipes on Windows).
- `internal/` — Go packages: color, template, theme, wallpaper, winsys,
  platform, extraction, blueprint, favorites, icontheme, batch, favexport,
  update, pending, wallhaven, githubsource, omarchy.
- `templates/` — embedded application config templates.
- `frontend/` — Svelte + Vite UI. `frontend/dist` is the embedded asset dir.
- `docs/` — audit report and recovery notes.
- `tests` live alongside source (`*_test.go`).

## Build commands
- `go build -o aether.exe .`
- `wails build -platform windows/amd64`
- `go test ./...`
- `go vet ./...`

## Test contracts
- Omarchy CLI is invoked synchronously; tests use `AETHER_COMMAND_LOG`.
- `theme.DefaultPalette` is Catppuccin Macchiato.
- `ipc.Server.Close()` returns error.
- `icontheme.NewCatalogWithRoots([]icontheme.Root{...})`.
- `wallpaper.WallpaperInfo` has a `Size int64` field.
- `color.DarkenRGB`/`LightenRGB` return a single string (empty on invalid).
- `color.HexToRGBA(hex, alpha)` returns an `rgba(...)` string.
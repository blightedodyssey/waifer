# Aether (Windows Port)

Aether is a desktop theming tool that extracts a 16-color palette from a
wallpaper and renders configuration files for a wide range of applications
(terminals, editors, GTK/Qt, icon themes, etc.). This repository is a
Windows port of the upstream Omarchy Aether project.

Upstream: https://github.com/omacom/aether

## Windows features

- Native Windows personalization via the registry and
  `SystemParametersInfoW` (accent color, light/dark mode, wallpaper).
- Windows Terminal, VS Code, Zed, and Neovim integration.
- Wails v2 + Svelte frontend embedded into a single `aether.exe`.
- Named-pipe IPC between the GUI and the `aether` CLI.

## Build

Prerequisites: Go 1.27+, Node 22+ (for the frontend), and (optionally) the
Wails CLI.

```powershell
# Frontend
cd frontend
npm install
npm run build
cd ..

# Backend
go build -o aether.exe .
```

Or with Wails:

```powershell
wails build -platform windows/amd64
```

## Test

```powershell
go test ./...
go vet ./...
```

## License

See LICENSE.
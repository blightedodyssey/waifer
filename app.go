package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aether/internal/aetherdir"
	"aether/internal/batch"
	"aether/internal/blueprint"
	"aether/internal/browsers"
	"aether/internal/bundle"
	"aether/internal/color"
	"aether/internal/extraction"
	"aether/internal/favexport"
	"aether/internal/favorites"
	"aether/internal/githubsource"
	"aether/internal/harnesses"
	"aether/internal/icontheme"
	"aether/internal/omarchy"
	"aether/internal/platform"
	"aether/internal/scheduler"
	"aether/internal/snapshot"
	"aether/internal/template"
	"aether/internal/theme"
	"aether/internal/tray"
	"aether/internal/update"
	"aether/internal/wallhaven"
	"aether/internal/wallpaper"
	"aether/internal/winsys"
	"aether/ipc"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the main Wails application struct.
// All exported methods are automatically bound as frontend API.
type App struct {
	ctx          context.Context
	state        *theme.ThemeState
	history      *theme.HistoryManager
	writer       *theme.Writer
	blueprints   *blueprint.Service
	favorites    *favorites.Service
	favExport    *favexport.Exporter
	wallhaven    *wallhaven.Client
	githubSource *githubsource.Client
	batch        *batch.Processor
	iconThemes   *icontheme.Catalog
	themeWatcher *theme.ThemeWatcher
	ipcServer    *ipc.Server
	pending      pendingImportState
	focusTab     string
}

// IsMacOS returns true when running on macOS.
func (a *App) IsMacOS() bool { return runtime.GOOS == "darwin" }

// IsWindows returns true when running on Windows. The frontend uses it to
// show the app targets that exist on this platform.
func (a *App) IsWindows() bool { return runtime.GOOS == "windows" }

// GetFocusTab returns the tab to focus on startup (empty = default editor).
func (a *App) GetFocusTab() string { return a.focusTab }

// GetThemeColors returns the current parsed colors.toml map. Frontend
// pulls this on mount; the event-emit race means push alone isn't safe.
func (a *App) GetThemeColors() map[string]string {
	return a.themeWatcher.CurrentColors()
}

// GetInitialState returns the current theme state snapshot. Frontend
// pulls this on mount so stores pick up backend-seeded defaults.
func (a *App) GetInitialState() theme.StateSnapshot {
	return a.state.Snapshot()
}

// GetReleaseStatus checks whether currentVersion has a newer GitHub release.
func (a *App) GetReleaseStatus(currentVersion string) (map[string]interface{}, error) {
	release, err := update.Check(context.Background(), currentVersion)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"currentVersion":  release.CurrentVersion,
		"latestVersion":   release.LatestVersion,
		"releaseURL":      release.ReleaseURL,
		"updateAvailable": release.UpdateAvailable,
		"updateCommand":   release.UpdateCommand,
	}, nil
}

// StartUpgrade opens a terminal running this executable's upgrade command.
func (a *App) StartUpgrade() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current executable: %w", err)
	}
	return update.OpenUpgradeTerminal(executable)
}

// NewApp creates a new App instance.
func NewApp() *App {
	wh := wallhaven.NewClient()
	return &App{
		state:        newSeededState(),
		history:      theme.NewHistoryManager(),
		writer:       theme.NewWriter(EmbeddedTemplates, "templates"),
		blueprints:   blueprint.NewService(),
		favorites:    favorites.NewService(),
		favExport:    favexport.New(wh),
		wallhaven:    wh,
		githubSource: githubsource.NewClient(),
		batch:        batch.NewProcessor(),
		iconThemes:   icontheme.NewCatalog(),
		themeWatcher: theme.NewThemeWatcher(),
	}
}

// ListInstalledIconThemes returns the cached installed desktop icon themes.
func (a *App) ListInstalledIconThemes() ([]icontheme.ThemeSummary, error) {
	return a.iconThemes.List(context.Background())
}

// RefreshInstalledIconThemes rescans approved icon roots.
func (a *App) RefreshInstalledIconThemes() ([]icontheme.ThemeSummary, error) {
	return a.iconThemes.Refresh(context.Background())
}

// GetIconThemePreview returns safe backend-rasterized samples for a theme ID.
func (a *App) GetIconThemePreview(themeID string) (icontheme.ThemePreview, error) {
	return a.iconThemes.Preview(context.Background(), themeID)
}

// newSeededState builds a ThemeState with the unmodified DefaultPalette
// and no wallpaper, so first launch (and ResetState) lands on the empty
// editor that walks new users through extracting and applying a theme.
func newSeededState() *theme.ThemeState {
	return theme.NewThemeState()
}

// startup is called by Wails when the application starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	_ = platform.EnsureAllDirs()
	if err := theme.RetireLegacyGTKStylesheets(); err != nil {
		log.Printf("retire legacy GTK stylesheets: %v", err)
	}
	if err := installDefaultWallpapers(); err != nil {
		log.Printf("install default wallpapers: %v", err)
	}
	if err := platform.EnsureURLHandler(ctx); err != nil {
		log.Printf("register waifer:// handler: %v", err)
	}
	a.themeWatcher.Start(ctx)

	// Start IPC server for remote control
	srv, err := ipc.NewServer(ipc.DefaultSocketPath(), a)
	if err != nil {
		log.Printf("ipc: %v", err)
	} else {
		a.ipcServer = srv
	}

	// Pick up a one-click import that was staged before the GUI started
	// (e.g. browser clicked waifer:// while Waifer was closed).
	a.loadPendingImport()

	// Start system tray (Windows + Linux; noop on other platforms)
	if runtime.GOOS == "windows" || runtime.GOOS == "linux" {
		a.startTray()
	}
}

// CloseIPC shuts down the IPC server. Called on app exit.
func (a *App) CloseIPC() {
	if a.ipcServer != nil {
		a.ipcServer.Close()
	}
}

// ---------------------------------------------------------------------------
// Color Extraction
// ---------------------------------------------------------------------------

// ExtractColors extracts a 16-color ANSI palette from an image.
func (a *App) ExtractColors(path string, lightMode bool, mode string) ([16]string, error) {
	if !theme.IsImageFile(path) {
		return [16]string{}, fmt.Errorf("unsupported image file: %s", path)
	}
	palette, err := extraction.ExtractColors(path, lightMode, mode)
	if err != nil {
		return palette, err
	}
	a.state.SetPalette(palette)
	a.state.NativeColors = map[string]string{}
	if a.state.WallpaperPath != path {
		a.state.WallpaperBlur = false
	}
	a.state.WallpaperPath = path
	a.state.LightMode = lightMode
	a.state.ExtractionMode = mode
	return palette, nil
}

// PreviewExtractColors returns a palette for the given mode without mutating
// app state. Used by the UI to render thumbnail strips for each extraction
// mode. extraction.ExtractColors is content-hashed and cached, so repeated
// calls for the same (path, lightMode, mode) tuple are cheap.
func (a *App) PreviewExtractColors(path string, lightMode bool, mode string) ([16]string, error) {
	if !theme.IsImageFile(path) {
		return [16]string{}, fmt.Errorf("unsupported image file: %s", path)
	}
	return extraction.ExtractColors(path, lightMode, mode)
}

// ExtractFromImagesResult is the Wails-friendly return value for ExtractColorsFromImages.
type ExtractFromImagesResult struct {
	Palette [16]string `json:"palette"`
	Skipped int        `json:"skipped"`
}

// ExtractColorsFromImages extracts a blended 16-color palette by sampling pixels
// from multiple images (typically the main wallpaper plus additional images).
// Unreadable or unsupported files are skipped and counted in the result.
func (a *App) ExtractColorsFromImages(paths []string, lightMode bool, mode string) (ExtractFromImagesResult, error) {
	palette, skipped, err := extraction.ExtractColorsFromImages(paths, lightMode, mode)
	if err != nil {
		return ExtractFromImagesResult{}, err
	}
	a.state.SetPalette(palette)
	a.state.NativeColors = map[string]string{}
	a.state.LightMode = lightMode
	a.state.ExtractionMode = mode
	return ExtractFromImagesResult{
		Palette: palette,
		Skipped: skipped,
	}, nil
}

// AdjustPaletteColors applies the adjustment pipeline to all palette colors.
// Uses []string (slice) instead of [16]string for Wails JSON compatibility.
func (a *App) AdjustPaletteColors(palette []string, adj color.Adjustments) []string {
	result := make([]string, len(palette))
	for i, hex := range palette {
		if hex != "" {
			result[i] = color.AdjustColor(hex, adj)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Theme State
// ---------------------------------------------------------------------------

// SetExtractionMode sets the palette extraction mode.
func (a *App) SetExtractionMode(mode string) {
	a.state.ExtractionMode = mode
}

// SyncStateRequest mirrors the frontend editor state into Go so IPC readers
// (`waifer status`, etc.) reflect live edits without requiring Apply.
type SyncStateRequest struct {
	Palette          []string                     `json:"palette"`
	WallpaperPath    string                       `json:"wallpaperPath"`
	WallpaperBlur    bool                         `json:"wallpaperBlur"`
	LightMode        bool                         `json:"lightMode"`
	ExtendedColors   map[string]string            `json:"extendedColors"`
	NativeColors     map[string]string            `json:"nativeColors"`
	AppOverrides     map[string]map[string]string `json:"appOverrides"`
	AdditionalImages []string                     `json:"additionalImages"`
	IconTheme        icontheme.Selection          `json:"iconTheme"`
}

// SyncState is called (debounced) by the frontend whenever the editor state
// changes. Uses SetAdjustedPalette so BasePalette from the last extraction
// is preserved as the "pristine" reference.
func (a *App) SyncState(req SyncStateRequest) error {
	iconTheme, err := icontheme.NormalizeSelection(req.IconTheme)
	if err != nil {
		return fmt.Errorf("iconTheme: %w", err)
	}
	if len(req.Palette) >= 16 {
		var p [16]string
		for i := 0; i < 16; i++ {
			p[i] = req.Palette[i]
		}
		a.state.SetAdjustedPalette(p)
	}
	a.state.WallpaperPath = req.WallpaperPath
	a.state.WallpaperBlur = req.WallpaperBlur
	a.state.LightMode = req.LightMode
	if req.ExtendedColors != nil {
		a.state.ExtendedColors = req.ExtendedColors
	}
	if req.NativeColors != nil {
		a.state.NativeColors = req.NativeColors
	}
	if req.AppOverrides != nil {
		a.state.AppOverrides = req.AppOverrides
	}
	if req.AdditionalImages != nil {
		a.state.AdditionalImages = req.AdditionalImages
	}
	a.state.IconTheme = iconTheme
	return nil
}

// ANSI 16-color palette positions. Names follow xterm convention; the role
// defaults below tie e.g. accent=blue and foreground=white as our chosen mapping.
const (
	idxBlack       = 0
	idxRed         = 1
	idxGreen       = 2
	idxYellow      = 3
	idxBlue        = 4
	idxMagenta     = 5
	idxCyan        = 6
	idxWhite       = 7
	idxBrightBlack = 8
)

// buildColorRoles converts a palette slice into a fixed [16]string array plus
// a populated template.ColorRoles, applying any user overrides from
// extendedColors. Centralizes the palette â†’ roles mapping shared by
// ComputeVariables, ApplyTheme, and ExportTheme.
func buildColorRoles(paletteSlice []string, extendedColors map[string]string) ([16]string, template.ColorRoles) {
	var palette [16]string
	for i := 0; i < 16 && i < len(paletteSlice); i++ {
		palette[i] = paletteSlice[i]
	}

	accent := palette[idxBlue]
	cursor := palette[idxWhite]
	selFg := palette[idxBlack]
	selBg := palette[idxWhite]
	if v := extendedColors["accent"]; v != "" {
		accent = v
	}
	if v := extendedColors["cursor"]; v != "" {
		cursor = v
	}
	if v := extendedColors["selection_foreground"]; v != "" {
		selFg = v
	}
	if v := extendedColors["selection_background"]; v != "" {
		selBg = v
	}

	roles := template.ColorRoles{
		Background: palette[idxBlack], Foreground: palette[idxWhite],
		Black: palette[idxBlack], Red: palette[idxRed], Green: palette[idxGreen], Yellow: palette[idxYellow],
		Blue: palette[idxBlue], Magenta: palette[idxMagenta], Cyan: palette[idxCyan], White: palette[idxWhite],
		BrightBlack: palette[idxBrightBlack], BrightRed: palette[idxBrightBlack+1], BrightGreen: palette[idxBrightBlack+2],
		BrightYellow: palette[idxBrightBlack+3], BrightBlue: palette[idxBrightBlack+4], BrightMagenta: palette[idxBrightBlack+5],
		BrightCyan: palette[idxBrightBlack+6], BrightWhite: palette[idxBrightBlack+7],
		Accent: accent, Cursor: cursor,
		SelectionForeground: selFg, SelectionBackground: selBg,
	}
	return palette, roles
}

// ComputeVariables builds the full template variable map from the given
// palette and extended colors. Returns all base + derived color values.
func (a *App) ComputeVariables(paletteSlice []string, extendedColors map[string]string, lightMode bool) map[string]string {
	_, roles := buildColorRoles(paletteSlice, extendedColors)
	return template.BuildVariables(roles, lightMode, extendedColors)
}

// ---------------------------------------------------------------------------
// Theme Application
// ---------------------------------------------------------------------------

// ApplyThemeRequest is the payload from the frontend containing all current state.
type ApplyThemeRequest struct {
	Palette          []string                     `json:"palette"`
	WallpaperPath    string                       `json:"wallpaperPath"`
	WallpaperBlur    bool                         `json:"wallpaperBlur"`
	LightMode        bool                         `json:"lightMode"`
	AdditionalImages []string                     `json:"additionalImages"`
	ExtendedColors   map[string]string            `json:"extendedColors"`
	NativeColors     map[string]string            `json:"nativeColors"`
	Settings         theme.Settings               `json:"settings"`
	AppOverrides     map[string]map[string]string `json:"appOverrides"`
	IconTheme        icontheme.Selection          `json:"iconTheme"`
}

// ApplyTheme processes all templates and applies the theme to the system.
func (a *App) ApplyTheme(req ApplyThemeRequest) (*theme.ApplyResult, error) {
	if len(req.Palette) < 16 {
		err := fmt.Errorf("apply theme requires a 16-color palette (got %d)", len(req.Palette))
		log.Printf("[ApplyTheme] %v", err)
		return nil, err
	}
	for i := 0; i < 16; i++ {
		if !color.IsHexColor(req.Palette[i]) {
			err := fmt.Errorf("palette color %d (%q) is not a valid hex color", i, req.Palette[i])
			log.Printf("[ApplyTheme] %v", err)
			return nil, err
		}
	}
	if req.WallpaperPath != "" {
		info, err := os.Stat(req.WallpaperPath)
		if err != nil {
			wrapped := fmt.Errorf("wallpaper %q is not accessible: %w", req.WallpaperPath, err)
			log.Printf("[ApplyTheme] %v", wrapped)
			return nil, wrapped
		}
		if !info.Mode().IsRegular() {
			wrapped := fmt.Errorf("wallpaper %q is not a regular file", req.WallpaperPath)
			log.Printf("[ApplyTheme] %v", wrapped)
			return nil, wrapped
		}
	}

	palette, roles := buildColorRoles(req.Palette, req.ExtendedColors)

	appOverrides := req.AppOverrides
	if appOverrides == nil {
		appOverrides = make(map[string]map[string]string)
	}

	state := &theme.ThemeState{
		Palette:          palette,
		WallpaperPath:    req.WallpaperPath,
		WallpaperBlur:    req.WallpaperBlur,
		LightMode:        req.LightMode,
		ColorRoles:       roles,
		ExtendedColors:   req.ExtendedColors,
		NativeColors:     req.NativeColors,
		AdditionalImages: req.AdditionalImages,
		AppOverrides:     appOverrides,
		IconTheme:        req.IconTheme,
	}

	if id, err := snapshot.Capture("apply"); err != nil {
		log.Printf("[ApplyTheme] snapshot warning: %v", err)
	} else {
		log.Printf("[ApplyTheme] snapshot captured: %s", id)
	}

	res, err := a.writer.ApplyTheme(state, req.Settings)
SyncAllNativeTargets(WaiferPalette{
Background: pAt(req.Palette, 0),
Red:        pAt(req.Palette, 1),
Green:      pAt(req.Palette, 2),
Yellow:     pAt(req.Palette, 3),
Blue:       pAt(req.Palette, 4),
Purple:     pAt(req.Palette, 5),
Cyan:       pAt(req.Palette, 6),
Foreground: pAt(req.Palette, 7),
Surface:    pAt(req.Palette, 8),
Accent:     pAt(req.Palette, 12),
})
return res, err
}

// SaveAndApplyThemeRequest is the payload for saving the current state as a
// named theme folder before activating it.
type SaveAndApplyThemeRequest struct {
	Name             string                       `json:"name"`
	UpdateExisting   bool                         `json:"updateExisting"`
	Palette          []string                     `json:"palette"`
	WallpaperPath    string                       `json:"wallpaperPath"`
	WallpaperBlur    bool                         `json:"wallpaperBlur"`
	LightMode        bool                         `json:"lightMode"`
	AdditionalImages []string                     `json:"additionalImages"`
	ExtendedColors   map[string]string            `json:"extendedColors"`
	NativeColors     map[string]string            `json:"nativeColors"`
	Settings         theme.Settings               `json:"settings"`
	AppOverrides     map[string]map[string]string `json:"appOverrides"`
	IconTheme        icontheme.Selection          `json:"iconTheme"`
}

// SaveAndApplyTheme writes a reusable named theme folder before applying it.
func (a *App) SaveAndApplyTheme(req SaveAndApplyThemeRequest) (*theme.ApplyResult, error) {
	name := req.Name
	if name == "" || name[0] == '-' {
		return nil, fmt.Errorf("theme name must start with a lowercase letter or digit")
	}
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
			return nil, fmt.Errorf("theme name must use lowercase letters, digits, and hyphens")
		}
	}

	palette, roles := buildColorRoles(req.Palette, req.ExtendedColors)
	state := &theme.ThemeState{
		Palette:          palette,
		WallpaperPath:    req.WallpaperPath,
		WallpaperBlur:    req.WallpaperBlur,
		LightMode:        req.LightMode,
		ColorRoles:       roles,
		ExtendedColors:   req.ExtendedColors,
		NativeColors:     req.NativeColors,
		AdditionalImages: req.AdditionalImages,
		AppOverrides:     req.AppOverrides,
		IconTheme:        req.IconTheme,
	}
	if state.AppOverrides == nil {
		state.AppOverrides = make(map[string]map[string]string)
	}

	isOmarchy := theme.IsOmarchyInstalled()
	targetDir := filepath.Join(platform.SavedThemesDir(), name)
	if isOmarchy {
		targetDir = filepath.Join(omarchy.UserThemesDir(), name)
		if !req.UpdateExisting && omarchy.ThemeExists(name) {
			return nil, fmt.Errorf("Omarchy theme %q already exists", name)
		}
	}
	if _, err := os.Stat(targetDir); err == nil {
		if !req.UpdateExisting {
			return nil, fmt.Errorf("theme folder %q already exists", name)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("check theme folder: %w", err)
	}
	if isOmarchy {
		return a.writer.SaveAndApplyOmarchyTheme(state, req.Settings, name)
	}
	if err := a.writer.GenerateOnly(state, req.Settings, targetDir); err != nil {
		return nil, fmt.Errorf("save theme: %w", err)
	}
	if id, err := snapshot.Capture("apply"); err != nil {
		log.Printf("[ApplyTheme] snapshot warning: %v", err)
	} else {
		log.Printf("[ApplyTheme] snapshot captured: %s", id)
	}

	res, err := a.writer.ApplyTheme(state, req.Settings)
SyncAllNativeTargets(WaiferPalette{
Background: pAt(req.Palette, 0),
Red:        pAt(req.Palette, 1),
Green:      pAt(req.Palette, 2),
Yellow:     pAt(req.Palette, 3),
Blue:       pAt(req.Palette, 4),
Purple:     pAt(req.Palette, 5),
Cyan:       pAt(req.Palette, 6),
Foreground: pAt(req.Palette, 7),
Surface:    pAt(req.Palette, 8),
Accent:     pAt(req.Palette, 12),
})
return res, err
}

// ThemeFolderExists checks the output adapter's destination for a saved theme.
func (a *App) ThemeFolderExists(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if !theme.ValidOmarchyThemeName(name) {
		return false
	}
	root := platform.SavedThemesDir()
	if omarchy.IsInstalled() {
		root = omarchy.UserThemesDir()
	}
	info, err := os.Stat(filepath.Join(root, name))
	return err == nil && info.IsDir()
}

// ListGitHubImages lists a public repository directory.
func (a *App) ListGitHubImages(rawURL string) (*githubsource.ListContentsResult, error) {
	return a.githubSource.ListImages(rawURL)
}

// GetGitHubThumbnail returns a bounded, cached preview for a remote wallpaper.
func (a *App) GetGitHubThumbnail(rawURL string) (*githubsource.ThumbnailResult, error) {
	return githubsource.DownloadThumbnail(rawURL)
}

// BlurWallpaper prepares a cached preview of the derived wallpaper.
func (a *App) BlurWallpaper(path string) (string, error) {
	return wallpaper.CreateBlurredVariant(path, platform.BlurDir())
}

// ApplyWallpaperOnly preserves the active theme and changes its background.
func (a *App) ApplyWallpaperOnly(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("wallpaper must be a readable regular file")
	}
	if err := wallpaper.ValidateImageFile(path); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		return winsys.SetWallpaper(path)
	}
	return omarchy.SetBackground(path)
}

// ClearTheme removes the Waifer theme and reverts to the default.
func (a *App) ClearTheme() error {
	return theme.ClearTheme()
}

// ---------------------------------------------------------------------------
// Blueprints
// ---------------------------------------------------------------------------

// ListBlueprints returns all saved blueprints.
func (a *App) ListBlueprints() ([]map[string]interface{}, error) {
	bps, err := a.blueprints.LoadAll()
	if err != nil {
		return nil, err
	}
	// Convert to raw maps to avoid Wails model conversion issues
	result := make([]map[string]interface{}, len(bps))
	for i, bp := range bps {
		iconTheme, err := bp.IconThemeSelection()
		if err != nil {
			return nil, fmt.Errorf("blueprint %q icon theme: %w", bp.Name, err)
		}
		result[i] = map[string]interface{}{
			"name":      bp.Name,
			"timestamp": bp.Timestamp,
			"palette": map[string]interface{}{
				"colors":           bp.Palette.Colors,
				"wallpaper":        bp.Palette.Wallpaper,
				"wallpaperBlur":    bp.Palette.WallpaperBlur,
				"lightMode":        bp.Palette.LightMode,
				"mode":             bp.Palette.Mode,
				"lockedColors":     bp.Palette.LockedColors,
				"extendedColors":   bp.Palette.ExtendedColors,
				"nativeColors":     bp.Palette.NativeColors,
				"additionalImages": bp.Palette.AdditionalImages,
			},
			"adjustments":  bp.Adjustments,
			"appOverrides": bp.AppOverrides,
			"iconTheme":    iconTheme,
		}
	}
	return result, nil
}

// SaveBlueprint saves the current state as a named blueprint.
// SaveBlueprintRequest contains all state needed to save a blueprint.
type SaveBlueprintRequest struct {
	Name             string                       `json:"name"`
	Palette          []string                     `json:"palette"`
	WallpaperPath    string                       `json:"wallpaperPath"`
	WallpaperBlur    bool                         `json:"wallpaperBlur"`
	LightMode        bool                         `json:"lightMode"`
	AdditionalImages []string                     `json:"additionalImages"`
	LockedColors     []int                        `json:"lockedColors"`
	ExtendedColors   map[string]string            `json:"extendedColors"`
	NativeColors     map[string]string            `json:"nativeColors"`
	AppOverrides     map[string]map[string]string `json:"appOverrides"`
	Adjustments      map[string]float64           `json:"adjustments"`
	IconTheme        icontheme.Selection          `json:"iconTheme"`
}

func (a *App) SaveBlueprint(req SaveBlueprintRequest) error {
	bp := blueprint.Blueprint{
		Palette: blueprint.PaletteData{
			Colors:           req.Palette,
			Wallpaper:        req.WallpaperPath,
			WallpaperBlur:    req.WallpaperBlur,
			LightMode:        req.LightMode,
			AdditionalImages: req.AdditionalImages,
			LockedColors:     req.LockedColors,
			ExtendedColors:   req.ExtendedColors,
			NativeColors:     req.NativeColors,
		},
		Adjustments:  req.Adjustments,
		AppOverrides: req.AppOverrides,
	}
	if err := bp.SetIconThemeSelection(req.IconTheme); err != nil {
		return fmt.Errorf("iconTheme: %w", err)
	}
	return a.blueprints.Save(req.Name, bp)
}

// resolveWallpaper returns a local wallpaper path from a blueprint. If the local
// path exists it is returned directly. Otherwise, if a URL is available, the
// wallpaper is downloaded first.
func (a *App) resolveWallpaper(palette blueprint.PaletteData) string {
	// Use local path if it exists.
	if palette.Wallpaper != "" && platform.FileExists(palette.Wallpaper) {
		return palette.Wallpaper
	}

	// Try downloading from URL.
	if palette.WallpaperURL != "" {
		localPath, err := a.wallhaven.DownloadFromURL(palette.WallpaperURL)
		if err != nil {
			log.Printf("Warning: could not download wallpaper from %s: %v", palette.WallpaperURL, err)
			return ""
		}
		log.Printf("Downloaded wallpaper: %s", localPath)
		return localPath
	}

	return ""
}

// LoadBlueprint loads a blueprint by name into the current state.
func (a *App) LoadBlueprint(name string) error {
	bp, err := a.blueprints.FindByName(name)
	if err != nil {
		return err
	}
	if bp == nil {
		return fmt.Errorf("blueprint %q not found", name)
	}

	a.history.Push(*a.state)

	var palette [16]string
	for i := 0; i < 16 && i < len(bp.Palette.Colors); i++ {
		palette[i] = bp.Palette.Colors[i]
	}

	if bp.Palette.ExtendedColors != nil {
		a.state.ExtendedColors = bp.Palette.ExtendedColors
	} else {
		a.state.ExtendedColors = map[string]string{}
	}
	if bp.Palette.NativeColors != nil {
		a.state.NativeColors = bp.Palette.NativeColors
	} else {
		a.state.NativeColors = map[string]string{}
	}

	a.state.SetPalette(palette)
	a.state.WallpaperPath = a.resolveWallpaper(bp.Palette)
	a.state.WallpaperBlur = bp.Palette.WallpaperBlur
	a.state.LightMode = bp.Palette.LightMode
	if bp.Palette.AdditionalImages != nil {
		a.state.AdditionalImages = bp.Palette.AdditionalImages
	} else {
		a.state.AdditionalImages = []string{}
	}
	a.state.Adjustments = a.adjustmentsFromBlueprint(bp)
	a.state.AppOverrides = a.appOverridesFromBlueprint(bp)
	iconTheme, err := bp.IconThemeSelection()
	if err != nil {
		return fmt.Errorf("blueprint iconTheme: %w", err)
	}
	a.state.IconTheme = iconTheme
	return nil
}

// ApplyBlueprint loads a blueprint by name and applies it as the active theme.
func (a *App) ApplyBlueprint(name string) (*theme.ApplyResult, error) {
	bp, err := a.blueprints.FindByName(name)
	if err != nil {
		return nil, err
	}
	if bp == nil {
		return nil, fmt.Errorf("blueprint %q not found", name)
	}

	var palette [16]string
	for i := 0; i < 16 && i < len(bp.Palette.Colors); i++ {
		palette[i] = bp.Palette.Colors[i]
	}

	if bp.Palette.ExtendedColors != nil {
		a.state.ExtendedColors = bp.Palette.ExtendedColors
	} else {
		a.state.ExtendedColors = map[string]string{}
	}
	if bp.Palette.NativeColors != nil {
		a.state.NativeColors = bp.Palette.NativeColors
	} else {
		a.state.NativeColors = map[string]string{}
	}

	a.state.SetPalette(palette)
	a.state.WallpaperPath = a.resolveWallpaper(bp.Palette)
	a.state.WallpaperBlur = bp.Palette.WallpaperBlur
	a.state.LightMode = bp.Palette.LightMode
	if bp.Palette.AdditionalImages != nil {
		a.state.AdditionalImages = bp.Palette.AdditionalImages
	} else {
		a.state.AdditionalImages = []string{}
	}
	a.state.Adjustments = a.adjustmentsFromBlueprint(bp)
	a.state.AppOverrides = a.appOverridesFromBlueprint(bp)
	iconTheme, err := bp.IconThemeSelection()
	if err != nil {
		return nil, fmt.Errorf("blueprint iconTheme: %w", err)
	}
	a.state.IconTheme = iconTheme

	return a.writer.ApplyTheme(a.state, theme.DefaultApplySettings())
}

// SnapshotCapture creates a configuration snapshot with the given label.
func (a *App) SnapshotCapture(label string) (string, error) {
	return snapshot.Capture(label)
}

// SnapshotList returns all available snapshots.
func (a *App) SnapshotList() ([]snapshot.Info, error) {
	return snapshot.List()
}

// SnapshotRollback restores configuration from the snapshot with the given ID.
func (a *App) SnapshotRollback(id string) error {
	return snapshot.Rollback(id)
}

// BundleExport exports the current theme as a .waifer bundle.
func (a *App) BundleExport(dest string) error {
	colorsPath := filepath.Join(aetherdir.CurrentDir(), "colors.toml")
	wallpaperPath := ""
	if wp := aetherdir.CurrentDir(); wp != "" {
		if files, err := os.ReadDir(wp); err == nil {
			for _, f := range files {
				if strings.HasPrefix(f.Name(), "wallpaper.") && !strings.HasSuffix(f.Name(), ".path") {
					wallpaperPath = filepath.Join(wp, f.Name())
					break
				}
			}
		}
	}
	customCSSPath := filepath.Join(aetherdir.CurrentDir(), "custom.css")
	if _, err := os.Stat(customCSSPath); os.IsNotExist(err) {
		customCSSPath = ""
	}
	return bundle.Export(dest, colorsPath, wallpaperPath, customCSSPath)
}

// BundleImport extracts a .waifer bundle to the given path.
func (a *App) BundleImport(src string) (*bundle.Result, error) {
	return bundle.Import(src)
}

// BundleExportDialog asks where to save a .waifer bundle, then writes the
// current theme's colors.toml, wallpaper and custom.css to it.
func (a *App) BundleExportDialog() (string, error) {
	path, err := a.saveFileDialog(wailsrt.SaveDialogOptions{
		Title:           "Export Waifer Bundle",
		DefaultFilename: "waifer-theme.waifer",
		Filters: []wailsrt.FileFilter{
			{DisplayName: "Waifer bundles (*.waifer)", Pattern: "*.waifer"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("cancelled")
	}
	if err := a.BundleExport(path); err != nil {
		return "", err
	}
	return path, nil
}

// BundleImportDialog asks for a .waifer bundle, extracts it to the bundles
// folder, and loads the extracted colors.toml (and wallpaper) into the editor.
func (a *App) BundleImportDialog() (*ImportResult, error) {
	path, err := a.openFileDialog(wailsrt.OpenDialogOptions{
		Title: "Import Waifer Bundle",
		Filters: []wailsrt.FileFilter{
			{DisplayName: "Waifer bundles (*.waifer)", Pattern: "*.waifer"},
			{DisplayName: "All Files", Pattern: "*"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("dialog error: %w", err)
	}
	if path == "" {
		return nil, fmt.Errorf("cancelled")
	}
	result, err := a.BundleImport(path)
	if err != nil {
		return nil, err
	}
	loaded, err := a.importFile(result.ColorsTOML, "toml")
	if err != nil {
		return nil, err
	}
	if result.Wallpaper != "" {
		loaded.WallpaperPath = result.Wallpaper
		a.state.WallpaperPath = result.Wallpaper
	}
	return loaded, nil
}

// SchedulerLoad loads scheduler settings from %LOCALAPPDATA%\Waifer\scheduler.json.
func (a *App) SchedulerLoad() (scheduler.Settings, error) {
	return scheduler.Load()
}

// SchedulerSave persists scheduler settings to %LOCALAPPDATA%\Waifer\scheduler.json.
func (a *App) SchedulerSave(settings scheduler.Settings) error {
	return scheduler.Save(settings)
}

// SchedulerDesiredMode returns the desired mode (light/dark) based on current time and settings.
func (a *App) SchedulerDesiredMode(now time.Time) string {
	var settings scheduler.Settings
	if loaded, err := scheduler.Load(); err == nil {
		settings = loaded
	}
	return settings.DesiredMode(now)
}

// SchedulerShouldSwitch returns true if the active blueprint should switch based on scheduler settings.
func (a *App) SchedulerShouldSwitch(current string) bool {
	var settings scheduler.Settings
	if loaded, err := scheduler.Load(); err == nil {
		settings = loaded
	}
	return settings.ShouldSwitch(current)
}

// BlueprintExists checks whether a blueprint with the given name already exists.
func (a *App) BlueprintExists(name string) bool {
	bp, _ := a.blueprints.FindByName(name)
	return bp != nil
}

// DeleteBlueprint removes a blueprint by name.
func (a *App) DeleteBlueprint(name string) error {
	return a.blueprints.Delete(name)
}

// adjustmentsFromBlueprint converts a blueprint's stored adjustments map to a
// typed Adjustments struct, falling back to defaults when the blueprint has none
// or when the round-trip fails (preserving defaults rather than half-applied state).
func (a *App) adjustmentsFromBlueprint(bp *blueprint.Blueprint) color.Adjustments {
	defaults := color.DefaultAdjustments()
	if len(bp.Adjustments) == 0 {
		return defaults
	}
	data, err := json.Marshal(bp.Adjustments)
	if err != nil {
		return defaults
	}
	adj := defaults
	if err := json.Unmarshal(data, &adj); err != nil {
		return defaults
	}
	return adj
}

// appOverridesFromBlueprint returns a deep copy of the blueprint's app overrides
// or an empty map. A copy is needed to avoid aliasing the blueprint's data.
func (a *App) appOverridesFromBlueprint(bp *blueprint.Blueprint) map[string]map[string]string {
	result := make(map[string]map[string]string, len(bp.AppOverrides))
	for app, roles := range bp.AppOverrides {
		m := make(map[string]string, len(roles))
		for k, v := range roles {
			m[k] = v
		}
		result[app] = m
	}
	return result
}

// ---------------------------------------------------------------------------
// Favorites
// ---------------------------------------------------------------------------

// GetFavorites returns all favorited wallpapers.
func (a *App) GetFavorites() []favorites.Favorite {
	return a.favorites.GetAll()
}

// ToggleFavorite adds or removes a favorite. Returns true if now favorited.
func (a *App) ToggleFavorite(path, favType string, data map[string]interface{}) bool {
	return a.favorites.Toggle(path, favType, data)
}

// IsFavorite checks if a path is favorited.
func (a *App) IsFavorite(path string) bool {
	return a.favorites.IsFavorite(path)
}

// ExportFavoritesRequest is the payload from the frontend for zipping favorites.
type ExportFavoritesRequest struct {
	Paths []string `json:"paths"` // favorite paths, in display order
}

// ExportFavorites archives the given favorites into a .zip in a user-chosen
// directory. Wallhaven favorites are remote URLs, so anything not already
// downloaded is fetched first â€” which makes this slow enough that the work runs
// in the background and reports through favorites-export-* events. Returns the
// path the archive is being written to.
func (a *App) ExportFavorites(req ExportFavoritesRequest) (string, error) {
	if a.favExport.IsRunning() {
		return "", fmt.Errorf("an export is already running")
	}
	items := a.favoriteItems(req.Paths)
	if len(items) == 0 {
		return "", fmt.Errorf("no favorites to export")
	}

	dir, err := a.openDirectoryDialog(wailsrt.OpenDialogOptions{
		Title:                "Choose Export Directory",
		CanCreateDirectories: true,
	})
	if err != nil {
		return "", fmt.Errorf("choose export directory: %w", err)
	}
	if dir == "" {
		return "", fmt.Errorf("export cancelled")
	}

	return a.favExport.Start(a.ctx, items, dir)
}

// CancelFavoritesExport stops a running favorites export.
func (a *App) CancelFavoritesExport() { a.favExport.Cancel() }

// IsFavoritesExportRunning reports whether an export is in flight. The frontend
// uses this to recover its progress state after a reload.
func (a *App) IsFavoritesExportRunning() bool { return a.favExport.IsRunning() }

// favoriteItems resolves frontend-supplied paths against the favorites store.
// Only the path crosses the boundary â€” names and metadata are read back from
// the service so the archive cannot be steered by the caller.
func (a *App) favoriteItems(paths []string) []favexport.Item {
	known := make(map[string]favorites.Favorite)
	for _, fav := range a.favorites.GetAll() {
		known[fav.Path] = fav
	}

	items := make([]favexport.Item, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, path := range paths {
		fav, ok := known[path]
		if !ok || seen[path] {
			continue
		}
		seen[path] = true

		item := favexport.Item{Path: fav.Path, Meta: map[string]interface{}{}}
		if fav.Type != "" {
			item.Meta["type"] = fav.Type
		}
		for k, v := range fav.Data {
			if v == nil {
				continue
			}
			item.Meta[k] = v
		}
		// The tile label is the local name, falling back to the wallhaven id.
		if name, ok := fav.Data["name"].(string); ok {
			item.Name = name
		} else if id, ok := fav.Data["id"].(string); ok {
			item.Name = id
		}
		items = append(items, item)
	}
	return items
}

// ---------------------------------------------------------------------------
// App Settings (template toggles, neovim config)
// ---------------------------------------------------------------------------

const wallpaperFolderSetting = "wallpaperFolder"

// GetSettings reads saved app settings from ~/.config/waifer/settings.json.
func (a *App) GetSettings() map[string]interface{} {
	config := readConfigJSON("settings.json")
	if config == nil {
		config = make(map[string]interface{})
	}
	config[wallpaperFolderSetting] = wallpaperFolderFromConfig(config)
	return config
}

// SaveSettings writes app settings to ~/.config/waifer/settings.json.
func (a *App) SaveSettings(config map[string]interface{}) error {
	return writeConfigJSON("settings.json", config)
}

// ChooseWallpaperFolder opens a native directory picker and saves the selected
// path as the local wallpaper library.
func (a *App) ChooseWallpaperFolder() (string, error) {
	defaultDir := wallpaperFolderFromConfig(readConfigJSON("settings.json"))
	if info, err := os.Stat(defaultDir); err != nil || !info.IsDir() {
		defaultDir = platform.WallpaperDir()
		if err := platform.EnsureDir(defaultDir); err != nil {
			return "", fmt.Errorf("prepare default wallpaper directory: %w", err)
		}
	}

	dir, err := a.openDirectoryDialog(wailsrt.OpenDialogOptions{
		Title:                "Choose Wallpaper Folder",
		DefaultDirectory:     defaultDir,
		CanCreateDirectories: true,
	})
	if err != nil || dir == "" {
		return dir, err
	}

	config := readConfigJSON("settings.json")
	if config == nil {
		config = make(map[string]interface{})
	}
	config[wallpaperFolderSetting] = dir
	if err := writeConfigJSON("settings.json", config); err != nil {
		return "", fmt.Errorf("save wallpaper folder: %w", err)
	}
	return dir, nil
}

// ---------------------------------------------------------------------------
// Wallpaper Tags
// ---------------------------------------------------------------------------

// GetWallpaperTags reads wallpaper tags from ~/.config/waifer/tags.json.
func (a *App) GetWallpaperTags() map[string]interface{} {
	return readConfigJSON("tags.json")
}

// SaveWallpaperTags writes wallpaper tags to ~/.config/waifer/tags.json.
func (a *App) SaveWallpaperTags(tags map[string]interface{}) error {
	return writeConfigJSON("tags.json", tags)
}

// ---------------------------------------------------------------------------
// Color Utilities
// ---------------------------------------------------------------------------

// GenerateGradient generates a 16-step gradient between two colors.
func (a *App) GenerateGradient(start, end string) [16]string {
	return color.GenerateGradient(start, end)
}

// GeneratePaletteFromColor generates a full palette from a single color.
func (a *App) GeneratePaletteFromColor(baseColor string) [16]string {
	return color.GeneratePaletteFromColor(baseColor)
}

// ContrastRatio calculates the WCAG contrast ratio between two colors.
func (a *App) ContrastRatio(hex1, hex2 string) float64 {
	return color.ContrastRatio(hex1, hex2)
}

// ---------------------------------------------------------------------------
// Wallhaven
// ---------------------------------------------------------------------------

// SetWallhavenAPIKey sets the wallhaven API key for authenticated requests.
func (a *App) SetWallhavenAPIKey(key string) {
	a.wallhaven.SetAPIKey(key)
}

// GetWallhavenConfig reads the saved wallhaven settings from ~/.config/waifer/wallhaven.json.
func (a *App) GetWallhavenConfig() map[string]interface{} {
	return readConfigJSON("wallhaven.json")
}

// SaveWallhavenConfig writes wallhaven settings to ~/.config/waifer/wallhaven.json.
func (a *App) SaveWallhavenConfig(config map[string]interface{}) error {
	return writeConfigJSON("wallhaven.json", config)
}

// SearchWallhaven searches wallhaven.cc for wallpapers.
func (a *App) SearchWallhaven(params wallhaven.SearchParams) (*wallhaven.SearchResult, error) {
	return a.wallhaven.SearchMultiPage(params, 2)
}

// DownloadWallpaper downloads a wallpaper from a URL. Returns local path.
func (a *App) DownloadWallpaper(imageURL string) (string, error) {
	return a.wallhaven.Download(imageURL)
}

// DownloadWallhaven saves a full-resolution wallhaven image (the data[].path
// value) into the user's wallpaper folder (~/Pictures/Wallpapers on Windows)
// as wallhaven-<id>.<ext> and returns the local path. It reuses an existing
// file when present. Errors are returned so the Wails frontend can surface
// them to the user.
func (a *App) DownloadWallhaven(id, imageURL string) (string, error) {
	return a.wallhaven.DownloadToWallpapers(id, imageURL)
}

// ---------------------------------------------------------------------------
// Local Wallpapers
// ---------------------------------------------------------------------------

// ScanLocalWallpapers scans the configured local wallpaper directory.
func (a *App) ScanLocalWallpapers() ([]wallpaper.WallpaperInfo, error) {
	dir := wallpaperFolderFromConfig(readConfigJSON("settings.json"))
	if dir == platform.WallpaperDir() {
		return wallpaper.ScanDefaultDirs()
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("access wallpaper folder: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("wallpaper folder is not a directory: %s", dir)
	}
	return wallpaper.ScanDirectory(dir)
}

// GetThumbnail returns a thumbnail path for an image.
func (a *App) GetThumbnail(path string) (string, error) {
	return wallpaper.GetThumbnail(path)
}

// GetPreview returns an 800px preview path for the slider widget.
func (a *App) GetPreview(path string) (string, error) {
	return wallpaper.GetPreview(path)
}

// IsPreviewCached returns true if an 800px preview already exists and is fresh.
func (a *App) IsPreviewCached(path string) bool {
	return wallpaper.IsPreviewCached(path)
}

// ---------------------------------------------------------------------------
// Omarchy Themes
// ---------------------------------------------------------------------------

// LoadOmarchyThemes discovers all installed Omarchy themes.
func (a *App) LoadOmarchyThemes() ([]omarchy.Theme, error) {
	return omarchy.LoadAllThemes()
}

// ApplyOmarchyThemeByName activates an existing theme through Omarchy without
// processing it through Waifer templates.
func (a *App) ApplyOmarchyThemeByName(name string) error {
	if !omarchy.ThemeExists(name) {
		return fmt.Errorf("Omarchy theme %q is available for editing only", name)
	}
	return omarchy.ActivateTheme(name, "")
}

// IsOmarchyInstalled returns true if the current system has Omarchy.
func (a *App) IsOmarchyInstalled() bool {
	return omarchy.IsInstalled()
}

// GetOmarchyCapabilities returns native Omarchy availability and state.
func (a *App) GetOmarchyCapabilities() omarchy.Capabilities {
	return omarchy.DetectCapabilities()
}

// ---------------------------------------------------------------------------
// Batch Processing
// ---------------------------------------------------------------------------

// StartBatchProcessing begins batch color extraction.
func (a *App) StartBatchProcessing(paths []string, lightMode bool) {
	a.batch.Start(a.ctx, paths, lightMode)
}

// CancelBatchProcessing stops the current batch.
func (a *App) CancelBatchProcessing() {
	a.batch.Cancel()
}

// ---------------------------------------------------------------------------
// File / Image Utilities
// ---------------------------------------------------------------------------

// ReadImageAsDataURL reads a local image file and returns it as a base64 data URL.
// This is needed because webkit2gtk cannot load file:// paths directly.
func (a *App) ReadImageAsDataURL(path string) (string, error) {
	if !theme.IsImageFile(path) {
		return "", fmt.Errorf("unsupported image file: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	// Detect MIME type from extension
	ext := strings.ToLower(filepath.Ext(path))
	mime := "image/jpeg"
	switch ext {
	case ".png":
		mime = "image/png"
	case ".gif":
		mime = "image/gif"
	case ".webp":
		mime = "image/webp"
	case ".bmp":
		mime = "image/bmp"
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mime, encoded), nil
}

// SaveDataURLToFile saves a base64 data URL as a file.
// Returns the saved file path.
func (a *App) SaveDataURLToFile(dataURL string, originalPath string) (string, error) {
	// Parse data URL: "data:image/jpeg;base64,/9j/4AAQ..."
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid data URL")
	}

	decoded, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	// Determine extension from the data URL mime type
	ext := ".jpg"
	if strings.Contains(parts[0], "image/png") {
		ext = ".png"
	}

	// Save to cache dir with a unique name based on original
	cacheDir := filepath.Join(platform.CacheDir(), "filtered")
	_ = platform.EnsureDir(cacheDir)

	baseName := strings.TrimSuffix(filepath.Base(originalPath), filepath.Ext(originalPath))
	outPath := filepath.Join(cacheDir, baseName+"_filtered"+ext)

	if err := os.WriteFile(outPath, decoded, 0644); err != nil {
		return "", fmt.Errorf("write file failed: %w", err)
	}

	log.Printf("[canvas-export] Saved filtered image: %s (%d bytes)", outPath, len(decoded))
	return outPath, nil
}

// OpenFileDialog opens a native file dialog for selecting an image.
// Returns the selected file path or empty string if cancelled.
func (a *App) OpenFileDialog() (string, error) {
	path, err := a.openFileDialog(wailsrt.OpenDialogOptions{
		Title: "Select Wallpaper",
		Filters: []wailsrt.FileFilter{
			{
				DisplayName: "Images",
				Pattern:     "*.jpg;*.jpeg;*.png;*.gif;*.webp;*.bmp",
			},
		},
	})
	if err != nil {
		return "", err
	}
	return path, nil
}

// HandleDroppedFiles processes file paths from drag-and-drop.
// Takes the first image file from the dropped list.
// Returns the file path if valid, error otherwise.
func (a *App) HandleDroppedFiles(paths []string) (string, error) {
	for _, path := range paths {
		if theme.IsImageFile(path) {
			return path, nil
		}
	}
	return "", fmt.Errorf("no valid image files in drop")
}

// ---------------------------------------------------------------------------
// Export / Import via GUI dialogs
// ---------------------------------------------------------------------------

// ExportThemeRequest is the payload from the frontend for exporting a theme.
type ExportThemeRequest struct {
	Name             string                       `json:"name"`
	IncludedApps     []string                     `json:"includedApps"`
	Palette          []string                     `json:"palette"`
	WallpaperPath    string                       `json:"wallpaperPath"`
	WallpaperBlur    bool                         `json:"wallpaperBlur"`
	LightMode        bool                         `json:"lightMode"`
	AdditionalImages []string                     `json:"additionalImages"`
	ExtendedColors   map[string]string            `json:"extendedColors"`
	NativeColors     map[string]string            `json:"nativeColors"`
	InstallToOmarchy bool                         `json:"installToOmarchy"`
	AppOverrides     map[string]map[string]string `json:"appOverrides"`
	IconTheme        icontheme.Selection          `json:"iconTheme"`
}

// allExportableApps is the full set of app names that can be exported.
var allExportableApps = map[string]bool{
	"alacritty": true, "btop": true, "chromium": true, "colors": true,
	"foot": true, "ghostty": true, "hyprland": true, "hyprlock": true,
	"icons": true, "kitty": true, "mako": true, "neovim": true,
	"obsidian": true, "swayosd": true, "vencord": true, "vscode": true,
	"walker": true, "warp": true, "waybar": true, "windows-terminal": true,
	"wofi": true, "zed": true,
}

// windowsHiddenApps are Linux desktop integrations that have no meaningful
// Windows target. They are filtered out of the app lists on Windows.
var windowsHiddenApps = map[string]bool{
	"btop": true, "chromium": true, "foot": true, "ghostty": true,
	"hyprland": true, "hyprlock": true, "icons": true, "kitty": true,
	"mako": true, "swayosd": true, "walker": true, "waybar": true, "wofi": true,
	"zellij": true,
}

// ExportTheme exports the current theme to a user-chosen directory.
func (a *App) ExportTheme(req ExportThemeRequest) (string, error) {
	// Sanitize the user-typed name. Omarchy's menu chokes on spaces and
	// punctuation when selecting an entry, so the directory and symlink
	// names need to be a slug regardless of installToOmarchy â€” the
	// resulting export folder is also more portable that way.
	slug := omarchy.SlugifyThemeName(req.Name)
	if slug == "" {
		return "", fmt.Errorf("theme name must contain letters or digits")
	}

	dir, err := a.openDirectoryDialog(wailsrt.OpenDialogOptions{
		Title: "Choose Export Directory",
	})
	if err != nil || dir == "" {
		return "", fmt.Errorf("export cancelled")
	}
	isOmarchy := theme.IsOmarchyInstalled()
	linkPath := ""
	if req.InstallToOmarchy && isOmarchy {
		if omarchy.ThemeExists(slug) {
			return "", fmt.Errorf("Omarchy theme %q already exists", slug)
		}
		linkPath = filepath.Join(platform.OmarchyThemesDir(), slug)
		if _, err := os.Lstat(linkPath); err == nil {
			return "", fmt.Errorf("Omarchy theme %q already exists", slug)
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("inspect Omarchy theme %q: %w", slug, err)
		}
	}

	// Build theme state from the frontend's current palette
	palette, roles := buildColorRoles(req.Palette, req.ExtendedColors)

	exportOverrides := req.AppOverrides
	if exportOverrides == nil {
		exportOverrides = make(map[string]map[string]string)
	}

	state := &theme.ThemeState{
		Palette:          palette,
		WallpaperPath:    req.WallpaperPath,
		WallpaperBlur:    req.WallpaperBlur,
		LightMode:        req.LightMode,
		ColorRoles:       roles,
		ExtendedColors:   req.ExtendedColors,
		NativeColors:     req.NativeColors,
		AdditionalImages: req.AdditionalImages,
		AppOverrides:     exportOverrides,
		IconTheme:        req.IconTheme,
	}

	// Build included set from the request
	included := make(map[string]bool, len(req.IncludedApps))
	for _, app := range req.IncludedApps {
		included[app] = true
	}

	// Derive excluded apps: everything in allExportableApps that is NOT included
	excluded := make(map[string]bool)
	for app := range allExportableApps {
		if !included[app] {
			excluded[app] = true
		}
	}

	settings := theme.Settings{
		IncludeZed:    included["zed"],
		IncludeVscode: included["vscode"],
		IncludeNeovim: included["neovim"],
		ExcludedApps:  excluded,
	}

	exportDir := filepath.Join(dir, "omarchy-"+slug+"-theme")
	var exportErr error
	if isOmarchy {
		exportErr = a.writer.GenerateOmarchyV4Only(state, settings, exportDir)
	} else {
		exportErr = a.writer.GenerateOnly(state, settings, exportDir)
	}
	if exportErr != nil {
		return "", fmt.Errorf("export failed: %w", exportErr)
	}

	if req.InstallToOmarchy && isOmarchy {
		if err := platform.EnsureDir(filepath.Dir(linkPath)); err != nil {
			return "", fmt.Errorf("create Omarchy themes directory: %w", err)
		}
		if err := os.Symlink(exportDir, linkPath); err != nil {
			return "", fmt.Errorf("install Omarchy theme: %w", err)
		}
		log.Printf("Installed as Omarchy theme: %s -> %s", linkPath, exportDir)
	}

	return exportDir, nil
}

// ImportResult is returned by ImportFileDialog with the imported colors.
type ImportResult struct {
	Colors         []string            `json:"colors"`
	ExtendedColors map[string]string   `json:"extendedColors"`
	NativeColors   map[string]string   `json:"nativeColors"`
	Name           string              `json:"name"`
	Path           string              `json:"path"`
	WallpaperPath  string              `json:"wallpaperPath"`
	WallpaperBlur  bool                `json:"wallpaperBlur"`
	LightMode      bool                `json:"lightMode"`
	IconTheme      icontheme.Selection `json:"iconTheme"`
}

// ImportFileDialog opens a file dialog for importing a theme file.
// fileType: "base16", "toml", or "blueprint"
// Returns the imported colors so the frontend can update its store.
func (a *App) ImportFileDialog(fileType string) (*ImportResult, error) {
	var filters []wailsrt.FileFilter
	var title string

	switch fileType {
	case "base16":
		title = "Import Base16 Scheme"
		filters = []wailsrt.FileFilter{
			{DisplayName: "YAML Files", Pattern: "*.yaml"},
			{DisplayName: "YML Files", Pattern: "*.yml"},
			{DisplayName: "All Files", Pattern: "*"},
		}
	case "base24":
		title = "Import Base24 Scheme"
		filters = []wailsrt.FileFilter{
			{DisplayName: "YAML Files", Pattern: "*.yaml"},
			{DisplayName: "YML Files", Pattern: "*.yml"},
			{DisplayName: "All Files", Pattern: "*"},
		}
	case "pywal":
		title = "Import Pywal / Wallust Palette"
		filters = []wailsrt.FileFilter{
			{DisplayName: "JSON Files", Pattern: "*.json"},
			{DisplayName: "All Files", Pattern: "*"},
		}
	case "toml":
		title = "Import Colors TOML"
		filters = []wailsrt.FileFilter{
			{DisplayName: "TOML Files", Pattern: "*.toml"},
			{DisplayName: "All Files", Pattern: "*"},
		}
	case "blueprint":
		title = "Import Blueprint"
		filters = []wailsrt.FileFilter{
			{DisplayName: "JSON Files", Pattern: "*.json"},
			{DisplayName: "All Files", Pattern: "*"},
		}
	default:
		return nil, fmt.Errorf("unknown file type: %s", fileType)
	}

	path, err := a.openFileDialog(wailsrt.OpenDialogOptions{
		Title:   title,
		Filters: filters,
	})
	if err != nil {
		log.Printf("[import] dialog error: %v", err)
		return nil, fmt.Errorf("dialog error: %w", err)
	}
	if path == "" {
		return nil, fmt.Errorf("cancelled")
	}

	log.Printf("[import] selected file: %s (type=%s)", path, fileType)
	return a.importFile(path, fileType)
}

func (a *App) importFile(path, fileType string) (*ImportResult, error) {
	var bp *blueprint.Blueprint
	var err error

	switch fileType {
	case "base16":
		bp, err = blueprint.ImportBase16(path)
	case "base24":
		bp, err = blueprint.ImportBase24(path)
	case "pywal":
		bp, err = blueprint.ImportPywal(path)
	case "toml":
		bp, err = blueprint.ImportColorsToml(path)
	case "blueprint":
		bp, err = blueprint.ImportJSON(path)
	default:
		return nil, fmt.Errorf("unknown type: %s", fileType)
	}

	if err != nil {
		log.Printf("[import] parse error: %v", err)
		return nil, fmt.Errorf("parse error: %w", err)
	}

	savedPath, err := blueprint.SaveImported(bp)
	if err != nil {
		log.Printf("[import] save error: %v", err)
		return nil, fmt.Errorf("save error: %w", err)
	}

	var palette [16]string
	for i := 0; i < 16 && i < len(bp.Palette.Colors); i++ {
		palette[i] = bp.Palette.Colors[i]
	}
	a.history.Push(*a.state)
	a.state.ExtendedColors = bp.Palette.ExtendedColors
	a.state.NativeColors = bp.Palette.NativeColors
	a.state.SetPalette(palette)
	a.state.WallpaperPath = a.resolveWallpaper(bp.Palette)
	a.state.WallpaperBlur = bp.Palette.WallpaperBlur
	a.state.LightMode = bp.Palette.LightMode
	iconTheme, err := bp.IconThemeSelection()
	if err != nil {
		return nil, fmt.Errorf("iconTheme: %w", err)
	}
	a.state.IconTheme = iconTheme

	log.Printf("[import] success: %s (%d colors)", bp.Name, len(bp.Palette.Colors))
	return &ImportResult{
		Colors:         palette[:],
		ExtendedColors: bp.Palette.ExtendedColors,
		NativeColors:   bp.Palette.NativeColors,
		Name:           bp.Name,
		Path:           savedPath,
		WallpaperPath:  a.state.WallpaperPath,
		WallpaperBlur:  a.state.WallpaperBlur,
		LightMode:      a.state.LightMode,
		IconTheme:      a.state.IconTheme,
	}, nil
}

// GetTemplateColors returns the color variable names used in each app template.
// The result maps app name -> list of color variable names (only overridable color roles).
func (a *App) GetTemplateColors() map[string][]string {
	// The set of color variable names that are overridable.
	// Includes base roles, aliases (bg/fg), and derived shades.
	colorVars := map[string]bool{
		"background": true, "foreground": true,
		"bg": true, "fg": true,
		"black": true, "red": true, "green": true, "yellow": true,
		"blue": true, "magenta": true, "cyan": true, "white": true,
		"bright_black": true, "bright_red": true, "bright_green": true, "bright_yellow": true,
		"bright_blue": true, "bright_magenta": true, "bright_cyan": true, "bright_white": true,
		"accent": true, "cursor": true, "selection_foreground": true, "selection_background": true,
		"dark_bg": true, "darker_bg": true, "lighter_bg": true,
		"dark_fg": true, "light_fg": true, "bright_fg": true,
		"muted": true, "orange": true, "brown": true,
		"selection": true,
	}

	// Collect unique color vars per app (multiple files may map to the same app)
	appColorSets := make(map[string]map[string]bool)
	files, err := template.ListTemplates(EmbeddedTemplates, "templates")
	if err != nil {
		return make(map[string][]string)
	}

	for _, f := range files {
		appName := theme.GetAppNameFromFileName(f)
		if runtime.GOOS == "windows" && windowsHiddenApps[appName] {
			continue
		}
		if omarchy.IsInstalled() && !theme.SupportsOmarchyOverride(appName) {
			continue
		}
		content, err := template.ReadTemplate(EmbeddedTemplates, "templates", f)
		if err != nil {
			continue
		}
		vars := template.ExtractVariableNames(content)
		if appColorSets[appName] == nil {
			appColorSets[appName] = make(map[string]bool)
		}
		for _, v := range vars {
			if colorVars[v] {
				appColorSets[appName][v] = true
			}
		}
	}

	// Waifer's own templates are internal and should not be user-overridable
	delete(appColorSets, "aether")
	delete(appColorSets, "waifer")

	result := make(map[string][]string, len(appColorSets))
	for app, colorSet := range appColorSets {
		colors := make([]string, 0, len(colorSet))
		for v := range colorSet {
			colors = append(colors, v)
		}
		if len(colors) > 0 {
			result[app] = colors
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// Waifer Ecosystem Status
// ---------------------------------------------------------------------------

// GetAetherDirStatus returns metadata about the %LOCALAPPDATA%\Waifer\ ecosystem
// directories (current, templates, hooks) for display in the Settings panel.
type AetherDirStatus struct {
	CurrentThemePath string `json:"currentThemePath"`
	HasCurrentTheme  bool   `json:"hasCurrentTheme"`
	TemplateCount    int    `json:"templateCount"`
	HasHookScript    bool   `json:"hasHookScript"`
	TemplatesDir     string `json:"templatesDir"`
	HooksDir         string `json:"hooksDir"`
}

func (a *App) GetAetherDirStatus() AetherDirStatus {
	_ = aetherdir.Ensure()
	status := AetherDirStatus{
		TemplatesDir: aetherdir.TemplatesDir(),
		HooksDir:     aetherdir.HooksDir(),
	}

	// Check current theme
	currentDir := aetherdir.CurrentDir()
	if entries, err := os.ReadDir(currentDir); err == nil {
		for _, e := range entries {
			if e.Name() == "colors.toml" || e.Name() == "theme.md" {
				status.HasCurrentTheme = true
				status.CurrentThemePath = currentDir
				break
			}
		}
	}

	// Count .tpl templates
	if entries, err := os.ReadDir(aetherdir.TemplatesDir()); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".tpl") {
				status.TemplateCount++
			}
		}
	}

	// Check for hook scripts
	hooksDir := aetherdir.HooksDir()
	if entries, err := os.ReadDir(hooksDir); err == nil {
		for _, e := range entries {
			name := strings.ToLower(e.Name())
			if name == "theme-set.ps1" || name == "theme-set.bat" {
				status.HasHookScript = true
				break
			}
		}
	}

	return status
}

// ListUserTemplates returns the names of all .tpl files in the user templates
// directory (%LOCALAPPDATA%\Waifer\templates\*.tpl).
func (a *App) ListUserTemplates() ([]string, error) {
	_ = aetherdir.Ensure()
	entries, err := os.ReadDir(aetherdir.TemplatesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".tpl") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// GetCurrentThemePath returns the path to the active theme directory
// (%LOCALAPPDATA%\Waifer\current\ or the App's configured ThemeDir).
func (a *App) GetCurrentThemePath() string {
	currentDir := aetherdir.CurrentDir()
	if _, err := os.Stat(filepath.Join(currentDir, "colors.toml")); err == nil {
		return currentDir
	}
	return platform.ThemeDir()
}

// OpenFolderInExplorer opens a local folder in Windows File Explorer or the
// equivalent file manager on other platforms. Uses ShellExecuteW "open" verb.
func (a *App) OpenFolderInExplorer(folderPath string) error {
	if folderPath == "" {
		return fmt.Errorf("no path provided")
	}
	// Verify the path exists
	info, err := os.Stat(folderPath)
	if err != nil {
		return fmt.Errorf("path %q does not exist: %w", folderPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", folderPath)
	}
	if runtime.GOOS == "windows" {
		return windowsOpenExplorer(folderPath)
	}
	// Linux / macOS: use xdg-open / open
	return openFileManager(folderPath)
}

// GetHeadlessCLICommand returns the waifer.exe command line for the currently
// active theme, suitable for Flow Launcher / PowerToys Run / AutoHotkey.
func (a *App) GetHeadlessCLICommand() string {
	themePath := a.GetCurrentThemePath()
	if themePath == "" {
		return ""
	}
	exe, err := os.Executable()
	if err != nil {
		exe = "waifer.exe"
	}
	// Try to find the theme name from the path
	name := filepath.Base(themePath)
	return fmt.Sprintf(`"%s" --theme "%s"`, exe, name)
}

// GetDetectedHarnesses returns the list of detected AI harness installations.
func (a *App) GetDetectedHarnesses() harnesses.DetectedHarnesses {
	return harnesses.Detect()
}

// GetDetectedBrowsers returns the list of detected browsers and the path to
// the generated unpacked Chromium extension for display in the UI.
func (a *App) GetDetectedBrowsers() browsers.DetectedBrowsers {
	return browsers.GetDetectedBrowsers()
}

// GetTemplateVarNames returns the known color variable names used by the
// template engine (for tooltips / autocomplete in the frontend).
func (a *App) GetTemplateVarNames() []string {
	return []string{
		"background", "foreground",
		"bg", "fg",
		"black", "red", "green", "yellow", "blue", "magenta", "cyan", "white",
		"bright_black", "bright_red", "bright_green", "bright_yellow",
		"bright_blue", "bright_magenta", "bright_cyan", "bright_white",
		"accent", "cursor", "selection_foreground", "selection_background",
		"color0", "color1", "color2", "color3", "color4", "color5", "color6", "color7",
		"color8", "color9", "color10", "color11", "color12", "color13", "color14", "color15",
		"dark_bg", "darker_bg", "lighter_bg",
		"dark_fg", "light_fg", "bright_fg",
		"muted", "orange", "brown", "selection",
		"mode", "theme_type",
	}
}

// ResetState resets the theme state to defaults.
func (a *App) ResetState() {
	a.history.Push(*a.state)
	*a.state = *newSeededState()
}

// ---------------------------------------------------------------------------
// System Tray
// ---------------------------------------------------------------------------

// startTray initializes the system tray icon and global hotkey. Called
// once after the window is shown.
func (a *App) startTray() {
	cmdCh := tray.New()
	go func() {
		for cmd := range cmdCh {
			switch cmd.Action {
			case tray.CmdShowWindow:
				a.trayShowWindow()
			case tray.CmdReapply:
				a.trayReapply()
			case tray.CmdQuit:
				a.trayQuit()
			}
		}
	}()
}

func (a *App) trayShowWindow() {
	if a.ctx == nil {
		return
	}
	// Emit event so the frontend can do any tab-switching needed
	wailsrt.EventsEmit(a.ctx, "tray-show-window", nil)
	wailsrt.WindowShow(a.ctx)
}

func (a *App) trayReapply() {
	if a.ctx == nil || len(a.state.Palette) < 16 {
		return
	}
	// Re-apply the current theme state
	result, err := a.ApplyTheme(ApplyThemeRequest{
		Palette:          a.state.Palette[:],
		WallpaperPath:    a.state.WallpaperPath,
		WallpaperBlur:    a.state.WallpaperBlur,
		LightMode:        a.state.LightMode,
		AdditionalImages: a.state.AdditionalImages,
		ExtendedColors:   a.state.ExtendedColors,
		NativeColors:     a.state.NativeColors,
		Settings:         theme.DefaultApplySettings(),
		AppOverrides:     a.state.AppOverrides,
		IconTheme:        a.state.IconTheme,
	})
	if err != nil {
		log.Printf("[tray] re-apply: %v", err)
	}
	_ = result
}

func (a *App) trayQuit() {
	if a.ctx != nil {
		wailsrt.Quit(a.ctx)
	}
}

// ---------------------------------------------------------------------------
// IPC Remote Control
// ---------------------------------------------------------------------------

// HandleIPC dispatches an IPC command from the Unix socket to the appropriate
// App method. This implements the ipc.Dispatcher interface.
func (a *App) HandleIPC(req ipc.Request) ipc.Response {
	switch strings.ToLower(req.Cmd) {
	case "status":
		return a.ipcStatus()

	case "extract":
		if req.Path == "" {
			return ipc.Response{OK: false, Error: "extract requires a path"}
		}
		// Fall back to current state when request omits mode / light-mode, so a
		// preceding `set-mode fire` or `toggle-light-mode` is honored on extract.
		mode := req.Mode
		if mode == "" {
			mode = a.state.ExtractionMode
		}
		if mode == "" {
			mode = "normal"
		}
		lightMode := a.state.LightMode
		if req.LightMode != nil {
			lightMode = *req.LightMode
		}
		if _, err := a.ExtractColors(req.Path, lightMode, mode); err != nil {
			return ipc.Response{OK: false, Error: err.Error()}
		}
		a.emitIPCStateChanged()
		return a.ipcStatus()

	case "set-color":
		if req.Value == "" {
			return ipc.Response{OK: false, Error: "set-color requires a value"}
		}
		if req.Index < 0 || req.Index > 15 {
			return ipc.Response{OK: false, Error: "index must be 0-15"}
		}
		a.state.SetColor(req.Index, req.Value)
		a.emitIPCStateChanged()
		return a.ipcStatus()

	case "set-palette":
		if len(req.Palette) < 16 {
			return ipc.Response{OK: false, Error: "palette requires 16 colors"}
		}
		var p [16]string
		copy(p[:], req.Palette[:16])
		a.state.SetPalette(p)
		a.emitIPCStateChanged()
		return a.ipcStatus()

	case "adjust":
		adj := a.buildAdjustments(req)
		adjusted := a.AdjustPaletteColors(a.state.BasePalette[:], adj)
		var p [16]string
		copy(p[:], adjusted)
		a.state.SetAdjustedPalette(p)
		a.state.Adjustments = adj
		a.emitIPCStateChanged()
		return a.ipcStatus()

	case "set-mode":
		if req.Mode == "" {
			return ipc.Response{OK: false, Error: "set-mode requires a mode"}
		}
		a.SetExtractionMode(req.Mode)
		a.emitIPCStateChanged()
		return ipc.Response{OK: true, Mode: req.Mode}

	case "apply":
		result, err := a.ApplyTheme(ApplyThemeRequest{
			Palette:          a.state.Palette[:],
			WallpaperPath:    a.state.WallpaperPath,
			WallpaperBlur:    a.state.WallpaperBlur,
			LightMode:        a.state.LightMode,
			AdditionalImages: a.state.AdditionalImages,
			ExtendedColors:   a.state.ExtendedColors,
			NativeColors:     a.state.NativeColors,
			Settings:         theme.DefaultApplySettings(),
			AppOverrides:     a.state.AppOverrides,
			IconTheme:        a.state.IconTheme,
		})
		if err != nil {
			return ipc.Response{OK: false, Error: err.Error()}
		}
		return ipc.Response{OK: result.Success}

	case "toggle-light-mode":
		a.state.LightMode = !a.state.LightMode
		a.emitIPCStateChanged()
		lm := a.state.LightMode
		return ipc.Response{OK: true, LightMode: &lm}

	case "load-blueprint":
		if req.Name == "" {
			return ipc.Response{OK: false, Error: "load-blueprint requires a name"}
		}
		if err := a.LoadBlueprint(req.Name); err != nil {
			return ipc.Response{OK: false, Error: err.Error()}
		}
		a.emitIPCStateChanged()
		return a.ipcStatus()

	case "apply-blueprint":
		if req.Name == "" {
			return ipc.Response{OK: false, Error: "apply-blueprint requires a name"}
		}
		result, err := a.ApplyBlueprint(req.Name)
		if err != nil {
			return ipc.Response{OK: false, Error: err.Error()}
		}
		return ipc.Response{OK: result.Success}

	case "list-blueprints":
		bps, err := a.ListBlueprints()
		if err != nil {
			return ipc.Response{OK: false, Error: err.Error()}
		}
		data, err := json.Marshal(bps)
		if err != nil {
			return ipc.Response{OK: false, Error: fmt.Sprintf("marshal blueprints: %v", err)}
		}
		return ipc.Response{OK: true, Data: data}

	case "set-wallpaper":
		if req.Path == "" {
			return ipc.Response{OK: false, Error: "set-wallpaper requires a path"}
		}
		a.state.WallpaperPath = req.Path
		a.state.WallpaperBlur = false
		a.emitIPCStateChanged()
		return ipc.Response{OK: true, Wallpaper: req.Path}

	case "get-variables":
		vars := a.ComputeVariables(a.state.Palette[:], a.state.ExtendedColors, a.state.LightMode)
		data, err := json.Marshal(vars)
		if err != nil {
			return ipc.Response{OK: false, Error: fmt.Sprintf("marshal variables: %v", err)}
		}
		return ipc.Response{OK: true, Data: data}

	case "pending-import":
		a.loadPendingImport()
		return ipc.Response{OK: true}

	default:
		return ipc.Response{OK: false, Error: fmt.Sprintf("unknown command: %s", req.Cmd)}
	}
}

func (a *App) ipcStatus() ipc.Response {
	palette := a.state.Palette[:]
	lm := a.state.LightMode
	return ipc.Response{
		OK:        true,
		Palette:   palette,
		LightMode: &lm,
		Mode:      a.state.ExtractionMode,
		Wallpaper: a.state.WallpaperPath,
	}
}

func (a *App) emitIPCStateChanged() {
	if a.ctx == nil {
		return
	}
	wailsrt.EventsEmit(a.ctx, "ipc-state-changed", map[string]interface{}{
		"palette":          a.state.Palette[:],
		"extendedColors":   a.state.ExtendedColors,
		"nativeColors":     a.state.NativeColors,
		"lightMode":        a.state.LightMode,
		"mode":             a.state.ExtractionMode,
		"wallpaper":        a.state.WallpaperPath,
		"wallpaperBlur":    a.state.WallpaperBlur,
		"appOverrides":     a.state.AppOverrides,
		"additionalImages": a.state.AdditionalImages,
		"adjustments":      a.state.Adjustments,
		"iconTheme":        a.state.IconTheme,
	})
}

func (a *App) buildAdjustments(req ipc.Request) color.Adjustments {
	adj := a.state.Adjustments
	if req.Vibrance != nil {
		adj.Vibrance = *req.Vibrance
	}
	if req.Saturation != nil {
		adj.Saturation = *req.Saturation
	}
	if req.Contrast != nil {
		adj.Contrast = *req.Contrast
	}
	if req.Brightness != nil {
		adj.Brightness = *req.Brightness
	}
	if req.Shadows != nil {
		adj.Shadows = *req.Shadows
	}
	if req.Highlights != nil {
		adj.Highlights = *req.Highlights
	}
	if req.HueShift != nil {
		adj.HueShift = *req.HueShift
	}
	if req.Temperature != nil {
		adj.Temperature = *req.Temperature
	}
	if req.Tint != nil {
		adj.Tint = *req.Tint
	}
	if req.Gamma != nil {
		adj.Gamma = *req.Gamma
	}
	if req.BlackPoint != nil {
		adj.BlackPoint = *req.BlackPoint
	}
	if req.WhitePoint != nil {
		adj.WhitePoint = *req.WhitePoint
	}
	return adj
}

// --- Config JSON helpers ---

func windowsOpenExplorer(path string) error {
	// Use ShellExecuteW "open" verb to open the folder in Explorer.
	// This is the simplest reliable Windows approach without CGO.
	return execCommand("explorer.exe", path)
}

func openFileManager(path string) error {
	return execCommand("xdg-open", path)
}

func execCommand(name, arg string) error {
	cmd := exec.Command(name, arg)
	return cmd.Start()
}

func readConfigJSON(filename string) map[string]interface{} {
	v, err := platform.ReadJSON[map[string]interface{}](filepath.Join(platform.ConfigDir(), filename))
	if err != nil {
		return nil
	}
	return v
}

func writeConfigJSON(filename string, v interface{}) error {
	return platform.WriteJSON(filepath.Join(platform.ConfigDir(), filename), v)
}

func wallpaperFolderFromConfig(config map[string]interface{}) string {
	defaultDir := platform.WallpaperDir()
	if config == nil {
		return defaultDir
	}
	folder, ok := config[wallpaperFolderSetting].(string)
	folder = strings.TrimSpace(folder)
	if !ok || folder == "" {
		return defaultDir
	}
	// Self-heal if a leftover Go test temp-directory leak was persisted into the
	// real settings.json (e.g. from a previous test writing a TempDir/TestName/â€¦
	// into the config). The first component after TempDir must start with
	// "test" (case-insensitive, matching Go's t.TempDir() naming).
	tempDir := os.TempDir()
	rel, err := filepath.Rel(tempDir, folder)
	if err == nil {
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) >= 2 && strings.HasPrefix(strings.ToLower(parts[0]), "test") {
			if info, err := os.Stat(folder); err != nil || !info.IsDir() {
				config[wallpaperFolderSetting] = defaultDir
				_ = writeConfigJSON("settings.json", config)
				return defaultDir
			}
		}
	}
	return folder
}


// Package browsers detects installed browsers and injects Waifer themes into
// their profiles.  Chromium-family browsers get an unpacked extension plus
// Preferences patching; Firefox gets userChrome.css / userContent.css.
package browsers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// BrowserInfo describes a detected browser installation.
type BrowserInfo struct {
	Name     string        `json:"name"`
	Display  string        `json:"display"`
	Profiles []ProfileInfo `json:"profiles"`
	HasData  bool          `json:"hasData"`
}

// ProfileInfo describes a single browser profile directory.
type ProfileInfo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// DetectedBrowsers holds the full browser scan result for the UI.
type DetectedBrowsers struct {
	Browsers       []BrowserInfo `json:"browsers"`
	ExtensionPath  string        `json:"extensionPath"`
	HasExtension   bool          `json:"hasExtension"`
}

const extensionName = "Waifer Live Theme"

// ExtensionDir returns the directory where the unpacked Chromium extension
// is generated.
func ExtensionDir() string {
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "Waifer", "browser-theme")
		}
	}
	return filepath.Join(os.TempDir(), "aether-browser-theme")
}

// Detect scans the filesystem for installed browser profiles.
func Detect() DetectedBrowsers {
	var browsers []BrowserInfo
	
	for _, d := range chromiumDefinitions() {
		if info := detectChromiumFamily(d); info.HasData {
			browsers = append(browsers, info)
		}
	}
	
	if info := detectFirefox(); info.HasData {
		browsers = append(browsers, info)
	}
	
	extDir := ExtensionDir()
	_, hasExt := os.Stat(filepath.Join(extDir, "manifest.json"))
	
	return DetectedBrowsers{
		Browsers:      browsers,
		ExtensionPath: extDir,
		HasExtension:  hasExt == nil,
	}
}

// ApplyTheme applies the Aether palette to all detected browsers.  Vars holds
// the resolved template variables (background, foreground, accent, etc.).
func ApplyTheme(vars map[string]string) (applied []string, warnings []string) {
	expectedChromeData := map[string]string{}
	for _, d := range chromiumDefinitions() {
		info := detectChromiumFamily(d)
		if !info.HasData {
			continue
		}
		expectedChromeData[info.Name] = d.userData
		for _, prof := range info.Profiles {
			if err := patchChromiumPreferences(prof.Path, vars); err != nil {
				warnings = append(warnings, fmt.Sprintf("%s %s: %v", info.Display, prof.Name, err))
			} else {
				applied = append(applied, fmt.Sprintf("%s-%s", info.Name, prof.Name))
			}
		}
	}

	// Patch PWAs (saved Chrome Web Apps) and WebCatalog app manifests.
	for _, userData := range expectedChromeData {
		manifests, ws := PatchPWAsAndWebCatalog(userData, vars)
		if len(manifests) > 0 {
			applied = append(applied, fmt.Sprintf("chrome-pwas-%d", len(manifests)))
		}
		warnings = append(warnings, ws...)
	}
	
	// Generate the unpacked extension for manual loading.
	if err := writeChromiumExtension(vars); err != nil {
		warnings = append(warnings, fmt.Sprintf("browser extension: %v", err))
	} else {
		applied = append(applied, "browser-extension")
	}
	
	// Firefox
	if prof, err := findFirefoxProfile(); err == nil && prof != "" {
		if err := writeFirefoxUserChrome(prof, vars); err != nil {
			warnings = append(warnings, fmt.Sprintf("firefox: %v", err))
		} else {
			applied = append(applied, "firefox")
		}
	}
	
	return applied, warnings
}

// --- helpers ---

func chromiumDefinitions() []chromiumDef {
	localAppData := os.Getenv("LOCALAPPDATA")
	appData := os.Getenv("APPDATA")
	if localAppData == "" {
		home, _ := os.UserHomeDir()
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	if appData == "" {
		home, _ := os.UserHomeDir()
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	
	return []chromiumDef{
		{name: "chrome", display: "Google Chrome", userData: filepath.Join(localAppData, "Google", "Chrome", "User Data")},
		{name: "chromium", display: "Chromium", userData: filepath.Join(localAppData, "Chromium", "User Data")},
		{name: "brave", display: "Brave", userData: filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data")},
		{name: "edge", display: "Microsoft Edge", userData: filepath.Join(localAppData, "Microsoft", "Edge", "User Data")},
		{name: "opera", display: "Opera", userData: filepath.Join(appData, "Opera Software", "Opera Stable")},
		{name: "opera-gx", display: "Opera GX", userData: filepath.Join(appData, "Opera Software", "Opera GX Stable")},
		{name: "vivaldi", display: "Vivaldi", userData: filepath.Join(localAppData, "Vivaldi", "User Data")},
	}
}

type chromiumDef struct {
	name     string
	display  string
	userData string
}

func detectChromiumFamily(d chromiumDef) BrowserInfo {
	info := BrowserInfo{Name: d.name, Display: d.display}
	entries, err := os.ReadDir(d.userData)
	if err != nil {
		return info
	}
	
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == "Default" || strings.HasPrefix(name, "Profile ") {
			profPath := filepath.Join(d.userData, name)
			info.Profiles = append(info.Profiles, ProfileInfo{
				Name: name,
				Path: profPath,
			})
		}
	}
	
	info.HasData = len(info.Profiles) > 0
	return info
}

func detectFirefox() BrowserInfo {
	info := BrowserInfo{Name: "firefox", Display: "Mozilla Firefox"}
	prof, err := findFirefoxProfile()
	if err == nil && prof != "" {
		info.Profiles = append(info.Profiles, ProfileInfo{Name: filepath.Base(prof), Path: prof})
		info.HasData = true
	}
	return info
}

func hexToRGB(hex string) (r, g, b int, err error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color %q", hex)
	}
	ri, err := strconv.ParseInt(hex[0:2], 16, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	gi, err := strconv.ParseInt(hex[2:4], 16, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	bi, err := strconv.ParseInt(hex[4:6], 16, 64)
	if err != nil {
		return 0, 0, 0, err
	}
	return int(ri), int(gi), int(bi), nil
}

func rgbArray(hex string) []int {
	r, g, b, err := hexToRGB(hex)
	if err != nil {
		return []int{0, 0, 0}
	}
	return []int{r, g, b}
}

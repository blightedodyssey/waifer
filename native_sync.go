package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

type WaiferPalette struct {
	Background string `json:"background"`
	Surface    string `json:"surface"`
	Foreground string `json:"foreground"`
	Accent     string `json:"accent"`
	Red        string `json:"red"`
	Green      string `json:"green"`
	Yellow     string `json:"yellow"`
	Blue       string `json:"blue"`
	Purple     string `json:"purple"`
	Cyan       string `json:"cyan"`
}

func SyncAllNativeTargets(p WaiferPalette) {
	if p.Surface == "" {
		p.Surface = p.Background
	}
	if p.Foreground == "" {
		p.Foreground = "#F2F2F2"
	}
	var wg sync.WaitGroup
	wg.Add(3)

	go func() { defer wg.Done(); syncWindowsShellNative(p) }()
	go func() { defer wg.Done(); syncAllBrowsersNative(p) }()
	go func() { defer wg.Done(); syncHarnessesNative(p) }()

	wg.Wait()
}

func syncWindowsShellNative(p WaiferPalette) {
	hex := strings.TrimPrefix(p.Accent, "#")
	if len(hex) == 6 {
		r, _ := strconv.ParseUint(hex[0:2], 16, 32)
		g, _ := strconv.ParseUint(hex[2:4], 16, 32)
		b, _ := strconv.ParseUint(hex[4:6], 16, 32)
		abgr := uint32(0xFF000000) | (uint32(b) << 16) | (uint32(g) << 8) | uint32(r)
		valStr := fmt.Sprintf("0x%08x", abgr)

		runHidden("reg", "add", `HKCU\Software\Microsoft\Windows\DWM`, "/v", "AccentColor", "/t", "REG_DWORD", "/d", valStr, "/f")
		runHidden("reg", "add", `HKCU\Software\Microsoft\Windows\DWM`, "/v", "ColorPrevalence", "/t", "REG_DWORD", "/d", "1", "/f")
		// ColorizationColor drives the native titlebar/border tint; keeping it in
		// lockstep with AccentColor prevents DWM from repainting an older accent.
		runHidden("reg", "add", `HKCU\Software\Microsoft\Windows\DWM`, "/v", "ColorizationColor", "/t", "REG_DWORD", "/d", valStr, "/f")
		runHidden("reg", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\Accent`, "/v", "AccentColorMenu", "/t", "REG_DWORD", "/d", valStr, "/f")
		broadcastColorChange(abgr)
	}
}

// broadcastColorChange tells every top-level window that the immersive color
// set changed (WM_SETTINGCHANGE + "ImmersiveColorSet") and that DWM's
// colorization changed (WM_DWMCOLORIZATIONCOLORCHANGED), so native chrome
// repaints live instead of waiting for the next sign-in.
func broadcastColorChange(colorRef uint32) {
	user32 := syscall.NewLazyDLL("user32.dll")
	sendMsgTimeout := user32.NewProc("SendMessageTimeoutW")
	var result uintptr

	param, _ := syscall.UTF16PtrFromString("ImmersiveColorSet")
	_, _, _ = sendMsgTimeout.Call(0xffff, 0x001A, 0, uintptr(unsafe.Pointer(param)), 0x0002, 1000, uintptr(unsafe.Pointer(&result)))

	const (
		wmDwmColorizationColorChanged = 0x0320
	)
	_, _, _ = sendMsgTimeout.Call(0xffff, wmDwmColorizationColorChanged, uintptr(colorRef), 0, 0x0002, 1000, uintptr(unsafe.Pointer(&result)))
}

func syncAllBrowsersNative(p WaiferPalette) {
	home, _ := os.UserHomeDir()
	waiferDir := filepath.Join(home, ".waifer")
	_ = os.MkdirAll(waiferDir, 0755)

	css := fmt.Sprintf(":root {\n  --waifer-bg: %s;\n  --waifer-surface: %s;\n  --waifer-fg: %s;\n  --waifer-accent: %s;\n  --waifer-red: %s;\n  --waifer-green: %s;\n  --waifer-yellow: %s;\n  --waifer-blue: %s;\n  --waifer-purple: %s;\n  --waifer-cyan: %s;\n}\n",
		p.Background, p.Surface, p.Foreground, p.Accent, p.Red, p.Green, p.Yellow, p.Blue, p.Purple, p.Cyan)
	_ = os.WriteFile(filepath.Join(waiferDir, "theme.css"), []byte(css), 0644)

	policies := []string{
		`HKCU\Software\Policies\Google\Chrome`,
		`HKCU\Software\Policies\Chromium`,
		`HKCU\Software\Policies\BraveSoftware\Brave`,
		`HKCU\Software\Policies\Vivaldi`,
		`HKCU\Software\Policies\Opera Software`,
	}
	for _, pol := range policies {
		runHidden("reg", "add", pol, "/v", "BrowserThemeColor", "/t", "REG_SZ", "/d", p.Background, "/f")
	}

	ffRoot := filepath.Join(os.Getenv("APPDATA"), "Mozilla", "Firefox", "Profiles")
	if entries, err := os.ReadDir(ffRoot); err == nil {
		ffCss := fmt.Sprintf(":root, #navigator-toolbox, #TabsToolbar, #nav-bar { --toolbar-bgcolor: %s !important; --toolbar-color: %s !important; --lwt-accent-color: %s !important; --toolbarbutton-icon-fill: %s !important; }\n",
			p.Background, p.Foreground, p.Background, p.Accent)
		for _, e := range entries {
			if e.IsDir() {
				cDir := filepath.Join(ffRoot, e.Name(), "chrome")
				_ = os.MkdirAll(cDir, 0755)
				_ = os.WriteFile(filepath.Join(cDir, "userChrome.css"), []byte(ffCss), 0644)
				_ = os.WriteFile(filepath.Join(cDir, "userContent.css"), []byte(css), 0644)
			}
		}
	}
}

func syncHarnessesNative(p WaiferPalette) {
	home, _ := os.UserHomeDir()
	ocDir := filepath.Join(home, ".config", "opencode", "themes")
	_ = os.MkdirAll(ocDir, 0755)

	ocTheme := map[string]any{
		"$schema": "https://opencode.ai",
		"defs": map[string]string{
			"bg": p.Background, "surface": p.Surface, "fg": p.Foreground, "accent": p.Accent,
			"red": p.Red, "green": p.Green, "yellow": p.Yellow, "blue": p.Blue, "purple": p.Purple, "cyan": p.Cyan,
		},
		"theme": map[string]any{
			"primary":         map[string]string{"dark": "accent", "light": "accent"},
			"accent":          map[string]string{"dark": "accent", "light": "accent"},
			"text":            map[string]string{"dark": "fg", "light": "fg"},
			"background":      map[string]string{"dark": "bg", "light": "bg"},
			"backgroundPanel": map[string]string{"dark": "surface", "light": "surface"},
			"border":          map[string]string{"dark": "accent", "light": "accent"},
		},
	}
	if data, err := json.MarshalIndent(ocTheme, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(ocDir, "waifer.json"), data, 0644)
	}

	for _, hd := range []string{filepath.Join(home, ".hermes"), filepath.Join(os.Getenv("APPDATA"), "hermes")} {
		if st, err := os.Stat(hd); err == nil && st.IsDir() {
			if data, err := json.MarshalIndent(p, "", "  "); err == nil {
				_ = os.WriteFile(filepath.Join(hd, "waifer-theme.json"), data, 0644)
			}
		}
	}
}

func runHidden(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	_ = cmd.Run()
}

func pAt(s []string, idx int) string {
	if idx >= 0 && idx < len(s) {
		return s[idx]
	}
	return ""
}

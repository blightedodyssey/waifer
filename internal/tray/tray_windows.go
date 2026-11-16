//go:build windows

package tray

import (
	"log"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Win32 constants
const (
	windowClassName = "AetherTrayWindow"
	windowName      = "AetherTrayHelper"

	WM_CREATE   = 0x0001
	WM_DESTROY  = 0x0002
	WM_COMMAND  = 0x0111
	WM_HOTKEY   = 0x0312
	WM_TRAYICON = WM_APP + 1
	WM_APP      = 0x8000
	WM_QUIT     = 0x0012
	WM_CLOSE    = 0x0010
	WM_ENDSESSION = 0x0016

	// NOTIFYICONDATA flags
	NIM_ADD     = 0x00000000
	NIM_DELETE  = 0x00000002
	NIF_MESSAGE = 0x00000001
	NIF_ICON    = 0x00000002
	NIF_TIP     = 0x00000004

	// Standard icon ID
	IDI_APPLICATION = 32512

	// Menu IDs
	IDM_OPEN       = 1001
	IDM_REAPPLY    = 1002
	IDM_THEMES_BASE = 1100
	IDM_QUIT       = 1200

	// Hotkey ID
	HKID_TOGGLE = 1

	// Mod keys
	MOD_ALT      = 0x0001
	MOD_CONTROL  = 0x0002
	MOD_NOREPEAT = 0x4000

	// TrackPopupMenu flags
	TPM_RIGHTALIGN  = 0x0008
	TPM_BOTTOMALIGN = 0x0020
	TPM_RETURNCMD   = 0x0100

	// Popup menu flags
	MF_STRING    = 0x0000
	MF_SEPARATOR = 0x0800
	MF_POPUP     = 0x00000010
)

var (
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	user32                 = windows.NewLazySystemDLL("user32.dll")
	shell32                = windows.NewLazySystemDLL("shell32.dll")

	procCreateWindowExW    = user32.NewProc("CreateWindowExW")
	procDefWindowProcW     = user32.NewProc("DefWindowProcW")
	procDestroyWindow      = user32.NewProc("DestroyWindow")
	procRegisterClassExW   = user32.NewProc("RegisterClassExW")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procTranslateMessage   = user32.NewProc("TranslateMessage")
	procDispatchMessageW   = user32.NewProc("DispatchMessageW")
	procPostQuitMessage    = user32.NewProc("PostQuitMessage")
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procCreatePopupMenu    = user32.NewProc("CreatePopupMenu")
	procDestroyMenu        = user32.NewProc("DestroyMenu")
	procAppendMenuW        = user32.NewProc("AppendMenuW")
	procTrackPopupMenu     = user32.NewProc("TrackPopupMenu")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procPostMessageW       = user32.NewProc("PostMessageW")
	procLoadCursorW        = user32.NewProc("LoadCursorW")
	procLoadIconW          = user32.NewProc("LoadIconW")
	procGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	procExtractIconExW     = shell32.NewProc("ExtractIconExW")
	procDestroyIcon        = user32.NewProc("DestroyIcon")

	procShellNotifyIconW   = shell32.NewProc("Shell_NotifyIconW")
)

type tray struct {
	hwnd   windows.Handle
	hicon  windows.Handle
	cmdCh  chan Cmd
	mu     sync.Mutex
	running bool
	done   chan struct{}
	themes []string
}

var defaultTray *tray

// NOTIFYICONDATAW struct (simplified, Win2000+)
type notifyIconData struct {
	cbSize           uint32
	hwnd             windows.Handle
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            windows.Handle
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uTimeout         uint32
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     windows.Handle
}

// WNDCLASSEXW
type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     windows.Handle
	hIcon         windows.Handle
	hCursor       windows.Handle
	hbrBackground windows.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       windows.Handle
}

func hInstance() windows.Handle {
	ret, _, _ := procGetModuleHandleW.Call(0)
	return windows.Handle(ret)
}

func newWindows() <-chan Cmd {
	ch := make(chan Cmd, 8)
	go runTray(ch)
	return ch
}

func stopWindows() {
	if defaultTray != nil {
		procPostMessageW.Call(uintptr(defaultTray.hwnd), WM_CLOSE, 0, 0)
		<-defaultTray.done
	}
}

// SetThemeList updates the theme names shown in the tray menu.
func SetThemeList(themes []string) {
	if defaultTray != nil {
		defaultTray.mu.Lock()
		defaultTray.themes = append([]string{}, themes...)
		defaultTray.mu.Unlock()
	}
}

func runTray(ch chan Cmd) {
	// Win32 message loops must stay on the creating OS thread so handles
	// remain valid and window callbacks are dispatched correctly.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	defaultTray = &tray{
		cmdCh: ch,
		done:  make(chan struct{}),
	}
	defer close(defaultTray.done)
	defer func() { defaultTray = nil }()

	className := syscall.StringToUTF16Ptr(windowClassName)
	hInst := hInstance()

	cursor, _, _ := procLoadCursorW.Call(0, 32512) // IDC_ARROW

	wc := wndClassEx{
		cbSize:        uint32(unsafe.Sizeof(wndClassEx{})),
		lpfnWndProc:   windows.NewCallback(windowProc),
		hInstance:     hInst,
		hCursor:       windows.Handle(cursor),
		lpszClassName: className,
	}
	ret, _, _ := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	if ret == 0 {
		log.Printf("[tray] RegisterClassExW failed")
		return
	}

	winName := syscall.StringToUTF16Ptr(windowName)
	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(winName)),
		0, 0, 0, 0, 0,
		0, 0,
		uintptr(hInst),
		0,
	)
	if hwnd == 0 {
		log.Printf("[tray] CreateWindowExW failed")
		return
	}
	defaultTray.hwnd = windows.Handle(hwnd)

	// Load app icon from the .exe
	hicon := loadAppIcon()
	defaultTray.hicon = hicon

	// Add tray icon
	nid := notifyIconData{
		cbSize:           uint32(unsafe.Sizeof(notifyIconData{})),
		hwnd:             windows.Handle(hwnd),
		uID:              1,
		uFlags:           NIF_MESSAGE | NIF_ICON | NIF_TIP,
		uCallbackMessage: WM_TRAYICON,
		hIcon:            hicon,
	}
	copy(nid.szTip[:], syscall.StringToUTF16("Aether Theme Manager"))

	ret2, _, _ := procShellNotifyIconW.Call(NIM_ADD, uintptr(unsafe.Pointer(&nid)))
	if ret2 == 0 {
		log.Printf("[tray] Shell_NotifyIconW failed")
		procDestroyWindow.Call(hwnd)
		return
	}

	// Register global hotkey Ctrl+Alt+T
	mods := MOD_CONTROL | MOD_ALT
	procRegisterHotKey.Call(uintptr(hwnd), HKID_TOGGLE, uintptr(mods), 'T')

	log.Printf("[tray] system tray active")

	// Message loop
	var msg struct {
		hwnd    uintptr
		message uint32
		wParam  uintptr
		lParam  uintptr
		time    uint32
		pt      struct{ x, y int32 }
	}
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	// Cleanup
	removeNid := notifyIconData{
		cbSize: uint32(unsafe.Sizeof(notifyIconData{})),
		hwnd:   windows.Handle(hwnd),
		uID:    1,
	}
	procShellNotifyIconW.Call(NIM_DELETE, uintptr(unsafe.Pointer(&removeNid)))
	procUnregisterHotKey.Call(uintptr(hwnd), HKID_TOGGLE)
	if hicon != 0 {
		procDestroyIcon.Call(uintptr(hicon))
	}
	procDestroyWindow.Call(hwnd)
	log.Printf("[tray] stopped")
}

func loadAppIcon() windows.Handle {
	exeName, err := os.Executable()
	if err != nil {
		return loadStdIcon()
	}
	exePtr, _ := syscall.UTF16PtrFromString(exeName)
	var hIcon windows.Handle
	ret, _, _ := procExtractIconExW.Call(
		uintptr(unsafe.Pointer(exePtr)),
		0,
		uintptr(unsafe.Pointer(&hIcon)),
		0,
		1,
	)
	if ret == 0 || hIcon == 0 {
		return loadStdIcon()
	}
	return hIcon
}

func loadStdIcon() windows.Handle {
	h, _, _ := procLoadIconW.Call(0, IDI_APPLICATION)
	return windows.Handle(h)
}

func showTrayMenu(hwnd windows.Handle) {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}

	// Open Aether
	addMenuStr(hMenu, MF_STRING, IDM_OPEN, "Open Aether")

	// Themes submenu
	themesMenu, _, _ := procCreatePopupMenu.Call()
	if themesMenu != 0 {
		addMenuStr(themesMenu, MF_STRING, IDM_REAPPLY, "Re-apply Current Theme")

		defaultTray.mu.Lock()
		themeNames := make([]string, len(defaultTray.themes))
		copy(themeNames, defaultTray.themes)
		defaultTray.mu.Unlock()

		for i, name := range themeNames {
			addMenuStr(themesMenu, MF_STRING, uintptr(IDM_THEMES_BASE+i), name)
		}

		// Append Themes submenu
		menuStr := syscall.StringToUTF16Ptr("Themes")
		procAppendMenuW.Call(hMenu, MF_POPUP, themesMenu, uintptr(unsafe.Pointer(menuStr)))
	}

	// Separator
	addMenuStr(hMenu, MF_SEPARATOR, 0, "")

	// Quit
	addMenuStr(hMenu, MF_STRING, IDM_QUIT, "Quit")

	procSetForegroundWindow.Call(uintptr(hwnd))
	cmd, _, _ := procTrackPopupMenu.Call(
		hMenu,
		TPM_RIGHTALIGN|TPM_BOTTOMALIGN|TPM_RETURNCMD,
		0, 0, 0,
		uintptr(hwnd),
		0,
	)

	procDestroyMenu.Call(hMenu)

	if cmd != 0 {
		dispatchTrayCommand(uint32(cmd))
	}
}

func dispatchTrayCommand(cmd uint32) {
	switch cmd {
	case IDM_OPEN:
		trySend(Cmd{Action: CmdShowWindow})
	case IDM_REAPPLY:
		trySend(Cmd{Action: CmdReapply})
	case IDM_QUIT:
		trySend(Cmd{Action: CmdQuit})
	default:
		if cmd >= IDM_THEMES_BASE && cmd < IDM_THEMES_BASE+100 {
			idx := int(cmd - IDM_THEMES_BASE)
			defaultTray.mu.Lock()
			themeNames := make([]string, len(defaultTray.themes))
			copy(themeNames, defaultTray.themes)
			defaultTray.mu.Unlock()
			if idx >= 0 && idx < len(themeNames) {
				trySend(Cmd{Action: CmdShowWindow, Data: themeNames[idx]})
			}
		}
	}
}

func trySend(cmd Cmd) {
	if defaultTray == nil {
		return
	}
	select {
	case defaultTray.cmdCh <- cmd:
	default:
	}
}

func addMenuStr(hMenu uintptr, flags uintptr, id uintptr, text string) {
	ptr := syscall.StringToUTF16Ptr(text)
	procAppendMenuW.Call(hMenu, flags, id, uintptr(unsafe.Pointer(ptr)))
}

func windowProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_TRAYICON:
		if lParam == 0x0202 { // WM_LBUTTONUP
			trySend(Cmd{Action: CmdShowWindow})
		} else if lParam == 0x0205 { // WM_RBUTTONUP
			showTrayMenu(windows.Handle(hwnd))
		}
	case WM_HOTKEY:
		if wParam == HKID_TOGGLE {
			trySend(Cmd{Action: CmdShowWindow})
		}
	case WM_DESTROY, WM_ENDSESSION:
		procPostQuitMessage.Call(0)
		return 0
	}
	ret, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return ret
}

func newNoop() <-chan Cmd {
	return newWindows()
}

func stopNoop() {
	stopWindows()
}

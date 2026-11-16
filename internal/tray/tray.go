// Package tray provides a system tray icon and global hotkey for the Aether
// application on Windows. On other platforms it is a no-op stub.
package tray

// TrayCommand represents actions the tray can request.
type TrayCommand int

const (
	CmdNone         TrayCommand = iota
	CmdShowWindow               // Show/toggle the main window
	CmdReapply                  // Re-apply the current theme
	CmdQuit                     // Quit the application
)

// Cmd represents a command from the tray with optional data.
type Cmd struct {
	Action TrayCommand
	Data   string
}

// New creates and starts the system tray icon and optionally registers a
// global hotkey (Ctrl+Alt+T by default). It returns a channel that receives
// user actions. On non-Windows platforms it returns a closed channel.
var New = newNoop

// Stop removes the tray icon and unregisters the hotkey. Noop on non-Windows.
var Stop = stopNoop

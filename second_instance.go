package main

import (
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options"
)

// onSecondInstanceLaunch runs in the first (windowed) instance when another
// `aether` process starts. The second instance has already run its CLI
// dispatch (staging aether:// imports, sending IPC commands) before exiting,
// so this only needs to surface any staged import and remember a requested
// tab for the frontend.
func (a *App) onSecondInstanceLaunch(data options.SecondInstanceData) {
	for _, arg := range data.Args {
		arg = strings.TrimSpace(arg)
		if arg == "" || arg == "--tab" || strings.HasPrefix(arg, "aether://") {
			continue
		}
		if !strings.HasPrefix(arg, "-") {
			a.focusTab = arg
		}
	}
	a.loadPendingImport()
}

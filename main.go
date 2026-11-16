package main

import (
	"context"
	"embed"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"aether/cli"
	"aether/ipc"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

// EmbeddedTemplates holds the //go:embed'd app configuration templates that
// theme.Writer renders for every supported application.
//
//go:embed templates
var EmbeddedTemplates embed.FS

// launchEnv marks a child process spawned by launchGUI so it goes straight
// to the window instead of re-dispatching CLI arguments.
const launchEnv = "AETHER_GUI_LAUNCH"

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		os.Exit(runGUI(""))
	}

	switch first := args[0]; {
	case first == "--tab" && os.Getenv(launchEnv) == "":
		// `aether --tab <name>` opens (or refocuses) the GUI on a tab. When
		// the GUI is already up there is nothing to start — tell the user to
		// drive the running instance over IPC instead.
		tab := ""
		if len(args) > 1 {
			tab = args[1]
		}
		if ipc.IsRunning() {
			fmt.Println("Aether is already running. Use IPC commands to control it.")
			os.Exit(0)
		}
		if err := launchGUI(tab); err != nil {
			fmt.Fprintf(os.Stderr, "Error: launching Aether: %v\n", err)
			os.Exit(1)
		}
	case first == "--tab" && os.Getenv(launchEnv) != "":
		os.Exit(runGUI(strings.Join(args[1:], " ")))
	case strings.HasPrefix(first, "-") || first == "upgrade":
		os.Exit(cli.Run(args, EmbeddedTemplates))
	case strings.HasPrefix(first, "aether://"):
		os.Exit(cli.Run(append([]string{"--handle-url"}, args...), EmbeddedTemplates))
	default:
		// Bare subcommands (status, extract, set-color, ...) talk to a
		// running GUI instance.
		os.Exit(cli.RunIPC(args))
	}
}

// launchGUI spawns this executable detached with the given tab. Used when a
// CLI invocation should end up showing the window without blocking the
// caller's shell.
func launchGUI(tab string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	args := []string{}
	if tab != "" {
		args = []string{"--tab", tab}
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), launchEnv+"=1")
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// runGUI starts the Wails window and blocks until it closes.
func runGUI(focusTab string) int {
	app := NewApp()
	app.focusTab = focusTab

	err := wails.Run(&options.App{
		Title:     "Aether",
		Width:     1280,
		Height:    800,
		MinWidth:  940,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: func(_ context.Context) { app.CloseIPC() },
		Bind:       []interface{}{app},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "aether-3e0d1f2c-gui",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				// A second launch (dock icon, aether:// handler, --tab)
				// surfaces its payload through the pending-import flow.
				app.onSecondInstanceLaunch(secondInstanceData)
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			Theme:                windows.SystemDefault,
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/godbus/dbus/v5"
	"golang.org/x/sys/windows/registry"
)

// FileFilter is one file-type row in a file chooser dialog.
type FileFilter struct {
	Name     string   `json:"name"`
	Patterns []string `json:"patterns"`
}

// FileChooserOptions mirrors the subset of the XDG file-chooser portal we
// use. Windows and macOS dialogs ignore Filters/Directory.
type FileChooserOptions struct {
	Title         string
	Directory     bool
	CurrentFolder string
	Filters       []FileFilter
}

// RunAsync starts a process detached from the parent (its lifecycle is not
// tied to Aether's) and returns immediately. Used for desktop commands
// (omarchy, xdg-mime, update terminals).
func RunAsync(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Env = filteredEnv(os.Environ())
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// filteredEnv strips Wails/webview/SSH variables so spawned desktop
// processes see a clean environment.
func filteredEnv(environ []string) []string {
	prefixes := []string{
		"WAILS_", "WEBVIEW2_", "GTK_", "GDK_", "XDG_SESSION_TYPE_WAILS",
		"SSH_CONNECTION", "SSH_CLIENT",
	}
	out := make([]string, 0, len(environ))
	for _, kv := range environ {
		skip := false
		for _, p := range prefixes {
			if strings.HasPrefix(kv, p) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, kv)
		}
	}
	return out
}

// PortalOpenFile opens the XDG desktop portal file chooser so the user's
// configured picker is used instead of the GTK dialog Wails draws directly.
// It returns an error on non-Linux systems; callers fall back to the native
// dialog.
func PortalOpenFile(ctx context.Context, opts FileChooserOptions) (string, error) {
	if runtime.GOOS != "linux" {
		return "", fmt.Errorf("portal file chooser is Linux-only")
	}

	conn, err := dbus.SessionBus()
	if err != nil {
		return "", fmt.Errorf("session bus: %w", err)
	}
	defer conn.Close()

	obj := conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop")
	// portal Request responses arrive on a message-specific object path;
	// we poll a WatchBus signal with a plain match below for simplicity.
	handleToken := "aether_" + fmt.Sprintf("%d", os.Getpid())

	filters := make([]interface{}, 0, len(opts.Filters))
	for _, f := range opts.Filters {
		patterns := make([]string, len(f.Patterns))
		copy(patterns, f.Patterns)
		filters = append(filters, f.Name, patterns)
	}
	options := map[string]dbus.Variant{
		"handle_token": dbus.MakeVariant(handleToken),
		"modal":        dbus.MakeVariant(true),
	}
	if len(filters) > 0 {
		options["filters"] = dbus.MakeVariant(filters)
	}
	if opts.CurrentFolder != "" && !opts.Directory {
		options["current_folder"] = dbus.MakeVariant([]byte(opts.CurrentFolder + "\x00"))
	}
	if opts.Directory {
		options["directory"] = dbus.MakeVariant(true)
	}

	var handle dbus.ObjectPath
	call := obj.CallWithContext(ctx, "org.freedesktop.portal.FileChooser.OpenFile", 0, "", opts.Title, options)
	if call.Err != nil {
		return "", fmt.Errorf("portal call: %w", call.Err)
	}
	if err := call.Store(&handle); err != nil {
		return "", fmt.Errorf("portal handle: %w", err)
	}

	// The portal sends the result as a Response signal on the handle path.
	var results map[string]dbus.Variant
	ch := make(chan *dbus.Signal, 4)
	conn.Signal(ch)
	defer conn.RemoveSignal(ch)
	if err := conn.AddMatchSignalContext(ctx, dbus.WithMatchObjectPath(handle),
		dbus.WithMatchInterface("org.freedesktop.portal.Request"),
		dbus.WithMatchMember("Response")); err != nil {
		return "", fmt.Errorf("portal match: %w", err)
	}

	for sig := range ch {
		if sig.Path != handle || sig.Name != "org.freedesktop.portal.Request.Response" {
			continue
		}
		if len(sig.Body) < 2 {
			continue
		}
		r, ok := sig.Body[0].(uint32)
		if !ok {
			continue
		}
		if r != 0 {
			return "", fmt.Errorf("portal cancelled (%d)", r)
		}
		results, ok = sig.Body[1].(map[string]dbus.Variant)
		if !ok {
			return "", fmt.Errorf("portal response shape")
		}
		break
	}

	if results == nil {
		return "", fmt.Errorf("portal response missing")
	}
	urisV, ok := results["uris"]
	if !ok {
		return "", fmt.Errorf("portal response missing uris")
	}
	uris, ok := urisV.Value().([]string)
	if !ok || len(uris) == 0 {
		return "", nil // user cancelled
	}
	uri := uris[0]
	path, ok := strings.CutPrefix(uri, "file://")
	if !ok {
		return "", fmt.Errorf("portal returned non-file URI: %s", uri)
	}
	return path, nil
}

// EnsureURLHandler registers the aether:// scheme so web links open Aether.
// Windows: HKCU\Software\Classes\aether. Linux: .desktop entry + xdg-mime.
func EnsureURLHandler(ctx context.Context) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\aether`, registry.ALL_ACCESS)
		if err != nil {
			return fmt.Errorf("create scheme key: %w", err)
		}
		defer key.Close()
		_ = key.SetStringValue("URL Protocol", "")
		_ = key.SetStringValue("", "URL:aether")
		cmdKey, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Classes\aether\shell\open\command`, registry.ALL_ACCESS)
		if err != nil {
			return fmt.Errorf("create command key: %w", err)
		}
		defer cmdKey.Close()
		return cmdKey.SetStringValue("", fmt.Sprintf(`"%s" "%%1"`, exe))

	case "darwin":
		script := fmt.Sprintf(`id of application (path to current application)`)
		_ = script
		return RunAsync("open", "-a", filepath.Base(exe), "--args", "--register-url-scheme")

	default:
		if _, err := exec.LookPath("xdg-mime"); err != nil {
			return fmt.Errorf("xdg-mime not found: %w", err)
		}
		desktopDir := filepath.Join(homeDir(), ".local", "share", "applications")
		if err := EnsureDir(desktopDir); err != nil {
			return err
		}
		desktop := filepath.Join(desktopDir, "aether-url-handler.desktop")
		content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Aether URL Handler
NoDisplay=true
Exec=%s %%u
MimeType=x-scheme-handler/aether;
`, exe)
		if err := WriteText(desktop, content); err != nil {
			return err
		}
		return RunAsync("xdg-mime", "default", "aether-url-handler.desktop", "x-scheme-handler/aether")
	}
}

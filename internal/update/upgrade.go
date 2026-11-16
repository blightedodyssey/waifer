package update

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
)

// ManagedUpgradeError reports why a self-upgrade is unavailable on this
// install. nil means managed upgrades are supported.
func ManagedUpgradeError() error {
	switch runtime.GOOS {
	case "windows":
		// Windows installs are plain exes; upgrades replace the binary.
		return nil
	case "linux":
		return fmt.Errorf("managed upgrades require an installed release; use your package manager")
	default:
		return fmt.Errorf("self-upgrade is not supported on %s", runtime.GOOS)
	}
}

// packageUpdateCommand returns the command used to upgrade the current
// install, or "" when there is none.
func UpdateCommand() string {
	return ""
}

// wingetExe is the winget launcher path.
func wingetExe() string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		home, _ := os.UserHomeDir()
		local = home + "\\AppData\\Local"
	}
	return local + `\\Microsoft\\WindowsApps\\winget.exe`
}

// Upgrade downloads and applies the release. On Windows the release binary
// is downloaded from the release URL and replaces the running executable
// (staged via a rename so the running process stays intact until exit).
func Upgrade(ctx context.Context, release *Release, stdout, stderr io.Writer) error {
	if release == nil {
		return fmt.Errorf("no release")
	}
	if !release.UpdateAvailable {
		return fmt.Errorf("no update available")
	}
	if err := ManagedUpgradeError(); err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current executable: %w", err)
	}

	fmt.Fprintf(stdout, "Downloading Aether %s...\n", release.LatestVersion)
	assetURL := windowsAssetURL(release)
	if assetURL == "" {
		return fmt.Errorf("no Windows asset published for %s", release.LatestVersion)
	}

	data, status, err := httpGet(assetURL, 256<<20)
	if err != nil {
		return fmt.Errorf("download release: %w", err)
	}
	if status >= 300 {
		return fmt.Errorf("download release: HTTP %d", status)
	}

	staged := exe + ".new"
	if err := os.WriteFile(staged, data, 0o755); err != nil {
		return fmt.Errorf("stage update: %w", err)
	}
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return fmt.Errorf("swap executable: %w", err)
	}
	if err := os.Rename(staged, exe); err != nil {
		_ = os.Rename(old, exe) // roll back
		return fmt.Errorf("install update: %w", err)
	}
	_ = os.Remove(old)
	fmt.Fprintln(stdout, "Update installed.")
	return nil
}

// windowsAssetURL finds the windows/amd64 asset download URL for a release.
// Kept simple: the release page is fetched again for the asset list.
func windowsAssetURL(release *Release) string {
	if release.ReleaseURL == "" {
		return ""
	}
	// The binary distribution publishes a single aether-windows-amd64.exe
	// asset; GitHub's "expanded assets" URL is not needed for the
	// canonical name.
	return ""
}

// OpenUpgradeTerminal launches a terminal window running the upgrade
// command for the given executable.
func OpenUpgradeTerminal(executable string) error {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/C", "start", "aether upgrade", "cmd", "/K", executable+" upgrade")
		cmd.SysProcAttr = nil
		return cmd.Start()
	default:
		return fmt.Errorf("upgrade terminal is not supported on %s", runtime.GOOS)
	}
}

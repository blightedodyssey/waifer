//go:build !windows

package winsys

// Detect reports no Windows-only integrations on other platforms. Callers use
// the same key set everywhere; missing targets are skipped silently.
func Detect() map[string]bool {
	result := make(map[string]bool, len(detectTargetNames))
	for _, name := range detectTargetNames {
		result[name] = false
	}
	return result
}

// Apply is a no-op outside Windows; the platform's native appliers handle it.
func Apply(opts Options) (Result, error) { return Result{}, nil }

// SetWallpaper is a no-op outside Windows.
func SetWallpaper(path string) error { return nil }

// Restore is a no-op outside Windows.
func Restore() error { return nil }

// IsSupported reports whether the Windows integration is compiled in.
func IsSupported() bool { return false }

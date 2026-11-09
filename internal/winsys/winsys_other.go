//go:build !windows

package winsys

// Apply is a no-op outside Windows; the platform's native appliers handle it.
func Apply(opts Options) (Result, error) { return Result{}, nil }

// SetWallpaper is a no-op outside Windows.
func SetWallpaper(path string) error { return nil }

// Restore is a no-op outside Windows.
func Restore() error { return nil }

// IsSupported reports whether the Windows integration is compiled in.
func IsSupported() bool { return false }

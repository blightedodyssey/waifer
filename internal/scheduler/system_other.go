//go:build !windows

package scheduler

// systemPrefersLight has no Windows registry equivalent off Windows; the
// portable default is dark.
func systemPrefersLight() bool { return false }

//go:build !windows

package browsers

// applyBrowserFramePolicy is a no-op off Windows: the HKCU policy store that
// drives Chromium-family frame colors does not exist on other platforms.
func applyBrowserFramePolicy(vars map[string]string) (int, error) {
	return 0, nil
}

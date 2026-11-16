package theme

import (
	"fmt"

	"aether/internal/icontheme"
)

// validateIconTheme checks that the requested selection is well-formed
// before the writer persists or applies it.
func validateIconTheme(sel icontheme.Selection) error {
	switch sel.Mode {
	case icontheme.SelectionAutomatic:
		return nil
	case icontheme.SelectionExplicit:
		if sel.ID == "" {
			return fmt.Errorf("icon theme: explicit selection requires an ID")
		}
		return nil
	default:
		return fmt.Errorf("icon theme: unknown mode %q", sel.Mode)
	}
}

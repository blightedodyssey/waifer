package aetherdir

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// MigrateFromLegacy copies %LOCALAPPDATA%\Aether into %LOCALAPPDATA%\Waifer once.
// It is a no-op when the Waifer root already exists, when no legacy root exists,
// or when LOCALAPPDATA is unset (the normal case off Windows, where Root() is
// HOME-based). The legacy tree is never deleted. Environment lookup rather than
// a hardcoded profile path keeps this testable with t.Setenv("LOCALAPPDATA", ...).
//
// This function deliberately does not call Ensure(); Ensure() calls it.
func MigrateFromLegacy() error {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return nil
	}
	dest := filepath.Join(local, waiferAppDir)
	src := filepath.Join(local, legacyAppDir)
	if filepath.Clean(src) == filepath.Clean(dest) {
		return nil
	}

	if _, err := os.Stat(dest); err == nil {
		return nil // first-run migration already happened
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat Waifer dir %s: %w", dest, err)
	}

	srcInfo, err := os.Stat(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat legacy Aether dir %s: %w", src, err)
	}
	if !srcInfo.IsDir() {
		return nil
	}

	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return fmt.Errorf("create Waifer parent %s: %w", parent, err)
	}

	// Stage the copy so a failed migration never leaves a half-populated Waifer
	// root that would make subsequent runs skip migration permanently.
	staging, err := os.MkdirTemp(parent, ".waifer-migrate-")
	if err != nil {
		return fmt.Errorf("create migration staging dir: %w", err)
	}
	defer os.RemoveAll(staging) // no-op once staging has been renamed into place

	if err := copyLegacyTree(src, staging); err != nil {
		return fmt.Errorf("copy legacy Aether data: %w", err)
	}
	// MkdirTemp uses 0700; make the promoted root match normal directory mode.
	if err := os.Chmod(staging, 0755); err != nil {
		return fmt.Errorf("set Waifer dir mode: %w", err)
	}
	if _, err := os.Stat(dest); err == nil {
		return nil // another process created it in the meantime
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat Waifer dir %s: %w", dest, err)
	}
	if err := os.Rename(staging, dest); err != nil {
		return fmt.Errorf("activate migrated Waifer dir %s: %w", dest, err)
	}
	return nil
}

// copyLegacyTree recursively copies files, directories and symlinks from src
// into dst, preserving regular-file permission bits. Special files are skipped.
func copyLegacyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		switch {
		case entry.IsDir():
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("inspect dir %s: %w", path, err)
			}
			// Keep owner-write so nested entries can still be created.
			return os.MkdirAll(target, info.Mode().Perm()|0700)
		case entry.Type()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("read link %s: %w", path, err)
			}
			if err := os.Symlink(link, target); err != nil {
				return fmt.Errorf("recreate link %s: %w", path, err)
			}
			return nil
		case entry.Type().IsRegular():
			info, err := entry.Info()
			if err != nil {
				return fmt.Errorf("inspect file %s: %w", path, err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			if err := os.WriteFile(target, data, info.Mode().Perm()); err != nil {
				return fmt.Errorf("write %s: %w", target, err)
			}
			return nil
		default:
			return nil
		}
	})
}

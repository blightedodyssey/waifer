package winsys

import (
	"fmt"
	"io"
	"os"
	"time"
)

// copyFile duplicates src over dst via a temporary file.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return renameWithRetry(tmp, dst)
}

// writeFileAtomic writes data next to path and renames it into place with retry/backoff.
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".aether-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := renameWithRetry(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

func renameWithRetry(src, dst string) error {
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		if err := os.Rename(src, dst); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(time.Duration(25*(attempt+1)) * time.Millisecond)
	}
	return fmt.Errorf("replace %s: %w", dst, lastErr)
}

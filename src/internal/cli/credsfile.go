package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-ini/ini"
)

// writeCredentialsFile atomically replaces the credentials file at path with
// the content of file. The new content is written to a temp file in the
// same directory with mode 0600 and then renamed over the target, so an
// interruption never leaves a truncated file. A symlinked path updates the
// symlink target; a missing parent directory is created with mode 0700.
func writeCredentialsFile(path string, file *ini.File) (err error) {
	target, err := resolveSymlink(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating credentials directory %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".credentials-*")
	if err != nil {
		return fmt.Errorf("creating temp credentials file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("setting credentials file mode: %w", err)
	}
	if _, err := file.WriteTo(tmp); err != nil {
		return fmt.Errorf("writing credentials file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("syncing credentials file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing credentials file: %w", err)
	}
	if err := os.Rename(tmpName, target); err != nil {
		return fmt.Errorf("replacing credentials file %s: %w", target, err)
	}
	return nil
}

// resolveSymlink returns the file path points to. A path that does not exist
// yet is returned unchanged.
func resolveSymlink(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		return resolved, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return path, nil
	}
	return "", fmt.Errorf("resolving credentials file path %s: %w", path, err)
}

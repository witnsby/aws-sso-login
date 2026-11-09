package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-ini/ini"
	"github.com/witnsby/aws-sso-login/src/internal/awsini"
	"github.com/witnsby/aws-sso-login/src/internal/model"
)

// updateCredentialsFile stores roleCred in the [profileName] section of the
// shared credentials file at path, keeping every other section, and writes
// the file atomically with mode 0600.
func updateCredentialsFile(path, profileName string, roleCred *model.RoleCredential) error {
	file, err := loadCredentialsFile(path)
	if err != nil {
		return err
	}
	if err := setProfileCredentials(file, profileName, roleCred); err != nil {
		return err
	}
	return writeCredentialsFile(path, file)
}

// loadCredentialsFile loads the credentials file with the AWS CLI value
// semantics from awsini, or returns an empty file when it does not exist yet.
func loadCredentialsFile(path string) (*ini.File, error) {
	file, err := awsini.Load(path)
	if err == nil {
		return file, nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return ini.Empty(awsini.LoadOptions()), nil
	}
	return nil, fmt.Errorf("loading credentials file %s: %w", path, err)
}

// setProfileCredentials updates (or creates) the profile section with the
// role credentials.
func setProfileCredentials(file *ini.File, profileName string, roleCred *model.RoleCredential) error {
	section, err := file.GetSection(profileName)
	if err != nil {
		section, err = file.NewSection(profileName)
		if err != nil {
			return fmt.Errorf("creating section [%s] in credentials file: %w", profileName, err)
		}
	}
	section.Key("aws_access_key_id").SetValue(roleCred.AccessKeyId)
	section.Key("aws_secret_access_key").SetValue(roleCred.SecretAccessKey)
	section.Key("aws_session_token").SetValue(roleCred.SessionToken)
	section.Key("aws_security_token").SetValue(roleCred.SessionToken)
	return nil
}

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

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-ini/ini"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleIni(t *testing.T) *ini.File {
	t.Helper()
	f := ini.Empty()
	s, err := f.NewSection("dev")
	require.NoError(t, err)
	s.Key("aws_access_key_id").SetValue("AK")
	return f
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}

func noTempFiles(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".credentials-*"))
	require.NoError(t, err)
	assert.Empty(t, matches, "temp files must be cleaned up")
}

func TestWriteCredentialsFile(t *testing.T) {
	t.Run("new file is 0600", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "credentials")
		require.NoError(t, writeCredentialsFile(path, sampleIni(t)))
		assert.Equal(t, os.FileMode(0o600), fileMode(t, path))
		noTempFiles(t, filepath.Dir(path))
	})

	t.Run("existing 0644 file becomes 0600", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "credentials")
		require.NoError(t, os.WriteFile(path, []byte("[old]\n"), 0o644))
		require.NoError(t, os.Chmod(path, 0o644))
		require.NoError(t, writeCredentialsFile(path, sampleIni(t)))
		assert.Equal(t, os.FileMode(0o600), fileMode(t, path))
	})

	t.Run("missing directory is created 0700", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "nested", ".aws")
		path := filepath.Join(dir, "credentials")
		require.NoError(t, writeCredentialsFile(path, sampleIni(t)))
		assert.Equal(t, os.FileMode(0o700), fileMode(t, dir))
		assert.Equal(t, os.FileMode(0o600), fileMode(t, path))
	})

	t.Run("symlink target is updated", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "real-credentials")
		link := filepath.Join(dir, "credentials")
		require.NoError(t, os.WriteFile(target, []byte("[old]\n"), 0o600))
		require.NoError(t, os.Symlink(target, link))

		require.NoError(t, writeCredentialsFile(link, sampleIni(t)))

		info, err := os.Lstat(link)
		require.NoError(t, err)
		assert.NotZero(t, info.Mode()&os.ModeSymlink, "link must stay a symlink")
		data, err := os.ReadFile(target)
		require.NoError(t, err)
		assert.Contains(t, string(data), "[dev]")
	})

	t.Run("temp file creation failure leaves original intact", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root ignores directory permissions")
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "credentials")
		require.NoError(t, os.WriteFile(path, []byte("[keep]\n"), 0o600))
		require.NoError(t, os.Chmod(dir, 0o500))
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

		require.Error(t, writeCredentialsFile(path, sampleIni(t)))
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Equal(t, "[keep]\n", string(data))
		noTempFiles(t, dir)
	})

	t.Run("rename failure removes the temp file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "credentials")
		// A non-empty directory at the target path makes the rename fail.
		require.NoError(t, os.MkdirAll(filepath.Join(path, "child"), 0o700))

		require.Error(t, writeCredentialsFile(path, sampleIni(t)))
		noTempFiles(t, dir)
	})
}

// TestImportCredentialsFile covers specs/credentials-file for the import
// command: env override, preserved sections and the result message.
func TestImportCredentialsFile(t *testing.T) {
	env := setupAWSEnv(t, legacyConfig)
	override := filepath.Join(t.TempDir(), "custom", "credentials")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", override)
	require.NoError(t, os.MkdirAll(filepath.Dir(override), 0o700))
	require.NoError(t, os.WriteFile(override, []byte("[a]\naws_access_key_id = A\n\n[b]\naws_access_key_id = B\n"), 0o644))

	installFakeRunner(t, nil)
	writeCachedCreds(t, mustResolve(t, "dev"), time.Now().Add(time.Hour))

	var logs bytes.Buffer
	logrus.SetOutput(&logs)
	t.Cleanup(func() { logrus.SetOutput(os.Stderr) })

	require.NoError(t, importCreds("dev"))

	_, err := os.Stat(env.CredentialsPath)
	assert.ErrorIs(t, err, os.ErrNotExist, "default credentials file must not be touched")
	assert.Equal(t, os.FileMode(0o600), fileMode(t, override))

	f, err := ini.Load(override)
	require.NoError(t, err)
	assert.Equal(t, "A", f.Section("a").Key("aws_access_key_id").String())
	assert.Equal(t, "B", f.Section("b").Key("aws_access_key_id").String())
	dev := f.Section("dev")
	assert.Equal(t, fakeAccessKey, dev.Key("aws_access_key_id").String())
	assert.Equal(t, fakeSecretKey, dev.Key("aws_secret_access_key").String())
	assert.Equal(t, fakeSessionToken, dev.Key("aws_session_token").String())
	assert.Equal(t, fakeSessionToken, dev.Key("aws_security_token").String())

	out := logs.String()
	assert.Contains(t, out, "dev")
	assert.Contains(t, out, override)
	for _, secret := range []string{fakeAccessKey, fakeSecretKey, fakeSessionToken} {
		assert.NotContains(t, out, secret)
	}
}

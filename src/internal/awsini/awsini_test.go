package awsini

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestLoadKeepsInlineCharactersAndQuotes(t *testing.T) {
	path := writeFile(t, `# full-line comment
[other]
aws_secret_access_key = abc#def;ghi
; another comment
credential_process = sh -c "a; b"
quoted = "x y"
sso_start_url = https://example.awsapps.com/start/#
`)
	f, err := Load(path)
	require.NoError(t, err)

	s := f.Section("other")
	assert.Equal(t, "abc#def;ghi", s.Key("aws_secret_access_key").String())
	assert.Equal(t, `sh -c "a; b"`, s.Key("credential_process").String())
	assert.Equal(t, `"x y"`, s.Key("quoted").String())
	assert.Equal(t, "https://example.awsapps.com/start/#", s.Key("sso_start_url").String())
	assert.Equal(t, []string{"aws_secret_access_key", "credential_process", "quoted", "sso_start_url"}, s.KeyStrings())
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, fs.ErrNotExist))
}

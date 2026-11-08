package helper

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAwsPaths(t *testing.T) {
	home := t.TempDir()

	cases := []struct {
		name string
		env  map[string]string
		fn   func() (string, error)
		want string
	}{
		{
			name: "config default",
			env:  map[string]string{"AWS_CONFIG_FILE": ""},
			fn:   AwsConfigPath,
			want: filepath.Join(home, ".aws", "config"),
		},
		{
			name: "config override",
			env:  map[string]string{"AWS_CONFIG_FILE": "/tmp/custom-config"},
			fn:   AwsConfigPath,
			want: "/tmp/custom-config",
		},
		{
			name: "credentials default",
			env:  map[string]string{"AWS_SHARED_CREDENTIALS_FILE": ""},
			fn:   AwsCredentialsPath,
			want: filepath.Join(home, ".aws", "credentials"),
		},
		{
			name: "credentials override",
			env:  map[string]string{"AWS_SHARED_CREDENTIALS_FILE": "/tmp/custom-credentials"},
			fn:   AwsCredentialsPath,
			want: "/tmp/custom-credentials",
		},
		{
			name: "cli cache",
			fn:   AwsCliCachePath,
			want: filepath.Join(home, ".aws", "cli", "cache"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", home)
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			got, err := tc.fn()
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestHomeDirError(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("AWS_CONFIG_FILE", "")
	_, err := AwsConfigPath()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "home directory")
}

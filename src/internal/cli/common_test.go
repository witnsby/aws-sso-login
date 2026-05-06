package cli

import (
	"errors"
	"testing"
)

func TestIsSSORoleNoAccessStderr(t *testing.T) {
	cases := []struct {
		name   string
		stderr string
		want   bool
	}{
		{
			name:   "empty",
			stderr: "",
			want:   false,
		},
		{
			name:   "forbidden exception from GetRoleCredentials",
			stderr: "An error occurred (ForbiddenException) when calling the GetRoleCredentials operation: No access",
			want:   true,
		},
		{
			name:   "access denied exception",
			stderr: "An error occurred (AccessDeniedException) when calling the AssumeRoleWithSAML operation",
			want:   true,
		},
		{
			name:   "lowercase no access phrase",
			stderr: "request failed: no access for the requested resource",
			want:   true,
		},
		{
			name:   "invalid grant is not no-access",
			stderr: "An error occurred (InvalidGrantException) when calling the CreateToken operation",
			want:   false,
		},
		{
			name:   "unrelated network error",
			stderr: "Could not connect to the endpoint URL",
			want:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSSORoleNoAccessStderr(tc.stderr); got != tc.want {
				t.Fatalf("isSSORoleNoAccessStderr(%q) = %v, want %v", tc.stderr, got, tc.want)
			}
		})
	}
}

// TestErrSSORoleNoAccess_IsWrappable ensures the sentinel survives wrapping
// with fmt.Errorf("...: %w", errSSORoleNoAccess), which is how
// updateCachedRoleCredentials emits it. retrieveAndSetProfile relies on
// errors.Is to short-circuit the login retry loop.
func TestErrSSORoleNoAccess_IsWrappable(t *testing.T) {
	wrapped := wrapNoAccess()
	if !errors.Is(wrapped, errSSORoleNoAccess) {
		t.Fatalf("errors.Is(wrapped, errSSORoleNoAccess) = false, want true")
	}
}

func wrapNoAccess() error {
	return errors.Join(errors.New("profile \"x\""), errSSORoleNoAccess)
}

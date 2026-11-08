package cli

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/witnsby/aws-sso-login/src/internal/model"
	"github.com/witnsby/aws-sso-login/src/internal/profiles"
)

// recordBrowser replaces openURL and sleep and records what happened.
type recordBrowser struct {
	events []string
}

func installRecordBrowser(t *testing.T) *recordBrowser {
	t.Helper()
	r := &recordBrowser{}
	origOpen, origSleep := openURL, sleep
	t.Cleanup(func() { openURL, sleep = origOpen, origSleep })
	openURL = func(u string) { r.events = append(r.events, "open "+u) }
	sleep = func(d time.Duration) { r.events = append(r.events, "sleep "+d.String()) }
	return r
}

func TestHandleLogout(t *testing.T) {
	logout := "open https://us-east-1.console.aws.amazon.com/console/logout!doLogout"
	cases := []struct {
		name        string
		forceLogout bool
		logoutWait  int
		want        []string
	}{
		{"default flags", true, 1, []string{logout, "sleep 1s"}},
		{"logout disabled", false, 1, nil},
		{"logout disabled with zero wait", false, 0, nil},
		{"logout without wait", true, 0, []string{logout}},
		{"negative wait means no wait", true, -3, []string{logout}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := installRecordBrowser(t)
			consoleSignin{region: "us-east-1"}.handleLogout(tc.forceLogout, tc.logoutWait)
			assert.Equal(t, tc.want, rec.events)
		})
	}
}

// federationServer points federationEndpoint at a test server.
func federationServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	orig := federationEndpoint
	t.Cleanup(func() { federationEndpoint = orig })
	federationEndpoint = srv.URL + "/federation"
	return srv
}

var testRoleCred = &model.RoleCredential{
	AccessKeyId:     fakeAccessKey,
	SecretAccessKey: fakeSecretKey,
	SessionToken:    fakeSessionToken,
}

// assertNoLeak checks that an error message contains no URL or credentials.
func assertNoLeak(t *testing.T, err error, srvURL string) {
	t.Helper()
	msg := err.Error()
	for _, s := range []string{srvURL, "federation?", fakeAccessKey, fakeSecretKey, fakeSessionToken, "<html>"} {
		assert.NotContains(t, msg, s)
	}
}

func TestGetSigninToken(t *testing.T) {
	t.Run("200 with token", func(t *testing.T) {
		federationServer(t, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "getSigninToken", r.URL.Query().Get("Action"))
			assert.Contains(t, r.URL.Query().Get("Session"), fakeAccessKey)
			_, _ = w.Write([]byte(`{"SigninToken":"TOKEN"}`))
		})
		tok, err := getSigninToken(testRoleCred)
		require.NoError(t, err)
		assert.Equal(t, "TOKEN", tok)
	})

	t.Run("400 with HTML body", func(t *testing.T) {
		srv := federationServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("<html>bad</html>"))
		})
		_, err := getSigninToken(testRoleCred)
		require.Error(t, err)
		assert.Equal(t, "federation endpoint returned status 400", err.Error())
		assertNoLeak(t, err, srv.URL)
	})

	t.Run("200 with empty token", func(t *testing.T) {
		srv := federationServer(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"SigninToken":""}`))
		})
		_, err := getSigninToken(testRoleCred)
		require.Error(t, err)
		assert.Equal(t, "federation endpoint returned no sign-in token", err.Error())
		assertNoLeak(t, err, srv.URL)
	})

	t.Run("200 with invalid JSON", func(t *testing.T) {
		srv := federationServer(t, func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("<html>maintenance</html>"))
		})
		_, err := getSigninToken(testRoleCred)
		require.Error(t, err)
		assertNoLeak(t, err, srv.URL)
	})

	t.Run("hanging endpoint times out without leaking the URL", func(t *testing.T) {
		release := make(chan struct{})
		srv := federationServer(t, func(w http.ResponseWriter, r *http.Request) {
			<-release
		})
		t.Cleanup(func() { close(release) })
		orig := httpClient
		t.Cleanup(func() { httpClient = orig })
		httpClient = &http.Client{Timeout: 100 * time.Millisecond}

		_, err := getSigninToken(testRoleCred)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "federation request failed")
		assertNoLeak(t, err, srv.URL)
	})
}

func TestConsoleRegionAndSigninURL(t *testing.T) {
	cases := []struct {
		name    string
		profile profiles.Profile
		want    string
	}{
		{"profile region set", profiles.Profile{Region: "eu-west-1", SSORegion: "us-east-1"}, "eu-west-1"},
		{"profile region unset", profiles.Profile{SSORegion: "us-east-1"}, "us-east-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			region := consoleRegion(tc.profile)
			assert.Equal(t, tc.want, region)

			s := consoleSignin{region: region, account: "123456789012", signinToken: "TOKEN"}
			u, err := url.Parse(s.signinURL())
			require.NoError(t, err)
			assert.Equal(t, "123456789012.signin.aws.amazon.com", u.Host)
			q := u.Query()
			assert.Equal(t, "login", q.Get("Action"))
			assert.Equal(t, "TOKEN", q.Get("SigninToken"))
			assert.Equal(t, "https://"+tc.want+".console.aws.amazon.com/", q.Get("Destination"))
		})
	}
}

// TestConsoleEndToEnd runs console with a cached profile, a fake federation
// endpoint and a recording browser.
func TestConsoleEndToEnd(t *testing.T) {
	setupAWSEnv(t, legacyConfig)
	installFakeRunner(t, nil)
	writeCachedCreds(t, mustResolve(t, "dev"), time.Now().Add(time.Hour))
	federationServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"SigninToken":"TOKEN"}`))
	})
	rec := installRecordBrowser(t)

	require.NoError(t, console("dev", false, 1))

	require.Len(t, rec.events, 1, "only the sign-in page opens with --force-logout=false")
	assert.Contains(t, rec.events[0], "https://123456789012.signin.aws.amazon.com/federation?")
	assert.Contains(t, rec.events[0], url.QueryEscape("https://eu-west-1.console.aws.amazon.com/"))
}

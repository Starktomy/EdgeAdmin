package loginutils

import (
	"crypto/tls"
	teaconst "github.com/TeaOSLab/EdgeAdmin/internal/const"
	"github.com/iwind/TeaGo/actions"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xlzd/gotp"
)

func TestSafeRedirectOnlyAcceptsLocalPaths(t *testing.T) {
	for _, value := range []string{"https://evil.example/", "//evil.example/", "javascript:alert(1)", "/\\evil.example", "/%2fexample.com", "/%5cexample.com", "/ok%0aLocation:x", "data:text/html,x", "/%", " /ok"} {
		if got := SafeRedirect(value); got != "" {
			t.Errorf("accepted redirect %q as %q", value, got)
		}
	}
	for _, value := range []string{"/dashboard", "/servers?keyword=a%20b", "/settings/login#password"} {
		if got := SafeRedirect(value); got != value {
			t.Errorf("rejected local redirect %q", value)
		}
	}
}

func TestProxyHeadersRequireExplicitTrustedPeer(t *testing.T) {
	t.Setenv("EDGE_TRUSTED_PROXY_CIDRS", "192.0.2.0/24,2001:db8::/32")
	req := httptest.NewRequest("GET", "http://admin.example/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	req.RemoteAddr = "198.51.100.1:1234"
	if TrustedProxy(req) || RequestIsHTTPS(req) {
		t.Fatal("trusted an unconfigured proxy")
	}
	req.RemoteAddr = "192.0.2.10:1234"
	if !TrustedProxy(req) || !RequestIsHTTPS(req) {
		t.Fatal("did not recognize trusted HTTPS proxy")
	}
	t.Setenv("EDGE_TRUSTED_PROXY_CIDRS", "")
	if TrustedProxy(req) || RequestIsHTTPS(req) {
		t.Fatal("forwarded headers trusted by default")
	}
	req.TLS = &tls.ConnectionState{}
	if !RequestIsHTTPS(req) {
		t.Fatal("direct TLS must be recognized")
	}
}

func TestOTPChallengesExpireAndAreBoundToBrowser(t *testing.T) {
	m := NewChallengeManager()
	now := time.Unix(1770000000, 0)
	m.now = func() time.Time { return now }
	secret := "JBSWY3DPEHPK3PXP"
	id, err := m.Create("browser-a", 42, true)
	if err != nil {
		t.Fatal(err)
	}
	code := gotp.NewDefaultTOTP(secret).At(now.Unix())
	if _, ok := m.Verify(id, "browser-b", secret, code); ok {
		t.Fatal("challenge accepted in another browser")
	}
	if _, ok := m.Details(id, "browser-a"); !ok {
		t.Fatal("attacker consumed owner's challenge")
	}
	now = now.Add(challengeLifetime)
	if _, ok := m.Verify(id, "browser-a", secret, gotp.NewDefaultTOTP(secret).At(now.Unix())); ok {
		t.Fatal("expired challenge accepted")
	}
}

func TestOTPChallengesLimitFailuresAcrossNewChallenges(t *testing.T) {
	m := NewChallengeManager()
	for attempt := 0; attempt < 2; attempt++ {
		id, err := m.Create("browser", 42, false)
		if err != nil {
			t.Fatal(err)
		}
		for i := 0; i < challengeAttempts; i++ {
			if _, ok := m.Verify(id, "browser", "JBSWY3DPEHPK3PXP", "bad"); ok {
				t.Fatal("wrong OTP accepted")
			}
		}
		if _, ok := m.Details(id, "browser"); ok {
			t.Fatal("exhausted challenge remained usable")
		}
	}
	if _, err := m.Create("new-browser", 42, false); err == nil {
		t.Fatal("renewed challenge reset account failure limit")
	}
}

func TestOTPChallengeConsumedAtomicallyAndStepCannotReplay(t *testing.T) {
	m := NewChallengeManager()
	now := time.Unix(1770000000, 0)
	m.now = func() time.Time { return now }
	secret := "JBSWY3DPEHPK3PXP"
	id, err := m.Create("browser", 42, true)
	if err != nil {
		t.Fatal(err)
	}
	code := gotp.NewDefaultTOTP(secret).At(now.Unix())
	var successes atomic.Int32
	var group sync.WaitGroup
	for i := 0; i < 32; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			if c, ok := m.Verify(id, "browser", secret, code); ok {
				if !c.Remember {
					t.Error("remember policy changed")
				}
				successes.Add(1)
			}
		}()
	}
	group.Wait()
	if successes.Load() != 1 {
		t.Fatalf("challenge accepted %d times", successes.Load())
	}
	id, err = m.Create("other-browser", 42, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Verify(id, "other-browser", secret, code); ok {
		t.Fatal("TOTP time step reused for a different challenge")
	}
	now = now.Add(30 * time.Second)
	if _, ok := m.Verify(id, "other-browser", secret, gotp.NewDefaultTOTP(secret).At(now.Unix())); !ok {
		t.Fatal("next time step rejected")
	}
}

func TestSessionIDUsesFixedLengthIndependentRandomness(t *testing.T) {
	a, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 32 || len(b) != 32 || a == b || strings.Trim(a, "0123456789abcdef") != "" {
		t.Fatal("unexpected session ID")
	}
}

func TestForwardedChainStopsAtLastUntrustedHop(t *testing.T) {
	t.Setenv("EDGE_TRUSTED_PROXY_CIDRS", "192.0.2.0/24")
	req := httptest.NewRequest("GET", "http://admin.example/", nil)
	req.RemoteAddr = "192.0.2.10:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.99, 198.51.100.4, 192.0.2.12")
	if got := forwardedClient(req, "X-Forwarded-For"); got != "198.51.100.4" {
		t.Fatalf("trusted attacker-controlled prefix: %q", got)
	}
	t.Setenv("EDGE_TRUSTED_PROXY_CIDRS", "192.0.2.0/24,bad")
	if TrustedProxy(req) || forwardedClient(req, "X-Forwarded-For") != "" {
		t.Fatal("malformed proxy trust did not fail closed")
	}
}

func TestMalformedOTPSecretFailsClosed(t *testing.T) {
	m := NewChallengeManager()
	id, err := m.Create("browser", 42, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := m.Verify(id, "browser", "not-base32!", "123456"); ok {
		t.Fatal("invalid OTP secret accepted")
	}
}

func TestRawAdminPasswordProtocolPreservesPasswordData(t *testing.T) {
	for _, password := range []string{"correct horse battery staple", " leading and trailing ", "raw-v1:literal-prefix", "密码 🔑", "d41d8cd98f00b204e9800998ecf8427e"} {
		got := AdminPasswordForRPC(password)
		if !strings.HasPrefix(got, "raw-v1:") || strings.TrimPrefix(got, "raw-v1:") != password {
			t.Fatalf("password data changed: %q", password)
		}
	}
}

type testBrowserSessionStore struct{}

func (*testBrowserSessionStore) Init(*actions.SessionConfig)           {}
func (*testBrowserSessionStore) Read(string) map[string]string         { return nil }
func (*testBrowserSessionStore) WriteItem(string, string, string) bool { return true }
func (*testBrowserSessionStore) Delete(string) bool                    { return true }

func TestAnonymousTokenSessionSetsCookieAndKeepsItOnRetry(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "https://admin.example/csrf/token", nil)
	action := &actions.ActionObject{Request: request, ResponseWriter: response, SessionManager: &testBrowserSessionStore{}}
	action.SetSessionCookieName(teaconst.CookieSID)
	if err := EnsureBrowserSession(action); err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != action.Session().Sid || !cookies[0].HttpOnly || !cookies[0].Secure || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("anonymous form token lacks a stable secure session cookie")
	}
	retry := &actions.ActionObject{Request: httptest.NewRequest(http.MethodGet, "https://admin.example/csrf/token", nil), ResponseWriter: httptest.NewRecorder(), SessionManager: &testBrowserSessionStore{}}
	retry.SetSessionCookieName(teaconst.CookieSID)
	retry.Request.AddCookie(cookies[0])
	if err := EnsureBrowserSession(retry); err != nil {
		t.Fatal(err)
	}
	if retry.Session().Sid != action.Session().Sid {
		t.Fatal("token retry changed the browser session and invalidated pending OTP")
	}
}

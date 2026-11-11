package actionutils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/TeaOSLab/EdgeAdmin/internal/csrf"
	"github.com/iwind/TeaGo/actions"
)

type noopSessionManager struct{}

func (*noopSessionManager) Init(*actions.SessionConfig)           {}
func (*noopSessionManager) Read(string) map[string]string         { return nil }
func (*noopSessionManager) WriteItem(string, string, string) bool { return false }
func (*noopSessionManager) Delete(string) bool                    { return true }

type csrfTestAction struct{ actions.ActionObject }

func (a *csrfTestAction) RunGet() {}

func csrfAction(sid, token string) *csrfTestAction {
	request := httptest.NewRequest(http.MethodPost, "http://admin.example/settings/login", nil)
	request.AddCookie(&http.Cookie{Name: "sid", Value: sid})
	request.Header.Set("X-CSRF-Token", token)
	return &csrfTestAction{ActionObject: actions.ActionObject{Request: request, ResponseWriter: httptest.NewRecorder(), SessionManager: &noopSessionManager{}}}
}

func TestCSRFHelperRejectsCrossSessionAndReplay(t *testing.T) {
	sessionA := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	sessionB := "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	token, err := csrf.Generate(sessionA)
	if err != nil {
		t.Fatal(err)
	}
	guard := &CSRF{}
	if guard.BeforeAction(csrfAction(sessionB, token), "CSRF") {
		t.Fatal("foreign-session token accepted")
	}
	if !guard.BeforeAction(csrfAction(sessionA, token), "CSRF") {
		t.Fatal("valid form rejected")
	}
	if guard.BeforeAction(csrfAction(sessionA, token), "CSRF") {
		t.Fatal("replayed form accepted")
	}
	if guard.BeforeAction(csrfAction(sessionA, ""), "CSRF") {
		t.Fatal("missing form token accepted")
	}
}

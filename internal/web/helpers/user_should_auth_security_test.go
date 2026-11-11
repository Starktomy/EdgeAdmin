package helpers

import (
	"github.com/iwind/TeaGo/actions"
	"net/http"
	"net/http/httptest"
	"testing"
)

type failingSessionStore struct {
	writes  int
	deleted string
}

func (*failingSessionStore) Init(*actions.SessionConfig)             {}
func (*failingSessionStore) Read(string) map[string]string           { return nil }
func (s *failingSessionStore) WriteItem(string, string, string) bool { s.writes++; return false }
func (s *failingSessionStore) Delete(id string) bool                 { s.deleted = id; return true }

func TestStoreAdminDoesNotPublishPartiallyPersistedSession(t *testing.T) {
	store := &failingSessionStore{}
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "http://admin.example/", nil)
	oldID := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	request.AddCookie(&http.Cookie{Name: "sid", Value: oldID})
	action := &actions.ActionObject{Request: request, ResponseWriter: response, SessionManager: store}
	auth := &UserShouldAuth{action: action}
	if err := auth.StoreAdmin(42, false, "local-id"); err == nil {
		t.Fatal("granted a session after persistence failed")
	}
	if store.writes != 1 || store.deleted == "" || store.deleted == oldID {
		t.Fatal("failed new session was not removed")
	}
	if action.Session().Sid != oldID || len(response.Result().Cookies()) != 0 {
		t.Fatal("published authenticated cookie on failure")
	}
}

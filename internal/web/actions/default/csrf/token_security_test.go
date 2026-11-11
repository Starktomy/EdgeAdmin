package csrf

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	teaconst "github.com/TeaOSLab/EdgeAdmin/internal/const"
	csrfutils "github.com/TeaOSLab/EdgeAdmin/internal/csrf"
	"github.com/iwind/TeaGo/actions"
)

type tokenSessionStore struct{}

func (*tokenSessionStore) Init(*actions.SessionConfig)           {}
func (*tokenSessionStore) Read(string) map[string]string         { return nil }
func (*tokenSessionStore) WriteItem(string, string, string) bool { return true }
func (*tokenSessionStore) Delete(string) bool                    { return true }

func TestRecoveryCanIssueBrowserBoundFormToken(t *testing.T) {
	old := teaconst.IsRecoverMode
	teaconst.IsRecoverMode = true
	defer func() { teaconst.IsRecoverMode = old }()
	response := httptest.NewRecorder()
	action := &TokenAction{}
	action.Request = httptest.NewRequest("GET", "http://admin.example/csrf/token", nil)
	action.ResponseWriter = response
	action.SessionManager = &tokenSessionStore{}
	action.SetSessionCookieName(teaconst.CookieSID)
	action.Data = actions.Data{}
	func() {
		defer func() {
			if value := recover(); value != nil {
				if _, ok := value.(*actions.ActionObject); !ok {
					panic(value)
				}
			}
		}()
		action.RunGet(struct{}{})
	}()
	var result struct {
		Code int `json:"code"`
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Code != 200 || !csrfutils.Validate(result.Data.Token, action.Session().Sid) {
		t.Fatal("recovery token issuance failed")
	}
	if len(response.Result().Cookies()) != 1 {
		t.Fatal("recovery browser session not persisted")
	}
}

package loginutils

import (
	"crypto/rand"
	"encoding/hex"

	teaconst "github.com/TeaOSLab/EdgeAdmin/internal/const"
	"github.com/iwind/TeaGo/actions"
)

func NewSessionID() (string, error) {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(data[:]), nil
}

// EnsureBrowserSession sets the anonymous session cookie before issuing a
// session-bound form token. Reading TeaGo's Session alone does not set a cookie.
func EnsureBrowserSession(action *actions.ActionObject) error {
	cookie, err := action.Request.Cookie(teaconst.CookieSID)
	if err == nil && cookie != nil && len(cookie.Value) == 32 {
		return nil
	}
	sid, err := NewSessionID()
	if err != nil {
		return err
	}
	action.Session().SetSid(sid)
	SetCookie(action, false)
	return nil
}

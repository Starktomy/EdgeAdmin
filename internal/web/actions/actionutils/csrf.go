package actionutils

import (
	"net/http"

	"github.com/TeaOSLab/EdgeAdmin/internal/csrf"
	"github.com/iwind/TeaGo/actions"
)

type CSRF struct {
}

func (this *CSRF) BeforeAction(actionPtr actions.ActionWrapper, paramName string) (goNext bool) {
	action := actionPtr.Object()
	token := action.Request.Header.Get("X-CSRF-Token")
	if len(token) == 0 {
		token = action.ParamString("csrfToken")
	}
	if !csrf.Validate(token, action.Session().Sid) {
		action.ResponseWriter.WriteHeader(http.StatusForbidden)
		action.WriteString("表单已失效，请刷新页面后重试(001)")
		return
	}

	return true
}

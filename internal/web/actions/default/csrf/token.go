package csrf

import (
	"github.com/TeaOSLab/EdgeAdmin/internal/csrf"
	"github.com/TeaOSLab/EdgeAdmin/internal/web/actions/actionutils"
	"github.com/TeaOSLab/EdgeAdmin/internal/web/actions/default/index/loginutils"
)

type TokenAction struct {
	actionutils.ParentAction
}

func (this *TokenAction) Init() {
	this.Nav("", "", "")
}

// Bootstrap and recovery forms also need anonymous browser-bound tokens.
func (this *TokenAction) RunGet(params struct{}) {
	if err := loginutils.EnsureBrowserSession(this.Object()); err != nil {
		this.Fail("无法创建浏览器会话，请稍后重试")
		return
	}
	token, err := csrf.Generate(this.Session().Sid)
	if err != nil {
		this.Fail("无法生成表单令牌，请稍后重试")
		return
	}
	this.AddHeader("Cache-Control", "no-store")
	this.Data["token"] = token
	this.Success()
}

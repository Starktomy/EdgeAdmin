package helpers

import (
	"errors"
	"net/http"

	"github.com/TeaOSLab/EdgeAdmin/internal/configloaders"
	teaconst "github.com/TeaOSLab/EdgeAdmin/internal/const"
	"github.com/TeaOSLab/EdgeAdmin/internal/utils/numberutils"
	"github.com/TeaOSLab/EdgeAdmin/internal/web/actions/default/index/loginutils"
	"github.com/iwind/TeaGo/actions"
)

type UserShouldAuth struct {
	action *actions.ActionObject
}

func (this *UserShouldAuth) BeforeAction(actionPtr actions.ActionWrapper, paramName string) (goNext bool) {
	if teaconst.IsRecoverMode {
		actionPtr.Object().RedirectURL("/recover")
		return false
	}

	this.action = actionPtr.Object()

	// 检查请求是否合法
	if isEvilRequest(this.action.Request) {
		this.action.ResponseWriter.WriteHeader(http.StatusForbidden)
		return false
	}

	// 检测注入
	if !safeFilterRequest(this.action.Request) {
		this.action.ResponseWriter.WriteHeader(http.StatusForbidden)
		_, _ = this.action.ResponseWriter.Write([]byte("Denied By WAF"))
		return false
	}

	// 安全相关
	var action = this.action
	securityConfig, _ := configloaders.LoadSecurityConfig()
	if securityConfig == nil {
		action.AddHeader("X-Frame-Options", "SAMEORIGIN")
	} else if len(securityConfig.Frame) > 0 {
		action.AddHeader("X-Frame-Options", securityConfig.Frame)
	}
	action.AddHeader("Content-Security-Policy", "default-src 'self' data:; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'")

	// 检查IP
	if !checkIP(securityConfig, loginutils.RemoteIP(action)) {
		action.ResponseWriter.WriteHeader(http.StatusForbidden)
		return false
	}

	// 检查请求
	if !checkRequestSecurity(securityConfig, action.Request) {
		action.ResponseWriter.WriteHeader(http.StatusForbidden)
		return false
	}

	return true
}

// StoreAdmin 存储用户名到SESSION
func (this *UserShouldAuth) StoreAdmin(adminId int64, remember bool, localSid string) error {
	if adminId <= 0 {
		return errors.New("invalid administrator")
	}
	newSID, err := loginutils.NewSessionID()
	if err != nil {
		return err
	}
	var session = this.action.Session()
	oldSession := *session
	session.SetSid(newSID)
	if !session.Write("adminId", numberutils.FormatInt64(adminId)) ||
		!session.Write("@fingerprint", loginutils.CalculateClientFingerprint(this.action)) ||
		!session.Write("@ip", loginutils.RemoteIP(this.action)) ||
		!session.Write("@localSid", localSid) {
		session.Delete()
		session.SetSid(oldSession.Sid)
		return errors.New("could not persist the authenticated session")
	}
	securityConfig, _ := configloaders.LoadSecurityConfig()
	remember = remember && securityConfig != nil && securityConfig.AllowRememberLogin
	loginutils.SetCookie(this.action, remember)
	oldSession.Delete()
	return nil
}

func (this *UserShouldAuth) IsUser() bool {
	return this.action.Session().GetInt("adminId") > 0
}

func (this *UserShouldAuth) AdminId() int {
	return this.action.Session().GetInt("adminId")
}

func (this *UserShouldAuth) Logout() {
	loginutils.UnsetCookie(this.action)
	this.action.Session().Delete()
}

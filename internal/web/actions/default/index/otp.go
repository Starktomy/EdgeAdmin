// Copyright 2023 GoEdge CDN goedge.cdn@gmail.com. All rights reserved. Official site: https://goedge.cloud .

package index

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/TeaOSLab/EdgeAdmin/internal/configloaders"
	teaconst "github.com/TeaOSLab/EdgeAdmin/internal/const"
	"github.com/TeaOSLab/EdgeAdmin/internal/oplogs"
	"github.com/TeaOSLab/EdgeAdmin/internal/rpc"
	"github.com/TeaOSLab/EdgeAdmin/internal/setup"
	"github.com/TeaOSLab/EdgeAdmin/internal/utils"
	"github.com/TeaOSLab/EdgeAdmin/internal/web/actions/actionutils"
	"github.com/TeaOSLab/EdgeAdmin/internal/web/actions/default/index/loginutils"
	"github.com/TeaOSLab/EdgeAdmin/internal/web/helpers"
	"github.com/TeaOSLab/EdgeCommon/pkg/langs/codes"
	"github.com/TeaOSLab/EdgeCommon/pkg/rpc/dao"
	"github.com/TeaOSLab/EdgeCommon/pkg/rpc/pb"
	"github.com/iwind/TeaGo/actions"
	"github.com/iwind/TeaGo/maps"
	stringutil "github.com/iwind/TeaGo/utils/string"
)

type OtpAction struct {
	actionutils.ParentAction
}

func (this *OtpAction) Init() {
	this.Nav("", "", "")
}

func (this *OtpAction) RunGet(params struct {
	From     string
	Sid      string
	Remember bool
}) {
	// 检查系统是否已经配置过
	if !setup.IsConfigured() {
		this.RedirectURL("/setup")
		return
	}

	// // 是否新安装
	if setup.IsNewInstalled() {
		this.RedirectURL("/setup/confirm")
		return
	}

	this.Data["isUser"] = false
	this.Data["menu"] = "signIn"

	var timestamp = fmt.Sprintf("%d", time.Now().Unix())
	this.Data["token"] = stringutil.Md5(TokenKey+timestamp) + timestamp
	this.Data["from"] = loginutils.SafeRedirect(params.From)
	this.Data["sid"] = params.Sid

	uiConfig, err := configloaders.LoadAdminUIConfig()
	if err != nil {
		this.ErrorPage(err)
		return
	}
	this.Data["systemName"] = uiConfig.AdminSystemName
	this.Data["showVersion"] = uiConfig.ShowVersion
	if len(uiConfig.Version) > 0 {
		this.Data["version"] = uiConfig.Version
	} else {
		this.Data["version"] = teaconst.Version
	}
	this.Data["faviconFileId"] = uiConfig.FaviconFileId
	this.Data["remember"] = params.Remember

	this.Show()
}

func (this *OtpAction) RunPost(params struct {
	Sid      string
	OtpCode  string
	Remember bool

	Must *actions.Must
	Auth *helpers.UserShouldAuth
	CSRF *actionutils.CSRF
}) {
	if len(params.OtpCode) == 0 {
		this.FailField("otpCode", "请输入正确的OTP动态密码")
		return
	}

	challenge, ok := loginutils.OTPChallenges.Details(params.Sid, this.Session().Sid)
	if !ok {
		this.Fail("验证已过期或尝试次数过多，请重新登录")
		return
	}
	adminId := challenge.AdminID
	// 检查OTP
	otpLoginResp, err := this.RPC().LoginRPC().FindEnabledLogin(this.AdminContext(), &pb.FindEnabledLoginRequest{
		AdminId: adminId,
		Type:    "otp",
	})
	if err != nil {
		this.ErrorPage(err)
		return
	}
	if otpLoginResp.Login == nil || !otpLoginResp.Login.IsOn {
		this.Fail("二次验证设置已变更，请重新登录")
		return
	}
	loginParams := maps.Map{}
	if err := json.Unmarshal(otpLoginResp.Login.ParamsJSON, &loginParams); err != nil {
		this.ErrorPage(err)
		return
	}
	challenge, ok = loginutils.OTPChallenges.Verify(params.Sid, this.Session().Sid, loginParams.GetString("secret"), params.OtpCode)
	if !ok {
		this.FailField("otpCode", "请输入正确的OTP动态密码；验证过期或多次失败后需重新登录")
		return
	}

	// 写入SESSION
	localSid, err := loginutils.NewSessionID()
	if err != nil {
		this.ErrorPage(err)
		return
	}
	this.Data["localSid"] = localSid
	this.Data["ip"] = loginutils.RemoteIP(&this.ActionObject)
	if err := params.Auth.StoreAdmin(adminId, challenge.Remember, localSid); err != nil {
		this.ErrorPage(err)
		return
	}

	// 记录日志
	rpcClient, err := rpc.SharedRPC()
	if err != nil {
		this.ErrorPage(err)
		return
	}
	err = dao.SharedLogDAO.CreateAdminLog(rpcClient.Context(adminId), oplogs.LevelInfo, this.Request.URL.Path, this.Lang(codes.AdminLogin_LogOtpVerifiedSuccess), loginutils.RemoteIP(&this.ActionObject), codes.AdminLogin_LogOtpVerifiedSuccess, nil)
	if err != nil {
		utils.PrintError(err)
	}

	this.Success()
}

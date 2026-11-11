// Copyright 2022 GoEdge CDN goedge.cdn@gmail.com. All rights reserved. Official site: https://goedge.cloud .

package files

import (
	"errors"
	"net/http"

	"github.com/TeaOSLab/EdgeAdmin/internal/configloaders"
	"github.com/TeaOSLab/EdgeAdmin/internal/web/actions/actionutils"
	"github.com/TeaOSLab/EdgeCommon/pkg/rpc/pb"
)

type FileAction struct {
	actionutils.ParentAction
}

func (this *FileAction) Init() {
	this.Nav("", "", "")
}

func (this *FileAction) RunGet(params struct {
	FileId int64
}) {
	this.serveFile(params.FileId, false)
}

// PrivateFileAction requires a validated super administrator. The public
// endpoint never promotes the backend service's credential to visitor access.
type PrivateFileAction struct{ FileAction }

func (this *PrivateFileAction) RunGet(params struct{ FileId int64 }) {
	if this.AdminId() <= 0 {
		this.ResponseWriter.WriteHeader(http.StatusForbidden)
		return
	}
	adminResp, err := this.RPC().AdminRPC().FindEnabledAdmin(this.AdminContext(), &pb.FindEnabledAdminRequest{AdminId: this.AdminId()})
	if err != nil {
		this.ErrorPage(err)
		return
	}
	if adminResp.Admin == nil || !adminResp.Admin.IsOn || !adminResp.Admin.CanLogin || !adminResp.Admin.IsSuper || !configloaders.CheckAdmin(this.AdminId()) {
		this.ResponseWriter.WriteHeader(http.StatusForbidden)
		return
	}
	this.serveFile(params.FileId, true)
}

func (this *FileAction) serveFile(fileID int64, allowPrivate bool) {
	err := downloadFile(this.AdminContext(), this.ResponseWriter.Header(), &this.ActionObject, this.RPC().FileRPC(), this.RPC().FileChunkRPC(), fileID, allowPrivate)
	if errors.Is(err, errFileForbidden) {
		this.ResponseWriter.WriteHeader(http.StatusForbidden)
		return
	}
	if errors.Is(err, errFileNotFound) {
		this.ResponseWriter.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		this.ErrorPage(err)
	}
}

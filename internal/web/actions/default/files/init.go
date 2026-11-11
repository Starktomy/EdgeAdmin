// Copyright 2022 GoEdge CDN goedge.cdn@gmail.com. All rights reserved. Official site: https://goedge.cloud .

package files

import (
	"github.com/TeaOSLab/EdgeAdmin/internal/configloaders"
	"github.com/TeaOSLab/EdgeAdmin/internal/web/helpers"
	"github.com/iwind/TeaGo"
)

func init() {
	TeaGo.BeforeStart(func(server *TeaGo.Server) {
		server.
			Prefix("/files").
			Get("/file", new(FileAction)).
			Helper(helpers.NewUserMustAuth(configloaders.AdminModuleCodeSetting)).
			Get("/private", new(PrivateFileAction)).
			EndAll()
	})
}

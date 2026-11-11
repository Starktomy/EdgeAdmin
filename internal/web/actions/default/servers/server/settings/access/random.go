// Copyright 2022 GoEdge CDN goedge.cdn@gmail.com. All rights reserved. Official site: https://goedge.cloud .

package access

import (
	"github.com/TeaOSLab/EdgeAdmin/internal/utils"
	"github.com/TeaOSLab/EdgeAdmin/internal/web/actions/actionutils"
)

type RandomAction struct {
	actionutils.ParentAction
}

func (this *RandomAction) RunPost(params struct{}) {
	token, err := utils.RandomToken()
	if err != nil {
		this.ErrorPage(err)
		return
	}
	this.Data["random"] = token

	this.Success()
}

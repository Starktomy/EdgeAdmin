// Copyright 2023 GoEdge CDN goedge.cdn@gmail.com. All rights reserved. Official site: https://goedge.cloud .

package updates

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/TeaOSLab/EdgeAdmin/internal/utils"
	executils "github.com/TeaOSLab/EdgeAdmin/internal/utils/exec"
	"github.com/TeaOSLab/EdgeAdmin/internal/web/actions/actionutils"
	"github.com/TeaOSLab/EdgeAdmin/internal/web/actions/default/settings/updates/updateutils"
)

var upgradeProgress atomic.Uint32
var isUpgrading atomic.Bool
var isUpgradingDB atomic.Bool

func upgradeProgressValue() float32 { return math.Float32frombits(upgradeProgress.Load()) }

type UpgradeAction struct {
	actionutils.ParentAction
}

func (this *UpgradeAction) RunGet(params struct {
}) {
	this.Data["isUpgrading"] = isUpgrading.Load()
	this.Data["isUpgradingDB"] = isUpgradingDB.Load()
	this.Data["upgradeProgress"] = fmt.Sprintf("%.2f", upgradeProgressValue()*100)
	this.Success()
}

func (this *UpgradeAction) RunPost(params struct {
	Url string

	CSRF *actionutils.CSRF
}) {
	if !isUpgrading.CompareAndSwap(false, true) {
		this.Success()
		return
	}

	upgradeProgress.Store(0)

	defer func() {
		isUpgrading.Store(false)
	}()

	var manager = utils.NewUpgradeManager("admin", params.Url)
	var ticker = time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	done := make(chan struct{})
	defer close(done)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
			}
			if manager.IsDownloading() {
				var progress = manager.Progress()
				if progress >= 0 {
					upgradeProgress.Store(math.Float32bits(progress))
				}
			} else {
				return
			}
		}
	}()
	err := manager.Start()
	if err != nil {
		this.Fail("下载失败：" + err.Error())
		return
	}

	// try to exec local 'edge-api upgrade'
	exePath, ok := updateutils.CheckLocalAPINode(this.RPC(), this.AdminContext())
	if ok && len(exePath) > 0 {
		isUpgradingDB.Store(true)
		var before = time.Now()
		var cmd = executils.NewCmd(exePath, "upgrade")
		_ = cmd.Run()
		var costSeconds = time.Since(before).Seconds()

		// sleep to show upgrading status
		if costSeconds < 3 {
			time.Sleep(3 * time.Second)
		}
		isUpgradingDB.Store(false)
	}

	// restart
	exe, _ := os.Executable()
	if len(exe) > 0 {
		go func() {
			var cmd = exec.Command(exe, "restart")
			_ = cmd.Run()
		}()
	}

	this.Success()
}

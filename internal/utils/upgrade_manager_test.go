// Copyright 2022 GoEdge CDN goedge.cdn@gmail.com. All rights reserved. Official site: https://goedge.cloud .

package utils_test

import (
	"os"
	"testing"
	"time"

	"github.com/TeaOSLab/EdgeAdmin/internal/utils"
)

func TestNewUpgradeManager(t *testing.T) {
	downloadURL := os.Getenv("EDGE_TEST_SIGNED_UPGRADE_URL")
	if downloadURL == "" {
		t.Skip("live upgrade integration test requires EDGE_TEST_SIGNED_UPGRADE_URL and local release trust configuration")
	}
	var manager = utils.NewUpgradeManager("admin", downloadURL)

	var ticker = time.NewTicker(2 * time.Second)
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
				t.Logf("%.2f%%", manager.Progress()*100)
			}
		}
	}()

	/**go func() {
		time.Sleep(5 * time.Second)
		if manager.IsDownloading() {
			t.Log("cancel downloading")
			_ = manager.Cancel()
		}
	}()**/

	err := manager.Start()
	if err != nil {
		t.Fatal(err)
	}
}

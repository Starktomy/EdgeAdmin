// Copyright 2022 GoEdge CDN goedge.cdn@gmail.com. All rights reserved. Official site: https://goedge.cloud .

package utils

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	teaconst "github.com/TeaOSLab/EdgeAdmin/internal/const"
	executils "github.com/TeaOSLab/EdgeAdmin/internal/utils/exec"
	"github.com/TeaOSLab/EdgeAdmin/internal/utils/upgradesecurity"
	"github.com/iwind/TeaGo/Tea"
	"github.com/iwind/TeaGo/maps"
	"github.com/iwind/TeaGo/types"
	stringutil "github.com/iwind/TeaGo/utils/string"
)

type UpgradeFileWriter struct {
	rawWriter io.Writer
	written   atomic.Int64
}

func NewUpgradeFileWriter(rawWriter io.Writer) *UpgradeFileWriter {
	return &UpgradeFileWriter{rawWriter: rawWriter}
}

func (this *UpgradeFileWriter) Write(p []byte) (n int, err error) {
	n, err = this.rawWriter.Write(p)
	this.written.Add(int64(n))
	return
}

func (this *UpgradeFileWriter) TotalWritten() int64 {
	return this.written.Load()
}

type UpgradeManager struct {
	client *http.Client

	component string

	newVersion    string
	contentLength atomic.Int64
	isDownloading atomic.Bool
	writer        *UpgradeFileWriter
	body          io.ReadCloser
	isCancelled   atomic.Bool

	downloadURL string
	mu          sync.Mutex
	cancel      context.CancelFunc
}

func NewUpgradeManager(component string, downloadURL string) *UpgradeManager {
	return &UpgradeManager{component: component, downloadURL: downloadURL}
}

func upgradeHTTPClient(trust *upgradesecurity.Trust) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	return &http.Client{
		Transport: transport,
		Timeout:   30 * time.Minute,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many upgrade redirects")
			}
			return trust.ValidateURL(req.URL.String())
		},
	}
}

func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("upgrade metadata exceeds size limit")
	}
	return data, nil
}

func (this *UpgradeManager) Start() error {
	if this.component != "admin" {
		return errors.New("unsupported upgrade component")
	}
	if !this.isDownloading.CompareAndSwap(false, true) {
		return errors.New("another process is running")
	}
	defer this.isDownloading.Store(false)
	this.isCancelled.Store(false)
	trustFile, err := os.Open(Tea.ConfigFile("upgrade-trust.json"))
	if err != nil {
		return errors.New("signed upgrades require configs/upgrade-trust.json; see doc/signed-upgrades.md")
	}
	trustData, err := readBounded(trustFile, upgradesecurity.MaxManifestBytes)
	trustFile.Close()
	if err != nil {
		return err
	}
	trust, err := upgradesecurity.ParseTrust(trustData)
	if err != nil {
		return err
	}
	this.client = upgradeHTTPClient(trust)
	defer this.client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	this.mu.Lock()
	this.cancel = cancel
	this.mu.Unlock()
	defer cancel()

	// 检查cp
	cpExe, _ := executils.LookPath("cp")
	if len(cpExe) == 0 {
		return errors.New("can not find 'cp' command")
	}

	// 检查新版本
	var downloadURL = this.downloadURL
	if len(downloadURL) == 0 {
		var url = teaconst.UpdatesURL
		var osName = runtime.GOOS
		if Tea.IsTesting() && osName == "darwin" {
			osName = "linux"
		}
		url = strings.ReplaceAll(url, "${os}", osName)
		url = strings.ReplaceAll(url, "${arch}", runtime.GOARCH)
		url = strings.ReplaceAll(url, "${version}", teaconst.Version)
		if err := trust.ValidateURL(url); err != nil {
			return err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return fmt.Errorf("create url request failed: %w", err)
		}
		req.Header.Set("User-Agent", "Edge-Admin/"+teaconst.Version)

		resp, err := this.client.Do(req)
		if err != nil {
			return fmt.Errorf("read latest version failed: %w", err)
		}

		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			return errors.New("read latest version failed: invalid response code '" + types.String(resp.StatusCode) + "'")
		}

		data, err := readBounded(resp.Body, upgradesecurity.MaxManifestBytes)
		if err != nil {
			return fmt.Errorf("read latest version failed: %w", err)
		}

		var m = maps.Map{}
		err = json.Unmarshal(data, &m)
		if err != nil {
			return fmt.Errorf("invalid response data: %w, origin data: %s", err, string(data))
		}

		var code = m.GetInt("code")
		if code != 200 {
			return errors.New(m.GetString("message"))
		}

		var dataMap = m.GetMap("data")
		var downloadHost = dataMap.GetString("host")
		var versions = dataMap.GetSlice("versions")
		var downloadPath = ""
		for _, component := range versions {
			var componentMap = maps.NewMap(component)
			if componentMap.Has("version") {
				if componentMap.GetString("code") == this.component {
					var version = componentMap.GetString("version")
					if stringutil.VersionCompare(version, teaconst.Version) > 0 {
						this.mu.Lock()
						this.newVersion = version
						this.mu.Unlock()
						downloadPath = componentMap.GetString("url")
						break
					}
				}
			}
		}

		if len(downloadPath) == 0 {
			return errors.New("no latest version to download")
		}

		downloadURL = downloadHost + downloadPath
	}

	if err := trust.ValidateURL(downloadURL); err != nil {
		return err
	}
	sidecarURL, err := upgradesecurity.SidecarURL(downloadURL)
	if err != nil {
		return err
	}
	manifestRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, sidecarURL, nil)
	if err != nil {
		return err
	}
	manifestResponse, err := this.client.Do(manifestRequest)
	if err != nil {
		return fmt.Errorf("download signed release manifest: %w", err)
	}
	manifestData, readErr := readBounded(manifestResponse.Body, upgradesecurity.MaxManifestBytes)
	manifestResponse.Body.Close()
	if manifestResponse.StatusCode != http.StatusOK || readErr != nil {
		return errors.New("a valid signed release manifest is required")
	}
	manifest, err := upgradesecurity.Verify(manifestData, trust, this.component, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	if stringutil.VersionCompare(manifest.Version, teaconst.Version) <= 0 {
		return errors.New("release manifest must specify a newer version")
	}
	this.mu.Lock()
	this.newVersion = manifest.Version
	this.mu.Unlock()

	{
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
		if err != nil {
			return err
		}
		req.Header.Set("User-Agent", "Edge-Admin/"+teaconst.Version)
		resp, err := this.client.Do(req)
		if err != nil {
			return fmt.Errorf("download failed: %w", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return errors.New("release artifact download failed")
		}
		if resp.ContentLength > 0 && resp.ContentLength != manifest.Size {
			return errors.New("release artifact size does not match signed manifest")
		}
		this.contentLength.Store(manifest.Size)
		this.mu.Lock()
		this.body = resp.Body
		this.mu.Unlock()
		tmpDir, err := os.MkdirTemp("", "edge-admin-upgrade-*")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)
		destFile := filepath.Join(tmpDir, "release.zip")
		fp, err := os.OpenFile(destFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		writer := NewUpgradeFileWriter(fp)
		this.mu.Lock()
		this.writer = writer
		this.mu.Unlock()
		copyErr := upgradesecurity.CopyArtifact(writer, resp.Body, manifest)
		closeErr := fp.Close()
		if copyErr != nil {
			return fmt.Errorf("download release: %w", copyErr)
		}
		if closeErr != nil {
			return closeErr
		}
		if this.isCancelled.Load() {
			return errors.New("upgrade cancelled")
		}
		unzipDir := filepath.Join(tmpDir, "extracted")
		if err := upgradesecurity.Extract(destFile, unzipDir); err != nil {
			return fmt.Errorf("unzip installation file: %w", err)
		}
		executable := filepath.Join(unzipDir, "edge-"+this.component, "bin", "edge-"+this.component)
		if info, err := os.Stat(executable); err != nil || !info.Mode().IsRegular() {
			return errors.New("signed release does not contain the expected executable")
		}

		installationFiles, err := filepath.Glob(unzipDir + "/edge-" + this.component + "/*")
		if err != nil {
			return fmt.Errorf("lookup installation files failed: %w", err)
		}

		// cp to target dir
		currentExe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("reveal current executable file path failed: %w", err)
		}
		var targetDir = filepath.Dir(filepath.Dir(currentExe))
		if !Tea.IsTesting() {
			for _, installationFile := range installationFiles {
				var cpCmd = exec.Command(cpExe, "-R", "-f", installationFile, targetDir)
				var cpStderr = &bytes.Buffer{}
				cpCmd.Stderr = cpStderr
				err = cpCmd.Run()
				if err != nil {
					return errors.New("overwrite installation files failed: '" + cpCmd.String() + "': " + cpStderr.String())
				}
			}
		}

		// remove tmp
		_ = os.RemoveAll(unzipDir)
	}

	return nil
}

func (this *UpgradeManager) IsDownloading() bool { return this.isDownloading.Load() }

func (this *UpgradeManager) Progress() float32 {
	length := this.contentLength.Load()
	this.mu.Lock()
	writer := this.writer
	this.mu.Unlock()
	if length <= 0 || writer == nil {
		return -1
	}
	return float32(writer.TotalWritten()) / float32(length)
}

func (this *UpgradeManager) NewVersion() string {
	this.mu.Lock()
	defer this.mu.Unlock()
	return this.newVersion
}

func (this *UpgradeManager) Cancel() error {
	this.isCancelled.Store(true)
	this.mu.Lock()
	cancel := this.cancel
	this.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

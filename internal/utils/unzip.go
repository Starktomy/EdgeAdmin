package utils

import (
	"errors"

	"github.com/TeaOSLab/EdgeAdmin/internal/utils/upgradesecurity"
)

type Unzip struct {
	zipFile   string
	targetDir string
}

func NewUnzip(zipFile, targetDir string) *Unzip {
	return &Unzip{zipFile: zipFile, targetDir: targetDir}
}

func (this *Unzip) Run() error {
	if this.zipFile == "" || this.targetDir == "" {
		return errors.New("zip file and target directory are required")
	}
	return upgradesecurity.Extract(this.zipFile, this.targetDir)
}

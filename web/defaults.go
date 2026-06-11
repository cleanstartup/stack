package web

import (
	"path/filepath"
	"strings"
)

func DefaultStackRoot(baseDir string) string {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return ".stack"
	}
	return filepath.Join(baseDir, ".stack")
}

func DefaultWorkspaceDir(baseDir string) string {
	return filepath.Join(DefaultStackRoot(baseDir), "workspace")
}

func DefaultOutputDir(baseDir string) string {
	return filepath.Join(DefaultStackRoot(baseDir), "public")
}

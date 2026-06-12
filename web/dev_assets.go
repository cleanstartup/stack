package web

import (
	"os"
	"sort"

	stencilpkg "github.com/cleanstartup/stack/stencil"
	tailwindpkg "github.com/cleanstartup/stack/tailwind"
)

func (e *BuildEngine) devOutputWatchPaths(outputDir string) []string {
	var paths []string
	tailwindOutput := tailwindpkg.OutputPath(outputDir)
	if _, err := os.Stat(tailwindOutput); err == nil {
		paths = append(paths, tailwindOutput)
	}
	stencilOutput := stencilpkg.OutputPath(outputDir)
	if _, err := os.Stat(stencilOutput); err == nil {
		paths = append(paths, stencilOutput)
	}
	sort.Strings(paths)
	return paths
}

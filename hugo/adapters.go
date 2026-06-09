package hugo

import (
	"github.com/cleanstartup/stack/web"
	tailwindpkg "github.com/cleanstartup/stack/tailwind"
	stencilpkg "github.com/cleanstartup/stack/stencil"
)

type tailwindWorkspaceAdapter struct {
	workspace tailwindpkg.Workspace
}

func (a tailwindWorkspaceAdapter) AssetDir(kind web.AssetKind, id string) string {
	if a.workspace == nil {
		return ""
	}
	return a.workspace.AssetDir(tailwindpkg.AssetKind(kind), id)
}

type tailwindBuildWorkspaceAdapter struct {
	workspace web.AssetWorkspace
}

func (a tailwindBuildWorkspaceAdapter) AssetDir(kind tailwindpkg.AssetKind, id string) string {
	if a.workspace == nil {
		return ""
	}
	return a.workspace.AssetDir(web.AssetKind(kind), id)
}

type stencilWorkspaceAdapter struct {
	workspace interface{ AssetDir(web.AssetKind, string) string }
}

func (a stencilWorkspaceAdapter) AssetDir(kind stencilpkg.AssetKind, id string) string {
	if a.workspace == nil {
		return ""
	}
	return a.workspace.AssetDir(web.AssetKind(kind), id)
}

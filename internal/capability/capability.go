package capability

import (
	"context"

	"github.com/cleanstartup/stack/internal/asset"
	"github.com/cleanstartup/stack/internal/pipeline"
)

type Context struct {
	ProjectDir  string
	Workspace   Workspace
	OutputDir   string
	BuildConfig any
	DevConfig   any
}

type Workspace interface {
	RootDir() string
	OutputDir() string
	AssetDir(asset.AssetKind, string) string
}

type Target interface {
	RegisterCSS(asset.AssetRef)
	RegisterJS(asset.AssetRef)
}

type Capability interface {
	Install(context.Context, Context) error
	Build(context.Context, Context) error
	Dev(context.Context, Context) ([]pipeline.WatchWorker, error)
	Register(Target)
}

type Source interface {
	SourcePaths() []string
	SourceChanged(string) bool
	Rebuild(context.Context, Context) error
}

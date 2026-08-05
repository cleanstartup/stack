package plugin

import (
	"context"

	"github.com/cleanstartup/stack/asset"
	"github.com/cleanstartup/stack/devwatch"
)

// Mode replaces the former BuildConfig any / DevConfig any pass-through.
type Mode string

const (
	ModeBuild Mode = "build"
	ModeDev   Mode = "dev"
)

// Context is the public contract stack hands to every plugin.
type Context struct {
	ProjectDir string      // module/command root
	OutputDir  string      // .assets destination
	Workspace  Workspace   // asset destination paths
	Mode       Mode        // build | dev
	NPM        NPM         // SHARED npm workspace (D3)
	Bin        BinProvider // SHARED binary provisioning (D3)
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

// NPM is the public surface of stack's single shared npm workspace. stack
// owns one *npm.Project and hands it to every plugin as ctx.NPM (avoids
// "duplicate node_modules" per D3).
type NPM interface {
	AddDependency(name, version string)
	AddDevDependency(name, version string)
	RequireBin(name string) // e.g. "tailwindcss", "stencil"
}

// BinProvider is the shared binary-provisioning service: download -> cache
// path -> chmod -> atomic rename. Plugin-specific is only the URL/platform/
// latest-version resolution.
type BinProvider interface {
	// Ensure downloads spec.URL into the shared cache under Name/Version,
	// makes the file executable, and returns the local path. Idempotent.
	Ensure(ctx context.Context, spec BinarySpec) (string, error)
}

type BinarySpec struct {
	Name    string // logical name, e.g. "tailwindcss"
	Version string // resolved, part of the cache path
	URL     string // fully resolved, platform-specific download URL
}

type Capability interface {
	Install(context.Context, Context) error
	Build(context.Context, Context) error
	Dev(context.Context, Context) ([]devwatch.WatchWorker, error)
	Register(Target)
}

type Source interface {
	SourcePaths() []string
	SourceChanged(string) bool
	Rebuild(context.Context, Context) error
}

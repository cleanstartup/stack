package assets

import (
	"path/filepath"

	"github.com/cleanstartup/stack/asset"
	"github.com/cleanstartup/stack/webasset"
)

// Tailwind activates the Tailwind CSS builder for this Target.
type tailwindActivate struct{}

func (tailwindActivate) StackTailwindInclude() {}

func Tailwind() tailwindActivate { return tailwindActivate{} }

// Stencil activates the Stencil web component builder for this Target.
type stencilActivate struct{}

func (stencilActivate) StackStencilInclude() {}

func Stencil() stencilActivate { return stencilActivate{} }

// DirSource declares a module directory whose files are routed to active
// builders by extension at build time (.css → Tailwind, .tsx → Stencil),
// with unrecognised files copied as static assets.
//
// Source paths are resolved lazily: filesystem access only occurs during
// build and dev, never during serve.
type DirSource struct {
	CallerDir string
	RelPath   string
}

// AbsPath returns the absolute path of the source directory.
func (d DirSource) AbsPath() string {
	if filepath.IsAbs(d.RelPath) {
		return filepath.Clean(d.RelPath)
	}
	return filepath.Clean(filepath.Join(d.CallerDir, d.RelPath))
}

// Apply is intentionally a no-op. DirSource is expanded into builder-specific
// parts by the stack based on which Asset Builders are active.
func (DirSource) Apply(_ *webasset.WebApp) {}

// Dir declares relPath (relative to the calling file) as an asset source
// directory. Omit relPath to use the directory of the calling file.
func Dir(relPath ...string) DirSource {
	rel := "."
	if len(relPath) > 0 {
		rel = relPath[0]
	}
	return DirSource{
		CallerDir: asset.CallerDir(1),
		RelPath:   rel,
	}
}

// StaticDirSource declares a directory to be copied verbatim without
// processing by any Asset Builder.
type StaticDirSource struct {
	CallerDir string
	RelPath   string
}

// AbsPath returns the absolute path of the static directory.
func (s StaticDirSource) AbsPath() string {
	if filepath.IsAbs(s.RelPath) {
		return filepath.Clean(s.RelPath)
	}
	return filepath.Clean(filepath.Join(s.CallerDir, s.RelPath))
}

func (s StaticDirSource) Apply(app *webasset.WebApp) {
	app.RegisterFile(webasset.FromDir(s.AbsPath()))
}

// StaticDir declares relPath as a static asset directory. Files are copied
// to the output without processing. Omit relPath to use the calling directory.
func StaticDir(relPath ...string) StaticDirSource {
	rel := "."
	if len(relPath) > 0 {
		rel = relPath[0]
	}
	return StaticDirSource{
		CallerDir: asset.CallerDir(1),
		RelPath:   rel,
	}
}

// Use declares an npm package dependency for this module's asset sources.
func Use(name, version string) webasset.Part {
	return webasset.NPMDependency(name, version)
}

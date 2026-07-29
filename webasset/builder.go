package webasset

import (
	"strings"

	assetpkg "github.com/cleanstartup/stack/asset"
	stencilpkg "github.com/cleanstartup/stack/internal/stencil"
	tailwindpkg "github.com/cleanstartup/stack/internal/tailwind"
	"github.com/cleanstartup/stack/plugin"
)

type npmDep struct {
	name    string
	version string
	dev     bool
}

// NPMDep is a public representation of an npm dependency.
type NPMDep struct {
	Name    string
	Version string
	Dev     bool
}

// DirSourceEntry records a module asset directory registered during Apply,
// used by Install to sync sources to .sources/<namespace>/<relPath>/.
type DirSourceEntry struct {
	Namespace string
	RelPath   string // relative path declared in assets.Dir (e.g. "client", ".")
	AbsPath   string // resolved absolute path on disk
}

// Builder accumulates asset registrations (CSS/JS/Stencil/File/DirSource/npm)
// for a WebApp. Routing lives in way2go's RouteAccumulator instead — Builder
// knows nothing about routes, activities, or mounts.
type Builder struct {
	tailwind   *tailwindpkg.Registry
	stencil    *stencilpkg.Registry
	assets     *AssetRegistry
	npm        []npmDep
	dirSources []DirSourceEntry
}

type manifestTarget struct {
	manifest *AssetManifest
}

func NewBuilder() *Builder {
	return &Builder{
		tailwind: tailwindpkg.NewRegistry(),
		stencil:  stencilpkg.NewRegistry(),
		assets:   NewAssetRegistry(),
	}
}

func (b *Builder) RegisterDirSource(namespace, relPath, absPath string) {
	if b == nil {
		return
	}
	b.dirSources = append(b.dirSources, DirSourceEntry{
		Namespace: namespace,
		RelPath:   relPath,
		AbsPath:   absPath,
	})
}

func (b *Builder) DirSources() []DirSourceEntry {
	if b == nil {
		return nil
	}
	return append([]DirSourceEntry{}, b.dirSources...)
}

// NPMDeps returns all registered npm dependencies as public NPMDep values.
func (b *Builder) NPMDeps() []NPMDep {
	if b == nil {
		return nil
	}
	out := make([]NPMDep, 0, len(b.npm))
	for _, dep := range b.npm {
		out = append(out, NPMDep{Name: dep.name, Version: dep.version, Dev: dep.dev})
	}
	return out
}

func (b *Builder) AddNPMDependency(name, version string, dev bool) {
	if b == nil || strings.TrimSpace(name) == "" {
		return
	}
	b.npm = append(b.npm, npmDep{name: strings.TrimSpace(name), version: strings.TrimSpace(version), dev: dev})
}

func (b *Builder) Assets() *AssetRegistry {
	if b == nil {
		return nil
	}
	return b.assets
}

func (b *Builder) Add(kind AssetKind, src AssetSource) AssetRef {
	if b == nil {
		return AssetRef{}
	}
	if b.assets == nil {
		return AssetRef{}
	}
	return b.assets.Add(kind, src)
}

func (b *Builder) CSS(src AssetSource) AssetRef {
	if b == nil || b.assets == nil {
		return AssetRef{}
	}
	return b.Add(AssetKindCSS, src)
}

func (b *Builder) TailwindCSS(src AssetSource) AssetRef {
	if b == nil || b.tailwind == nil {
		return AssetRef{}
	}
	ref := b.tailwind.AddInput(wrapTailwindSource(src))
	return AssetRef{Kind: AssetKind(ref.Kind), ID: ref.ID, Files: append([]string{}, ref.Files...)}
}

func (b *Builder) Stencil(src AssetSource) AssetRef {
	if b == nil || b.stencil == nil {
		return AssetRef{}
	}
	ref := b.stencil.AddInput(wrapStencilSource(src))
	return AssetRef{Kind: AssetKind(ref.Kind), ID: ref.ID, Files: append([]string{}, ref.Files...)}
}

func (b *Builder) JS(src AssetSource) AssetRef   { return b.Add(AssetKindJS, src) }
func (b *Builder) File(src AssetSource) AssetRef { return b.Add(AssetKindFile, src) }

func (b *Builder) TailwindScan(paths ...string) {
	if b == nil || b.tailwind == nil {
		return
	}
	b.tailwind.AddScan(paths...)
}

func (b *Builder) StencilScan(paths ...string) {
	if b == nil || b.stencil == nil {
		return
	}
	b.stencil.AddScan(paths...)
}

func (b *Builder) Styles() *tailwindpkg.Registry {
	if b == nil {
		return nil
	}
	return b.tailwind
}

func (b *Builder) Components() *stencilpkg.Registry {
	if b == nil {
		return nil
	}
	return b.stencil
}

func (b *Builder) Manifest() AssetManifest {
	if b == nil {
		return AssetManifest{}
	}
	manifest := AssetManifest{}
	if b.assets != nil {
		manifest = b.assets.Manifest()
	}
	target := &manifestTarget{manifest: &manifest}
	for _, capability := range b.registrationCapabilities() {
		if capability == nil {
			continue
		}
		capability.Register(target)
	}
	return manifest
}

func (t *manifestTarget) RegisterCSS(ref AssetRef) {
	if t == nil || t.manifest == nil || strings.TrimSpace(ref.ID) == "" {
		return
	}
	t.manifest.Styles = append(t.manifest.Styles, ref)
}

func (t *manifestTarget) RegisterJS(ref AssetRef) {
	if t == nil || t.manifest == nil || strings.TrimSpace(ref.ID) == "" {
		return
	}
	t.manifest.Scripts = append(t.manifest.Scripts, ref)
}

func (b *Builder) registrationCapabilities() []plugin.Capability {
	if b == nil {
		return nil
	}
	var caps []plugin.Capability
	if b.tailwind != nil && len(b.tailwind.Inputs()) > 0 {
		caps = append(caps, tailwindpkg.NewCapability(b.tailwind, nil))
	}
	if b.stencil != nil && len(b.stencil.Inputs()) > 0 {
		caps = append(caps, stencilpkg.NewCapability(b.stencil, nil))
	}
	return caps
}

type tailwindWorkspaceAdapter struct {
	workspace tailwindpkg.Workspace
}

func (a tailwindWorkspaceAdapter) AssetDir(kind AssetKind, id string) string {
	if a.workspace == nil {
		return ""
	}
	return a.workspace.AssetDir(tailwindpkg.AssetKind(kind), id)
}

type tailwindBuildWorkspaceAdapter struct {
	workspace AssetWorkspace
}

func (a tailwindBuildWorkspaceAdapter) AssetDir(kind tailwindpkg.AssetKind, id string) string {
	if a.workspace == nil {
		return ""
	}
	return a.workspace.AssetDir(AssetKind(kind), id)
}

type tailwindSourceAdapter struct {
	source AssetSource
}

func (a tailwindSourceAdapter) ID() string {
	if a.source == nil {
		return ""
	}
	return a.source.ID()
}

func (a tailwindSourceAdapter) Materialize(ws tailwindpkg.Workspace, kind tailwindpkg.AssetKind) ([]string, error) {
	if a.source == nil {
		return nil, nil
	}
	return a.source.Materialize(tailwindWorkspaceAdapter{workspace: ws}, AssetKind(kind))
}

func (a tailwindSourceAdapter) SourcePaths() []string {
	if a.source == nil {
		return nil
	}
	paths, err := assetpkg.SourcePaths(a.source)
	if err != nil {
		return nil
	}
	return paths
}

func (a tailwindSourceAdapter) SourceFiles() []string {
	if a.source == nil {
		return nil
	}
	if p, ok := a.source.(assetpkg.SourceFileProvider); ok {
		return p.SourceFiles()
	}
	return nil
}

func wrapTailwindSource(src AssetSource) tailwindpkg.Source {
	if src == nil {
		return nil
	}
	return tailwindSourceAdapter{source: src}
}

type stencilBuildWorkspaceAdapter struct {
	workspace stencilpkg.Workspace
}

func (a stencilBuildWorkspaceAdapter) AssetDir(kind AssetKind, id string) string {
	if a.workspace == nil {
		return ""
	}
	return a.workspace.AssetDir(stencilpkg.AssetKind(kind), id)
}

type stencilWorkspaceAdapter struct {
	workspace AssetWorkspace
}

func (a stencilWorkspaceAdapter) AssetDir(kind stencilpkg.AssetKind, id string) string {
	if a.workspace == nil {
		return ""
	}
	return a.workspace.AssetDir(AssetKind(kind), id)
}

type stencilSourceAdapter struct {
	source AssetSource
}

func (a stencilSourceAdapter) ID() string {
	if a.source == nil {
		return ""
	}
	return a.source.ID()
}

func (a stencilSourceAdapter) Materialize(ws stencilpkg.Workspace, kind stencilpkg.AssetKind) ([]string, error) {
	if a.source == nil {
		return nil, nil
	}
	return a.source.Materialize(stencilBuildWorkspaceAdapter{workspace: ws}, AssetKind(kind))
}

func (a stencilSourceAdapter) SourcePaths() []string {
	if a.source == nil {
		return nil
	}
	paths, err := assetpkg.SourcePaths(a.source)
	if err != nil {
		return nil
	}
	return paths
}

func wrapStencilSource(src AssetSource) stencilpkg.Source {
	if src == nil {
		return nil
	}
	return stencilSourceAdapter{source: src}
}

package web

import (
	"net/http"
	"strings"

	"github.com/cleanstartup/stack/activity"
	assetpkg "github.com/cleanstartup/stack/asset"
	stencilpkg "github.com/cleanstartup/stack/stencil"
	tailwindpkg "github.com/cleanstartup/stack/tailwind"
)

type routeRegistration interface {
	register(*Registry)
}

type activityRouteRegistration[P any, I any] struct {
	def      *activity.Definition[P, I]
	resolver activity.Resolver[P, I]
}

func (r activityRouteRegistration[P, I]) register(reg *Registry) {
	Register(reg, r.def, r.resolver)
}

type webActivityRouteRegistration[C any] struct {
	activity *WebActivity[C]
}

func (r webActivityRouteRegistration[C]) register(reg *Registry) {
	RegisterWebActivity(reg, r.activity)
}

type mountRouteRegistration struct {
	path    string
	handler http.Handler
}

func (r mountRouteRegistration) register(reg *Registry) {
	if reg == nil || r.handler == nil {
		return
	}
	reg.Mount(r.path, r.handler)
}

type Builder struct {
	routes   []routeRegistration
	mounts   []mountRouteRegistration
	tailwind *tailwindpkg.Registry
	stencil  *stencilpkg.Registry
	assets   *AssetRegistry
}

type manifestTarget struct {
	manifest *AssetManifest
}

func NewBuilder() *Builder {
	return &Builder{
		routes:   []routeRegistration{},
		mounts:   []mountRouteRegistration{},
		tailwind: tailwindpkg.NewRegistry(),
		stencil:  stencilpkg.NewRegistry(),
		assets:   NewAssetRegistry(),
	}
}

func AddActivity[P any, I any](b *Builder, def *activity.Definition[P, I], resolver activity.Resolver[P, I]) {
	if b == nil || def == nil || resolver == nil {
		return
	}
	b.routes = append(b.routes, activityRouteRegistration[P, I]{
		def:      def,
		resolver: resolver,
	})
}

func AddWebActivity[C any](b *Builder, activity *WebActivity[C]) {
	if b == nil || activity == nil {
		return
	}
	b.routes = append(b.routes, webActivityRouteRegistration[C]{activity: activity})
}

func AddMount(b *Builder, path string, handler http.Handler) {
	if b == nil || handler == nil {
		return
	}
	b.mounts = append(b.mounts, mountRouteRegistration{path: path, handler: handler})
}

func (b *Builder) AddMount(path string, handler http.Handler) {
	AddMount(b, path, handler)
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

func (b *Builder) registerRoutes(reg *Registry) {
	if b == nil || reg == nil {
		return
	}
	for _, route := range b.routes {
		if route == nil {
			continue
		}
		route.register(reg)
	}
	for _, mount := range b.mounts {
		if mount.handler == nil {
			continue
		}
		mount.register(reg)
	}
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

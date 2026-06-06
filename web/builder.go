package web

import "github.com/cleanstartup/way2go/activity"

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

type Builder struct {
	routes   []routeRegistration
	tailwind *TailwindRegistry
	assets   *AssetRegistry
}

func NewBuilder() *Builder {
	return &Builder{
		routes:   []routeRegistration{},
		tailwind: NewTailwindRegistry(),
		assets:   NewAssetRegistry(),
	}
}

func (b *Builder) Apply(mods ...Module) {
	for _, module := range mods {
		if module == nil {
			continue
		}
		module.Apply(b)
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
	return b.tailwind.AddInput(src)
}

func (b *Builder) JS(src AssetSource) AssetRef   { return b.Add(AssetKindJS, src) }
func (b *Builder) File(src AssetSource) AssetRef { return b.Add(AssetKindFile, src) }

func (b *Builder) TailwindScan(paths ...string) {
	if b == nil || b.tailwind == nil {
		return
	}
	b.tailwind.AddScan(paths...)
}

func (b *Builder) Styles() *TailwindRegistry {
	if b == nil {
		return nil
	}
	return b.tailwind
}

func (b *Builder) Manifest() AssetManifest {
	if b == nil {
		return AssetManifest{}
	}
	manifest := AssetManifest{}
	if b.assets != nil {
		manifest = b.assets.Manifest()
	}
	if b.tailwind != nil && len(b.tailwind.Inputs()) > 0 {
		manifest.Styles = append(manifest.Styles, b.tailwind.BundleRef())
	}
	return manifest
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
}

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
	routes []routeRegistration
	styles *StyleRegistry
	assets *AssetRegistry
}

func NewBuilder() *Builder {
	return &Builder{
		routes: []routeRegistration{},
		styles: NewStyleRegistry(),
		assets: NewAssetRegistry(),
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
	switch kind {
	case AssetKindCSS:
		if b.styles == nil {
			return AssetRef{}
		}
		return b.styles.AddCSS(src)
	default:
		if b.assets == nil {
			return AssetRef{}
		}
		b.assets.Add(kind, src)
		if src == nil {
			return AssetRef{}
		}
		ref := AssetRef{Kind: kind, ID: src.ID()}
		if namer, ok := src.(AssetNamer); ok {
			ref.Files = append([]string{}, namer.AssetNames()...)
		}
		return ref
	}
}

func (b *Builder) CSS(src AssetSource) AssetRef {
	if b == nil || b.styles == nil {
		return AssetRef{}
	}
	return b.styles.AddCSS(src)
}

func (b *Builder) TailwindCSS(src AssetSource) AssetRef { return b.CSS(src) }

func (b *Builder) JS(src AssetSource) AssetRef   { return b.Add(AssetKindJS, src) }
func (b *Builder) File(src AssetSource) AssetRef { return b.Add(AssetKindFile, src) }

func (b *Builder) TailwindScan(paths ...string) {
	if b == nil || b.styles == nil {
		return
	}
	b.styles.AddScan(paths...)
}

func (b *Builder) Styles() *StyleRegistry {
	if b == nil {
		return nil
	}
	return b.styles
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

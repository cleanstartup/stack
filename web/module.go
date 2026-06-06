package web

import "github.com/cleanstartup/way2go/activity"

type Contributor interface {
	Apply(*Builder)
}

type contributorFunc func(*Builder)

func (f contributorFunc) Apply(b *Builder) {
	if f == nil || b == nil {
		return
	}
	f(b)
}

type ModulePart interface {
	apply(*Builder)
}

type modulePartFunc func(*Builder)

func (f modulePartFunc) apply(b *Builder) {
	if f == nil || b == nil {
		return
	}
	f(b)
}

func Module(parts ...ModulePart) Contributor {
	return contributorFunc(func(b *Builder) {
		for _, part := range parts {
			if part == nil {
				continue
			}
			part.apply(b)
		}
	})
}

func Include(mods ...Contributor) ModulePart {
	return modulePartFunc(func(b *Builder) {
		for _, mod := range mods {
			if mod == nil {
				continue
			}
			mod.Apply(b)
		}
	})
}

func BindActivity[P any, I any](def *activity.Definition[P, I], resolver activity.Resolver[P, I]) ModulePart {
	return modulePartFunc(func(b *Builder) {
		AddActivity(b, def, resolver)
	})
}

func CSS(src AssetSource) ModulePart {
	return modulePartFunc(func(b *Builder) {
		b.CSS(src)
	})
}

func TailwindCSS(src AssetSource) ModulePart {
	return modulePartFunc(func(b *Builder) {
		b.TailwindCSS(src)
	})
}

func JS(src AssetSource) ModulePart {
	return modulePartFunc(func(b *Builder) {
		b.JS(src)
	})
}

func File(src AssetSource) ModulePart {
	return modulePartFunc(func(b *Builder) {
		b.File(src)
	})
}

func TailwindScan(paths ...string) ModulePart {
	return modulePartFunc(func(b *Builder) {
		b.TailwindScan(paths...)
	})
}

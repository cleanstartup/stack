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

func Compose(contribs ...Contributor) Contributor {
	return contributorFunc(func(b *Builder) {
		for _, contrib := range contribs {
			if contrib == nil {
				continue
			}
			contrib.Apply(b)
		}
	})
}

func Module(contribs ...Contributor) Contributor { return Compose(contribs...) }

func BindActivity[P any, I any](def *activity.Definition[P, I], resolver activity.Resolver[P, I]) Contributor {
	return contributorFunc(func(b *Builder) { AddActivity(b, def, resolver) })
}

func CSS(src AssetSource) Contributor {
	return contributorFunc(func(b *Builder) { b.CSS(src) })
}

func TailwindCSS(src AssetSource) Contributor {
	return contributorFunc(func(b *Builder) { b.TailwindCSS(src) })
}

func JS(src AssetSource) Contributor {
	return contributorFunc(func(b *Builder) { b.JS(src) })
}

func File(src AssetSource) Contributor {
	return contributorFunc(func(b *Builder) { b.File(src) })
}

func TailwindScan(paths ...string) Contributor {
	return contributorFunc(func(b *Builder) { b.TailwindScan(paths...) })
}

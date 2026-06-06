package web

type Module interface {
	Apply(*Builder)
}

type ModuleFunc func(*Builder)

func (f ModuleFunc) Apply(b *Builder) {
	if f == nil || b == nil {
		return
	}
	f(b)
}

type moduleGroup struct {
	modules []Module
}

func (g moduleGroup) Apply(b *Builder) {
	for _, module := range g.modules {
		if module == nil {
			continue
		}
		module.Apply(b)
	}
}

func Bind(mods ...Module) Module {
	return moduleGroup{modules: mods}
}

func Compose(mods ...Module) Module {
	return Bind(mods...)
}

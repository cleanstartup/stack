package web

import (
	"net/http"
	"strings"
)

// Registrar is the target that runtime Parts (routes, activities, mounts)
// bind to. It has no knowledge of Builder or asset registration — those stay
// in stack, bound to their own Part flavor.
type Registrar interface {
	AddActivity(a RouteActivity)
	AddMount(path string, handler http.Handler)
}

// RouteActivity hides the type parameter of WebActivity[C] so heterogeneous
// activities can share one Registrar.
type RouteActivity interface {
	routeRegister(r *Registry)
}

func (a *WebActivity[C]) routeRegister(r *Registry) { RegisterWebActivity(r, a) }

// Part is the composition primitive for runtime-only contributions (routes,
// activities, mounts). Asset contributions use stack's own Part
// (Apply(*WebApp)) instead.
type Part interface {
	Apply(Registrar)
}

type Contributor = Part

type partFunc func(Registrar)

func (f partFunc) Apply(reg Registrar) {
	if f == nil || reg == nil {
		return
	}
	f(reg)
}

func Compose(parts ...Part) Part {
	return partFunc(func(reg Registrar) {
		for _, part := range parts {
			if part == nil {
				continue
			}
			part.Apply(reg)
		}
	})
}

func Mount(path string, handler http.Handler) Part {
	return partFunc(func(reg Registrar) {
		if reg == nil || handler == nil {
			return
		}
		if strings.TrimSpace(path) == "" {
			path = "/"
		}
		reg.AddMount(path, handler)
	})
}

type mountEntry struct {
	path    string
	handler http.Handler
}

// RouteAccumulator is the concrete Registrar: it collects registrations at
// composition time and only builds the actual router when Handler() is
// called. Replaces the routing half of the old Builder.
type RouteAccumulator struct {
	routes []RouteActivity
	mounts []mountEntry
}

func NewRouteAccumulator() *RouteAccumulator {
	return &RouteAccumulator{}
}

func (r *RouteAccumulator) AddActivity(a RouteActivity) {
	if r == nil || a == nil {
		return
	}
	r.routes = append(r.routes, a)
}

func (r *RouteAccumulator) AddMount(path string, handler http.Handler) {
	if r == nil || handler == nil {
		return
	}
	r.mounts = append(r.mounts, mountEntry{path: path, handler: handler})
}

// Handler replays accumulated registrations into a fresh *Registry.
func (r *RouteAccumulator) Handler() *Registry {
	reg := NewRegistry()
	if r == nil {
		return reg
	}
	for _, a := range r.routes {
		if a == nil {
			continue
		}
		a.routeRegister(reg)
	}
	for _, m := range r.mounts {
		if m.handler == nil {
			continue
		}
		reg.Mount(m.path, m.handler)
	}
	return reg
}

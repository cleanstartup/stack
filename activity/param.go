package activity

import "github.com/cleanstartup/stack/param"

// Param reads a typed parameter from ctx. The ctx must implement param.Resolver
// (CLI and HTTP transports do). If the value is missing and the param has a default,
// the default is returned. If the param is required and missing, Param panics.
func Param[T any](ctx Context, p param.Param[T]) T {
	if r, ok := ctx.(param.Resolver); ok {
		if raw, found := r.Resolve(p.Names()); found {
			if v, err := param.ParseValue(p, raw); err == nil {
				return v
			}
		}
	}
	if p.IsRequired() {
		panic("param: required param " + p.Names()[0] + " is missing")
	}
	return p.Default()
}

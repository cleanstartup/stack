package activity

import "github.com/cleanstartup/stack/way2go/param"

// Param reads a typed parameter from ctx. The ctx must implement param.Resolver
// (CLI and HTTP transports do). If the value is missing and the param has a prompt
// text set, and the transport implements param.Prompter, the user is prompted
// interactively. If the param is required and still missing, Param panics.
func Param[T any](ctx Context, p param.Param[T]) T {
	raw := ""
	found := false

	if r, ok := ctx.(param.Resolver); ok {
		raw, found = r.Resolve(p.Names())
	}

	if !found && p.PromptText() != "" {
		if pr, ok := ctx.(param.Prompter); ok {
			if prompted, err := pr.Prompt(p.PromptText()); err == nil && prompted != "" {
				raw = prompted
				found = true
			}
		}
	}

	if found {
		if v, err := param.ParseValue(p, raw); err == nil {
			return v
		}
	}

	if p.IsRequired() {
		panic("param: required param " + p.Names()[0] + " is missing")
	}
	return p.Default()
}

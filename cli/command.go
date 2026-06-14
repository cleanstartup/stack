package cli

// Command is the common interface for anything that can be registered into a Registry.
// Both *CliActivity[C] and Groups (returned by Group()) implement it.
type Command interface {
	registerInto(r *Registry)
}

// Group bundles commands under a named subcommand prefix.
// Example: cli.Group("setup", checkCmd, encryptionCmd)
func Group(name string, cmds ...Command) Command {
	return group{name: name, cmds: cmds}
}

// BuildRegistry constructs a Registry from a flat list of Commands.
// Commands that are Groups create sub-registries; plain activities are registered directly.
func BuildRegistry(cmds ...Command) *Registry {
	r := NewRegistry()
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}
		cmd.registerInto(r)
	}
	return r
}

type group struct {
	name string
	cmds []Command
}

func (g group) registerInto(r *Registry) {
	if g.name == "" || r == nil {
		return
	}
	sub := r.Group(g.name)
	for _, cmd := range g.cmds {
		if cmd == nil {
			continue
		}
		cmd.registerInto(sub)
	}
}

// registerInto makes *CliActivity[C] implement Command.
func (a *CliActivity[C]) registerInto(r *Registry) {
	RegisterActivity(r, a)
}

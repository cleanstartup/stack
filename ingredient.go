package stack

// Ingredient is anything composable into a target: a Module, a
// plugin.Builder-typed Producer, or the result of Mount(...). Modeled as a
// plain alias rather than a marker interface: arbitrary third-party
// Producer types (tailwind, hugo, a future lit plugin) can never be
// retrofitted with an unexported stack-owned marker method, so dispatch is
// a runtime type-switch on nature — the same style Mount itself uses below.
type Ingredient = any

// mountEdge is the private result of Mount(subject, at): the subject is one
// of Producer | Module | http.Handler, carrying the edge's explicit wiring
// parameter.
type mountEdge struct {
	subject any
	at      string
}

// Mount is the overloaded, subject-first edge combinator (D-M) — one verb,
// three natures, dispatched on the argument's type by the composition
// orchestrator:
//
//	Mount(Hugo(), "/docs")         // producer edge: build+serve an asset tree under a prefix
//	Mount(docs.Module(), "/docs")  // module edge: routes+assets under a prefix, namespaced
//	Mount(authRouter, "/auth")     // runtime route: raw http.Handler
//
// This replaces the old path-first stack.Mount(path, handler) — a small,
// deliberate breaking change (see PRD §6): Go has no overloading, so the
// old and new signatures can't coexist under one name, and nothing in this
// module or its tests called the old one.
func Mount(subject any, at string) Ingredient {
	return mountEdge{subject: subject, at: at}
}

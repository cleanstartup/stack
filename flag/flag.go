package flag

import (
	"sync"

	"github.com/theway2go/way2go/activity"
)

// Resolver is implemented by providers (e.g. PostHog) to evaluate a flag
// for the caller behind ctx. Mirrors param.Resolver: the stack defines the
// interface, an artifact-level provider implements it.
type Resolver interface {
	Resolve(ctx activity.Context, name string) (enabled, found bool)
}

// Flag is a declarative, named feature flag with a fallback default.
type Flag struct {
	name string
	def  bool
}

// Option configures a Flag.
type Option func(*Flag)

// WithDefault sets the fallback value used when no provider is registered or
// the provider has no value for this flag.
func WithDefault(v bool) Option {
	return func(f *Flag) { f.def = v }
}

// Bool declares a boolean feature flag.
func Bool(name string, opts ...Option) Flag {
	f := Flag{name: name}
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

// Name returns the flag's name.
func (f Flag) Name() string { return f.name }

// Default returns the flag's fallback value.
func (f Flag) Default() bool { return f.def }

var (
	mu       sync.RWMutex
	resolver Resolver
)

// Use registers the active flag provider. Call once at startup.
func Use(r Resolver) {
	mu.Lock()
	defer mu.Unlock()
	resolver = r
}

// Enabled evaluates f for the caller behind ctx. Falls back to f.Default()
// when no provider is registered or the provider has no value for this name —
// flags degrade safely without a provider wired up (local dev, tests, CLI).
func Enabled(ctx activity.Context, f Flag) bool {
	mu.RLock()
	r := resolver
	mu.RUnlock()
	if r == nil {
		return f.Default()
	}
	if v, found := r.Resolve(ctx, f.Name()); found {
		return v
	}
	return f.Default()
}

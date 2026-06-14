package param

import (
	"fmt"
	"strconv"
	"strings"
)

// Resolver is implemented by transports (CLI, HTTP, ...) to look up raw param values.
// It receives all names for the param (primary name first, then aliases) and returns
// the first matching raw string value.
type Resolver interface {
	Resolve(names []string) (string, bool)
}

// Prompter is implemented by transports that support interactive prompting.
// If the resolved value is empty and a prompt is set, the transport is asked
// to prompt the user for a value.
type Prompter interface {
	Prompt(text string) (string, error)
}

// Param[T] is a typed, named parameter descriptor. Create instances with String, Int, Bool.
type Param[T any] struct {
	names      []string
	required   bool
	hasDefault bool
	def        T
	prompt     string
	parse      func(string) (T, error)
}

// Names returns all names for this param (primary name first, then aliases).
func (p Param[T]) Names() []string { return p.names }

// Default returns the default value for this param.
func (p Param[T]) Default() T { return p.def }

// IsRequired reports whether the param is required.
func (p Param[T]) IsRequired() bool { return p.required }

// PromptText returns the interactive prompt text, or empty if prompting is disabled.
func (p Param[T]) PromptText() string { return p.prompt }

// Option configures a Param.
type Option[T any] func(*Param[T])

// Required marks the param as required. activity.Param panics if the value is missing.
func Required[T any]() Option[T] {
	return func(p *Param[T]) { p.required = true }
}

// WithDefault sets a default value used when the param is not provided.
func WithDefault[T any](v T) Option[T] {
	return func(p *Param[T]) {
		p.def = v
		p.hasDefault = true
	}
}

// WithPrompt sets a text shown to the user when the param is not supplied and
// the transport supports interactive prompting (e.g. CLI).
func WithPrompt[T any](text string) Option[T] {
	return func(p *Param[T]) { p.prompt = text }
}

// WithAlias adds additional names that can be used to supply this param.
func WithAlias[T any](aliases ...string) Option[T] {
	return func(p *Param[T]) {
		for _, a := range aliases {
			a = strings.TrimSpace(a)
			if a != "" {
				p.names = append(p.names, a)
			}
		}
	}
}

func String(name string, opts ...Option[string]) Param[string] {
	return newParam(name, func(s string) (string, error) { return s, nil }, opts...)
}

func Int(name string, opts ...Option[int]) Param[int] {
	return newParam(name, func(s string) (int, error) {
		v, err := strconv.Atoi(s)
		return v, err
	}, opts...)
}

func Bool(name string, opts ...Option[bool]) Param[bool] {
	return newParam(name, func(s string) (bool, error) {
		v, err := strconv.ParseBool(s)
		return v, err
	}, opts...)
}

func newParam[T any](name string, parse func(string) (T, error), opts ...Option[T]) Param[T] {
	name = strings.TrimSpace(name)
	if name == "" {
		panic("param: name must not be empty")
	}
	p := Param[T]{names: []string{name}, parse: parse}
	for _, opt := range opts {
		opt(&p)
	}
	return p
}

// ParseValue converts a raw string to T using the param's parser.
func ParseValue[T any](p Param[T], raw string) (T, error) {
	if p.parse == nil {
		var zero T
		return zero, fmt.Errorf("param %q: no parser defined", p.names[0])
	}
	return p.parse(raw)
}

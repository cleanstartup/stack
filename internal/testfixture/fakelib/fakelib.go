// Package fakelib is a test-only fixture standing in for CUP-27's
// cleanstartup/ui: a Module meant to be composed as an Ingredient, whose
// rootDir() (captured from *this file's own directory* by stack.Bundle's
// webasset.CallerDir) is deliberately a different directory than whatever
// test file composes it — the same relationship a real app's own module and
// ui.Module() have (ui.Module() lives in a different repo/checkout entirely).
// Exists so an external stack_test-package test can prove
// StageContext.ProjectDir defaults from the WebApp's own module, not from
// any ingredient Module's rootDir, without needing a second real repo.
//
// Lives under internal/testfixture (not internal/lit or the stack package
// root) and is only ever imported from stack_test (external test package):
// it imports github.com/cleanstartup/stack itself (to construct a real
// stack.Module via the only exported constructor), so an internal
// package-stack test file importing this package would be a genuine import
// cycle (stack -> fakelib -> stack) — confirmed by trying exactly that and
// hitting `go vet`'s "import cycle not allowed in test" before settling on
// this external-test-package shape.
package fakelib

import (
	"path/filepath"
	"runtime"

	"github.com/cleanstartup/stack"
)

// Module mirrors the shape of cleanstartup/ui's ui.Module(): a bare
// stack.Bundle with no parts of its own beyond a namespace, meant to be
// composed as an Ingredient (`stack.WebApp(appModule, fakelib.Module())`),
// never as a WebApp's own module.
func Module() stack.Module {
	return stack.Bundle("fakelib-ingredient-test")
}

// Dir returns this package's own source directory — the same directory
// Module()'s underlying stack.Bundle call captures as its rootDir() (an
// unexported method a stack_test-package caller can't call directly), so a
// test can assert against it without needing access to stack-internal
// state.
func Dir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Dir(file)
}

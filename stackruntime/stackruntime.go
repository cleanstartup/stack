// Package stackruntime holds the single process-global signal a release
// binary uses to tell (*stack.WebAppTarget).Main it was built with `build`
// (CUP-30): a generated, `//go:build release`-tagged file in the consumer's
// command dir calls Register from its init(), embedding the pre-built asset
// tree. Without that file compiled in (the default, tag-free build), nothing
// ever calls Register, Registered reports false, and Main runs its normal
// dev registry (npm install, in-process build, serve from disk) instead.
//
// The *presence* of the tagged file is the mode signal, not the tag itself —
// Main never inspects build tags; it only ever calls Registered.
package stackruntime

import "io/fs"

var registered fs.FS

// Register records assets as the embedded asset tree for this process.
// Called only from a `//go:build release`-tagged generated file's init().
func Register(assets fs.FS) {
	registered = assets
}

// Registered reports the embedded asset tree registered by a release build,
// if any.
func Registered() (fs.FS, bool) {
	return registered, registered != nil
}

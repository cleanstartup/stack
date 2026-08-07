package lit

import "github.com/cleanstartup/stack/plugin"

// AddNPMDependencies declares the npm dependency a real Lit component under
// a discovered entry's source tree would need at bundle time — e.g.
// `import { LitElement, html } from "lit"`. Mirrors tailwind's
// AddNPMDependencies (internal/tailwind/npm.go) in shape and call site
// (Producer.Build calls this once real work is about to happen), but the
// parallel is not yet load-bearing the way tailwind's is.
//
// Honest limitation, updated by CUP-27: declaring this only records intent
// on the shared plugin.NPM registry (D3) — it does not itself turn "lit"
// into an actual node_modules entry on disk. That half of the gap is now
// closed differently: StageContext (plugin/stage.go) gained a ProjectDir
// field, threaded into esbuild's AbsWorkingDir/NodePaths (build.go), so a
// bare `import ... from "lit"` resolves against a real, already-installed
// node_modules tree rooted at ProjectDir. What's still missing is the other
// half — nothing in this repo runs `npm install` to populate that
// node_modules tree in the first place; a consuming app (e.g. CUP-27's
// cleanstartup/ui, via its ui.Module()) must install "lit" (and any other
// real npm package it needs, e.g. Web Awesome) into StageContext.ProjectDir
// itself today. See internal/lit/build.go's Build doc comment and
// TestBuildResolvesBareSpecifierViaNodePaths for the resolution mechanism
// this now relies on.
func AddNPMDependencies(project plugin.NPM) {
	if project == nil {
		return
	}
	project.AddDependency("lit", "^3.2.0")
}

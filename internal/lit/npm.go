package lit

import "github.com/cleanstartup/stack/plugin"

// AddNPMDependencies declares the npm dependency a real Lit component under
// a discovered entry's source tree would need at bundle time — e.g.
// `import { LitElement, html } from "lit"`. Mirrors tailwind's
// AddNPMDependencies (internal/tailwind/npm.go) in shape and call site
// (Producer.Build calls this once real work is about to happen), but the
// parallel is not yet load-bearing the way tailwind's is.
//
// Honest limitation: declaring this only records intent on the shared
// plugin.NPM registry (D3) — nothing today turns it into an actual
// node_modules tree on disk that esbuild's api.Build can resolve a bare
// specifier against. Unlike tailwind (which shells out to a real npm
// project directory it controls via Config.ProjectDir), esbuild's Go API
// runs in-process against StageContext alone, and StageContext
// (plugin/stage.go) has no ProjectDir/workspace-root field a Producer could
// point esbuild's AbsWorkingDir/NodePaths at. That's a genuine contract gap
// this task cannot close unilaterally (it's CUP-26-contract-shaped, not
// CUP-25-producer-shaped) — today, Producer.Build only builds entries whose
// imports resolve via plain relative paths (this package's own tests; a
// real CUP-27 ui.Module() component library, once it exists, is what will
// force this gap closed).
func AddNPMDependencies(project plugin.NPM) {
	if project == nil {
		return
	}
	project.AddDependency("lit", "^3.2.0")
}

package stack

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cleanstartup/stack/plugin"
	"github.com/cleanstartup/stack/webasset"
)

// flattenIngredients walks ingredients in wiring order, gathering
// plugin.Builder Contributions and applying the D-J dedup/order rule: a
// repeated Asset.Path from a different producer edge is a composition bug,
// so it errors rather than silently last-wins (no cascade/override
// semantics exist elsewhere in this design to make a silent overwrite
// legitimate — same fail-fast posture as validateNamespaces below).
//
// app is non-nil only for a WebApp target, which has a live registrar to
// route Module and http.Handler mounts into (D-M: "Module edge: routes+
// assets under a prefix"). Website/CLI targets have no such thing — for
// those, app is nil, a bare or mounted Module is accepted but contributes
// nothing (Module→Sources beyond CUP-21's tailwind-content case isn't
// modeled yet, deferred to CUP-25/27 per the design note's D-N), and a
// mounted http.Handler is rejected as meaningless without a live server.
func flattenIngredients(ctx context.Context, stageCtx plugin.StageContext, app *webasset.WebApp, ingredients []Ingredient) ([]plugin.Contribution, error) {
	var contributions []plugin.Contribution
	seenPaths := map[string]string{} // Asset.Path -> describing mount point of first contributor

	addContribution := func(assets []plugin.Asset, mount string) error {
		if len(assets) == 0 {
			return nil
		}
		for _, a := range assets {
			if prev, ok := seenPaths[a.Path]; ok {
				return fmt.Errorf("stack: duplicate asset path %q (contributed via mount %q, again via %q)", a.Path, prev, mount)
			}
			seenPaths[a.Path] = mount
		}
		contributions = append(contributions, plugin.Contribution{Assets: assets, Mount: mount})
		return nil
	}

	for _, ingredient := range ingredients {
		switch v := ingredient.(type) {
		case Module:
			if app != nil {
				app.Apply(v)
			}
			// Website/CLI (app == nil): accepted, no-op — see doc comment.
		case plugin.Builder:
			assets, err := v.Build(ctx, stageCtx)
			if err != nil {
				return nil, err
			}
			if err := addContribution(assets, ""); err != nil {
				return nil, err
			}
		case mountEdge:
			if err := applyMountEdge(ctx, stageCtx, app, v, addContribution); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("stack: unsupported ingredient type %T", ingredient)
		}
	}
	return contributions, nil
}

func applyMountEdge(ctx context.Context, stageCtx plugin.StageContext, app *webasset.WebApp, edge mountEdge, addContribution func([]plugin.Asset, string) error) error {
	switch subject := edge.subject.(type) {
	case plugin.Builder:
		assets, err := subject.Build(ctx, stageCtx)
		if err != nil {
			return err
		}
		return addContribution(assets, edge.at)
	case Module:
		if app == nil {
			return nil // Website/CLI: accepted, no-op — see flattenIngredients doc comment.
		}
		// D-M's Module-edge Mount ("routes+assets under a prefix, namespaced")
		// isn't implemented: mounting the sub-app's handler as-is has no
		// prefix-strip (routes under edge.at 404 against the sub-router,
		// which only knows its own unprefixed paths) and drops the
		// sub-module's asset contributions entirely — "+ Assets" would be a
		// lie. Fenced with a loud error rather than silently shipping a
		// half-working nature. Not CUP-21's job (that task scoped narrowly to
		// tailwind's app-level stage, confirmed against the PRD's task
		// breakdown table) — still unscheduled; pick up whenever a task
		// actually needs namespaced sub-app mounting to work.
		return fmt.Errorf("stack: Mount(Module, %q) on a WebApp target is not yet supported", edge.at)
	case http.Handler:
		if app == nil {
			return fmt.Errorf("stack: Mount(http.Handler, %q): requires a WebApp target", edge.at)
		}
		app.Registrar().AddMount(edge.at, subject)
		return nil
	default:
		return fmt.Errorf("stack: Mount: unsupported subject type %T", edge.subject)
	}
}

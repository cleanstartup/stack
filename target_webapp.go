package stack

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cleanstartup/stack/plugin"
	way2goweb "github.com/cleanstartup/stack/way2go/web"
	"github.com/cleanstartup/stack/webasset"
)

// WebAppTarget is the WebApp target kind (D-L): it embeds+serves text/css
// and application/javascript assets and injects them via way2go's
// AssetLinks, and requires an explicit Mount for any other content-type —
// there's no sensible default root for a foreign tree in a WebApp.
type WebAppTarget struct {
	module      Module
	ingredients []Ingredient

	app      *webasset.WebApp
	links    way2goweb.AssetLinks
	assetSeq int
}

var _ plugin.TargetKind = (*WebAppTarget)(nil)

// WebApp composes a WebApp target from a Module plus Ingredients (mounted
// Producers, Modules, or raw http.Handlers). tailwind/lit deliberately do
// not appear as ingredients here (D-N: they're app-level singleton stages
// of the WebApp's own nature, not composed args) — CUP-26 defines the
// contract and mechanics; wiring a real tailwind/lit stage in is CUP-21/25.
// module itself contributes routes only — its asset contributions (if any
// legacy stack.CSS/JS/File Parts are present) are not part of this target's
// asset pipeline; see Handler.
func WebApp(module Module, ingredients ...Ingredient) *WebAppTarget {
	return &WebAppTarget{module: module, ingredients: ingredients}
}

func (t *WebAppTarget) ensureApp() *webasset.WebApp {
	if t.app == nil {
		t.app = webasset.NewApp(t.module)
	}
	return t.app
}

// Build flattens the composed ingredients into Contributions and consumes
// them. Deliberately not an os.Args-driven terminal call like the old
// bundle.WebApp(opts...) — the self-rebuild lifecycle (build/run/dev/test/
// clean subcommands, host-native orchestrator, -tags release embed-gen) is
// a distinct, large design (D-E–D-I) outside CUP-26's scope.
//
// stageCtx.ProjectDir defaults from t.module.rootDir() — the WebApp's own
// module, never an ingredient's — when the caller leaves it empty (CUP-27):
// today nothing constructs a real StageContext in production (see
// plugin.StageContext's doc comment), so this default is what makes a
// directly-called Build(ctx, plugin.StageContext{OutputDir: ...}) — the
// shape every existing caller and test uses — still resolve a real
// `import ... from "lit"`/Web Awesome specifier via the lit stage's esbuild
// AbsWorkingDir, without requiring every caller to know to set ProjectDir
// itself. An explicit stageCtx.ProjectDir always wins.
//
// Deliberately t.module only, never any ingredient Module's rootDir (code
// review asked this be verified, not assumed — see
// TestWebAppTargetDefaultsProjectDirFromOwnModuleNotIngredient): in the
// shape this exists for, `stack.WebApp(myModule, ui.Module())`, ProjectDir
// must land on the *consuming app's* project root, since that's the only
// place a real npm project (package.json/node_modules with lit + Web
// Awesome installed) can plausibly live. ui.Module()'s own rootDir() points
// at wherever the cleanstartup/ui Go module was checked out — never the
// app's npm project root — so falling back to an ingredient's rootDir here
// would be wrong for the primary use case, not just an untested edge case.
// This is asymmetric with moduleContentDirs/ingredientModules on purpose:
// those fold ingredient Modules in because *content dirs* (component
// sources) legitimately live in the ingredient; ProjectDir does not.
func (t *WebAppTarget) Build(ctx context.Context, stageCtx plugin.StageContext) error {
	app := t.ensureApp()
	if strings.TrimSpace(stageCtx.ProjectDir) == "" && t.module != nil {
		stageCtx.ProjectDir = t.module.rootDir()
	}
	contributions, err := flattenIngredients(ctx, stageCtx, app, t.ingredients)
	if err != nil {
		return err
	}
	contentDirs := moduleContentDirs(append([]Module{t.module}, ingredientModules(t.ingredients)...)...)
	twContribution, err := t.buildTailwindStage(ctx, stageCtx, contentDirs)
	if err != nil {
		return err
	}
	if twContribution != nil {
		if err := appendSingletonContribution(&contributions, *twContribution, "the tailwind stage"); err != nil {
			return err
		}
	}
	litContribution, err := t.buildLitStage(ctx, stageCtx, contentDirs)
	if err != nil {
		return err
	}
	if litContribution != nil {
		if err := appendSingletonContribution(&contributions, *litContribution, "the lit stage"); err != nil {
			return err
		}
	}
	return t.Consume(ctx, contributions)
}

// appendSingletonContribution appends contribution to *contributions after
// checking its Assets against every Asset.Path already collected (D-J).
// The WebApp target's app-level singleton stages (tailwind, lit) run after
// flattenIngredients and are appended afterward, so they never pass through
// that function's own seenPaths dedup — re-checking here catches a
// manually-wired ingredient colliding with an auto-stage's output path and
// fails fast instead of silently double-mounting the same file under two
// URLs. label names the stage in the error for debuggability (e.g. "the
// tailwind stage", "the lit stage").
func appendSingletonContribution(contributions *[]plugin.Contribution, contribution plugin.Contribution, label string) error {
	seenPaths := map[string]string{}
	for _, c := range *contributions {
		for _, a := range c.Assets {
			seenPaths[a.Path] = c.Mount
		}
	}
	for _, a := range contribution.Assets {
		if prevMount, ok := seenPaths[a.Path]; ok {
			return fmt.Errorf("stack: duplicate asset path %q (contributed via mount %q, again via %s)", a.Path, prevMount, label)
		}
	}
	*contributions = append(*contributions, contribution)
	return nil
}

// Consume implements plugin.TargetKind. It branches only on content-type
// (D-L): text/css and application/javascript families are embedded, served,
// and linked; anything else requires an explicit Contribution.Mount.
//
// Known limitation, documented not solved here: +head/+footer/+module hints
// are recognized (family-matched) but not differentiated in placement,
// since way2go's AssetLinks has no per-entry metadata — extending that
// shape is a way2go-module change outside CUP-26 (stack's contract, not
// way2go's asset-link shape), and D-K explicitly permits a target to ignore
// a hint it doesn't act on.
func (t *WebAppTarget) Consume(ctx context.Context, contributions []plugin.Contribution) error {
	app := t.ensureApp()
	usedForeignMounts := map[string]bool{}
	for _, c := range contributions {
		for _, a := range c.Assets {
			if err := ctx.Err(); err != nil {
				return err
			}
			switch {
			case plugin.ContentTypeInFamily(a.ContentType, "text/css"):
				url, err := t.mountGeneratedAsset(app, "css", a)
				if err != nil {
					return err
				}
				t.links.Styles = append(t.links.Styles, url)
			case plugin.ContentTypeInFamily(a.ContentType, "application/javascript"):
				url, err := t.mountGeneratedAsset(app, "js", a)
				if err != nil {
					return err
				}
				t.links.Scripts = append(t.links.Scripts, url)
			default:
				mount := strings.TrimSpace(c.Mount)
				if mount == "" {
					return fmt.Errorf("stack: webapp target: content-type %q requires an explicit Mount point", a.ContentType)
				}
				// A foreign mount hosts one self-contained tree (Asset.Path's
				// contract: "file, or a self-contained sub-tree dir") — a
				// second asset at the same prefix would silently shadow the
				// first (chi.Handle overwrites on an identical pattern), so
				// reusing a mount point here is a genuine composition error.
				if usedForeignMounts[mount] {
					return fmt.Errorf("stack: webapp target: mount point %q is already used by another asset", mount)
				}
				usedForeignMounts[mount] = true
				if err := mountAssetTree(app, mount, a); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Handler builds the runtime HTTP handler. A Module composed via WebApp
// contributes routes only — asset contributions flow exclusively through
// the new Asset/Contribution contract (Ingredients, not Module Parts), so
// any legacy stack.CSS/JS/File Part on the module is intentionally ignored
// here: this is a hard cut, not a merge, and SetAssets below deliberately
// overwrites whatever app.Handler() populated from the old webasset.Builder
// manifest with only the new-contract-derived links.
func (t *WebAppTarget) Handler() http.Handler {
	app := t.ensureApp()
	reg := app.Handler()
	reg.SetAssets(t.links)
	return reg.Handler()
}

// AssetLinks returns the CSS/JS URLs Consume has injected so far — mirrors
// WebsiteTarget.Manifest() as an introspection point for tests/tooling.
func (t *WebAppTarget) AssetLinks() way2goweb.AssetLinks {
	return t.links
}

func (t *WebAppTarget) mountGeneratedAsset(app *webasset.WebApp, kind string, a plugin.Asset) (string, error) {
	t.assetSeq++
	url := fmt.Sprintf("/assets/%s/%d-%s", kind, t.assetSeq, filepath.Base(a.Path))
	if err := mountAssetTree(app, url, a); err != nil {
		return "", err
	}
	return url, nil
}

// mountAssetTree mounts a.Path (file or self-contained dir) as an HTTP
// handler at the given prefix on app's registrar.
func mountAssetTree(app *webasset.WebApp, at string, a plugin.Asset) error {
	info, err := os.Stat(a.Path)
	if err != nil {
		return fmt.Errorf("stack: webapp target: asset %q: %w", a.Path, err)
	}
	var handler http.Handler
	if info.IsDir() {
		handler = http.StripPrefix(at, http.FileServer(http.Dir(a.Path)))
	} else {
		path := a.Path
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, path)
		})
	}
	app.Registrar().AddMount(at, handler)
	return nil
}

package stack

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cleanstartup/stack/asset"
	"github.com/cleanstartup/stack/plugin"
)

// WebsiteTarget is the Website target kind (D-L): it copies every asset
// into an output tree. Unlike WebApp, copies default to the output root —
// there's no server, so there's no reason to force a Mount.
type WebsiteTarget struct {
	// OutputDir must be set before Consume runs. Build populates it from
	// StageContext.OutputDir if it's still empty; set it directly to call
	// Consume standalone (e.g. in tests) without going through Build.
	OutputDir string

	module      Module
	ingredients []Ingredient
	manifest    map[string][]string // content-type -> relative output paths
}

var _ plugin.TargetKind = (*WebsiteTarget)(nil)

// Website composes a Website target from a Module plus Ingredients.
func Website(module Module, ingredients ...Ingredient) *WebsiteTarget {
	return &WebsiteTarget{module: module, ingredients: ingredients}
}

func (t *WebsiteTarget) Build(ctx context.Context, stageCtx plugin.StageContext) error {
	if strings.TrimSpace(t.OutputDir) == "" {
		t.OutputDir = stageCtx.OutputDir
	}
	contributions, err := flattenIngredients(ctx, stageCtx, nil, t.ingredients)
	if err != nil {
		return err
	}
	return t.Consume(ctx, contributions)
}

// Consume implements plugin.TargetKind: every asset is copied to
// OutputDir/<Mount>/<basename> if Mount is set, else OutputDir/<basename>
// (D-L: "Website kopiert nach Root als Default" — the opposite forcing
// default from WebApp, which requires an explicit Mount instead).
//
// This is an addition beyond D-J, which only mandates dedup on a producer's
// source Asset.Path (see flattenIngredients): two producers can legitimately
// emit distinct source paths that collide once copied to the same
// destination (e.g. two "style.css" files from different source dirs, both
// Mount == ""). That's caught here, target-side, by tracking every
// destination path a Consume call is about to write and erroring fail-fast
// on the first repeat — before any bytes move for the colliding asset.
func (t *WebsiteTarget) Consume(ctx context.Context, contributions []plugin.Contribution) error {
	if strings.TrimSpace(t.OutputDir) == "" {
		return fmt.Errorf("stack: website target: OutputDir not set")
	}
	if t.manifest == nil {
		t.manifest = map[string][]string{}
	}
	seenDests := map[string]string{} // dest path -> describing mount of first contributor
	for _, c := range contributions {
		destRoot := t.OutputDir
		if mount := strings.TrimSpace(c.Mount); mount != "" {
			destRoot = filepath.Join(t.OutputDir, filepath.FromSlash(strings.TrimPrefix(mount, "/")))
		}
		for _, a := range c.Assets {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := t.copyAsset(destRoot, c.Mount, a, seenDests); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *WebsiteTarget) copyAsset(destRoot, mount string, a plugin.Asset, seenDests map[string]string) error {
	info, err := os.Stat(a.Path)
	if err != nil {
		return fmt.Errorf("stack: website target: asset %q: %w", a.Path, err)
	}
	if info.IsDir() {
		relFiles, err := asset.ListFiles(a.Path)
		if err != nil {
			return fmt.Errorf("stack: website target: asset %q: %w", a.Path, err)
		}
		dests := make([]string, len(relFiles))
		for i, rel := range relFiles {
			dest := filepath.Join(destRoot, filepath.FromSlash(rel))
			if err := t.checkDestCollision(seenDests, dest, mount); err != nil {
				return err
			}
			dests[i] = dest
		}
		if err := os.MkdirAll(destRoot, 0o755); err != nil {
			return fmt.Errorf("stack: website target: asset %q: %w", a.Path, err)
		}
		if _, err := asset.CopyDir(destRoot, a.Path); err != nil {
			return fmt.Errorf("stack: website target: asset %q: %w", a.Path, err)
		}
		for _, dest := range dests {
			t.recordManifest(a.ContentType, dest)
		}
		return nil
	}
	dest := filepath.Join(destRoot, filepath.Base(a.Path))
	if err := t.checkDestCollision(seenDests, dest, mount); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("stack: website target: asset %q: %w", a.Path, err)
	}
	if err := asset.CopyFile(dest, a.Path); err != nil {
		return fmt.Errorf("stack: website target: asset %q: %w", a.Path, err)
	}
	t.recordManifest(a.ContentType, dest)
	return nil
}

func (t *WebsiteTarget) checkDestCollision(seenDests map[string]string, dest, mount string) error {
	if prev, ok := seenDests[dest]; ok {
		return fmt.Errorf("stack: website target: duplicate destination %q (already written via mount %q, again via %q)", dest, prev, mount)
	}
	seenDests[dest] = mount
	return nil
}

func (t *WebsiteTarget) recordManifest(contentType, dest string) {
	rel, err := filepath.Rel(t.OutputDir, dest)
	if err != nil {
		rel = dest
	}
	t.manifest[contentType] = append(t.manifest[contentType], rel)
}

// Manifest returns the content-type-keyed list of paths (relative to
// OutputDir) that Consume has written so far.
//
// D-L specifies Website as "copy + relative <link>" — this only does the
// copy half. InjectManifestLinks below is the real consumer of this
// building block (CUP-22), doing the relative-<link> half against hugo's
// real HTML output.
func (t *WebsiteTarget) Manifest() map[string][]string {
	return t.manifest
}

// InjectManifestLinks is the "relative <link>" half of D-L's Website policy
// ("copy + relative <link>") that Manifest()'s doc comment names CUP-22 as
// landing: it walks every .html file already written under OutputDir (by a
// prior Consume) and, immediately before each file's first "</head>",
// inserts a <link rel="stylesheet"> for every text/css Manifest() entry and
// a <script> for every application/javascript(+*) entry — each href/src
// computed relative to that HTML file's own directory, e.g. mounting a
// hugo tree at /docs and tailwind's CSS at the output root yields
// "../app.css" inside docs/index.html.
//
// Scope, deliberately minimal (CUP-22 judgment call — see task notes): a
// plain byte-level "</head>" insertion, not an HTML parser or a templating
// layer. That's enough to prove Manifest()-driven relative linking
// end-to-end against a real producer's (hugo's) output tree without
// inventing machinery (a templating system) that doesn't exist yet.
// Explicitly out of scope, left for a real need to justify: per-page link
// selection (every page gets every asset — hugo's own templates are the
// right place for that, not a stack-level post-process), case-insensitive/
// attribute-bearing "</head>" variants, +head/+footer hint-driven
// placement (there's exactly one insertion point here), and idempotency
// (calling this twice double-injects — callers invoke it once, after
// Consume has finished writing). +module IS honored (unlike +head/+footer,
// D-K classifies it as real byte semantics, not a placement hint an
// injector is free to ignore) — a script contributed as
// "application/javascript+module" gets type="module" so ESM import/export
// actually parses in the browser.
func (t *WebsiteTarget) InjectManifestLinks() error {
	if strings.TrimSpace(t.OutputDir) == "" {
		return fmt.Errorf("stack: website target: OutputDir not set")
	}
	cssRels := t.manifestFamily("text/css")
	jsEntries := t.manifestJSEntries()
	if len(cssRels) == 0 && len(jsEntries) == 0 {
		return nil
	}

	return filepath.WalkDir(t.OutputDir, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(current), ".html") {
			return nil
		}
		return t.injectLinksIntoFile(current, cssRels, jsEntries)
	})
}

// manifestFamily gathers every Manifest() entry whose content-type is base
// or a "+hint" of base (D-K's family match) — used for text/css, where no
// hint changes byte semantics, so hints can be safely ignored.
func (t *WebsiteTarget) manifestFamily(base string) []string {
	var out []string
	for ct, rels := range t.manifest {
		if ct == base || strings.HasPrefix(ct, base+"+") {
			out = append(out, rels...)
		}
	}
	sort.Strings(out)
	return out
}

// jsManifestEntry pairs a manifest-relative path with whether it needs
// type="module" (D-K: +module is byte semantics, not a droppable hint).
type jsManifestEntry struct {
	rel    string
	module bool
}

func (t *WebsiteTarget) manifestJSEntries() []jsManifestEntry {
	const base = "application/javascript"
	var out []jsManifestEntry
	for ct, rels := range t.manifest {
		if ct != base && !strings.HasPrefix(ct, base+"+") {
			continue
		}
		module := ct == base+"+module"
		for _, rel := range rels {
			out = append(out, jsManifestEntry{rel: rel, module: module})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].rel < out[j].rel })
	return out
}

func (t *WebsiteTarget) injectLinksIntoFile(htmlPath string, cssRels []string, jsEntries []jsManifestEntry) error {
	body, err := os.ReadFile(htmlPath)
	if err != nil {
		return err
	}
	idx := bytes.Index(body, []byte("</head>"))
	if idx < 0 {
		return nil // no <head> to inject into — left untouched, see doc comment.
	}
	htmlDir := filepath.Dir(htmlPath)

	var tags strings.Builder
	for _, rel := range cssRels {
		href, err := t.relativeAssetPath(htmlDir, rel)
		if err != nil {
			return err
		}
		fmt.Fprintf(&tags, "<link rel=\"stylesheet\" href=\"%s\">\n", href)
	}
	for _, entry := range jsEntries {
		src, err := t.relativeAssetPath(htmlDir, entry.rel)
		if err != nil {
			return err
		}
		if entry.module {
			fmt.Fprintf(&tags, "<script type=\"module\" src=\"%s\"></script>\n", src)
		} else {
			fmt.Fprintf(&tags, "<script src=\"%s\"></script>\n", src)
		}
	}

	injected := make([]byte, 0, len(body)+tags.Len())
	injected = append(injected, body[:idx]...)
	injected = append(injected, []byte(tags.String())...)
	injected = append(injected, body[idx:]...)
	return os.WriteFile(htmlPath, injected, 0o644)
}

func (t *WebsiteTarget) relativeAssetPath(htmlDir, manifestRel string) (string, error) {
	rel, err := filepath.Rel(htmlDir, filepath.Join(t.OutputDir, filepath.FromSlash(manifestRel)))
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

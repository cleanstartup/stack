package stack

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
// copy half. Injecting relative <link>s needs a real HTML/template context
// to inject into, which doesn't exist without an actual Website consumer;
// deferred until one lands (CUP-22). Manifest() is that consumer's building
// block for doing so itself.
func (t *WebsiteTarget) Manifest() map[string][]string {
	return t.manifest
}

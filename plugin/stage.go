package plugin

import (
	"context"
	"strings"

	"github.com/cleanstartup/stack/devwatch"
)

// StageContext carries the shared services for the Asset/Contribution
// contract (D-J–D-N). Deliberately smaller than Context: producers now do
// their own discovery, so there's no shared AssetKind-keyed workspace to
// hand out.
type StageContext struct {
	OutputDir string
	Mode      Mode
	NPM       NPM
	Bin       BinProvider
}

// Asset is a flat, typed build output. No placement, no claim, no ownership
// (D-J) — a producer does its own discovery and emits what it found.
type Asset struct {
	Path        string // file, or a self-contained sub-tree dir (e.g. hugo's output)
	ContentType string // "text/css", "application/javascript", "text/html", "font/woff2", …
	// ContentType is open and convention-driven. An optional "+" suffix
	// refines it (D-K): "+module" is real byte semantics (ESM); "+head"/
	// "+footer" is a placement hint. The plugin hints, the target decides —
	// a target may honor or ignore any hint, but placement stays target-owned.
}

// Assets is a producer's output list, filterable by content-type.
type Assets []Asset

// OfType matches assets by content-type (D-K):
//
//	OfType("application/javascript")      -> only the unsuffixed default
//	OfType("application/javascript+head") -> only that exact hint
//	OfType("application/javascript+*")    -> the whole family (bare + every hint)
func (as Assets) OfType(contentType string) Assets {
	contentType = strings.TrimSpace(contentType)
	var out Assets
	if base, ok := strings.CutSuffix(contentType, "+*"); ok {
		for _, a := range as {
			if a.ContentType == base || strings.HasPrefix(a.ContentType, base+"+") {
				out = append(out, a)
			}
		}
		return out
	}
	for _, a := range as {
		if a.ContentType == contentType {
			out = append(out, a)
		}
	}
	return out
}

// Contribution groups one producer edge's assets with that edge's wiring
// parameter (D-M) — not flattened, so the edge's Mount point survives.
type Contribution struct {
	Assets Assets
	Mount  string // "" = target's default policy; set = explicit mount point
}

// Producer-side optional per-stage interfaces (D-E). A plugin opts in per
// stage instead of implementing a fixed method set with no-op stubs.
type Builder interface {
	Build(context.Context, StageContext) ([]Asset, error)
}

type Dever interface {
	Dev(context.Context, StageContext) ([]devwatch.WatchWorker, error)
}

type Tester interface {
	Test(context.Context, StageContext) error
}

type Cleaner interface {
	Clean(context.Context, StageContext) error
}

// TargetKind is the consumer side (D-L): the target kind owns the whole
// consumption strategy for a build, branching only on content-type, never
// on source dir or build tool. Named TargetKind, not Target, because Target
// (RegisterCSS/RegisterJS) already exists above and is unrelated.
type TargetKind interface {
	Consume(context.Context, []Contribution) error
}

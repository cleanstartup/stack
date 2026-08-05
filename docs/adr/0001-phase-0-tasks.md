# ADR-0001 — Phase 0: Contract & Gerüst

- Status: Accepted, umgesetzt
- Datum: 2026-07-25
- Bezug: [0001-stack-as-plugin-orchestrator-and-way2go-split.md](./0001-stack-as-plugin-orchestrator-and-way2go-split.md)

Detaillierung von **Phase 0** aus ADR-0001: den Plugin-Contract real und
öffentlich machen und ihn an den zwei Bestands-Capabilities (tailwind, stencil)
beweisen — ohne Blast-Radius auf Artifacts. Registrierungs-Inversion,
`web`-Zerlegung und way2go-Inhalt blieben späteren Phasen vorbehalten.

## Findings, die Phase 0 zuschneiden

1. **Der Contract lässt sich nicht allein promoten.** `capability.Capability`
   führt `asset.AssetRef`, `asset.AssetKind` und `pipeline.WatchWorker` in der
   Signatur, und Plugins konsumieren deren Helfer (tailwind baut Assets über
   `asset.FromDir`/`Materialize`, startet Dev-Worker über
   `pipeline.StartCommandWatchSpec`). `stack/plugin` öffentlich zu machen zieht
   `internal/asset` → `stack/asset` und `internal/pipeline` → `stack/devwatch`
   zwingend mit. Task 0.2 ist drei Promotions.

2. **D12 ist fast gratis.** `BuildConfig any` trägt heute nur Mode +
   tailwind/stencil-Felder. tailwind liest `STACK_TAILWIND_*` in `ResolveBinary`
   (`internal/tailwind/build.go`) ohnehin direkt aus dem Environment. Also:
   `BuildConfig any`/`DevConfig any` fallen weg, ersetzt durch ein explizites
   `Mode`; die `tailwindConfig`/`stencilConfig`-Resolver in
   `internal/build/capabilities.go` werden gelöscht.

## Zielpaket-Layout

```
way2go/                         (nur Skelett in Phase 0)
stack/plugin/     ← internal/capability   (Capability, Source, Context, Workspace, Target, NPM, Bin, Mode)
stack/asset/      ← internal/asset        (AssetRef, AssetKind, AssetSource, AssetWorkspace, FromDir, …)
stack/devwatch/   ← internal/pipeline     (WatchWorker, StartCommandWatchSpec, SnapshotPaths, …)
```

## Contract-Signaturen — `stack/plugin`

```go
package plugin

import (
	"context"

	"github.com/cleanstartup/stack/asset"
	"github.com/cleanstartup/stack/devwatch"
)

// Mode ersetzt das heutige BuildConfig any / DevConfig any.
type Mode string

const (
	ModeBuild Mode = "build"
	ModeDev   Mode = "dev"
)

// Context ist der öffentliche Vertrag, den stack an jedes Plugin reicht.
type Context struct {
	ProjectDir string      // Modul-/Command-Wurzel
	OutputDir  string      // .assets-Ziel
	Workspace  Workspace   // Asset-Zielpfade
	Mode       Mode        // build | dev
	NPM        NPM         // GETEILTER npm-Workspace (D3)
	Bin        BinProvider // GETEILTES Binary-Provisioning (D3)
}

// Workspace: unverändert aus internal/capability, nur öffentlich.
type Workspace interface {
	RootDir() string
	OutputDir() string
	AssetDir(asset.AssetKind, string) string
}

// Target: unverändert.
type Target interface {
	RegisterCSS(asset.AssetRef)
	RegisterJS(asset.AssetRef)
}

// Capability: unverändert bis auf devwatch-Typ.
type Capability interface {
	Install(context.Context, Context) error
	Build(context.Context, Context) error
	Dev(context.Context, Context) ([]devwatch.WatchWorker, error)
	Register(Target)
}

// Source: optionales Interface für inkrementellen Dev-Rebuild (wie heute
// per Type-Assertion in rebuildChangedCapabilities).
type Source interface {
	SourcePaths() []string
	SourceChanged(string) bool
	Rebuild(context.Context, Context) error
}
```

## Geteilte Services

**NPM** — die heutigen public Methoden von `*npm.Project` als Interface. `stack`
besitzt weiter *einen* `Project` und reicht ihn als `ctx.NPM` an alle Plugins
(löst „doppelte node_modules" aus D3). tailwind/stencil rufen
`ctx.NPM.AddDependency(...)` statt des heutigen `AddNPMDependencies(project)` in
`internal/build/capabilities.go`.

```go
type NPM interface {
	AddDependency(name, version string)
	AddDevDependency(name, version string)
	RequireBin(name string) // z.B. "tailwindcss", "stencil"
}
```

**BinProvider** — extrahiert den wiederverwendbaren Kern aus
`tailwind.downloadBinary` (`internal/tailwind/build.go`): Download → Cache-Pfad →
chmod → atomarer Rename. Plugin-spezifisch bleibt die URL-/Plattform-/
Latest-Auflösung.

```go
type BinProvider interface {
	// Ensure lädt spec.URL in den geteilten Cache unter Name/Version, macht
	// die Datei ausführbar und gibt den lokalen Pfad zurück. Idempotent.
	Ensure(ctx context.Context, spec BinarySpec) (string, error)
}

type BinarySpec struct {
	Name    string // logischer Name, z.B. "tailwindcss"
	Version string // aufgelöst, Teil des Cache-Pfads
	URL     string // fertig aufgelöste, plattformspezifische Download-URL
}
```

**Env-Auflösung des Cache-Roots** — `plugin.DefaultBinCacheDir()` (genutzt von
`NewEngine`, um den geteilten `BinProvider` zu bauen):

1. `STACK_BIN_CACHE_DIR` — kanonisch, generisch (bedient künftig alle Plugins).
2. `STACK_TAILWIND_CACHE_DIR` — BC-Fallback, solange tailwind der einzige
   Downloader ist. tailwind liest dieselbe Variable auch für die eigene
   `latest.version`-Pointer-Datei — beide landen dadurch garantiert im selben
   Root, kein Split-Brain.
3. `$UserCacheDir/stack/bin`, sonst `$TMPDIR/stack/bin`.

## Umsetzung

Paket-Promotions `internal/asset`→`stack/asset`, `internal/pipeline`→`stack/devwatch`,
`internal/capability`→`stack/plugin`; `plugin.Context` von `BuildConfig any`/`DevConfig any`
auf `Mode`+`NPM`+`Bin` umgestellt; `BinProvider` aus `downloadBinary` extrahiert;
tailwind/stencil auf `ctx.NPM`/`ctx.Bin` umgestellt (Registrierungs-Inversion bleibt
Phase 2).

## Exit-Kriterium Phase 0

`NPM` + `BinProvider` reichen als einzige geteilten Services, damit tailwind *und*
stencil ohne stack-Interna auskommen — stencil schreibt `stencil.config.ts`/
`tsconfig.json` rein filesystembasiert, braucht keinen Service. Der Contract ist
damit für hugo (Phase 3) tragfähig.

## Nicht in Phase 0

- Registrierungs-Inversion `WebApp(tailwind.Plugin(), …)` → Phase 2
- `web`-Zerlegung (D11) → Phase 1
- way2go-Inhalt (activity/param/cli/config/web-runtime) → Phase 1

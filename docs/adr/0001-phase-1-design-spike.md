# ADR-0001 — Phase 1 / Task 1.0: Design-Spike, Part/WebApp reshapen

- Status: Proposed (Rev. 2 — Changes Requested aus Richies Review vom
  2026-07-29 eingearbeitet: F1 Asset-Manifest-Zyklus, F2 Artifact-Scope,
  F3 ein `stack.Part`)
- Datum: 2026-07-29
- Autor: Dave
- Bezug: [0001-phase-1-tasks.md](./0001-phase-1-tasks.md),
  [0001-stack-as-plugin-orchestrator-and-way2go-split.md](./0001-stack-as-plugin-orchestrator-and-way2go-split.md)

Kein Code wurde verschoben. Dies ist reines Design für Task 1.0.

**Rev. 2:** Registrar/RouteAccumulator-Design (Abschnitt 1), R3-Vierer-Befund
und die Namensfrage sind von Richie gegengezeichnet. Diese Revision behebt
die zwei Blocker (F1, F2) und die Klärung (F3) aus seinem Review (Inbox-Msg
`26f441d7`, 2026-07-29).

## R3-Befund zuerst (kritisch für Zuschnitt von 1.4)

**Die Trennung streut breiter als `app.go`.** Konkret drei zusätzliche
Fundstellen, alle mit Beleg:

1. **`web/builder.go` — `Builder` hält Routing-State, nicht nur Assets.**
   `Builder.routes []routeRegistration`, `Builder.mounts
   []mountRouteRegistration`, `AddWebActivity(b *Builder, ...)`, `(b
   *Builder) AddMount(...)` und `(b *Builder) BuildRouteHandler() *Registry`
   sind reine Runtime-Konzepte (Routen/Mounts/Chi-Router), die heute im
   "Builder" stecken. Der ADR-Befund "Builder-Seite importiert nur
   plugin/asset" stimmt für die Imports, nicht für die Verantwortung.

2. **`web/web.go` — `WebActivity[C].Apply` umgeht `WebApp`s öffentliche
   API.** `func (a *WebActivity[C]) Apply(app *WebApp) { AddWebActivity(app.builder, a) }`
   (web/web.go:138-143) greift direkt auf das unexportierte Feld
   `app.builder` zu, nicht über `RegisterCSS`/... — geht nur, weil `web.go`
   und `app.go` heute im selben Package sitzen. Nach dem Split kann way2go
   (`web.go`) nicht mehr auf stacks `*WebApp`/`*Builder` zeigen.

3. **`web/module.go` teilt sich nicht atomisch in "Runtime".** Die Datei
   enthält zwei Gruppen: (a) `Part`/`Contributor`/`Compose`/`Mount` — echte
   Runtime-Kompositionsprimitive; (b) `CSS`/`TailwindCSS`/`Stencil`/`JS`/
   `File`/`TailwindScan`/`StencilScan`/`NPMDependency`/`NPMDevDependency` —
   Asset-Part-Konstruktoren, die `app.builder` direkt berühren (z.B.
   `NPMDependency`: `partFunc(func(app *WebApp){ app.builder.AddNPMDependency(...) })`,
   module.go:71-73). Gruppe (b) gehört zu stack, nicht way2go.

4. **`internal/build/build.go` verdrahtet direkt gegen `*Builder`.**
   `NewEngine(b *web.Builder)` (build.go:79) und `Serve()`: `registry :=
   e.builder.BuildRouteHandler()` (build.go:346) — der Build-/Serve-Motor
   zieht den fertigen HTTP-Router aus dem Builder, nicht aus einer
   Kompositions-Wurzel. Bestätigt durch `stack.go:509`:
   `buildpkg.NewEngine(app.Builder())` — die einzige Verdrahtung des
   Build-Engines läuft heute über `*Builder` allein.

**Konsequenz:** Der Cut ist immer noch klein und lokal, aber er ist kein
reiner `app.go`-Refactor — er zieht `web/builder.go` (Routing-Teil raus),
`web/module.go` (in zwei Hälften gesplittet) und `internal/build/build.go`
(Signaturänderung `NewEngine`) mit. Task 1.4 sollte das im
Akzeptanzkriterium widerspiegeln. **Rev. 2 erweitert die Fläche um zwei
weitere Punkte (F1, F2 unten) — siehe dort.**

### F1 — Asset-Manifest-Zyklus in der Runtime (Rev. 2, war in Rev. 1 übersehen)

Der Routing-Cut allein löst nicht die **Asset-Typ**-Abhängigkeit der
Runtime. Beleg:

- `web/page.go:63` `renderAssetLinks(..., refs []AssetRef, ...)`
- `web/web.go:38` `Registry.assets AssetManifest`, `:276` `SetAssets(AssetManifest)`
- `web/dev.go:103/118` `withAssetManifest`/`assetManifestFromContext(AssetManifest)`

`AssetRef`/`AssetManifest` aliasieren via `web/assets.go` auf `stack/asset`
(seit Phase 0 im stack-Modul). Gingen `page.go`/`web.go`/`dev.go` unverändert
nach way2go, bräuchte way2go diese Typen aus stack →
**`stack → way2go → stack`**, ein Zyklus. Die ursprüngliche Move-Tabelle
„page.go 1:1" war falsch — Fix und korrigierte Tabelle in Abschnitt 1 bzw.
Abschnitt 4.

### F2 — Artifact-Nutzung von `web.*` (Rev. 2, korrigiert Rev.-1-Fehlfund)

Rev. 1 behauptete, kein Artifact-Code riefe `web.*` direkt auf — **falsch**.
`fortego-btc/app` importiert `github.com/cleanstartup/stack/web` direkt in
8 Dateien (meist als `stackweb`/`wayweb`). Symbolkartierung:

| Datei | genutzte Symbole | Ziel |
| --- | --- | --- |
| `providers/flags/flags_test.go` | `NewActivity`, `NewRegistry`, `RegisterWebActivity`, `WithCrossMiddleware` | way2go |
| `providers/auth/auth.go` | `ActivityHandler`, `ActivityOption`, `Context`, `WithMiddleware` | way2go |
| `providers/auth/supertokens/authprovider.go` | `Page`, `RenderResult` | way2go |
| `providers/orders/web/activities.go` | `Page` | way2go |
| `providers/analytics/posthog/posthog.go` | `ActivityMetaFromRequest` | way2go |
| `internal/web/activities.go` | `Page`, `Screen` | way2go |
| `internal/web/backup/activities.go` | `Page`, `Screen` | way2go |
| `internal/web/restore/activities.go` | `Page`, `Screen` | way2go |

Alle 8 Fundstellen nutzen ausschliesslich Runtime-Symbole (kein `Builder`,
kein `AssetSource`, keine Asset-Part-Konstruktoren) — jedes Symbol landet
ohnehin schon nach der ursprünglichen Datei-Zuordnung in way2go. **Konsequenz
für Task 1.7 (nicht 1.0):** reines Repointen des Imports
`github.com/cleanstartup/stack/web` → way2go-Modulpfad in diesen 8 Dateien,
keine Symbol-Änderung nötig — vorausgesetzt F1 wird wie unten gelöst (sonst
bräche `Page`/`Screen`, die intern `AssetLinks` nutzen). Die öffentliche
Kompatibilitätsfläche ist also **`stack.*` plus dieser Runtime-Symbol-Satz
aus way2go**, nicht nur `stack.*` wie in Rev. 1 behauptet.

## 1. Runtime-Registrar-Interface (way2go)

```go
// way2go, package web (Runtime-Hälfte)

// Registrar ist das Ziel, an das Runtime-Parts (Routen, Activities, Mounts)
// binden. Kennt weder Builder noch AssetSource/AssetRef.
type Registrar interface {
	AddActivity(a RouteActivity)
	AddMount(path string, handler http.Handler)
}

// RouteActivity versteckt den Typparameter von WebActivity[C], damit
// heterogene Activities in einem Registrar landen können.
type RouteActivity interface {
	routeRegister(r *Registry)
}

func (a *WebActivity[C]) routeRegister(r *Registry) { RegisterWebActivity(r, a) }

// Part ist die Kompositionsprimitive für reine Runtime-Beiträge.
// Asset-Beiträge nutzen stacks eigenes Part (Apply(*Builder)), siehe unten.
type Part interface {
	Apply(Registrar)
}

// RouteAccumulator ist die konkrete Registrar-Implementierung: sammelt
// Registrierungen bei Apply-Zeit, baut den Router erst bei Handler().
// Ersetzt den Routing-Teil des heutigen Builder (Befund 1).
type RouteAccumulator struct{ /* routes []RouteActivity; mounts []mountEntry */ }

func NewRouteAccumulator() *RouteAccumulator
func (r *RouteAccumulator) AddActivity(a RouteActivity)
func (r *RouteAccumulator) AddMount(path string, handler http.Handler)
func (r *RouteAccumulator) Handler() *Registry // war Builder.BuildRouteHandler()
```

`WebActivity[C].Apply` ändert sich von `Apply(app *WebApp)` (stack-Typ) zu
`Apply(reg Registrar)` (way2go-Interface) — das ist der eigentliche Schnitt,
der die Rückwärts-Abhängigkeit auflöst (Befund 2). `Mount(path, handler)
Part` (module.go, Gruppe a) zieht nach way2go und ruft `reg.AddMount(...)`.

### 1b. `AssetLinks` — Fix für F1

`page.go`/`web.go`/`dev.go` dürfen keine `stack/asset`-Typen mehr sehen.
`AssetRef.URLs()`/`URLsWithVersion()` (asset/asset.go:45-85) tun zwei
Dinge: (a) `Files []string` zu konkreten URLs auflösen — braucht
`AssetURL()`, bleibt in stack; (b) einen Versions-Query-Param anhängen —
zehn Zeilen `net/url`-Code (`withAssetVersion`, asset/asset.go:489-500),
kein Grund, dafür stack zu importieren. Trennung entlang dieser Naht:

```go
// way2go, package web

// AssetLinks ist die Runtime-lokale Sicht auf registrierte Assets:
// bereits aufgelöste Basis-URLs, keine Kenntnis von stack/asset. Wird einmal
// an der Kompositionsgrenze gebaut (stack.WebApp.Handler()), pro Request von
// Page.Render konsumiert.
type AssetLinks struct {
	Styles  []string
	Scripts []string
}

// withVersion hängt einen Cache-Buster an — lokale Kopie der zehn Zeilen
// net/url-Logik aus asset.withAssetVersion, keine stack-Abhängigkeit.
func withVersion(assetURL, version string) string { /* ... */ }
```

`Registry.assets` wechselt von `AssetManifest` auf `AssetLinks`;
`SetAssets(AssetManifest)` → `SetAssets(AssetLinks)`; `withAssetManifest`/
`assetManifestFromContext` bleiben strukturell gleich, nur der Typ ändert
sich. `page.go`s `renderAssetLinks(out, refs []AssetRef, kind, version)`
wird zu `renderAssetLinks(out, urls []string, kind, version)` — pro URL
`withVersion(u, version)` statt `ref.URLsWithVersion(version)`; die
Dedupe-Logik (`seen map[string]struct{}`) bleibt unverändert.

Auflösung passiert in stack, an der Kompositionsgrenze:

```go
// stack
func (a *WebApp) Handler() *way2goweb.Registry {
	reg := a.runtime.Handler()
	reg.SetAssets(toAssetLinks(a.builder.Manifest()))
	return reg
}

func toAssetLinks(m asset.AssetManifest) way2goweb.AssetLinks {
	var links way2goweb.AssetLinks
	for _, ref := range m.Styles {
		links.Styles = append(links.Styles, ref.URLs()...)
	}
	for _, ref := range m.Scripts {
		links.Scripts = append(links.Scripts, ref.URLs()...)
	}
	return links
}
```

`asset` bleibt vollständig in stack (Phase-0-Invariante und
Plugin-Unabhängigkeit intakt) — way2go sieht nie `AssetRef`/`AssetManifest`.
Betrifft `web/web.go`, `web/page.go`, `web/dev.go` — diese Dateien sind also
**nicht** 1:1 verschiebbar, siehe korrigierte Tabelle in Abschnitt 4.

## 2. Asset-Registrierung (stack) — bleibt am Builder

`Builder` verliert `routes`/`mounts`/`AddWebActivity`/`AddMount`/
`BuildRouteHandler`/`registerRoutes` (→ way2go, Befund 1) und bleibt
sonst wie heute (`tailwind`, `stencil`, `assets`, `npm`, `dirSources`).

**Rev. 2 / F3-Korrektur:** Rev. 1 zeigte hier fälschlich ein eigenes
`Part interface { Apply(*Builder) }` — im Widerspruch zu Abschnitt 3, wo
`WebApp` bereits als einzige Kompositionswurzel beschrieben war. Richies
Empfehlung übernommen: **genau ein `stack.Part` mit `Apply(*WebApp)`**
(Definition in Abschnitt 3), sonst zerfällt `Bundle`s gemischte Part-Liste
(Activities + CSS + Mounts) in zwei inkompatible Typen und die bestehende
Type-Switch-Maschinerie in `stack.go` (`partsFor`, `filteredPart`,
`case web.Part:`) bricht. Die Asset-Part-Konstruktoren (Gruppe b aus Befund
3), umgezogen von `web/module.go` nach stack, sind also `partFunc`-Closures
über `*WebApp`, nicht über `*Builder`:

```go
// stack — kein eigenes Part-Interface, nutzt das aus Abschnitt 3
func CSS(src AssetSource) Part            { return partFunc(func(app *WebApp) { app.Builder().CSS(src) }) }
func TailwindCSS(src AssetSource) Part    { return partFunc(func(app *WebApp) { app.Builder().TailwindCSS(src) }) }
func Stencil(src AssetSource) Part        { return partFunc(func(app *WebApp) { app.Builder().Stencil(src) }) }
func JS(src AssetSource) Part             { return partFunc(func(app *WebApp) { app.Builder().JS(src) }) }
func File(src AssetSource) Part           { return partFunc(func(app *WebApp) { app.Builder().File(src) }) }
func TailwindScan(paths ...string) Part   { return partFunc(func(app *WebApp) { app.Builder().TailwindScan(paths...) }) }
func StencilScan(paths ...string) Part    { return partFunc(func(app *WebApp) { app.Builder().StencilScan(paths...) }) }
func NPMDependency(name, version string) Part    { return partFunc(func(app *WebApp) { app.Builder().AddNPMDependency(name, version, false) }) }
func NPMDevDependency(name, version string) Part { return partFunc(func(app *WebApp) { app.Builder().AddNPMDependency(name, version, true) }) }
```

## 3. `WebApp`-Kompositionsvertrag (stack)

`WebApp` bleibt in stack, hält jetzt zwei Ziele statt eines `*Builder`. Die
**öffentliche Methodenliste von `WebApp` ändert sich nicht** (kein Diff für
`stack.go`s `ActivityDef.Apply`, `assets.StaticDirSource.Apply`,
`dirSourceRegistration.Apply` — die rufen weiterhin `app.RegisterFile(...)`
etc.) — nur die Bodies werden auf `builder` bzw. `runtime` verteilt:

```go
// stack
type WebApp struct {
	runtime *way2goweb.RouteAccumulator // way2go-Typ; erfüllt way2goweb.Registrar
	builder *Builder                    // stack-Typ, jetzt asset-only
}

func newWebApp() *WebApp {
	return &WebApp{runtime: way2goweb.NewRouteAccumulator(), builder: NewBuilder()}
}

func (a *WebApp) Builder() *Builder                     { return a.builder }
func (a *WebApp) Registrar() way2goweb.Registrar        { return a.runtime } // neu
func (a *WebApp) AddActivity(act way2goweb.RouteActivity) { a.runtime.AddActivity(act) } // neu, von ActivityDef genutzt

// unverändert in Signatur, Body zeigt jetzt auf builder:
func (a *WebApp) RegisterCSS(src AssetSource) AssetRef         { return a.builder.CSS(src) }
func (a *WebApp) RegisterTailwindCSS(src AssetSource) AssetRef { return a.builder.TailwindCSS(src) }
func (a *WebApp) RegisterStencil(src AssetSource) AssetRef     { return a.builder.Stencil(src) }
func (a *WebApp) RegisterJS(src AssetSource) AssetRef          { return a.builder.JS(src) }
func (a *WebApp) RegisterFile(src AssetSource) AssetRef        { return a.builder.File(src) }
func (a *WebApp) RegisterDirSource(ns, rel, abs string)        { a.builder.RegisterDirSource(ns, rel, abs) }
func (a *WebApp) RegisterTailwindScan(paths ...string)         { a.builder.TailwindScan(paths...) }
func (a *WebApp) RegisterStencilScan(paths ...string)          { a.builder.StencilScan(paths...) }

// unverändert in Signatur, Body zeigt jetzt auf runtime statt builder:
func (a *WebApp) Mount(path string, handler http.Handler) { a.runtime.AddMount(path, handler) }

// neu: Kompositionswurzel liefert den fertigen Runtime-Handler. Ersetzt
// internal/build's direkten Zugriff auf Builder.BuildRouteHandler() (Befund 4)
// und konvertiert an der Grenze asset.AssetManifest → way2goweb.AssetLinks
// (F1, siehe Abschnitt 1b) — way2go bekommt nie ein AssetRef zu sehen.
func (a *WebApp) Handler() *way2goweb.Registry {
	reg := a.runtime.Handler()
	reg.SetAssets(toAssetLinks(a.builder.Manifest()))
	return reg
}
```

**Notwendige Konsequenz, kein Rewrite, aber ein echter Cut:** `type Part =
web.Part` (stack.go:28) kann nach dem Split **kein Type-Alias mehr sein** —
way2gos `web.Part` hat `Apply(Registrar)`, stacks `Part` hat
`Apply(*WebApp)`. Das sind zwei verschiedene Interfaces. `stack.Part` wird
zur eigenständigen Definition; `Mount`, `Compose`, `NPMDependency`, `CSS`,
`JS`, `File` (heute dünne `return web.X(...)`-Passthroughs in stack.go)
werden zu echten Implementierungen, die auf `app.Registrar()` bzw.
`app.Builder()` zeigen. `ActivityDef.Apply` ändert sich um eine Zeile:
`web.NewActivity(...).Apply(app)` → `way2goweb.NewActivity(...).Apply(app.Registrar())`.

`internal/build`s `BuildEngine` bekommt `*WebApp` statt `*Builder` injiziert
(`NewEngine(app *WebApp)`), `Serve()` ruft `app.Handler()` statt
`builder.BuildRouteHandler()`. Alle anderen `e.builder.X`-Aufrufe
(`DirSources`, `Assets`, `Styles`, `Components`, `NPMDeps`, `Manifest`)
bleiben unverändert, nur über `app.Builder()` erreicht.

### Namensfrage — entschieden (Richie, Review 26f441d7)

`stack.go` importiert nach dem Split zwei `web`-Pakete: way2gos
Runtime-`web` und stacks Builder-Rest. **Entscheidung: Builder-Rest
(`builder.go` + `assets.go`, ohne Routing ca. 300 Zeilen) wird in
`package stack` (Root) aufgelöst**, kein eigenes Package mehr — kein
Importalias, kein Namenskonflikt. `Builder` war ohnehin nur noch via
`stack.go` erreichbar. Gilt für Task 1.4.

## 4. Move-Grenze (Tabelle)

| Datei | Heute | Ziel Phase 1 | Bemerkung |
| --- | --- | --- | --- |
| `web/web.go` | `web` | way2go `web` | `Registry`, `WebActivity[C]`, `RegisterWebActivity`, `RuntimeContext`, `renderResult` — unverändert bis auf `WebActivity.Apply` (→ `Registrar`), neues `routeRegister`, und `Registry.assets` `AssetManifest` → `AssetLinks` (F1). **Nicht 1:1.** |
| `web/screen.go` | `web` | way2go `web` | 1:1, keine Builder-/Asset-Berührung. |
| `web/page.go` | `web` | way2go `web` | `renderAssetLinks` wechselt von `[]AssetRef` auf `[]string` + `withVersion` statt `AssetRef.URLsWithVersion` (F1). **Nicht 1:1.** |
| `web/dev.go` | `web` | way2go `web` | `withAssetManifest`/`assetManifestFromContext` wechseln Typ auf `AssetLinks` (F1). **Nicht 1:1.** |
| `web/module.go` — Gruppe a: `Part`, `Contributor`, `Compose`, `Mount` | `web` | way2go `web` | `Part.Apply` jetzt `Registrar`. Neu: `RouteAccumulator`, `Registrar`, `RouteActivity`. |
| `web/module.go` — Gruppe b: `CSS`, `TailwindCSS`, `Stencil`, `JS`, `File`, `TailwindScan`, `StencilScan`, `NPMDependency`, `NPMDevDependency` | `web` | **stack** (neue Datei, z.B. `stack/asset_parts.go`) | Nicht way2go — siehe R3 Befund 3. `Apply(*WebApp)`, nicht `Apply(*Builder)` (F3). |
| `web/app.go` (`WebApp`) | `web` | stack (Root-Package, siehe Namensfrage) | Reshaped: zwei Ziele (`runtime`, `builder`) statt einem. Öffentliche Methodenliste unverändert (Abschnitt 3). Neu: `Handler()` inkl. `toAssetLinks`-Konvertierung (F1). |
| `web/builder.go` | `web` | stack (Root-Package) | Minus Routing (`routes`, `mounts`, `routeRegistration`, `AddWebActivity`, `AddMount`, `BuildRouteHandler`, `registerRoutes`) — die gehen nach way2go `RouteAccumulator`. Rest (tailwind/stencil/assets/npm/dirSources) bleibt. |
| `web/assets.go` | `web` | stack (Root-Package) | 1:1, reine Typ-Aliase zu `stack/asset`. |
| `internal/build/build.go`, `capabilities.go` | stack | stack (Ort gleich) | `NewEngine(b *web.Builder)` → `NewEngine(app *WebApp)`; `Serve()` nutzt `app.Handler()` statt `builder.BuildRouteHandler()`. |
| `stack.go` | stack | stack (Ort gleich) | `type Part` wird eigenständig (kein Alias mehr, F3). `CSS`/`Mount`/`NPMDependency`/etc. werden echte Implementierungen statt Passthrough. `ActivityDef.Apply` eine Zeile geändert. |
| `assets/assets.go` | stack | stack (Ort gleich) | Unverändert — nutzt weiter `app.RegisterFile`/`app.RegisterDirSource`, die unverändert erreichbar bleiben. |
| `fortego-btc/app` (8 Dateien, siehe F2-Tabelle) | importiert `stack/web` | importiert way2go `web` | Nur Import-Repoint, keine Symbol-Änderung — **Task 1.7**, nicht 1.4. Voraussetzung: F1 gelöst, sonst brechen `Page`/`Screen`. |

## Nicht in diesem Task

Kein Code-Move, keine go.work/replace-Änderungen (1.6/1.7), kein
Verschieben von param/config/activity/cli (1.1–1.3). Das Repointen der 8
`fortego-btc/app`-Importe (F2) ist Task 1.7, hier nur kartiert.

## Zusammenfassung Rev. 2 — Status der offenen Punkte aus dem Review

- **F1 (Asset-Manifest-Zyklus): gelöst.** `AssetLinks` + `withVersion` in
  way2go, Konvertierung `asset.AssetManifest → AssetLinks` in
  `WebApp.Handler()`. `web/web.go`/`page.go`/`dev.go` als „nicht 1:1"
  markiert.
- **F2 (Artifact-Scope): korrigiert.** Falschaussage aus Rev. 1 entfernt,
  8-Dateien-Symbolkarte ergänzt, Konsequenz nach 1.7 verortet.
- **F3 (ein oder zwei `Part`): entschieden.** Genau ein `stack.Part` mit
  `Apply(*WebApp)`; Abschnitt 2 zeigt jetzt Closures über `*WebApp`, kein
  eigenes `Apply(*Builder)`-Interface mehr.
- Registrar/RouteAccumulator, R3-Vierer-Befund, Namensfrage: unverändert
  gegenüber Rev. 1, bereits gegengezeichnet.

## Rückfrage an Richie

Zuschnitt für 1.4 jetzt: `web/builder.go`, `web/web.go`, `web/page.go`,
`web/dev.go`, `web/module.go`-Split, `internal/build/build.go` — sechs
Dateien statt vier (F1 kommt dazu), plus `stack.go` (F3, kein Alias mehr).
Kein neuer Zyklus, keine Paket-Explosion. Vorschlag: Akzeptanzkriterium von
1.4 entsprechend fassen; 1.7 um die 8-Dateien-Importliste aus F2 ergänzen.
Warte auf dein OK für 1.1.

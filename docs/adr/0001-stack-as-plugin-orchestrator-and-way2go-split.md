# 1. Stack als Plugin-Orchestrator, App-Framework nach way2go

- Status: Accepted
- Datum: 2026-07-24
- Autoren: Adrian Pauli, Richie (Architect)

## Kontext

Der Cleanstartup Stack vereint heute zwei Verantwortungen in einem Modul:
die Orchestrierung von Build-Steps (Tailwind, Stencil) und ein
transport-agnostisches Application-Framework (Activities, Params, Middleware,
Web-/CLI-Runtime).

Tailwind und Stencil sind bereits als internes `capability.Capability`-Interface
modelliert (`Install/Build/Dev/Register`), aber unter `internal/` versteckt und
in `internal/build/capabilities.go` fest reingecodet. Die Aktivierung läuft über
Marker-Optionen (`assets.Tailwind()`) plus einen Type-Switch in `stack.go`. Neue
Build-Steps wie Hugo lassen sich damit nicht ohne Core-Änderung ergänzen.

**Primärer Treiber:** Erweiterbarkeit ohne Core-Änderung — ein neuer Build-Step
soll als externes Go-Modul dazukommen, ohne den Stack-Core anzufassen.

Der Plugin-Unterbau existiert also im Kern bereits; der Umbau ist evolutionär,
kein Rewrite. Sekundär wird die Trennung von Build-Orchestrierung (`stack`) und
Application-Framework (`way2go`) als eigenständiges Ziel mitgenommen.

## Entscheidung

### Zielarchitektur

```
way2go        (activity, param, cli, config, web-runtime)      — keine Build-Deps
   ▲
stack         (plugin-contract, build-engine, Bundle,          — importiert way2go
               WebApp/CLIApp, assets-descriptors,
               generischer Builder-Contract)
   ▲
tailwind  stencil  hugo   (je ein Modul, implementieren nur stack/plugin)
   ▲
Artifacts (fortego-app, affiliate-funnel, branding, …)  → stack + way2go + Plugins
```

Kein Zyklus: `stack` importiert nie ein Plugin — der Nutzer verdrahtet Plugins
explizit an `WebApp()`. Plugins hängen nur am `stack/plugin`-Contract.

### Entscheidungslog

| #   | Entscheidung          | Gewählt                                                                                              |
| --- | --------------------- | --------------------------------------------------------------------------------------------------- |
| D1  | Treiber               | Erweiterbarkeit ohne Core-Änderung                                                                   |
| D2  | Plugin-Aktivierung    | Explizit als Wert an `WebApp(hugo.Plugin(), …)`                                                      |
| D3  | Toolchain             | stack stellt geteilte Services (1 npm-Workspace + Binary-Provisioning) via `plugin.Context`         |
| D4  | way2go-Split          | In-Scope, dieser Umbau                                                                               |
| D5  | Dep-Richtung          | `stack → way2go`; `WebApp/CLIApp` bleiben Orchestrierung in stack                                    |
| D6  | Contract-Ort          | Öffentliches Paket im stack-Modul (`stack/plugin`)                                                   |
| D7  | First-Party-Plugins   | tailwind, stencil, hugo = echte separate Module (Contract-Dogfooding)                                |
| D8  | Repo-Layout           | Multi-Modul-Monorepo im heutigen stack-Repo; Split auf eigene Repos später                           |
| D9  | Modulpfade            | Repo-lokal vorerst (`github.com/cleanstartup/stack/…`); Rename-Schuld bewusst                        |
| D10 | Migration             | Harter Schnitt, keine BC-Fassade, Artifacts sofort umstellen                                         |
| D11 | `web`-Zerlegung       | 3-teilig: Runtime→way2go, generischer Builder-Contract→stack, tailwind/stencil-Parts + Routing→Plugins |
| D12 | Config-Ownership      | Plugin-spezifisches Env-Reading zieht ins Plugin; `stack.BuildConfig` verliert tailwind/stencil-Felder |
| OF2 | way2go-Inventar       | `activity`, `param`, `cli`, `config`, `web`-Runtime                                                  |

### Kern-Konsequenzen des Contracts

- `plugin.Context` reicht die geteilten Services **öffentlich** durch:
  npm-Workspace (`AddDependency`/`AddDevDependency`) und einen
  Binary-Provisioning-Helper (name/version/url → cached path).
- Plugin-spezifische Config wird vom Plugin selbst aus dem Environment gelesen.
  `stack`s `BuildConfig` kennt keine `TailwindBinary`/`StencilBinary`-Felder mehr.
- Ein aktives Plugin erhält die von Modulen deklarierten `assets.Dir()`-Quellen
  und trägt seine **eigene** Discovery bei. Das heute in `stack.go`
  (`expandDirSource`, `.css→tailwind` / `.tsx→stencil`) hartverdrahtete Routing
  wandert in die jeweiligen Plugins.

## Offene Flags

- **OF1 — Artifact-Wiring: AUFGELÖST (2026-07-25).** Build-tragend ist `replace`
  im Artifact-go.mod auf `./deps/stack/<nested>` (deckt den standalone Fly-Deploy,
  der die Superrepo-`go.work` nicht sieht); Workspace-`go.work` bleibt optionale
  Dev-Bequemlichkeit. Phase 1 zieht die Artifact-Migration (ehem. Phase 4) im
  harten Schnitt mit (P1-D1). Details + R1-Faktenlage:
  [0001-phase-1-tasks.md](./0001-phase-1-tasks.md).
- **OF3 — Repo-Split & Import-Rename:** Aus D9 folgt eine terminierte Schuld:
  beim späteren Split auf eigene Repos müssen die `github.com/cleanstartup/stack/…`
  Importpfade projektweit umbenannt werden.

## Umsetzungsstand

Phase 0 (Contract & Gerüst) und Phase 1 (way2go-Extraktion + `web`-Zerlegung)
sind umgesetzt — Design-Rationale in [0001-phase-0-tasks.md](./0001-phase-0-tasks.md),
[0001-phase-1-design-spike.md](./0001-phase-1-design-spike.md) und
[0001-phase-1-tasks.md](./0001-phase-1-tasks.md). Die Artifact-Migration ist in
Phase 1 aufgegangen (P1-D1).

tailwind/stencil als eigene Plugin-Module, ein hugo-Plugin und der spätere
Repo-Split (OF3) sind nicht begonnen. Planung und laufender Stand dazu gehören
ins Projekt-Backlog, nicht in diese ADR.

## Alternativen (verworfen)

- **way2go-Split weglassen:** Für D1 technisch nicht nötig — der Contract ist
  heute schon frei von templ/chi. Verworfen, weil der Split als eigenständiges
  Concern-Trennungsziel gewollt ist (D4).
- **Selbst-Registrierung via `init()`:** Versteckte Kopplung, Reihenfolge- und
  Testbarkeitsprobleme; widerspricht dem expliziten Stil des Stacks (D2).
- **Voll self-contained Plugins (eigene Toolchain je Plugin):** Doppelte
  `node_modules`, konkurrierende npm-Installs (D3).
- **Contract in eigenem Mini-Modul:** Sauberste Kopplung, aber ein zusätzliches
  versioniertes Modul; verworfen zugunsten weniger Module (D6).
- **BC-Fassade in stack:** Kleinerer Blast-Radius, aber Übergangs-Altlast; im
  Monorepo mit go.work ist der harte Schnitt atomar machbar (D10).

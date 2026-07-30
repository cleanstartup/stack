# ADR-0001 — Phase 1: way2go extrahieren + web-Zerlegung (Task-Breakdown)

- Status: Ready
- Datum: 2026-07-25
- Bezug: [0001-stack-as-plugin-orchestrator-and-way2go-split.md](./0001-stack-as-plugin-orchestrator-and-way2go-split.md),
  [0001-phase-0-tasks.md](./0001-phase-0-tasks.md)

Detaillierung von **Phase 1**. Ziel: das transport-agnostische App-Framework
(`activity`, `param`, `cli`, `config`, web-Runtime) nach **way2go** auslagern und
`web` entlang der Builder/Runtime-Naht zerschneiden — bei `stack → way2go` als
einziger Abhängigkeitsrichtung.

## Neue Entscheidungen (dieser Umbau)

| #     | Entscheidung        | Gewählt                                                                                      |
| ----- | ------------------- | ------------------------------------------------------------------------------------------- |
| P1-D1 | Sequencing          | **Harter Schnitt jetzt, Phase 4 mitziehen** — keine Fassade; Artifacts im selben Schwung.   |
| OF1   | Artifact-Wiring     | **`replace` im Artifact-go.mod → `./deps/stack/<nested>`** (build-tragend, deckt Fly-Deploy). Workspace-`go.work` nur optionale Dev-Bequemlichkeit. |
| D10   | Migration           | Bestätigt: harter Schnitt, keine BC-Aliase in stack.                                         |

## Findings, die Phase 1 zuschneiden

1. **Abhängigkeitsgraph der zu bewegenden Pakete** (nur cleanstartup/stack-intern):

   ```
   param    → (nichts)          Blatt
   config   → (nichts)          Blatt
   activity → param
   cli      → activity, param
   web      → activity, param, asset, plugin
   ```

   Move-Reihenfolge nach way2go: `param`, `config` → `activity` → `cli` → `web`
   (zuletzt, weil es asset/plugin berührt).

2. **Zyklus-Randbedingung.** `asset` und `plugin` liegen seit Phase 0 **im
   stack-Modul**. `stack → way2go` ist die einzige erlaubte Richtung. Daraus
   folgt hart: **alles, was `asset`/`plugin`/`Builder` berührt, muss in stack
   bleiben** — sonst `way2go → stack → way2go`.

3. **Die Verzahnung ist konzentriert, nicht diffus.** Entgegen erster Annahme ist
   `web/page.go` bereits entkoppelt: Page-Rendering zieht das Asset-Manifest über
   **Context** (`assetManifestFromContext`), nicht über `Builder`. Die einzige
   echte Runtime↔Builder-Kopplung sitzt in **`web/app.go`**: `WebApp` besitzt
   `*Builder`, und `RegisterCSS/RegisterTailwindCSS/RegisterStencil/RegisterJS/
   RegisterFile` delegieren alle an den Builder. `WebApp` ist damit die
   Kompositions-Wurzel, die Part-Komposition und Asset-Registrierung verbindet.

4. **`web` teilt sich sauber nach Datei — bis auf app.go/module.go:**
   - Builder-Seite (importiert nur `plugin`/`asset`): `web/builder.go`,
     `web/assets.go` → **stack**.
   - Runtime-Seite (importiert `activity`/`param`): `web/web.go`, `web/screen.go`,
     `web/module.go` (Part), `web/dev.go`, `web/page.go` → **way2go**.
   - Kompositions-Wurzel `web/app.go` (`WebApp`, `Part`-Anwendung, `Register*`)
     ist der zu **reshapende** Knoten.

## Zielaufteilung nach Phase 1

```
way2go/               (stack → way2go; keine asset/plugin/Builder-Deps)
  activity, param, cli, config
  web-runtime:  Router/Mux, Request-Decoding, param.Resolver(web),
                Screen/Element/Page (Manifest via Context), DevState, Mount
  runtime-registrar: schlankes Interface, an das Route-/Activity-Parts binden

stack/                (importiert way2go)
  Builder, assets (asset-source-Registrierung), web/builder.go+assets.go
  WebApp-Komposition: bridged way2go-Runtime + stack-Builder
  Bundle, WebApp(), CLIApp(), stack/plugin, stack/asset, stack/devwatch
```

## Zentrale Design-Aufgabe: Part/WebApp reshapen

`WebApp` und das `Part`-Modell sind heute _eine_ Naht für zwei Belange:
App-Definition (Routes, Activities → way2go) und Asset-Registrierung (CSS/JS/
Stencil → stack). Phase 1 muss diese zwei Anwendungsziele trennen:

- **Runtime-Registrierung** (Routes, Mounts, Activities-als-View) bindet an einen
  Runtime-Registrar in **way2go** — kennt keinen Builder.
- **Asset-Registrierung** (CSS/JS/Stencil/File/DirSource) bindet an den `Builder`
  in **stack**.
- Die `WebApp`-Kompositions-Wurzel bleibt in **stack** (berührt den Builder) und
  komponiert beide Ziele. stacks `Bundle`/`WebApp()` verdrahten way2go-Runtime +
  stack-Builder.

Das ist die eigentliche Arbeit — kein reiner File-Move. Deshalb **Task 1.0 = ein
Design-Spike**, der die Registrar-Interfaces und den neuen `Part`-Zuschnitt
festnagelt, bevor Code wandert.

## Tasks

| Task     | Inhalt                                                                                                                                   | Akzeptanz                                                                                   |
| -------- | -------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| **1.0**  | Design-Spike: Runtime-Registrar-Interface(s), neuer `Part`-Zuschnitt (Runtime- vs. Asset-Ziel), `WebApp`-Kompositionsvertrag. Als kurze ADR-Notiz. | ✅ **Fertig & abgenommen (Rev. 2, Review `26f441d7`/`3fde7c78`, 2026-07-29).** Ergebnis: [0001-phase-1-design-spike.md](./0001-phase-1-design-spike.md). |
| **1.1**  | `param`, `config` → way2go (Blätter, reiner Move + Import-Umbiegung).                                                                    | way2go baut; stack baut gegen way2go; Tests grün.                                            |
| **1.2**  | `activity` → way2go (hängt an param).                                                                                                    | dito.                                                                                        |
| **1.3**  | `cli` → way2go (hängt an activity/param).                                                                                                | `CLIApp`-Pfad unverändert funktional.                                                        |
| **1.4**  | ✅ **Fertig & abgenommen (2026-07-30, Commits `d6437a6`+`0021ac2`).** `web` zerschnitten: Runtime → way2go (`web.go`, `screen.go`, `page.go`, `dev.go`, `module.go`-Gruppe-a) + `Registrar`/`RouteAccumulator`/`AssetLinks`; Builder-Rest → **eigenes Package `webasset`** (`Builder` minus Routing, `WebApp`=`runtime`+`builder`, `Handler()`+`toAssetLinks`, Asset-Parts). **Korrektur zur Namensfrage:** NICHT in `package stack` (Root) wie im Spike abgenickt — das hätte `stack → assets → stack` erzeugt (`assets/assets.go` implementiert `Part`/braucht `WebApp`, `stack.go` importiert `assets`). `webasset` ist azyklisch. | Erfüllt: `way2go` modulrein (kein stack-Import), way2go/web asset-frei, beide Module `build`/`test -count=1` grün, altes `web/` weg, Commit 1 bisect-grün. |
| **1.5**  | stack-Root (`stack.go`, `ActivityDef`, `WebApp()`/`CLIApp()`) auf way2go-Typen umverdrahten. **Keine** Re-Export-Aliase (D10).           | Build/Dev/Serve unverändert; stack-Tests grün.                                               |
| **1.6**  | Pro Artifact: `deps/stack`-Submodul-Pointer auf den Phase-1-stack-Commit bumpen; `replace`-Zeilen für `way2go` (+ später Plugins) auf `./deps/stack/<nested>` ergänzen. Optional: Workspace-`go.work` für lokale Multi-Modul-Dev. | Artifact baut **standalone** (`go build ./...` ohne go.work) grün.                            |
| **1.7**  | Artifact-Migration auf way2go-Imports. **Realer Scope (2026-07-30, lokal verifiziert, breiter als die Spike-F2-Liste):** repointet werden ALLE Importer der verschobenen Pakete (`web`+`activity`/`cli`/`param`/`config`), nicht nur `stack/web`. Betroffen: **fortego-app** ~31 Dateien (reines Repoint), **affiliate-funnel** 2, **branding** nur go.mod, **cleanstartup/os** 2 (nachträglich aufgenommen, Adrian-Entscheid), **happend-store**/**fortego-ecies** nichts. **Regel:** jeder Stack-Consumer braucht `require`+`replace github.com/cleanstartup/stack/way2go => …/stack/way2go` in seiner eigenen go.mod — auch ohne direkten way2go-Import (Go wertet `replace` nur im Hauptmodul aus). happend-store-Cleanup (R2) out of scope. | jedes betroffene Repo `go build ./...` grün standalone; manueller E2E gegen fortego-app grün; Fly-Deploy baut ohne Workspace-Kontext. **Landet erst nach `new`-Push** (deps/stack-Pointer muss fetchbar sein); ein Commit pro Artifact/Repo. |

## Offene Risiken / zu bestätigende Annahmen

- **R1 — AUFGELÖST (2026-07-25).** Fakt aus `fortego-btc/app/.github/workflows/fly-deploy.yml`:
  der Deploy checkt nur das Artifact-Repo aus (`submodules: recursive`) und baut
  via `cmd/web/Dockerfile` auf Flys Remote-Builder — **Workspace-`go.work` ist
  dort unsichtbar.** Deshalb ist die build-tragende Auflösung `replace` im
  Artifact-go.mod auf `./deps/stack/<nested>` (Task 1.6), nicht go.work. Nebenbei:
  die CI regeneriert happend-store-sqlc selbst (bleibt so, R2 out of scope).
- **R2 — happend-store-Durability: KEIN Blocker, out of scope (2026-07-30).**
  Frühere Annahme „blockiert den fortego-app-E2E" war falsch: die Fly-CI hat
  einen `sqlc generate`-Schritt in `deps/happend-store`, fortego-app baut/deployed
  also auch ohne den committeten Fix. Das Cleanup (generierten Code committen,
  Pointer bumpen, CI-Schritt löschen) ist nice-to-have, nicht Teil von Phase 1.
  Echte externe Abhängigkeit für 1.7 ist stattdessen: **stack `new` pushen**,
  damit der `deps/stack`-Pointer committbar auf den Phase-1-Commit zeigen kann
  (nur fürs durable Landen nötig, nicht fürs lokale Arbeiten).
- **R3 — Reshape-Risiko app.go.** Wenn der Spike (1.0) zeigt, dass die
  Part/WebApp-Trennung breiter streut als app.go, wächst 1.4 — früh im Spike
  verifizieren.

## Nicht in Phase 1

- Registrierungs-Inversion `WebApp(tailwind.Plugin(), …)` + `compat.go`-Entkopplung
  → Phase 2.
- hugo-Plugin → Phase 3.
```

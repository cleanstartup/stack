# way2go

`way2go` is a small Go foundation for building activities, web handlers, and CLIs with a shared model.

## Packages

- `activity`: typed activity definitions, instances, URI building, and execution helpers
- `web`: HTTP registration, routing, and request decoding
- `cli`: command registration, parsing, and execution
- `auth`: optional session helpers for the core packages
- `config`: env-based config loading with `.env` support

## Import

```go
import (
	"github.com/cleanstartup/way2go/activity"
	"github.com/cleanstartup/way2go/web"
)
```

## Example

```go
package main

import (
	"fmt"

	"github.com/cleanstartup/way2go/activity"
	"github.com/cleanstartup/way2go/web"
)

type ShowParams struct {
	AccountID string
}

var showDef = activity.As[ShowParams, activity.NoInput]("account.show")

func Show(params ShowParams) activity.Instance[ShowParams, activity.NoInput] {
	return showDef.Take(params).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
		return "account=" + params.AccountID
	})
}

func main() {
	fmt.Println(Show(ShowParams{AccountID: "42"}).URI())

	r := web.NewRegistry()
	web.Register(r, showDef, Show)
}
```

## Web Runtime

`web` stellt eine zentrale Runtime mit modularem Build bereit:

- Module registrieren Activities und Assets dezentral.
- `web build` materialisiert Assets in `.way2go/public`.
- `web run` serviert nur fertige Assets und erwartet, dass `build` bereits gelaufen ist.
- `web dev` startet Tailwind- und Stencil-Watch-Worker, beobachtet deren Outputs und triggert nach dem Settling per SSE einen Browser-Reload.
- Lokale Dependency-Assets können Watch-Pfade mitbringen, z. B. über `web.FromFS(..., watchPath)` oder `web.WithWatchPaths(...)`.
- Das app-weite Styles-Set wird automatisch aus allen `*.css` Dateien im Go-Modul der aufrufenden App angewendet, egal ob sie direkt im Demo-/Paketverzeichnis oder in Unterordnern liegen.
- Der Styles-Build läuft zentral über einen Tailwind-Output (`/assets/css/app/app.css`). Das Tailwind-Binary wird automatisch heruntergeladen und im Cache abgelegt, wenn es nicht bereits verfügbar ist.
- Die Komponenten-Konvention greift automatisch alle `*.tsx` und `*.ts` Dateien im Go-Modul; Stencil-Komponenten und Hilfslogik können so quer durch das Modul organisiert werden, auch in einer flachen Struktur direkt unter dem jeweiligen Paket- oder Demo-Verzeichnis.
- Der Stencil-Build erzeugt einen zentralen JS-Output (`/assets/js/way2go/way2go.esm.js`). Das CLI wird bei Bedarf über `npm exec` nachgeladen; du kannst das via `WAY2GO_STENCIL_BINARY` überschreiben.
- Zusätzliche Tailwind-Scan-Pfade kannst du mit `web.TailwindScan(...)` registrieren.
- Die Demo zeigt das Styles-Default-Set über flach abgelegte `cmd/demo/*.css`-Dateien plus eine Stencil-Komponente mit TS-Helper im selben Verzeichnis. Das Tailwind-Binary wird automatisch geladen; du kannst es bei Bedarf über die `WAY2GO_TAILWIND_*`-Variablen überschreiben.
- Einfache Web-Activities können direkt `templ.Component` oder Text zurückgeben; den Seitentitel setzt du über `web.WithStaticTitle(...)`. Die HTML-Shell und die globalen CSS-/JS-Assets werden dabei von `web` automatisch injiziert. `web.Page` bleibt für Spezialfälle verfügbar.

`WAY2GO_TAILWIND_BINARY`, `WAY2GO_TAILWIND_VERSION`, `WAY2GO_TAILWIND_CACHE_DIR`, `WAY2GO_TAILWIND_DOWNLOAD_BASE` und `WAY2GO_STENCIL_BINARY` überschreiben die Default-Auflösung bei Bedarf.

## Demo

Zum Ausprobieren gibt es jetzt ein kleines Beispiel unter `cmd/demo`:

```bash
go run ./cmd/demo build
go run ./cmd/demo run
go run ./cmd/demo dev
```

Die Demo registriert eine Activity; Styles und Komponenten werden automatisch aus dem lokalen Demo-Modul übernommen. Für externe Dependencies gilt: `web.Styles(baseDir)` und `web.Components(baseDir)` brauchen einen passenden Modul-Root, damit `way2go` die CSS-/TS-/TSX-Dateien im Dependency-Checkout finden kann. Für die lokale Demo ist das nicht nötig; dort greifen die Defaults automatisch.

## Externe Module

Wenn ein Modul aus einer Dependency kommt, gib seinen Root explizit an, damit `way2go` die Assets findet. Die üblichen Bausteine sind:

- `web.Styles(baseDir)` für `*.css`
- `web.Components(baseDir)` für `*.ts` und `*.tsx`

Für das lokale Hauptpaket erkennt `web.App(...)` die Konventionen automatisch. Externe Module sollten ihre Parts dagegen selbst mit dem passenden Root zusammensetzen oder die Helper direkt mit einem Root aufrufen.

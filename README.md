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
- `web dev` rebuildet bei Änderungen per Polling und triggert per SSE einen Browser-Reload.
- Lokale Dependency-Assets können Watch-Pfade mitbringen, z. B. über `web.FromFS(..., watchPath)` oder `web.WithWatchPaths(...)`.
- Das app-weite Styles-Set wird automatisch aus `assets/css` der aufrufenden App angewendet; darin landen alle `*.css` Dateien im zentralen Tailwind-Build.
- Der Styles-Build läuft zentral über einen Tailwind-Output (`/assets/css/app/app.css`). Das Tailwind-Binary wird automatisch heruntergeladen und im Cache abgelegt, wenn es nicht bereits verfügbar ist.
- Die Komponenten-Konvention greift automatisch `assets/components`; dort landen `*.tsx` für Stencil-Komponenten und `*.ts` für Hilfslogik, die von Komponenten importiert werden kann.
- Der Stencil-Build erzeugt einen zentralen JS-Output (`/assets/js/way2go/way2go.esm.js`). Das CLI wird bei Bedarf über `npm exec` nachgeladen; du kannst das via `WAY2GO_STENCIL_BINARY` überschreiben.
- Zusätzliche Tailwind-Scan-Pfade kannst du mit `web.TailwindScan(...)` registrieren.
- Die Demo zeigt das Styles-Default-Set in `cmd/demo/assets/css/*.css` plus eine Stencil-Komponente in `cmd/demo/assets/components/*.tsx`. Das Tailwind-Binary wird automatisch geladen; du kannst es bei Bedarf über die `WAY2GO_TAILWIND_*`-Variablen überschreiben.
- Einfache Web-Activities können direkt `templ.Component` oder Text zurückgeben; den Seitentitel setzt du über `web.WithStaticTitle(...)`. Die HTML-Shell und die globalen CSS-/JS-Assets werden dabei von `web` automatisch injiziert. `web.Page` bleibt für Spezialfälle verfügbar.

`WAY2GO_TAILWIND_BINARY`, `WAY2GO_TAILWIND_VERSION`, `WAY2GO_TAILWIND_CACHE_DIR`, `WAY2GO_TAILWIND_DOWNLOAD_BASE` und `WAY2GO_STENCIL_BINARY` überschreiben die Default-Auflösung bei Bedarf.

## Demo

Zum Ausprobieren gibt es jetzt ein kleines Beispiel unter `cmd/demo`:

```bash
go run ./cmd/demo build
go run ./cmd/demo run
go run ./cmd/demo dev
```

Die Demo registriert eine Activity, während Styles und Komponenten automatisch aus `assets/css` bzw. `assets/components` der Demo übernommen werden; sie gibt direkt ein `templ.Component` zurück und nutzt dieselbe zentrale `web`-Runtime wie die spätere Anwendung.

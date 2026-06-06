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
- Normale CSS-Assets werden direkt als `<link>` ausgeliefert.
- Tailwind-Assets laufen zentral durch einen Tailwind-Build-Output (`/assets/css/app/app.css`). Das Tailwind-Binary wird automatisch heruntergeladen und im Cache abgelegt, wenn es nicht bereits verfügbar ist.
- Zusätzliche Tailwind-Scan-Pfade kannst du mit `b.TailwindScan(...)` registrieren.
- Die Demo zeigt beides: direktes CSS in `cmd/demo/assets/css/site.css` plus Tailwind-Fragmente in `cmd/demo/assets/css/*.tailwind.css`. Das Binary wird automatisch geladen; du kannst es bei Bedarf über die `WAY2GO_TAILWIND_*`-Variablen überschreiben.
- `web.Page` rendert eine minimale HTML-Shell und kann `templ.Component` als Body verwenden; CSS/JS werden global aus dem registrierten Asset-Manifest injiziert.

`WAY2GO_TAILWIND_BINARY`, `WAY2GO_TAILWIND_VERSION`, `WAY2GO_TAILWIND_CACHE_DIR` und `WAY2GO_TAILWIND_DOWNLOAD_BASE` überschreiben die Default-Auflösung bei Bedarf.

## Demo

Zum Ausprobieren gibt es jetzt ein kleines Beispiel unter `cmd/demo`:

```bash
go run ./cmd/demo build
go run ./cmd/demo run
go run ./cmd/demo dev
```

Die Demo registriert eine Activity plus CSS-, Tailwind- und JS-Assets direkt über `web.Run(web.ConventionalAssets(), web.Simple(...))`, rendert eine `web.Page` und nutzt dieselbe zentrale `web`-Runtime wie die spätere Anwendung.

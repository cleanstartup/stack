# stack

`stack` is a small Go foundation for building activities, web handlers, CLIs, and Hugo-backed sites with a shared model.

## Packages

- `activity`: typed activity definitions, instances, URI building, and execution helpers
- `web`: HTTP registration, routing, asset pipelines, and request decoding
- `cli`: command registration, parsing, help text, and execution
- `config`: env-based config loading with `.env` support

## Import

```go
import (
	"github.com/cleanstartup/stack/activity"
	"github.com/cleanstartup/stack/web"
)
```

`web` exposes two target entrypoints:

- `web.App(...)` for Go web apps with Tailwind and Stencil
- `web.Site(...)` for static sites rendered by Hugo with the same Tailwind and Stencil asset pipeline

## Example

```go
package main

import (
	"fmt"

	"github.com/cleanstartup/stack/activity"
	"github.com/cleanstartup/stack/web"
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

`web` provides a central runtime with a modular build pipeline:

- Modules register activities and assets independently.
- All CLI entry points support `--help`; unknown commands print help instead of only failing. Calling the binary without a command also shows help.
- Command help text can be declared declaratively with `cli.WithHelp("...")` on each activity.
- `web build` materializes assets into `.stack/public`.
- `web run` serves already-built assets and expects `build` to have run first.
- `web dev` starts a `templ` supervisor for Go/templ changes and combines it with Tailwind and Stencil watch workers for the asset pipeline. The browser runs through the templ proxy; CSS/JS changes are still handled through the internal dev reload.
- Web commands have their own help text: `run`, `build`, and `dev` explain themselves via `--help` and show up in the CLI listing.
- Local dependency assets can bring watch paths, for example through `web.FromFS(..., watchPath)` or `web.WithWatchPaths(...)`.
- The app-wide style set is collected automatically from all `*.css` files in the calling Go module, whether they live directly in the demo/package directory or deeper in subdirectories.
- The styles build runs through a single Tailwind output (`/assets/css/app/app.css`). The Tailwind binary is downloaded automatically and cached if it is not already available.
- The component convention automatically picks up all `*.tsx` and `*.ts` files in the Go module; Stencil components and helper logic can therefore be organized anywhere in the module, including a flat layout directly under a package or demo directory.
- The Stencil build produces a central JS output (`/assets/js/stack/stack.esm.js`). The CLI is fetched on demand through `npm exec`; you can override that via `STACK_STENCIL_BINARY`.
- Additional Tailwind scan paths can be registered with `web.TailwindScan(...)`.
- The demo shows the default style set through flat `cmd/demo/*.css` files plus a Stencil component with a TS helper in the same directory. The Tailwind binary is downloaded automatically; you can override it via the `STACK_TAILWIND_*` variables.
- Simple web activities can return `templ.Component` or plain text directly; set the page title with `web.WithStaticTitle(...)`. The HTML shell and the global CSS/JS assets are injected automatically by `web`. `web.Page` remains available for special cases.

`STACK_TAILWIND_BINARY`, `STACK_TAILWIND_VERSION`, `STACK_TAILWIND_CACHE_DIR`, `STACK_TAILWIND_DOWNLOAD_BASE`, and `STACK_STENCIL_BINARY` override the default resolution when needed. The dev mode uses `templ generate --watch --proxy=... --cmd=...` as a supervisor for Go/templ files; the inner child mode (`--child`) is responsible for the asset watchers.

## Demo

A small example lives in `cmd/demo`:

```bash
go run ./cmd/demo build
go run ./cmd/demo run
go run ./cmd/demo dev
```

The demo registers a single activity; styles and components are picked up automatically from the local demo module. For external dependencies, `web.Styles(baseDir)` and `web.Components(baseDir)` need an explicit module root so `stack` can discover the CSS/TS/TSX files in the dependency checkout. For the local demo that is not necessary; the defaults are applied automatically.

A Hugo-backed site demo lives in `cmd/site`:

```bash
go run ./cmd/site build
go run ./cmd/site run
go run ./cmd/site dev
```

The site demo renders content through Hugo while using the same Tailwind and Stencil asset pipeline.
If Hugo is not already installed, the stack will build it on demand via `go install` and cache the binary locally.
`go run ./cmd/site dev` starts `hugo server` and keeps the generated CSS and JS mirrored into `cmd/site/static/assets`, so Hugo can serve them directly during development.

Environment overrides:

- `STACK_HUGO_BINARY` to use a preinstalled binary
- `STACK_HUGO_VERSION` to pin the Hugo version built via Go
- `STACK_HUGO_CACHE_DIR` to change the local Hugo cache location

Generated site outputs are ignored by Git:

- `cmd/site/public/`
- `cmd/site/.stack/`
- `cmd/site/.hugo_build.lock`
- `cmd/site/static/assets/`

## External Modules

If a module comes from a dependency, pass its root explicitly so `stack` can find the assets. The usual building blocks are:

- `web.Styles(baseDir)` for `*.css`
- `web.Components(baseDir)` for `*.ts` and `*.tsx`

For the local main package, `web.App(...)` discovers the conventions automatically. External modules should compose their parts with the appropriate root or call the helpers directly with a root.
`web.Site(...)` follows the same asset conventions but renders the page tree through Hugo.

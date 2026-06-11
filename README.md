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

The root `stack` package exposes declarative modules:

- `stack.Bundle(...)` bundles multiple `Part`s into a reusable module.
- `Module().WebApp(...)` starts the app-specific CLI for `install`, `build`, `dev`, and `run`.

`stack.Content(baseDir, patterns...)` materializes matching Markdown files into a temporary Hugo module, and `stack.Layouts(baseDir, patterns...)` does the same for Hugo layout and type files.

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
- `install` materializes toolchain metadata and dependency state in the active module package.
- `build` materializes assets into `.assets` and builds the current command binary into `cmd/<name>/.bin/<name>`.
- `run` serves already-built assets and expects `build` to have run first.
- `dev` starts asset watch workers and the current command runtime.
- Web commands have their own help text: `run`, `build`, and `dev` explain themselves via `--help` and show up in the CLI listing.
- Local dependency assets can bring watch paths, for example through `web.FromFS(..., watchPath)` or `web.WithWatchPaths(...)`.
- `web.Mount(path, handler)` propagates the current asset manifest and dev-state context into mounted routers, and preserves the original request path so sub-apps such as auth flows can still render `web.Page` responses with the correct CSS/JS bundles and API endpoints.
- The app-wide style set is collected from declared Tailwind source directories and their `*.tailwind.css` files.
- The styles build runs through a single Tailwind output (`/assets/css/app/app.css`). The Tailwind binary is downloaded automatically and cached if it is not already available.
- The component convention picks up declared `*.stencil.ts` and `*.stencil.tsx` files.
- `stack.Content(baseDir, patterns...)` materializes matching content files into a temporary Hugo module so repo-local docs can live next to the source while still rendering through the site target.
- `stack.Layouts(baseDir, patterns...)` materializes matching Hugo layout files into a temporary Hugo module so consumer repos can declare site templates explicitly.
- The Stencil build produces a central JS output (`/assets/js/stack/stack.esm.js`). The CLI is fetched on demand through `npm exec`; you can override that via `STACK_STENCIL_BINARY`.
- Additional Tailwind scan paths can be registered with `web.TailwindScan(...)`.
- The demo declares styles and components from its importable root package and executes them through `cmd/app`. The Tailwind binary is downloaded automatically; you can override it via the `STACK_TAILWIND_*` variables.
- Simple web activities can return `templ.Component` or plain text directly; set the page title with `web.WithStaticTitle(...)`. For Stencil-backed screens, `web.Screen(name, props)` renders a `screen-*` custom element and serializes props for hydration. Use `showcase.md` alongside the screen to document default props and variants. The HTML shell and the global CSS/JS assets are injected automatically by `web`. `web.Page` remains available for special cases.

`STACK_TAILWIND_BINARY`, `STACK_TAILWIND_VERSION`, `STACK_TAILWIND_CACHE_DIR`, `STACK_TAILWIND_DOWNLOAD_BASE`, and `STACK_STENCIL_BINARY` override the default resolution when needed. The dev mode uses the local `templ generate --watch --proxy=... --cmd=...` supervisor for Go/templ files; the inner child mode (`--child`) is responsible for the asset watchers.

## Demo

A small example now lives in the sibling module `../demo`:

```bash
cd ../demo
go run ./cmd/app install
go run ./cmd/app build
go run ./cmd/app run
go run ./cmd/site
```

The demo root package exposes `demo.Module()`. The app command executes that module as a WebApp and owns `install`, `build`, `dev`, and `run`.

If Hugo is not already installed, the stack will build it on demand via `go install` and cache the binary locally. The default Hugo version is pinned to `0.162.1` in code and resolved as the Hugo module tag `v0.162.1`. It can be overridden when needed.

Environment overrides:

- `STACK_HUGO_BINARY` to use a preinstalled binary
- `STACK_HUGO_VERSION` to override the pinned Hugo version built via Go. You can pass either `0.162.1` or `v0.162.1`.
- `STACK_HUGO_CACHE_DIR` to change the local Hugo cache location

## External Modules

If a module comes from a dependency, pass its root explicitly so `stack` can find the assets. The usual building blocks are:

- `stack.WithTailwindStyles(baseDir)` for `*.tailwind.css`
- `stack.WithStencilComponents(baseDir)` for `*.stencil.ts` and `*.stencil.tsx`
- `stack.Bundle(...)` to bundle multiple parts into one reusable module
- `stack.Content(baseDir, patterns...)` for Markdown content
- `stack.Layouts(baseDir, patterns...)` for Hugo layouts and types

Commands should execute a module with `Module().WebApp(...)`. External modules compose their parts into the active module; generated metadata is written to the module on which `WebApp` is called.

The site contract and Hugo conventions now live in `docs/site/`:

- [`docs/site/architecture.md`](docs/site/architecture.md)
- [`docs/site/hugo.md`](docs/site/hugo.md)

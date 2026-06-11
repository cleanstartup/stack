# Cleanstartup Stack

The Cleanstartup Stack is an opinionated build and runtime system for Go-based applications.

It uses:

- Go for application structure and orchestration
- Tailwind for styling
- Stencil for Web Components
- npm-compatible tooling for frontend builds

The stack is bundle-first: Go packages expose reusable Stack modules, and commands execute those modules as concrete applications.

## Modules

A Go package may expose one Stack module:

    package portal

    func Module() stack.Module {
        return stack.Bundle(
            auth.Module(),
            billing.Module(),
            stack.WithTailwindStyles("styles"),
            stack.WithStencilComponents("components"),
            stack.Activity(...),
        )
    }

This `Module()` function is the package's declarative SDK surface.

A module may contribute:

- dependency modules
- Tailwind sources
- Stencil components
- static assets
- application features
- activities, routes, and screens

Best practice: expose one `Module()` per Go package. If a package deliberately exposes multiple modules, callers must understand that each module is a separate composition root when executed.

## Composition Root

The module on which an app method is called is the composition root:

    func main() {
        portal.Module().WebApp(
            tailwind.Include(),
            stencil.Include(),
        )
    }

The composition root owns generated metadata and asset outputs.

If `portal.Module()` imports `auth.Module()`, the transitive asset inputs and npm dependencies from `auth` are aggregated into the `portal` build. The dependency package is not modified.

## Package Layout

Recommended layout:

    portal/
      module.go
      styles/
      components/
      package.json
      package-lock.json
      tailwind.input.css
      stencil.config.ts
      tsconfig.json
      .assets/

    cmd/
      portal/
        main.go
        .bin/
          portal

The root package remains importable. Executable entrypoints live below `cmd/*`.

## Source Files

Source files always remain at their original location.

The stack never copies source files into a build workspace.

Examples:

    styles/
      app.tailwind.css

    components/
      button/
        button.stencil.tsx

A dependency module may contribute source files from the Go module cache:

    $GOMODCACHE/
      github.com/vendor/auth@v1.2.3/
        styles/
          auth.tailwind.css
        components/
          login-form.stencil.tsx

Generated configuration references these original files.

This ensures:

- no duplicated sources
- no broken relative references
- no asset synchronization problems
- native watch support

## Capabilities

Modules contribute capabilities.

A capability has four lifecycle hooks:

- `Install`
- `Build`
- `Dev`
- `Register`

`Install` prepares generated metadata and dependencies.

`Build` produces deployable outputs.

`Dev` watches and rebuilds outputs where supported.

`Register` installs the capability into the active target. For a WebApp this means registering CSS and JavaScript outputs with the app manifest.

Asset builders are explicitly enabled by the command:

    portal.Module().WebApp(
        tailwind.Include(),
        stencil.Include(),
    )

If Tailwind is included, Tailwind inputs from the active module graph are built.

If Stencil is included, Stencil inputs from the active module graph are built.

Generated metadata is written by capabilities to the composition root:

    package.json
    package-lock.json
    tailwind.input.css
    stencil.config.ts
    tsconfig.json

Build outputs are written to:

    .assets/

Built asset capabilities register their runtime outputs with the active target.

For a WebApp target:

- Tailwind registers `/assets/css/app/app.css`
- Stencil registers `/assets/js/stack/stack.esm.js`

Manual includes such as `stack.WithCSS(...)` and `stack.WithJS(...)` are only needed for externally provided assets.

### npm

The npm capability receives dependency inputs from other capabilities.

It writes `package.json` and `package-lock.json` during `Install`, then runs the package manager as needed.

It does nothing during `Build`, `Dev`, and `Register`.

### Tailwind

The Tailwind capability receives CSS source inputs.

It writes `tailwind.input.css` during `Install`, builds `app.css` during `Build`, watches during `Dev`, and registers the built stylesheet with WebApp targets.

### Stencil

The Stencil capability receives TypeScript and TSX component inputs.

It contributes npm dependencies, writes `stencil.config.ts` and `tsconfig.json` during `Install`, builds the Stencil distribution during `Build`, watches during `Dev`, and registers the Stencil loader with WebApp targets.

## App CLI

`Module().WebApp(...)` starts the CLI for that concrete app.

Commands:

    go run ./cmd/portal install
    go run ./cmd/portal build
    go run ./cmd/portal dev
    go run ./cmd/portal run

### install

Prepares the composition root:

- generate package metadata
- generate TypeScript configuration
- generate Tailwind input metadata when Tailwind is included
- generate Stencil configuration when Stencil is included
- install npm dependencies

### build

Builds everything required for deployment:

- run install if required
- build enabled assets into `.assets/`
- build the current command binary into `cmd/<name>/.bin/<name>`

### dev

Starts a local development workflow:

- run install if required
- start native Tailwind and Stencil watchers for enabled builders
- start the current command runtime through its `run` subcommand
- write rebuilt assets continuously to `.assets/`

### run

Starts the runtime using already built assets.

## Responsibilities

Modules:

- define dependency modules
- define asset inputs
- define features and activities

Composition root:

- aggregates transitive module inputs
- owns generated metadata
- owns `.assets/`

Commands:

- choose a module
- choose runtime/build capabilities
- expose `install`, `build`, `dev`, and `run`

Runtime:

- reads configuration from environment variables
- serves the app and already built assets

The central idea is:

The app command executes a module graph, and the executed root module owns the generated build state.

# Bundle Architecture

Stack is bundle-first.

A Go package exposes one reusable module:

    func Module() stack.Module

Commands execute that module:

    portal.Module().WebApp(
        tailwind.Include(),
        stencil.Include(),
    )

## Concepts

Module:

- reusable declaration of features, activities, assets, and dependency modules
- owns local source paths such as `styles/` and `components/`
- remains importable as a normal Go package

Composition root:

- the module on which `.WebApp(...)` is called
- aggregates all transitive module inputs
- owns generated metadata and `.assets`

Command:

- executable entrypoint below `cmd/*`
- exposes `install`, `build`, `dev`, and `run`
- writes its binary to `cmd/<name>/.bin/<name>`

Capability:

- has `Install`, `Build`, `Dev`, and `Register` lifecycle hooks
- is enabled explicitly by options such as `tailwind.Include()` and `stencil.Include()`
- reads source files in place
- writes generated metadata to the composition root
- registers runtime outputs with the active target

Examples:

- npm installs dependency metadata and otherwise has no runtime registration
- Tailwind builds CSS and registers it with WebApp targets
- Stencil builds JS and registers it with WebApp targets

## Paths

Relative module sources are resolved where they are declared:

    stack.WithTailwindStyles("styles")
    stack.WithStencilComponents("components")

Generated files are written to the composition root:

    package.json
    package-lock.json
    tailwind.input.css
    stencil.config.ts
    tsconfig.json
    .assets/

Command binaries are written next to the command:

    cmd/portal/.bin/portal

## Commands

    go run ./cmd/portal install
    go run ./cmd/portal build
    go run ./cmd/portal dev
    go run ./cmd/portal run

`build` always builds enabled assets and the current command binary.

`dev` starts enabled asset watchers and restarts/runs the current command through its `run` subcommand.

## Go Dependencies

The Go module system is the source of truth for module dependencies:

- `go.mod`
- `go.work`
- `replace` directives
- module cache paths

Stack should reference source files directly and avoid copying source trees. Copying is only acceptable for tool-specific generated metadata or for tools that cannot operate on source paths directly.

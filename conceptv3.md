# Cleanstartup Stack

The Cleanstartup Stack is an opinionated build system for Go-based applications.

It uses:

- Go for application structure and orchestration
- Tailwind for styling
- Stencil for Web Components
- Hugo for content generation
- npm-compatible tooling for frontend builds

The stack separates Modules, Asset Sources and Targets.

## Modules

A Module defines:

- a namespace
- asset source directories
- asset dependencies
- reusable application features

Example:

    func Module() stack.Module {
        return stack.Module(
            "fortego.auth",

            module.AssetSources(
                assets.Dir("client"),
                assets.Dir("content"),
                assets.Use("embla-carousel", "^8.0.0"),
            ),

            auth.Module(),
        )
    }

A Module should expose a single:

    func Module() stack.Module

per Go package.

The namespace uniquely identifies the Module.

It is used for:

- Activity IDs
- Asset Source namespaces
- Analytics
- Logging
- Routing
- Manifests

## Activities

Activities define user-visible capabilities.

Activities are identified relative to the Module namespace.

Example:

    activity.Name("EmailOtpLogin")

Produces:

    fortego.auth.EmailOtpLogin

The Module provides the namespace.
Activities only define their local name.

## Asset Sources

Asset Sources are build inputs used to produce deliverables.

Typical examples include:

- CSS
- TSX
- JavaScript
- Markdown
- Images
- Fonts

Example:

    client/
      components/
        carousel.tsx
        carousel.css

      assets/
        logo.svg

    content/
      docs/
        getting-started.md

Asset source directories must be subdirectories of the Module package.

Asset dependencies are declared alongside Asset Sources:

    module.AssetSources(
        assets.Dir("client"),
        assets.Dir("content"),
        assets.Use("embla-carousel", "^8.0.0"),
    )

Asset dependencies are used by Asset Sources and are materialized as npm dependencies during build.

The stack automatically processes supported files:

    *.css   -> Tailwind
    *.tsx   -> Stencil
    *.md    -> Hugo

Additional Asset Builders may be enabled by a Target.

Files that should be copied without processing are declared explicitly:

    assets.StaticDir("assets")

## Targets

A Target instantiates a Module and activates Asset Builders and Features.

Example:

    func main() {
        Module().WebApp(
            assets.Tailwind(),
            assets.Stencil(),
            assets.Hugo(),

            auth.EmailOtpAuth(),
        )
    }

Future targets may use the same Asset Sources differently:

    Module().Pdf(
        assets.Hugo(),
        assets.WeasyPrint(),
    )

The Module defines what is available.

The Target defines what gets built.

## Build Process

During build, the stack:

1. Creates a target-specific source workspace.
2. Copies all configured Asset Sources into that workspace.
3. Generates package.json and build configuration.
4. Installs npm dependencies.
5. Runs the enabled Asset Builders.
6. Writes outputs to `.assets`.

Example:

    cmd/
      app/
        main.go

        .sources/
          fortego/
            auth/
              client/
              content/

        .assets/
          styles.css
          components.js
          docs/
            getting-started.html

The namespace determines the structure inside `.sources`.

Example:

    stack.Module(
        "fortego.auth",

        module.AssetSources(
            assets.Dir("client"),
            assets.Dir("content"),
        ),
    )

Produces:

    .sources/
      fortego/
        auth/
          client/
          content/

`.sources` contains synchronized Asset Sources for the Target.

`.assets` contains generated build outputs.

## Runtime

Targets embed and serve generated assets.

Example:

    //go:embed .assets/*
    var assets embed.FS

    func main() {
        Module().WebApp(
            stack.Assets(assets),

            assets.Tailwind(),
            assets.Stencil(),
            assets.Hugo(),

            auth.EmailOtpAuth(),
        )
    }

Targets do not define:

- npm dependencies
- package.json
- build configuration
- asset source locations

Those concerns belong to the Module.

## Configuration

Configuration always comes from environment variables.

No defaults are defined in code.

Development defaults may be provided through:

    .env

Production defaults may be provided through:

    Dockerfile ENV

or deployment-specific environment variables.

Missing required configuration results in a startup error.

## Commands

### install

Purpose:

Prepare the build environment.

Command:

    go run artifact.go install

Actions:

- create `.sources`
- synchronize Asset Sources
- generate package.json
- generate TypeScript configuration
- generate build configuration
- install npm dependencies

The install command materializes all metadata and dependencies required for building the Target.

### build

Purpose:

Produce a production build of a Target.

Command:

    go run artifact.go build app

Actions:

- run install if required
- build CSS assets
- build JavaScript assets
- build content
- collect static assets
- generate `.assets`
- build the Go binary

The build command produces everything required for deployment.

### dev

Purpose:

Start a development environment for a Target.

Command:

    go run artifact.go dev app

Actions:

- run install if required
- start Tailwind watchers
- start Stencil watchers
- start Hugo watchers
- rebuild assets on change
- update `.assets`
- start the application in development mode

The dev command provides a complete local development workflow.

## Principles

- One Go package should expose at most one Module().
- Every Module must define a namespace.
- Asset Sources are declared by the Module.
- Asset Builders are activated by the Target.
- Asset source directories must be subdirectories of the Module package.
- Asset Sources are descriptors. Filesystem access occurs only during `build` and `dev`. The `run` command never accesses source paths.
- `.sources` contains synchronized build inputs.
- `.assets` contains build outputs.
- package.json is generated from Asset Sources.
- Configuration comes from ENV.
- Go remains the single source of truth.

# Cleanstartup Stack

The Cleanstartup Stack is an opinionated build system for Go-based applications.

It uses:

- Go for application structure and orchestration
- Tailwind for styling
- Stencil for Web Components
- npm-compatible tooling for frontend builds

The stack separates Artifact, Modules and Apps.

## Artifact

The Artifact defines how the repository is built.

It is declared in `artifact.go`:

    func main() {
        stack.Artifact(
            supertokens.Module(config),
            posthog.Module(config),
        ).Run()
    }

An Artifact defines:

- modules
- npm dependencies
- CSS sources
- TSX sources
- static assets
- build configuration

The Artifact is responsible for producing build outputs.

## Modules

Modules provide reusable functionality.

A module may contribute:

- npm dependencies
- Tailwind sources
- Stencil components
- static assets
- application features

Example:

    supertokens.Module(...)

A module may expose features such as:

    supertokens.EmailOtpAuth()
    supertokens.PasswordAuth()
    supertokens.AdminScreen()

Modules define capabilities.

Apps choose which capabilities to use.

## Source Files

Source files always remain at their original location.

The stack never copies source files into a build workspace.

Examples:

    styles/
      app.tailwind.css

    components/
      button/
        button.stencil.tsx

    static/
      logo.svg

A module dependency may also contribute source files from the Go module cache.

Example:

    $GOMODCACHE/
      github.com/vendor/auth@v1.2.3/
        styles/
          auth.tailwind.css
        components/
          login-form.stencil.tsx

The stack generates build configuration that references these original files.

This ensures:

- no duplicated sources
- no broken relative references
- no asset synchronization problems
- native watch support

## Generated Metadata

The stack generates metadata required by frontend tooling.

Example:

    stack.generated/
      apps/
        app/
          tailwind.input.css
          tailwind.config.ts
          stencil.config.ts

Generated files reference original source locations.

The generated metadata is disposable and may be recreated at any time.

## Asset Build

The Artifact build process generates all frontend assets.

Outputs are written to:

    .assets/<app>/

Example:

    .assets/
      app/
        manifest.json
        styles.css
        components.js
        public/

The generated assets are considered build artifacts and are consumed by Apps.

## Apps

Apps define runtime behaviour.

Example:

    //go:embed .assets/*
    var assets embed.FS

    func main() {
        stack.WebApp(
            stack.Assets(assets),
            supertokens.EmailOtpAuth(),
            posthog.PageViewTracking(),
        )
    }

Apps:

- embed built assets
- activate features
- define routes and screens
- start the runtime

Apps do not define:

- npm dependencies
- Tailwind configuration
- Stencil configuration
- frontend build pipelines

Those concerns belong to the Artifact.

## Configuration

Configuration always comes from environment variables.

No defaults are defined in code.

Development defaults may be provided through:

    .env

Production defaults may be provided through:

    Dockerfile ENV

or deployment-specific environment variables.

Missing required configuration results in a startup error.

## Development Mode

The stack uses the native watch capabilities of the underlying tools.

Examples:

- Tailwind CLI watch mode
- Stencil watch mode
- Go source watcher
- static asset watcher

The stack acts as a process orchestrator.

It starts and manages all required watch processes for a given App.

Source files remain at their original location.

Generated assets are continuously written to:

    .assets/<app>/

## Commands

### install

Purpose:

Prepare the build environment.

Command:

    go run artifact.go install

Actions:

- generate package.json
- generate TypeScript configuration
- generate Tailwind configuration
- generate Stencil configuration
- install npm dependencies

The install command materializes all metadata and dependencies required for building the Artifact.

### build

Purpose:

Produce a production build of an App.

Command:

    go run artifact.go build app

Actions:

- run install if required
- build CSS assets
- build JavaScript assets
- collect static assets
- generate .assets/<app>
- build the Go binary

The build command produces everything required for deployment.

### dev

Purpose:

Start a development environment for an App.

Command:

    go run artifact.go dev app

Actions:

- run install if required
- start Tailwind watchers
- start Stencil watchers
- start static asset watchers
- rebuild assets on change
- write outputs to .assets/<app>
- start the application in development mode

The dev command provides a complete local development workflow.

## Responsibilities

Artifact

- defines build inputs
- defines dependencies
- defines modules
- builds assets

Modules

- provide assets
- provide features

Build

- generates metadata
- generates .assets

Apps

- embed .assets
- activate features
- run the application

Runtime

- provides configuration through ENV

The central idea is:

The Artifact builds assets.
Apps consume assets.
Source files remain at their original location.
Configuration comes from ENV.
Go remains the single source of truth for structure and build configuration.

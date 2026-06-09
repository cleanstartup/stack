# Target Architecture

This document captures the target-and-asset architecture we want for `stack`.

## Core Idea

The system should be defined declaratively as a graph of targets and assets.

- A `Target` is an executable unit or process boundary.
- An `Asset` is a buildable artifact that targets can consume.
- Targets may depend on other targets.
- Targets may declare which asset types they consume.
- Assets may be collected across dependency graphs and built once per asset type.
- The declaration should describe structure and dependencies, not imperative execution.

The main goal is to keep the user-facing API declarative:

- define targets in `main`
- attach child targets and asset requirements
- let the runtime interpret the graph

There should be no `Run()` call at the end of the declarative definition.

## Target

`Target` means something executable or process-oriented, for example:

- Webapp
- Static app with Go webserver
- CLI

Targets describe:

- how they compose other targets
- which assets they consume
- which assets they declare directly
- which runtime mode they support
- which input sources they need for their own behavior

Targets do not own asset compilation details. They only depend on built asset outputs.

## Target Workspace

Each target has a dedicated workspace rooted at `cmd/<target>/`.

That directory is treated as the target's project root for all non-Go toolchains:

- `package.json`
- `package-lock.json`
- `tsconfig.json`
- `stencil.config.ts`
- other tool-specific config files as needed

The workspace should look and behave as if the target lived in its own repository.
This is important for:

- editor and language-server discovery
- local dependency resolution
- lockfile-based pinning
- tool-specific conventions that expect project-root config files

The Go entrypoint remains `cmd/<target>/main.go`, but the surrounding directory is also
the synthetic root for asset tooling.

## Asset

`Asset` means a buildable artifact type, for example:

- CSS from Tailwind
- JS from Stencil
- static files
- content or generated markup where appropriate

Assets are grouped by type and built centrally across all loaded modules and dependencies.

Typical asset outputs:

- `dist.css`
- `dist.js`
- type-specific derived files

Targets then include and use those built asset outputs.

## Mode

The architecture distinguishes between:

- `Target`: what is being executed
- `Mode`: how the target is executed

Typical modes:

- `build`
- `dev`

The same target should usually reuse the same context across modes.
Mode-specific behavior belongs to the runtime or the target implementation, not to separate config models unless absolutely necessary.

## Content and Sources

I/O inputs should stay explicit and relatively direct.

Useful source categories:

- repository source files
- content files
- layout/template files
- asset source files
- generated entrypoints

The preferred model is to watch real source files directly whenever possible.
Targets and assets may expose:

- source roots
- include patterns
- exclude patterns
- generated entrypoints

This allows the dev runtime to:

- watch real repository files
- regenerate only the necessary entry files
- point tools at the correct source locations
- avoid copying whole source trees unless required

## Go Dependency Handling

For Go-based targets and assets, the Go module system should be the primary dependency mechanism.

The runtime should rely on:

- `go.mod`
- `go.work`
- `replace` directives

In the common case, Go sources do not need to be copied into a separate workspace. The runtime can execute tools directly against the real repository layout as long as the module/workspace context is correct.

For npm- or TS-based asset targets, `cmd/<target>/` is the primary workspace root.
Tooling should write its root-level project files there rather than to a shared global
workspace, so that every target keeps its own isolated project context.

## When Copying Is Useful

Copying source files should be the exception, not the default.

It can still be useful when:

- a tool cannot work well with direct source paths
- a tool requires a fully local and isolated workspace
- a tool writes outputs next to its inputs
- a hermetic, disposable workspace is desirable
- a tool is fragile around symlinks, absolute paths, or nested module layouts

If copying is needed, it should be scoped narrowly to the target or asset that truly requires it.

## Suggested Shape

The likely structure is:

- `stack` provides the declarative facade and graph composition
- `targets` define executable units
- `assets` define buildable artifact types
- `pipeline` provides generic runtime orchestration
- `tailwind`, `stencil`, `hugo`, and similar packages implement asset- or target-specific behavior

The runtime is responsible for:

- watching source files
- collecting asset inputs across dependencies
- building assets once per asset type
- starting and stopping processes
- prefixing process output
- wiring dependency targets together

## Migrations

This architecture implies a gradual migration away from a dev/build-first package layout toward a target-and-asset-first layout.

The important direction is:

- keep the declaration declarative
- keep target dependencies explicit
- keep asset collection centralized
- keep asset declarations first-class on targets
- keep source access direct whenever possible
- keep copying as a fallback, not the base model

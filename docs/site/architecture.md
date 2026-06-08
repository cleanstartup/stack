---
title: Site Architecture
description: Technical contract for the Hugo-backed site target in stack.
---

## Purpose

The `site` target is a Hugo-backed rendering path that still shares the same Tailwind and Stencil asset pipeline as `app`.
This keeps the rendering model explicit while allowing the brand-oriented content layer to live above the stack.

## Axes

The site contract is intentionally split into three layers:

- `target`: the stack entrypoint and runtime mode (`app` or `site`)
- `layout`: the outer shell and page composition (`site`, `documentation`, `blog`, ...)
- `type`: the semantic content class inside a layout (`landing`, `article`, `reference`, ...)

Why this split matters:

- `target` selects the execution model.
- `layout` chooses the page shell.
- `type` chooses the inner content behavior.

Keeping those concerns separate makes Hugo templates easier to reason about and keeps the brand repo focused on content rather than infrastructure.

## Resolution Model

The stack uses Hugo templates and partials to resolve a page in this order:

1. resolve the page `layout`
2. resolve the page `type`
3. render the layout shell
4. render the type-specific content wrapper

If no explicit front matter is present, the site can fall back to sensible defaults.

Current demo defaults:

- home page: `layout = site`, `type = landing`
- docs section: `layout = documentation`, `type = article`

## Directory Contract

The reusable site contract lives in `site/`, while `cmd/site` is only the runnable harness:

- `site/layouts/` for shell templates and partial resolution
- `site/go.mod` for the Hugo module identity

The runnable harness stays in `cmd/site/` and points at the `site/` module automatically.

The same Tailwind and Stencil bundles are shared with the app target.

## Change Policy

When changing the site contract, update in the same change:

- the content front matter,
- the layout and type partials,
- the docs in `docs/site/`,
- and the demo content in `cmd/site/`.

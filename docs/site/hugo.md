---
title: Site Hugo Contract
description: Hugo-specific conventions for the stack site target.
---

## Scope

This document describes how Hugo is used inside the stack `site` target.
The goal is not to invent a second rendering engine, but to keep the brand-style contract explicit in Hugo terms.

The stack injects its own Hugo module automatically and can collect additional module metadata from any bundled `web.Module` values.

## Front Matter

The demo uses two explicit front matter fields:

- `layout`: selects the outer shell
- `type`: selects the semantic content wrapper

Examples:

```yaml
layout: site
type: landing
```

```yaml
layout: documentation
type: article
```

## Template Structure

The site module keeps the lookup chain small and readable:

- `site/layouts/index.html` renders the home page
- `site/layouts/_default/list.html` renders section/list pages
- `site/layouts/_default/single.html` renders leaf pages
- `site/layouts/partials/layouts/*.hugo.html` contains layout shells
- `site/layouts/partials/types/*.hugo.html` contains type wrappers

The shared rendering helper resolves the requested `layout` and `type` and dispatches to the right partials.

## Asset Pipeline

Hugo renders the HTML, while the stack still owns the CSS and JS outputs:

- Tailwind writes `assets/css/app/app.css`
- Stencil writes `assets/js/stack/stack.esm.js`
- `site dev` mirrors the generated assets into `cmd/site/static/assets`

This lets Hugo serve the final site directly during development without giving up the shared asset pipeline.

## Binary Resolution

The site target resolves Hugo in this order:

1. `STACK_HUGO_BINARY`
2. a cached Hugo binary built through `go install`
3. a pinned default module version

The default version is pinned in the stack so the site target remains reproducible over time.

## Next Step

When the brand repo is moved onto the stack, it should treat this contract as the source of truth and only add brand-specific content and templates on top.

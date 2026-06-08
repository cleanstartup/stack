---
title: Site Hugo Contract
description: Hugo-specific conventions for the stack site target.
---

## Scope

The stack site target does not rely on a local `hugo.toml` in the consumer repo.
Instead, the entrypoint provides Hugo settings via `web.SiteConfig(...)` and the consumer contributes content and templates through `web.Content(...)` and `web.Layouts(...)`.

## Page Model

Stack does not impose a fixed content structure.
The consumer decides which Markdown files exist and which front matter keys they use.

If a page uses explicit `layout` or `type` front matter, Hugo resolves the matching template from the consumer module or from any imported Hugo module using the explicit `layouts/<layout>/content.hugo.html` and `types/<type>/content.hugo.html` conventions.

## Asset Pipeline

Hugo renders the HTML, while stack still owns the CSS and JS outputs:

- Tailwind writes `assets/css/app/app.css`
- Stencil writes `assets/js/stack/stack.esm.js`
- `site dev` mirrors the generated assets into the temporary `.stack/site-assets-module/static/assets` Hugo module

## Binary Resolution

The site target resolves Hugo in this order:

1. `STACK_HUGO_BINARY`
2. a cached Hugo binary built through `go install`
3. a pinned default module version

The default version is pinned in the stack so the site target remains reproducible over time.

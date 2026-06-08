---
title: Site Architecture
description: Technical contract for the Hugo-backed site target in stack.
---

## Purpose

The `site` target is a Hugo-backed rendering path that shares the same Tailwind and Stencil asset pipeline as `app`.
Stack owns the technical shell, asset mirroring, and Hugo module wiring. The consumer module owns the actual content files, layouts, and page semantics.

## Contract

The site target expects explicit configuration instead of hidden repo files:

- `web.SiteConfig(...)` defines Hugo settings such as title, base URL, disabled kinds, params, and renderer flags.
- `web.Content(baseDir, includes...)` materializes matching Markdown files into a temporary Hugo module.
- `web.Layouts(baseDir, includes...)` materializes matching Hugo layout and type files into a temporary Hugo module.

That means a brand repository can keep its own Markdown files and templates next to the component source while the stack only handles the technical build path.

## Rendering

The stack supplies the shared HTML shell and asset links.
Brand-specific layout and type templates can live in the consumer module and are resolved through the explicit `layouts/<layout>/content.hugo.html` and `types/<type>/content.hugo.html` conventions.

There are no hard-coded defaults for home copy or documentation copy in stack itself.
If the consumer wants home pages or section pages, those pages must exist as real content files in the configured source tree.

## Demo

The `cmd/site` harness shows the explicit setup pattern used by the site target.

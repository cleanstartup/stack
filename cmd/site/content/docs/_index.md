---
title: Site Contract Docs
description: Layout and type contract for the stack site target.
layout: documentation
type: article
---

The stack site target now exposes the same architectural axes as the Brand Library:

- `target`
- `layout`
- `type`

## Why this matters

It keeps Hugo templates explicit:

- the `site` target owns the runtime and asset pipeline,
- `layout` decides the page shell,
- `type` decides the semantic wrapper,
- and the brand repo can later consume the contract instead of defining it again.

## Current demo pages

- home page: `layout = site`, `type = landing`
- docs section: `layout = documentation`, `type = article`

This section is intentionally small for now, but it gives us a stable place to grow the Hugo structure as the site and brand layers expand.

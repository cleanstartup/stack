---
title: Site Contract Docs
description: Explicit Hugo settings and consumer-owned content for the stack site target.
---

The stack site target keeps the runtime and asset pipeline, while the consumer repo decides which content files and Hugo templates to use.

## Why this matters

It keeps Hugo templates explicit:

- the `site` target owns the runtime and asset pipeline,
- `web.SiteConfig(...)` owns the site-level settings,
- the consumer repo owns the Markdown content and layout/type template files.

This section stays intentionally small for now, but it gives us a stable place to grow the Hugo structure as the site and brand layers expand.

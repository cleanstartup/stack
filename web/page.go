package web

import (
	"context"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

type Page struct {
	Title         string
	Body          any
	LiveReloadURL string
}

func (p Page) Render(ctx context.Context, w io.Writer) error {
	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = "stack"
	}

	var out strings.Builder
	out.WriteString("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\">")
	out.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	out.WriteString("<title>")
	out.WriteString(html.EscapeString(title))
	out.WriteString("</title>")
	manifest := assetManifestFromContext(ctx)
	version := assetVersionFromContext(ctx)
	renderAssetLinks(&out, manifest.Styles, "stylesheet", version)
	out.WriteString("</head><body>")
	if err := renderPageBody(&out, p.Body, ctx); err != nil {
		return err
	}
	renderAssetLinks(&out, manifest.Scripts, "script", version)
	if strings.TrimSpace(p.LiveReloadURL) != "" {
		out.WriteString("<script>")
		out.WriteString(`(()=>{const u=`)
		out.WriteString(strconv.Quote(p.LiveReloadURL))
		out.WriteString(`;const es=new EventSource(u);es.onmessage=()=>location.reload();es.onerror=()=>{try{es.close()}catch(_){}};})()`)
		out.WriteString("</script>")
	}
	out.WriteString("</body></html>")

	_, err := io.WriteString(w, out.String())
	if err != nil {
		return fmt.Errorf("render page: %w", err)
	}
	return nil
}

func assetVersionFromContext(ctx context.Context) string {
	if state := devStateFromContext(ctx); state != nil {
		return fmt.Sprintf("%d", state.Revision())
	}
	return ""
}

func renderAssetLinks(out *strings.Builder, refs []AssetRef, kind string, version string) {
	seen := map[string]struct{}{}
	for _, ref := range refs {
		for _, url := range ref.URLsWithVersion(version) {
			if _, exists := seen[url]; exists {
				continue
			}
			seen[url] = struct{}{}
			switch kind {
			case "stylesheet":
				out.WriteString("<link rel=\"stylesheet\" href=\"")
				out.WriteString(html.EscapeString(url))
				out.WriteString("\">")
			case "script":
				out.WriteString("<script")
				if strings.Contains(url, ".esm.js") {
					out.WriteString(" type=\"module\"")
				} else {
					out.WriteString(" defer")
				}
				out.WriteString(" src=\"")
				out.WriteString(html.EscapeString(url))
				out.WriteString("\"></script>")
			}
		}
	}
}

func renderPageBody(out *strings.Builder, body any, ctx context.Context) error {
	switch typed := body.(type) {
	case nil:
		return nil
	case string:
		out.WriteString("<main>")
		out.WriteString(html.EscapeString(typed))
		out.WriteString("</main>")
		return nil
	case []byte:
		out.WriteString("<main>")
		out.WriteString(html.EscapeString(string(typed)))
		out.WriteString("</main>")
		return nil
	case templ.Component:
		return typed.Render(ctx, out)
	case interface {
		Render(context.Context, io.Writer) error
	}:
		out.WriteString("<main>")
		if err := typed.Render(ctx, out); err != nil {
			return err
		}
		out.WriteString("</main>")
		return nil
	default:
		out.WriteString("<main>")
		out.WriteString(html.EscapeString(fmt.Sprint(typed)))
		out.WriteString("</main>")
		return nil
	}
}

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
	Styles        []AssetRef
	Scripts       []AssetRef
	AssetVersion  string
	LiveReloadURL string
}

func (p Page) Render(ctx context.Context, w io.Writer) error {
	title := strings.TrimSpace(p.Title)
	if title == "" {
		title = "way2go"
	}

	var out strings.Builder
	out.WriteString("<!doctype html><html lang=\"en\"><head><meta charset=\"utf-8\">")
	out.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">")
	out.WriteString("<title>")
	out.WriteString(html.EscapeString(title))
	out.WriteString("</title>")
	for _, style := range p.Styles {
		for _, url := range style.URLsWithVersion(p.AssetVersion) {
			out.WriteString("<link rel=\"stylesheet\" href=\"")
			out.WriteString(html.EscapeString(url))
			out.WriteString("\">")
		}
	}
	out.WriteString("</head><body>")
	out.WriteString("<main>")
	if err := renderPageBody(&out, p.Body, ctx); err != nil {
		return err
	}
	out.WriteString("</main>")
	for _, script := range p.Scripts {
		for _, url := range script.URLsWithVersion(p.AssetVersion) {
			out.WriteString("<script defer src=\"")
			out.WriteString(html.EscapeString(url))
			out.WriteString("\"></script>")
		}
	}
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

func renderPageBody(out *strings.Builder, body any, ctx context.Context) error {
	switch typed := body.(type) {
	case nil:
		return nil
	case string:
		out.WriteString(html.EscapeString(typed))
		return nil
	case []byte:
		out.WriteString(html.EscapeString(string(typed)))
		return nil
	case templ.Component:
		return typed.Render(ctx, out)
	case interface {
		Render(context.Context, io.Writer) error
	}:
		return typed.Render(ctx, out)
	default:
		out.WriteString(html.EscapeString(fmt.Sprint(typed)))
		return nil
	}
}

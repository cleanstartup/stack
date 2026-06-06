package main

import (
	"context"
	"io"
	"log"
	"path/filepath"
	"runtime"

	"github.com/a-h/templ"
	"github.com/cleanstartup/way2go/activity"
	"github.com/cleanstartup/way2go/web"
)

func main() {
	if err := web.Run(demoModule()); err != nil {
		log.Fatal(err)
	}
}

func demoModule() web.Module {
	baseDir := mustModuleDir()
	cssPath := filepath.Join(baseDir, "assets", "css", "site.css")
	tailwindPath := filepath.Join(baseDir, "assets", "css", "tailwind.css")
	jsPath := filepath.Join(baseDir, "assets", "js", "site.js")
	cssSource := web.FromFile(cssPath)
	tailwindSource := web.FromFile(tailwindPath)
	jsSource := web.FromFile(jsPath)
	var cssRef web.AssetRef
	var tailwindRef web.AssetRef
	var jsRef web.AssetRef

	return web.ModuleFunc(func(b *web.Builder) {
		b.TailwindScan(baseDir)

		def := activity.As[struct{}, activity.NoInput]("root")
		web.AddActivity(b, def, func(params struct{}) activity.Instance[struct{}, activity.NoInput] {
			return def.Take(params).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
				return web.Page{
					Title:   "way2go demo",
					Body:    demoBody{CSS: cssRef.URL(), Tailwind: tailwindRef.URL()},
					Styles:  []web.AssetRef{cssRef, tailwindRef},
					Scripts: []web.AssetRef{jsRef},
				}
			})
		})
		cssRef = b.CSS(cssSource)
		tailwindRef = b.TailwindCSS(tailwindSource)
		jsRef = b.JS(jsSource)
	})
}

type demoBody struct {
	CSS      string
	Tailwind string
}

func (d demoBody) Render(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, `<section class="card"><h1 id="headline" class="title">hello from way2go</h1><p class="lead">CSS asset: <code>`+d.CSS+`</code></p><p class="hint">Tailwind bundle: <code>`+d.Tailwind+`</code></p><p class="hint">Open the console to see the JS asset run.</p></section>`)
	return err
}

var _ templ.Component = demoBody{}

func mustModuleDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("unable to resolve demo module directory")
	}
	return filepath.Dir(file)
}

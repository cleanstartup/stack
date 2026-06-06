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
	jsPath := filepath.Join(baseDir, "assets", "js", "site.js")
	cssSource := web.FromFile(cssPath)
	jsSource := web.FromFile(jsPath)
	var cssRef web.AssetRef
	var jsRef web.AssetRef

	return web.ModuleFunc(func(b *web.Builder) {
		b.TailwindScan(baseDir)

		def := activity.As[struct{}, activity.NoInput]("root")
		web.AddActivity(b, def, func(params struct{}) activity.Instance[struct{}, activity.NoInput] {
			return def.Take(params).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
				return web.Page{
					Title:   "way2go demo",
					Body:    demoBody{CSS: cssRef.URL()},
					Styles:  []web.AssetRef{cssRef},
					Scripts: []web.AssetRef{jsRef},
				}
			})
		})
		cssRef = b.CSS(cssSource)
		jsRef = b.JS(jsSource)
	})
}

type demoBody struct {
	CSS string
}

func (d demoBody) Render(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, `<section class="rounded-lg border border-slate-200 bg-white p-6 shadow-sm"><h1 id="headline" class="text-3xl font-bold tracking-tight text-slate-900">hello from way2go</h1><p class="mt-3 text-slate-600">CSS bundle: <code>`+d.CSS+`</code></p><p class="mt-2 text-slate-500">Open the console to see the JS asset run.</p></section>`)
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

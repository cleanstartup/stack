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
	assets := web.NewConventionalAssets(baseDir)

	return web.ModuleFunc(func(b *web.Builder) {
		assets.Register(b)

		def := activity.As[struct{}, activity.NoInput]("root")
		web.AddActivity(b, def, func(params struct{}) activity.Instance[struct{}, activity.NoInput] {
			return def.Take(params).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
				return web.Page{
					Title: "way2go demo",
					Body:  demoBody{},
				}
			})
		})
	})
}

type demoBody struct {
}

func (d demoBody) Render(_ context.Context, w io.Writer) error {
	_, err := io.WriteString(w, `<section class="card"><h1 id="headline" class="title">hello from way2go</h1><p class="lead">This page loads global CSS, Tailwind CSS, and JS assets.</p><p class="hint">Open the console and click the headline.</p></section>`)
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

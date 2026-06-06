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
	if err := web.Run(web.Compose(
		web.ConventionalAssets(mustModuleDir()),
		web.Simple(web.RootRef(), func(ctx activity.Context) activity.Result {
			return web.Page{
				Title: "way2go demo",
				Body:  demoBody{},
			}
		}),
	)); err != nil {
		log.Fatal(err)
	}
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

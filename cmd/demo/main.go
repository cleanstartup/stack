package main

import (
	"context"
	"io"

	"github.com/a-h/templ"
	"github.com/cleanstartup/way2go/activity"
	"github.com/cleanstartup/way2go/web"
)

func main() {
	web.Run(
		web.ConventionalAssets(),
		web.Simple(
			web.RootRef(),
			func(ctx activity.Context) activity.Result {
				return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
					_, err := io.WriteString(w, `<section class="card"><h1 id="headline" class="title">hello from way2go</h1><p class="lead">This page loads global CSS, Tailwind CSS, and JS assets.</p><p class="hint">Open the console and click the headline.</p></section>`)
					return err
				})
			},
			web.WithStaticTitle("way2go demo"),
		),
	)
}

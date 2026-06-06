package main

import (
	"context"
	"io"

	"github.com/a-h/templ"
	"github.com/cleanstartup/way2go/activity"
	"github.com/cleanstartup/way2go/web"
)

func main() {
	web.App(
		web.Module(
			web.Activity(
				web.RootRef(),
				func(ctx activity.Context) activity.Result {
					return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
						_, err := io.WriteString(w, `<demo-card></demo-card>`)
						return err
					})
				},
				web.WithStaticTitle("way2go demo"),
			),
		),
	)
}

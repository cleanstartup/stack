package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/a-h/templ"
	"github.com/cleanstartup/stack"
	"github.com/cleanstartup/stack/activity"
)

func main() {
	if err := stack.Execute(stack.App(
		stack.WithBaseDir("cmd/demo"),
		stack.WithParts(
			stack.Module(
				stack.Activity(
					stack.RootRef(),
					func(ctx activity.Context) activity.Result {
						return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
							_, err := io.WriteString(w, `<demo-card></demo-card>`)
							return err
						})
					},
					stack.WithStaticTitle("stack demo"),
				),
			),
		),
	)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

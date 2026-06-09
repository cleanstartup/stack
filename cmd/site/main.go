package main

import (
	"fmt"
	"os"

	"github.com/cleanstartup/stack"
)

func main() {
	unsafe := true

	target := stack.Hugo(
		stack.WithBaseDir("cmd/site"),
		stack.WithAssets(
			stack.Styles("cmd/site"),
			stack.Components("cmd/site"),
		),
		stack.WithParts(
			stack.SiteConfig(stack.SiteOptions{
				Title:        "stack site demo",
				BaseURL:      "http://127.0.0.1:8080/",
				DisableKinds: []string{"taxonomy", "term"},
				MarkupUnsafe: &unsafe,
			}),
		),
	)

	if err := stack.Execute(target); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

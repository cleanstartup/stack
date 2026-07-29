package cli_test

import (
	"fmt"

	"github.com/cleanstartup/stack/way2go/cli"
)

func ExampleActivity() {
	r := cli.NewRegistry()
	xy := cli.Param("xy", "x", "X")

	a := cli.Activity(
		"sample",
		func(inv *cli.Invocation) int { return inv.IntParam(xy) },
		func(ctx cli.Context[int]) cli.Result {
			return cli.Textf("xy=%d", ctx.Data())
		},
	)
	cli.RegisterActivity(r, a)

	res := r.Execute([]string{"sample", "--xy=5"})
	fmt.Println(res.ExitCode)
	fmt.Println(res.Stdout)

	// Output:
	// 0
	// xy=5
}

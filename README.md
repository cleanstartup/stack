# way2go

`way2go` is a small Go foundation for building activities, web handlers, and CLIs with a shared model.

## Packages

- `activity`: typed activity definitions, instances, URI building, and execution helpers
- `web`: HTTP registration, routing, and request decoding
- `cli`: command registration, parsing, and execution
- `auth`: optional session helpers for the core packages
- `config`: env-based config loading with `.env` support

## Import

```go
import (
	"github.com/cleanstartup/way2go/activity"
	"github.com/cleanstartup/way2go/web"
)
```

## Example

```go
package main

import (
	"fmt"

	"github.com/cleanstartup/way2go/activity"
	"github.com/cleanstartup/way2go/web"
)

type ShowParams struct {
	AccountID string
}

var showDef = activity.As[ShowParams, activity.NoInput]("account.show")

func Show(params ShowParams) activity.Instance[ShowParams, activity.NoInput] {
	return showDef.Take(params).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
		return "account=" + params.AccountID
	})
}

func main() {
	fmt.Println(Show(ShowParams{AccountID: "42"}).URI())

	r := web.NewRegistry()
	web.Register(r, showDef, Show)
}
```

package web_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/cleanstartup/way2go/activity"
	"github.com/cleanstartup/way2go/web"
)

type pageParams struct {
	AccountID string
}

func ExampleRegister() {
	r := web.NewRegistry()
	def := activity.As[pageParams, activity.NoInput]("account.page")

	page := func(params pageParams) activity.Instance[pageParams, activity.NoInput] {
		return def.Take(params).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
			return "account=" + params.AccountID
		})
	}

	web.Register(r, def, page)

	req := httptest.NewRequest(http.MethodGet, "/account/page?accountId=99", nil)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)

	fmt.Println(rec.Code)
	fmt.Println(rec.Body.String())

	// Output:
	// 200
	// account=99
}

func ExampleActivity() {
	r := web.NewRegistry()
	xy := web.Input("xy")

	a := web.Activity(
		"sample.show",
		func(req *web.Request) int { return req.IntParam(xy) },
		func(ctx web.Context[int]) activity.Result {
			return fmt.Sprintf("xy=%d", ctx.Data())
		},
	)
	web.RegisterWebActivity(r, a)

	req := httptest.NewRequest(http.MethodGet, "/sample/show?xy=5", nil)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)

	fmt.Println(rec.Code)
	fmt.Println(rec.Body.String())

	// Output:
	// 200
	// xy=5
}

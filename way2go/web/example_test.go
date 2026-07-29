package web_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/cleanstartup/stack/way2go/activity"
	"github.com/cleanstartup/stack/way2go/web"
)

func ExampleNewActivity() {
	r := web.NewRegistry()

	a := web.NewActivity("account.page", func(ctx activity.Context) activity.Result {
		return "account-page"
	})
	web.RegisterWebActivity(r, a)

	req := httptest.NewRequest(http.MethodGet, "/account/page", nil)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)

	fmt.Println(rec.Code)
	fmt.Println(rec.Body.String())

	// Output:
	// 200
	// account-page
}

package web_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cleanstartup/way2go/activity"
	"github.com/cleanstartup/way2go/web"
)

type showParams struct {
	AccountID string
}

type renameInput struct {
	NewName string
}

func TestRegisterGetAndPostFlow(t *testing.T) {
	r := web.NewRegistry()

	showDef := activity.As[showParams, activity.NoInput]("account.show")
	renameDef := activity.As[showParams, renameInput]("account.rename")

	var accountShow func(showParams) activity.Instance[showParams, activity.NoInput]
	var accountRename func(showParams) activity.Instance[showParams, renameInput]

	accountShow = func(params showParams) activity.Instance[showParams, activity.NoInput] {
		return showDef.Take(params).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
			renameURI := accountRename(showParams{AccountID: params.AccountID}).URI()
			return fmt.Sprintf("show:%s rename:%s", params.AccountID, renameURI)
		})
	}

	accountRename = func(params showParams) activity.Instance[showParams, renameInput] {
		return renameDef.Take(params).Then(func(ctx activity.Context, input renameInput) activity.Result {
			web.RedirectTo(ctx, accountShow(showParams{AccountID: params.AccountID}))
			return nil
		})
	}

	web.Register(r, showDef, accountShow)
	web.Register(r, renameDef, accountRename)

	h := r.Handler()

	getReq := httptest.NewRequest(http.MethodGet, "/account/show?accountId=42", nil)
	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("expected GET /account/show => 200, got %d", getRec.Code)
	}
	body := getRec.Body.String()
	if !strings.Contains(body, "show:42") {
		t.Fatalf("unexpected response body: %q", body)
	}
	if !strings.Contains(body, "/account/rename?accountID=42") {
		t.Fatalf("expected rename URI in response body, got %q", body)
	}

	form := url.Values{"newName": []string{"Savings"}}
	postReq := httptest.NewRequest(http.MethodPost, "/account/rename?accountId=42", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postRec := httptest.NewRecorder()
	h.ServeHTTP(postRec, postReq)

	if postRec.Code != http.StatusFound {
		t.Fatalf("expected POST /account/rename => 302, got %d", postRec.Code)
	}
	if got := postRec.Header().Get("Location"); got != "/account/show?accountID=42" {
		t.Fatalf("expected redirect to /account/show?accountID=42, got %q", got)
	}
}

func TestDuplicateRegistrationPanics(t *testing.T) {
	r := web.NewRegistry()
	def := activity.As[showParams, activity.NoInput]("account.show")
	resolver := func(params showParams) activity.Instance[showParams, activity.NoInput] {
		return def.Take(params).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
			return "ok"
		})
	}

	web.Register(r, def, resolver)

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on duplicate registration")
		}
	}()

	web.Register(r, def, resolver)
}

func TestRegistryErrorHandlerIsUsed(t *testing.T) {
	r := web.NewRegistry()
	r.SetErrorHandler(func(ctx *web.RuntimeContext, err error) activity.Result {
		return "custom-error:" + err.Error()
	})

	def := activity.As[showParams, activity.NoInput]("account.show")
	resolver := func(params showParams) activity.Instance[showParams, activity.NoInput] {
		return def.Take(params).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
			return ctx.Error(fmt.Errorf("boom"))
		})
	}
	web.Register(r, def, resolver)

	req := httptest.NewRequest(http.MethodGet, "/account/show?accountId=1", nil)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from custom renderer, got %d", rec.Code)
	}
	if rec.Body.String() != "custom-error:boom" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestWebActivitySkipsHandlerWhenRequestFailed(t *testing.T) {
	r := web.NewRegistry()
	xy := web.Input("xy")
	handlerCalled := false

	a := web.Activity(
		"sample.invalid",
		func(req *web.Request) int {
			return req.IntParam(xy, func(v int) error {
				if v <= 0 {
					return errors.New("must be > 0")
				}
				return nil
			})
		},
		func(ctx web.Context[int]) activity.Result {
			handlerCalled = true
			return fmt.Sprintf("ok:%d", ctx.Data())
		},
	)
	web.RegisterWebActivity(r, a)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sample/invalid?xy=0", nil)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
	if handlerCalled {
		t.Fatalf("handler must not be called on failed request")
	}
	if !strings.Contains(rec.Body.String(), "must be > 0") {
		t.Fatalf("expected validation error in body, got %q", rec.Body.String())
	}
}

func TestWebActivityCallsHandlerWhenRequestValid(t *testing.T) {
	r := web.NewRegistry()
	xy := web.Input("xy")

	a := web.Activity(
		"sample.valid",
		func(req *web.Request) int {
			return req.IntParam(xy, func(v int) error {
				if v <= 0 {
					return errors.New("must be > 0")
				}
				return nil
			})
		},
		func(ctx web.Context[int]) activity.Result {
			return fmt.Sprintf("ok:%d", ctx.Data())
		},
	)
	web.RegisterWebActivity(r, a)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/sample/valid?xy=7", nil)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "ok:7" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestRegisterSupportsRootActivity(t *testing.T) {
	r := web.NewRegistry()
	def := activity.As[struct{}, activity.NoInput]("root")
	resolver := func(params struct{}) activity.Instance[struct{}, activity.NoInput] {
		return def.Take(struct{}{}).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
			return "root-ok"
		})
	}
	web.Register(r, def, resolver)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "root-ok" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestGroupPrefixesRegisteredPaths(t *testing.T) {
	r := web.NewRegistry()
	group := r.Group("wallet")
	def := activity.As[struct{}, activity.NoInput]("show")
	resolver := func(params struct{}) activity.Instance[struct{}, activity.NoInput] {
		return def.Take(struct{}{}).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
			return "wallet-show"
		})
	}

	web.Register(group, def, resolver)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/wallet/show", nil)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if rec.Body.String() != "wallet-show" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestURIUsesRegisteredQualifiedPath(t *testing.T) {
	web.Reset()
	group := web.DefaultGroup("wallet")
	a := web.Simple(
		web.Ref("show"),
		func(ctx activity.Context) activity.Result { return "ok" },
	)

	web.RegisterWebActivity(group, a)

	got := web.URI(a)
	if got != "/wallet/show" {
		t.Fatalf("expected /wallet/show, got %q", got)
	}
}

func TestURIOkBeforeRegistration(t *testing.T) {
	web.Reset()
	a := web.Simple(
		web.Ref("show"),
		func(ctx activity.Context) activity.Result { return "ok" },
	)

	_, ok := web.URIOk(a)
	if ok {
		t.Fatalf("expected URIOk=false before registration")
	}
}

func TestGlobalMiddlewareIsAppliedToWebActivities(t *testing.T) {
	r := web.NewRegistry()
	called := false
	r.UseGlobalMiddleware(func(next func(ctx activity.Context) activity.Result) func(ctx activity.Context) activity.Result {
		return func(ctx activity.Context) activity.Result {
			called = true
			return next(ctx)
		}
	})

	a := web.Simple(
		web.Ref("tracked"),
		func(ctx activity.Context) activity.Result { return "ok" },
	)
	web.RegisterWebActivity(r, a)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tracked", nil)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !called {
		t.Fatalf("expected global middleware to be called")
	}
}

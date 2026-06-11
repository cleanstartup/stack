package web_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cleanstartup/stack/activity"
	"github.com/cleanstartup/stack/web"
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

	a := web.RawActivity(
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

	a := web.RawActivity(
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
	a := web.Activity(
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
	a := web.Activity(
		web.Ref("show"),
		func(ctx activity.Context) activity.Result { return "ok" },
	)

	_, ok := web.URIOk(a)
	if ok {
		t.Fatalf("expected URIOk=false before registration")
	}
}

func TestURIFallsBackToPathFromIDForUnregisteredRef(t *testing.T) {
	web.Reset()
	got := web.URI(web.Ref("backup.intro"))
	if got != "/backup/intro" {
		t.Fatalf("expected /backup/intro, got %q", got)
	}
}

func TestURIFallsBackToPathFromIDForUnregisteredActivity(t *testing.T) {
	web.Reset()
	a := web.Activity(
		web.Ref("backup.intro"),
		func(ctx activity.Context) activity.Result { return "ok" },
	)

	got := web.URI(a)
	if got != "/backup/intro" {
		t.Fatalf("expected /backup/intro, got %q", got)
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

	a := web.Activity(
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

func TestStaticTitleRendersMinimalHtmlShell(t *testing.T) {
	r := web.NewRegistry()
	pageRef := web.AssetRef{Kind: web.AssetKindCSS, ID: "app.css"}
	scriptRef := web.AssetRef{Kind: web.AssetKindJS, ID: "stack", Files: []string{"stack.esm.js"}}
	r.SetAssets(web.AssetManifest{Styles: []web.AssetRef{pageRef}, Scripts: []web.AssetRef{scriptRef}})
	a := web.Activity(
		web.Ref("page"),
		func(ctx activity.Context) activity.Result {
			return templBody(func(_ context.Context, w io.Writer) error {
				_, err := io.WriteString(w, "hello")
				return err
			})
		}, web.WithStaticTitle("demo"),
	)
	web.RegisterWebActivity(r, a)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/page", nil)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<!doctype html>") {
		t.Fatalf("expected html shell, got %q", body)
	}
	if !strings.Contains(body, "<title>demo</title>") {
		t.Fatalf("expected title, got %q", body)
	}
	if !strings.Contains(body, "<link rel=\"stylesheet\" href=\"/assets/css/app.css\">") {
		t.Fatalf("expected stylesheet link, got %q", body)
	}
	if !strings.Contains(body, "<script type=\"module\" src=\"/assets/js/stack/stack.esm.js\">") {
		t.Fatalf("expected module script, got %q", body)
	}
	if !strings.Contains(body, "hello") {
		t.Fatalf("expected body content, got %q", body)
	}
}

func TestPageRendersTemplComponentBody(t *testing.T) {
	r := web.NewRegistry()
	a := web.Activity(
		web.Ref("templ"),
		func(ctx activity.Context) activity.Result {
			return templBody(func(_ context.Context, w io.Writer) error {
				_, err := io.WriteString(w, "<strong>templ body</strong>")
				return err
			})
		}, web.WithStaticTitle("demo"),
	)
	web.RegisterWebActivity(r, a)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/templ", nil)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<title>demo</title>") {
		t.Fatalf("expected title, got %q", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "<strong>templ body</strong>") {
		t.Fatalf("expected templ body, got %q", rec.Body.String())
	}
}

func TestScreenRendersCustomElementWithProps(t *testing.T) {
	r := web.NewRegistry()
	a := web.Activity(
		web.Ref("screen"),
		func(ctx activity.Context) activity.Result {
			return web.Page{Title: "screen", Body: web.Screen("abc", map[string]any{
				"title": "Hello",
				"count": 3,
			})}
		},
	)
	web.RegisterWebActivity(r, a)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/screen", nil)
	r.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "<screen-abc") {
		t.Fatalf("expected screen element, got %q", body)
	}
	if !strings.Contains(body, "data-screen-props=\"") || !strings.Contains(body, "count") || !strings.Contains(body, "Hello") {
		t.Fatalf("expected serialized screen props, got %q", body)
	}
	if !strings.Contains(body, "Object.assign(el,props)") {
		t.Fatalf("expected hydration script, got %q", body)
	}
}

func TestPageRendersDevReloadAndVersionedAssets(t *testing.T) {
	r := web.NewRegistry()
	devState := web.NewDevState()
	devState.MarkBuilt()
	r.SetDevState(devState)

	pageRef := web.AssetRef{Kind: web.AssetKindCSS, ID: "app.css"}
	r.SetAssets(web.AssetManifest{Styles: []web.AssetRef{pageRef}})
	a := web.Activity(
		web.Ref("dev"),
		func(ctx activity.Context) activity.Result {
			return "hello"
		}, web.WithStaticTitle("dev"),
	)
	web.RegisterWebActivity(r, a)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/dev", nil)
	r.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "?v=1") {
		t.Fatalf("expected versioned asset URL, got %q", body)
	}
	if !strings.Contains(body, "<title>dev</title>") {
		t.Fatalf("expected title, got %q", body)
	}
	if !strings.Contains(body, "/__stack/dev/events") {
		t.Fatalf("expected dev reload script, got %q", body)
	}
}

func TestDevEventsEndpointIsMounted(t *testing.T) {
	r := web.NewRegistry()
	state := web.NewDevState()
	r.SetDevState(state)
	r.RegisterDevEndpoints(state)

	rec := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/__stack/dev/events", nil).WithContext(ctx)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK && rec.Code != 0 {
		t.Fatalf("expected dev events endpoint to start cleanly, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "connected") {
		t.Fatalf("expected SSE handshake, got %q", rec.Body.String())
	}
}

func TestMountedHandlerReceivesAssetManifest(t *testing.T) {
	r := web.NewRegistry()
	pageRef := web.AssetRef{Kind: web.AssetKindCSS, ID: "app.css"}
	r.SetAssets(web.AssetManifest{Styles: []web.AssetRef{pageRef}})

	r.Mount("/auth", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_ = web.RenderResult(w, req, web.Page{Title: "mounted", Body: "hello"})
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/auth", nil)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<link rel=\"stylesheet\" href=\"/assets/css/app.css\">") {
		t.Fatalf("expected mounted handler to receive asset manifest, got %q", body)
	}
}

func TestMountedHandlerSeesOriginalPath(t *testing.T) {
	r := web.NewRegistry()
	var gotPath string

	r.Mount("/auth", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		gotPath = req.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/session/refresh", nil)
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if gotPath != "/auth/session/refresh" {
		t.Fatalf("expected original path to be preserved, got %q", gotPath)
	}
}

type templBody func(context.Context, io.Writer) error

func (b templBody) Render(ctx context.Context, w io.Writer) error {
	if b == nil {
		return nil
	}
	return b(ctx, w)
}

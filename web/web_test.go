package web_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cleanstartup/stack/activity"
	"github.com/cleanstartup/stack/web"
)

func TestDuplicateRegistrationPanics(t *testing.T) {
	r := web.NewRegistry()
	a := web.NewActivity("account.show", func(ctx activity.Context) activity.Result { return "ok" })

	web.RegisterWebActivity(r, a)

	defer func() {
		if recover() == nil {
			t.Fatalf("expected panic on duplicate registration")
		}
	}()

	web.RegisterWebActivity(r, a)
}

func TestRegistryErrorHandlerIsUsed(t *testing.T) {
	r := web.NewRegistry()
	r.SetErrorHandler(func(ctx *web.RuntimeContext, err error) activity.Result {
		return "custom-error:" + err.Error()
	})

	a := web.NewActivity("account.show", func(ctx activity.Context) activity.Result {
		return ctx.Error(fmt.Errorf("boom"))
	})
	web.RegisterWebActivity(r, a)

	req := httptest.NewRequest(http.MethodGet, "/account/show", nil)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from custom renderer, got %d", rec.Code)
	}
	if rec.Body.String() != "custom-error:boom" {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestDefaultErrorHandlerMapsErrNotFoundTo404(t *testing.T) {
	r := web.NewRegistry()
	a := web.NewActivity("payment.checkout", func(ctx activity.Context) activity.Result {
		return ctx.Error(activity.ErrNotFound)
	})
	web.RegisterWebActivity(r, a)

	req := httptest.NewRequest(http.MethodGet, "/payment/checkout", nil)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for activity.ErrNotFound, got %d", rec.Code)
	}
}

func TestDefaultErrorHandlerMapsOtherErrorsTo500(t *testing.T) {
	r := web.NewRegistry()
	a := web.NewActivity("account.show", func(ctx activity.Context) activity.Result {
		return ctx.Error(fmt.Errorf("boom"))
	})
	web.RegisterWebActivity(r, a)

	req := httptest.NewRequest(http.MethodGet, "/account/show", nil)
	rec := httptest.NewRecorder()
	r.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 for a generic error, got %d", rec.Code)
	}
}

func TestRegisterSupportsRootActivity(t *testing.T) {
	r := web.NewRegistry()
	a := web.NewActivity("root", func(ctx activity.Context) activity.Result { return "root-ok" })
	web.RegisterWebActivity(r, a)

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
	a := web.NewActivity("show", func(ctx activity.Context) activity.Result { return "wallet-show" })
	web.RegisterWebActivity(group, a)

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

func TestGlobalMiddlewareIsAppliedToWebActivities(t *testing.T) {
	r := web.NewRegistry()
	called := false
	r.UseGlobalMiddleware(func(next activity.ContextHandler) activity.ContextHandler {
		return func(ctx activity.Context) activity.Result {
			called = true
			return next(ctx)
		}
	})

	a := web.NewActivity("tracked", func(ctx activity.Context) activity.Result { return "ok" })
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

func TestPageRendersWithTitleAndAssets(t *testing.T) {
	r := web.NewRegistry()
	pageRef := web.AssetRef{Kind: web.AssetKindCSS, ID: "app.css"}
	scriptRef := web.AssetRef{Kind: web.AssetKindJS, ID: "stack", Files: []string{"stack.esm.js"}}
	r.SetAssets(web.AssetManifest{Styles: []web.AssetRef{pageRef}, Scripts: []web.AssetRef{scriptRef}})
	a := web.NewActivity("page", func(ctx activity.Context) activity.Result {
		return web.Page{
			Title: "demo",
			Body: templBody(func(_ context.Context, w io.Writer) error {
				_, err := io.WriteString(w, "hello")
				return err
			}),
		}
	})
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
	a := web.NewActivity("templ", func(ctx activity.Context) activity.Result {
		return web.Page{
			Title: "demo",
			Body: templBody(func(_ context.Context, w io.Writer) error {
				_, err := io.WriteString(w, "<strong>templ body</strong>")
				return err
			}),
		}
	})
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
	a := web.NewActivity("screen", func(ctx activity.Context) activity.Result {
		return web.Page{Title: "screen", Body: web.Screen("abc", map[string]any{
			"title": "Hello",
			"count": 3,
		})}
	})
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
}

func TestPageRendersDevReloadAndVersionedAssets(t *testing.T) {
	r := web.NewRegistry()
	devState := web.NewDevState()
	devState.MarkBuilt()
	r.SetDevState(devState)

	pageRef := web.AssetRef{Kind: web.AssetKindCSS, ID: "app.css"}
	r.SetAssets(web.AssetManifest{Styles: []web.AssetRef{pageRef}})
	a := web.NewActivity("dev", func(ctx activity.Context) activity.Result {
		return web.Page{Title: "dev", Body: "hello"}
	})
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

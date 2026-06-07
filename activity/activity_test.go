package activity_test

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/cleanstartup/stack/activity"
)

type showParams struct {
	AccountID string
	Tab       string
}

type renameInput struct {
	NewName string
}

type testCtx struct{}

func (testCtx) Request() *http.Request                             { return nil }
func (testCtx) ResponseWriter() http.ResponseWriter                { return nil }
func (testCtx) RedirectToURI(uri string)                           {}
func (testCtx) RedirectToURIWithStatus(uri string, statusCode int) {}
func (testCtx) Error(err error) activity.Result                    { return fmt.Sprintf("err:%v", err) }

func TestURIBuildsPathAndQuery(t *testing.T) {
	def := activity.As[showParams, activity.NoInput]("account.show")
	uri := def.Take(showParams{AccountID: "42", Tab: "details"}).URI()

	if uri != "/account/show?accountID=42&tab=details" {
		t.Fatalf("unexpected uri: %s", uri)
	}
}

func TestPathFromIDRootMapsToSlash(t *testing.T) {
	got := activity.PathFromID("root")
	if got != "/" {
		t.Fatalf("expected / for root, got %q", got)
	}
}

func TestDecodeParamsFromPathAndQuery(t *testing.T) {
	def := activity.As[showParams, activity.NoInput]("account.show")

	params, err := def.DecodeParams(
		map[string]string{},
		url.Values{
			"accountId": []string{"42"},
			"tab":       []string{"security"},
		},
	)
	if err != nil {
		t.Fatalf("decode params failed: %v", err)
	}

	if params.AccountID != "42" {
		t.Fatalf("expected AccountID=42, got %q", params.AccountID)
	}
	if params.Tab != "security" {
		t.Fatalf("expected Tab=security, got %q", params.Tab)
	}
}

func TestRunWithoutExecutorFails(t *testing.T) {
	def := activity.As[showParams, renameInput]("account.rename")
	result := def.Take(showParams{AccountID: "42"}).Run(testCtx{}, renameInput{NewName: "Savings"})
	if result == nil {
		t.Fatalf("expected error result when executor is missing")
	}
}

func TestDefinitionMiddlewareWrapsExecutor(t *testing.T) {
	def := activity.As[showParams, activity.NoInput]("account.show").Use(
		func(next activity.Executor[showParams, activity.NoInput]) activity.Executor[showParams, activity.NoInput] {
			return func(ctx activity.Context, input activity.NoInput) activity.Result {
				result := next(ctx, input)
				return result.(string) + "|mw"
			}
		},
	)

	result := def.Take(showParams{AccountID: "42"}).Then(func(ctx activity.Context, input activity.NoInput) activity.Result { return "ok" }).Run(testCtx{}, activity.NoInput{})
	if result != "ok|mw" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestHandleAppliesDecoratorsAfterMiddlewares(t *testing.T) {
	events := []string{}
	def := activity.As[showParams, activity.NoInput]("account.show").Use(
		func(next activity.Executor[showParams, activity.NoInput]) activity.Executor[showParams, activity.NoInput] {
			return func(ctx activity.Context, input activity.NoInput) activity.Result {
				events = append(events, "mw:before")
				result := next(ctx, input)
				events = append(events, "mw:after")
				return result
			}
		},
	)
	instance := def.Take(showParams{AccountID: "42"}).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
		events = append(events, "exec")
		return "ok"
	})

	instance.Handle(testCtx{}, activity.NoInput{},
		func(next activity.Handler[activity.NoInput]) activity.Handler[activity.NoInput] {
			return func(ctx activity.Context, input activity.NoInput) activity.Result {
				events = append(events, "dec1:before")
				result := next(ctx, input)
				events = append(events, "dec1:after")
				return result
			}
		},
		func(next activity.Handler[activity.NoInput]) activity.Handler[activity.NoInput] {
			return func(ctx activity.Context, input activity.NoInput) activity.Result {
				events = append(events, "dec2:before")
				result := next(ctx, input)
				events = append(events, "dec2:after")
				return result
			}
		},
	)

	want := []string{"dec1:before", "dec2:before", "mw:before", "exec", "mw:after", "dec2:after", "dec1:after"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("unexpected call order: got=%v want=%v", events, want)
	}
}

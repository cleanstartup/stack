package auth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cleanstartup/way2go/activity"
	"github.com/cleanstartup/way2go/auth"
	"github.com/cleanstartup/way2go/web"

	"github.com/a-h/templ"
)

type mockSession struct{ userID string }

func (m mockSession) GetUserId() string         { return m.userID }
func (m mockSession) GetEmail() string          { return "test@example.com" }
func (m mockSession) GetRequest() *http.Request { return nil }

type mockProvider struct {
	err error
}

func (m mockProvider) GetSession(http.ResponseWriter, *http.Request) (auth.Session, error) {
	if m.err != nil {
		return nil, m.err
	}
	return mockSession{userID: "u-1"}, nil
}

func (m mockProvider) HandleError(err error) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, err.Error(), http.StatusUnauthorized)
	}
}

func (m mockProvider) HandleActivityError(_ activity.Context, err error) activity.Result {
	return "auth-error:" + err.Error()
}

func (m mockProvider) LogoutButton() templ.Component {
	return templ.NopComponent
}

func TestWithSessionDecorator(t *testing.T) {
	auth.Use(mockProvider{})

	ctx := web.NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	decorator := auth.WithSession(func(session auth.Session, ctx activity.Context, input activity.NoInput) activity.Result {
		return "user=" + session.GetUserId()
	})

	h := decorator(func(ctx activity.Context, input activity.NoInput) activity.Result {
		return "next"
	})

	result := h(ctx, activity.NoInput{})
	if result != "user=u-1" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestEnforceDecoratorUnauthorized(t *testing.T) {
	auth.Use(mockProvider{err: errors.New("no session")})

	ctx := web.NewContext(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	h := auth.EnforceDecorator[activity.NoInput]()(func(ctx activity.Context, input activity.NoInput) activity.Result {
		return "next"
	})

	result := h(ctx, activity.NoInput{})
	if result != "auth-error:no session" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

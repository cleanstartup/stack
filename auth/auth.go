package auth

import (
	"net/http"

	"github.com/cleanstartup/way2go/activity"
	"github.com/cleanstartup/way2go/web"

	"github.com/a-h/templ"
)

type Session interface {
	GetUserId() string
	GetEmail() string
	GetRequest() *http.Request
}

type Provider interface {
	GetSession(http.ResponseWriter, *http.Request) (Session, error)
	HandleError(error) http.HandlerFunc
	HandleActivityError(activity.Context, error) activity.Result
	LogoutButton() templ.Component
}

var provider Provider

func Use(p Provider) {
	provider = p
}

// Deprecated: kept for compatibility naming.
func UseProvider(p Provider) {
	Use(p)
}

func WithSession[I any](fn func(Session, activity.Context, I) activity.Result) activity.Decorator[I] {
	return func(next activity.Handler[I]) activity.Handler[I] {
		return func(ctx activity.Context, input I) activity.Result {
			if provider == nil {
				return next(ctx, input)
			}

			r := ctx.Request()
			w := ctx.ResponseWriter()
			session, err := provider.GetSession(w, r)
			if err != nil {
				return provider.HandleActivityError(ctx, err)
			}

			return fn(session, ctx, input)
		}
	}
}

func EnforceDecorator[I any]() activity.Decorator[I] {
	return func(next activity.Handler[I]) activity.Handler[I] {
		return func(ctx activity.Context, input I) activity.Result {
			if provider == nil {
				return next(ctx, input)
			}

			r := ctx.Request()
			w := ctx.ResponseWriter()
			_, err := provider.GetSession(w, r)
			if err != nil {
				return provider.HandleActivityError(ctx, err)
			}
			return next(ctx, input)
		}
	}
}

func EnforceMiddleware[P any, I any]() activity.Middleware[P, I] {
	return func(next activity.Executor[P, I]) activity.Executor[P, I] {
		return func(ctx activity.Context, input I) activity.Result {
			if provider == nil {
				return next(ctx, input)
			}
			_, err := provider.GetSession(ctx.ResponseWriter(), ctx.Request())
			if err != nil {
				return provider.HandleActivityError(ctx, err)
			}
			return next(ctx, input)
		}
	}
}

func EnforceWebMiddleware[C any]() web.ActivityOption[C] {
	return web.WithMiddleware(func(next web.ActivityHandler[C]) web.ActivityHandler[C] {
		return func(ctx web.Context[C]) activity.Result {
			if provider == nil {
				return next(ctx)
			}
			_, err := provider.GetSession(ctx.ResponseWriter(), ctx.Request())
			if err != nil {
				return provider.HandleActivityError(ctx, err)
			}
			return next(ctx)
		}
	})
}

// EnforceLegacy keeps compatibility with app.Screen/app.Action option style.
func UserId(ctx activity.Context) string {
	if provider == nil {
		return ""
	}
	s, err := provider.GetSession(ctx.ResponseWriter(), ctx.Request())
	if err != nil {
		return ""
	}
	return s.GetUserId()
}

func Required(next func(Session) http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if provider == nil {
			http.Error(w, "auth provider is not configured", http.StatusInternalServerError)
			return
		}
		session, err := provider.GetSession(w, r)
		if err != nil {
			provider.HandleError(err).ServeHTTP(w, r)
			return
		}

		next(session).ServeHTTP(w, r)
	})
}

func LogoutButton() templ.Component {
	if provider == nil {
		return templ.NopComponent
	}
	return provider.LogoutButton()
}

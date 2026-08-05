package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/cleanstartup/stack/way2go/activity"
	"github.com/cleanstartup/stack/way2go/param"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
)

var _ param.Resolver = (*handlerContext[struct{}])(nil)

type activityMetaContextKey struct{}
type devStateContextKey struct{}
type assetManifestContextKey struct{}

type ActivityMeta struct {
	ID      string
	Pattern string
}

type Registry struct {
	router       chi.Router
	byID         map[string]string
	byPath       map[string]string
	errorHandler ErrorHandler
	pathPrefix   string
	idPrefix     string
	globalMW     []GlobalMiddleware
	devState     *DevState
	assets       AssetLinks
}

type RuntimeContext struct {
	responseWriter http.ResponseWriter
	request        *http.Request
	redirectURI    string
	redirectStatus int
	renderError    ErrorHandler
}

type ErrorHandler func(ctx *RuntimeContext, err error) activity.Result

type Context[C any] interface {
	activity.Context
	Data() C
}
type ActivityHandler[C any] func(ctx Context[C]) activity.Result
type ActivityMiddleware[C any] func(next ActivityHandler[C]) ActivityHandler[C]

// GlobalMiddleware is a registry-wide middleware applied to every activity.
type GlobalMiddleware = activity.ContextMiddleware
type ActivityOption[C any] func(a *WebActivity[C])

type WebActivity[C any] struct {
	id               string
	pattern          string
	handler          ActivityHandler[C]
	middlewares      []ActivityMiddleware[C]
	crossMiddlewares []activity.ContextMiddleware
}

type handlerContext[C any] struct {
	runtime *RuntimeContext
}

func NewRegistry() *Registry {
	return &Registry{
		router:       chi.NewRouter(),
		byID:         map[string]string{},
		byPath:       map[string]string{},
		errorHandler: defaultErrorHandler,
		globalMW:     []GlobalMiddleware{},
	}
}

// NewActivity creates a WebActivity that handles HTTP GET and POST for the given ID.
func NewActivity(id string, handler func(ctx activity.Context) activity.Result, opts ...ActivityOption[struct{}]) *WebActivity[struct{}] {
	if strings.TrimSpace(id) == "" {
		panic("activity id must not be empty")
	}
	if handler == nil {
		panic("handler function must not be nil")
	}
	a := &WebActivity[struct{}]{
		id:      id,
		pattern: activity.PathFromID(id),
		handler: func(ctx Context[struct{}]) activity.Result { return handler(ctx) },
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func WithMiddleware[C any](mw ActivityMiddleware[C]) ActivityOption[C] {
	return func(a *WebActivity[C]) {
		a.middlewares = append(a.middlewares, mw)
	}
}

// WithCrossMiddleware attaches transport-agnostic middlewares to this activity.
// They run at the activity.Context level and work identically in web and CLI.
func WithCrossMiddleware[C any](mw ...activity.ContextMiddleware) ActivityOption[C] {
	return func(a *WebActivity[C]) {
		a.crossMiddlewares = append(a.crossMiddlewares, mw...)
	}
}

func (a *WebActivity[C]) ID() string {
	if a == nil {
		return ""
	}
	return a.id
}

func (a *WebActivity[C]) Pattern() string {
	if a == nil {
		return ""
	}
	return a.pattern
}

func (a *WebActivity[C]) URI() string {
	if a == nil {
		return ""
	}
	return a.pattern
}

// Apply implements Part — binds this activity to a Registrar (routes,
// activities, mounts only; no knowledge of Builder or asset registration).
func (a *WebActivity[C]) Apply(reg Registrar) {
	if reg == nil {
		return
	}
	reg.AddActivity(a)
}

func RegisterWebActivity[C any](r *Registry, a *WebActivity[C]) {
	if r == nil {
		panic("registry is nil")
	}
	if a == nil {
		panic("web activity is nil")
	}
	id := prefixedID(r.idPrefix, a.id)
	pattern := prefixedPath(r.pathPrefix, a.pattern)
	if registeredPattern, exists := r.byID[id]; exists {
		panic(fmt.Sprintf("activity id '%s' already registered for pattern '%s'", id, registeredPattern))
	}
	if registeredID, exists := r.byPath[pattern]; exists {
		panic(fmt.Sprintf("activity pattern '%s' already used by '%s'", pattern, registeredID))
	}

	handler := func(w http.ResponseWriter, req *http.Request) {
		req = withActivityMeta(req, ActivityMeta{ID: id, Pattern: pattern})
		req = withAssetLinks(req, r.assets)
		req = withDevState(req, r.devState)
		ctx := newContext(w, req)
		ctx.renderError = r.errorHandler
		hctx := &handlerContext[C]{runtime: ctx}

		exec := a.handler
		for idx := len(a.middlewares) - 1; idx >= 0; idx-- {
			exec = a.middlewares[idx](exec)
		}
		run := activity.ApplyMiddlewares(
			func(ac activity.Context) activity.Result {
				typedCtx, ok := ac.(Context[C])
				if !ok {
					return ac.Error(fmt.Errorf("unexpected activity context type %T", ac))
				}
				return exec(typedCtx)
			},
			a.crossMiddlewares,
		)
		run = activity.ApplyMiddlewares(run, r.globalMW)

		result := run(hctx)
		if uri, status, ok := redirectState(ctx); ok {
			http.Redirect(w, req, uri, status)
			return
		}
		if err := renderResult(w, req, result); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	r.router.Get(pattern, handler)
	r.router.Post(pattern, handler)
	r.byID[id] = pattern
	r.byPath[pattern] = id
}

func (r *Registry) Handler() http.Handler {
	if r == nil {
		return nil
	}
	return r.router
}

func (r *Registry) Mount(path string, handler http.Handler) {
	if r == nil {
		return
	}
	if handler == nil {
		return
	}
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		req = withAssetLinks(req, r.assets)
		req = withDevState(req, r.devState)
		handler.ServeHTTP(w, req)
	})
	path = strings.TrimSpace(path)
	if path == "" {
		path = "/"
	}
	if path == "/" {
		r.router.Handle(path, wrapped)
		return
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	r.router.Handle(path, wrapped)
	r.router.Handle(path+"/*", wrapped)
}

func (r *Registry) Group(path string) *Registry {
	if r == nil {
		panic("registry is nil")
	}
	path = strings.TrimSpace(path)
	if path == "" || path == "/" {
		return r
	}
	segment := strings.Trim(path, "/")
	if segment == "" {
		return r
	}
	return &Registry{
		router:       r.router,
		byID:         r.byID,
		byPath:       r.byPath,
		errorHandler: r.errorHandler,
		pathPrefix:   prefixedPath(r.pathPrefix, "/"+segment),
		idPrefix:     prefixedID(r.idPrefix, strings.ReplaceAll(segment, "/", ".")),
		globalMW:     r.globalMW,
	}
}

func (r *Registry) UseGlobalMiddleware(mw ...GlobalMiddleware) {
	if r == nil {
		return
	}
	r.globalMW = append(r.globalMW, mw...)
}

func (r *Registry) SetDevState(state *DevState) {
	if r == nil {
		return
	}
	r.devState = state
}

func (r *Registry) SetAssets(links AssetLinks) {
	if r == nil {
		return
	}
	r.assets = links
}

func (r *Registry) RegisterDevEndpoints(state *DevState) {
	if r == nil || state == nil {
		return
	}
	r.router.Get("/__stack/dev/events", state.ServeHTTP)
}

func (r *Registry) SetErrorHandler(handler ErrorHandler) {
	if r == nil || handler == nil {
		return
	}
	r.errorHandler = handler
}

func prefixedPath(prefix string, path string) string {
	if prefix == "" {
		return path
	}
	if path == "/" {
		return prefix
	}
	return strings.TrimSuffix(prefix, "/") + path
}

func prefixedID(prefix string, id string) string {
	if prefix == "" {
		return id
	}
	return prefix + "." + id
}

func NewContext(responseWriter http.ResponseWriter, request *http.Request) *RuntimeContext {
	return newContext(responseWriter, request)
}

func newContext(responseWriter http.ResponseWriter, request *http.Request) *RuntimeContext {
	return &RuntimeContext{
		responseWriter: responseWriter,
		request:        request,
		renderError:    defaultErrorHandler,
	}
}

func (c *RuntimeContext) RedirectToURI(uri string) {
	if c == nil {
		return
	}
	c.redirectURI = uri
	c.redirectStatus = http.StatusFound
}

func (c *RuntimeContext) RedirectToURIWithStatus(uri string, statusCode int) {
	if c == nil {
		return
	}
	c.redirectURI = uri
	c.redirectStatus = statusCode
}

func (c *RuntimeContext) Request() *http.Request {
	if c == nil {
		return nil
	}
	return c.request
}

func (c *RuntimeContext) ResponseWriter() http.ResponseWriter {
	if c == nil {
		return nil
	}
	return c.responseWriter
}

func (c *RuntimeContext) Error(err error) activity.Result {
	if c == nil {
		return err.Error()
	}
	return c.renderError(c, err)
}

func redirectState(ctx activity.Context) (string, int, bool) {
	rctx, ok := runtimeFromActivityContext(ctx)
	if !ok || rctx.redirectURI == "" {
		return "", 0, false
	}
	status := rctx.redirectStatus
	if status == 0 {
		status = http.StatusFound
	}
	return rctx.redirectURI, status, true
}

func (c *handlerContext[C]) Data() C {
	var zero C
	return zero
}

func (c *handlerContext[C]) Request() *http.Request {
	if c == nil || c.runtime == nil {
		return nil
	}
	return c.runtime.Request()
}

func (c *handlerContext[C]) ResponseWriter() http.ResponseWriter {
	if c == nil || c.runtime == nil {
		return nil
	}
	return c.runtime.ResponseWriter()
}

func (c *handlerContext[C]) RedirectToURI(uri string) {
	if c == nil || c.runtime == nil {
		return
	}
	c.runtime.RedirectToURI(uri)
}

func (c *handlerContext[C]) RedirectToURIWithStatus(uri string, statusCode int) {
	if c == nil || c.runtime == nil {
		return
	}
	c.runtime.RedirectToURIWithStatus(uri, statusCode)
}

func (c *handlerContext[C]) Error(err error) activity.Result {
	if c == nil || c.runtime == nil {
		return err
	}
	return c.runtime.Error(err)
}

// Resolve implements param.Resolver so activity.Param[T] works in web handlers.
// It checks chi path params first, then the query string.
func (c *handlerContext[C]) Resolve(names []string) (string, bool) {
	if c == nil || c.runtime == nil || c.runtime.request == nil {
		return "", false
	}
	r := c.runtime.request
	for _, name := range names {
		if v := chi.URLParam(r, name); v != "" {
			return v, true
		}
	}
	q := r.URL.Query()
	for _, name := range names {
		if v := q.Get(name); v != "" {
			return v, true
		}
	}
	return "", false
}

func (c *handlerContext[C]) runtimeContext() *RuntimeContext {
	if c == nil {
		return nil
	}
	return c.runtime
}

func runtimeFromActivityContext(ctx activity.Context) (*RuntimeContext, bool) {
	if ctx == nil {
		return nil, false
	}
	if runtime, ok := ctx.(*RuntimeContext); ok {
		return runtime, true
	}
	type runtimeCarrier interface {
		runtimeContext() *RuntimeContext
	}
	if carrier, ok := ctx.(runtimeCarrier); ok {
		if runtime := carrier.runtimeContext(); runtime != nil {
			return runtime, true
		}
	}
	return nil, false
}

func ActivityMetaFromRequest(req *http.Request) (ActivityMeta, bool) {
	if req == nil {
		return ActivityMeta{}, false
	}
	v := req.Context().Value(activityMetaContextKey{})
	meta, ok := v.(ActivityMeta)
	if !ok {
		return ActivityMeta{}, false
	}
	return meta, true
}

func withActivityMeta(req *http.Request, meta ActivityMeta) *http.Request {
	if req == nil {
		return nil
	}
	ctx := context.WithValue(req.Context(), activityMetaContextKey{}, meta)
	return req.WithContext(ctx)
}

func renderResult(w http.ResponseWriter, r *http.Request, result activity.Result) error {
	if result == nil {
		w.WriteHeader(http.StatusNoContent)
		return nil
	}

	switch typed := result.(type) {
	case Page:
		if state := devStateFromContext(r.Context()); state != nil {
			typed.LiveReloadURL = devLiveReloadScriptURL()
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return typed.Render(r.Context(), w)
	case *Page:
		if typed == nil {
			w.WriteHeader(http.StatusNoContent)
			return nil
		}
		if state := devStateFromContext(r.Context()); state != nil {
			typed.LiveReloadURL = devLiveReloadScriptURL()
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return typed.Render(r.Context(), w)
	case templ.Component:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		templ.Handler(typed).ServeHTTP(w, r)
		return nil
	case interface {
		Render(context.Context, io.Writer) error
	}:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		return typed.Render(r.Context(), w)
	case http.Handler:
		typed.ServeHTTP(w, r)
		return nil
	case http.HandlerFunc:
		typed(w, r)
		return nil
	case []byte:
		_, err := w.Write(typed)
		return err
	case string:
		_, err := io.WriteString(w, typed)
		return err
	default:
		return fmt.Errorf("unsupported web result type %T", result)
	}
}

// RenderResult writes an activity result using the stack renderer.
func RenderResult(w http.ResponseWriter, r *http.Request, result activity.Result) error {
	return renderResult(w, r, result)
}

func defaultErrorHandler(_ *RuntimeContext, err error) activity.Result {
	if errors.Is(err, activity.ErrNotFound) {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	})
}

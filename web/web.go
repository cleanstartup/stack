package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/cleanstartup/stack/activity"
	"github.com/cleanstartup/stack/param"

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
	byActivity   map[uintptr]string
	byRef        map[string]string
	globalMW     []GlobalMiddleware
	devState     *DevState
	assets       AssetManifest
}

type RuntimeContext struct {
	responseWriter http.ResponseWriter
	request        *http.Request
	redirectURI    string
	redirectStatus int
	renderError    ErrorHandler
}

type ErrorHandler func(ctx *RuntimeContext, err error) activity.Result

type InputKey struct {
	name string
}

type Request struct {
	raw        *http.Request
	pathParams map[string]string
	query      url.Values
	form       url.Values
	failed     bool
	errs       []error
}

type IntValidator func(int) error
type DecodeFunc[C any] func(r *Request) C
type Context[C any] interface {
	activity.Context
	Data() C
}
type ActivityHandler[C any] func(ctx Context[C]) activity.Result
type ActivityMiddleware[C any] func(next ActivityHandler[C]) ActivityHandler[C]
// GlobalMiddleware is a registry-wide middleware applied to every activity.
// It is equivalent to activity.ContextMiddleware and can be used interchangeably.
type GlobalMiddleware = activity.ContextMiddleware
type ActivityOption[C any] func(a *WebActivity[C])
type TitleFunc func(ctx activity.Context) string

type URIRef = activity.URIRef

type WebActivity[C any] struct {
	id               string
	pattern          string
	decode           DecodeFunc[C]
	handler          ActivityHandler[C]
	middlewares      []ActivityMiddleware[C]
	crossMiddlewares []activity.ContextMiddleware
	ref              URIRef
	title            TitleFunc
}

type handlerContext[C any] struct {
	runtime *RuntimeContext
	data    C
	req     *Request
}

func NewRegistry() *Registry {
	return &Registry{
		router:       chi.NewRouter(),
		byID:         map[string]string{},
		byPath:       map[string]string{},
		errorHandler: defaultErrorHandler,
		pathPrefix:   "",
		idPrefix:     "",
		byActivity:   map[uintptr]string{},
		byRef:        map[string]string{},
		globalMW:     []GlobalMiddleware{},
		devState:     nil,
		assets:       AssetManifest{},
	}
}

var defaultRegistry = NewRegistry()
var uriBindingsMu sync.RWMutex

func Reset() { defaultRegistry = NewRegistry() }

func RegisterDefault[P any, I any](a *activity.Activity[P, I]) { RegisterActivity(defaultRegistry, a) }

func RegisterWebDefault[C any](a *WebActivity[C]) { RegisterWebActivity(defaultRegistry, a) }

func SetDefaultErrorHandler(handler ErrorHandler) { defaultRegistry.SetErrorHandler(handler) }

func DefaultHandler() http.Handler { return defaultRegistry.Handler() }

func MountDefault(path string, handler http.Handler) { defaultRegistry.Mount(path, handler) }

func DefaultGroup(path string) *Registry { return defaultRegistry.Group(path) }

func UseDefaultGlobalMiddleware(mw ...GlobalMiddleware) {
	defaultRegistry.UseGlobalMiddleware(mw...)
}

func URI(target any) string {
	uri, ok := URIOk(target)
	if ok {
		return uri
	}
	switch t := target.(type) {
	case nil:
		panic("web uri target is nil")
	case URIRef:
		return activity.PathFromID(t.ID())
	case *URIRef:
		if t == nil {
			panic("web uri target is nil")
		}
		return activity.PathFromID(t.ID())
	default:
		if uriTarget, ok := target.(interface{ ID() string }); ok {
			return activity.PathFromID(uriTarget.ID())
		}
		panic("web uri target is not registered")
	}
}

func URIOk(target any) (string, bool) {
	switch t := target.(type) {
	case nil:
		return "", false
	case URIRef:
		return URIOkByRef(t)
	case *URIRef:
		if t == nil {
			return "", false
		}
		return URIOkByRef(*t)
	}
	v := reflect.ValueOf(target)
	if !v.IsValid() || v.Kind() != reflect.Ptr || v.IsNil() {
		return "", false
	}
	if _, ok := target.(interface{ ID() string }); !ok {
		return "", false
	}
	key := v.Pointer()
	uriBindingsMu.RLock()
	defer uriBindingsMu.RUnlock()
	uri, ok := defaultRegistry.byActivity[key]
	return uri, ok
}

func URIOkByRef(ref URIRef) (string, bool) {
	uriBindingsMu.RLock()
	defer uriBindingsMu.RUnlock()
	uri, ok := defaultRegistry.byRef[ref.ID()]
	return uri, ok
}

func Input(name string) InputKey {
	name = strings.TrimSpace(name)
	if name == "" {
		panic("input name must not be empty")
	}
	return InputKey{name: name}
}

func RawActivity[C any](id string, decode DecodeFunc[C], handler ActivityHandler[C], opts ...ActivityOption[C]) *WebActivity[C] {
	if strings.TrimSpace(id) == "" {
		panic("activity id must not be empty")
	}
	if decode == nil {
		panic("decode function must not be nil")
	}
	if handler == nil {
		panic("handler function must not be nil")
	}
	a := &WebActivity[C]{
		id:      id,
		pattern: activity.PathFromID(id),
		decode:  decode,
		handler: handler,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func Activity(ref URIRef, handler func(ctx activity.Context) activity.Result, opts ...ActivityOption[struct{}]) *WebActivity[struct{}] {
	if handler == nil {
		panic("handler function must not be nil")
	}
	if strings.TrimSpace(ref.ID()) == "" {
		panic("ref must not be nil")
	}
	opts = append(opts, WithRef[struct{}](ref))
	return RawActivity(
		ref.ID(),
		func(r *Request) struct{} { return struct{}{} },
		func(ctx Context[struct{}]) activity.Result {
			return handler(ctx)
		},
		opts...,
	)
}

func Simple(ref URIRef, handler func(ctx activity.Context) activity.Result, opts ...ActivityOption[struct{}]) *WebActivity[struct{}] {
	return Activity(ref, handler, opts...)
}

func WithTitle[C any](title TitleFunc) ActivityOption[C] {
	return func(a *WebActivity[C]) {
		if a == nil {
			return
		}
		a.title = title
	}
}

func WithStaticTitle(title string) ActivityOption[struct{}] {
	return WithTitle[struct{}](func(ctx activity.Context) string {
		_ = ctx
		return title
	})
}

// WithCrossMiddleware attaches transport-agnostic middlewares to this activity.
// They run at the activity.Context level, after typed middlewares and before
// registry-wide GlobalMiddleware, so they work identically in web and CLI.
func WithCrossMiddleware[C any](mw ...activity.ContextMiddleware) ActivityOption[C] {
	return func(a *WebActivity[C]) {
		a.crossMiddlewares = append(a.crossMiddlewares, mw...)
	}
}

func WithMiddleware[C any](mw ActivityMiddleware[C]) ActivityOption[C] {
	return func(a *WebActivity[C]) {
		a.middlewares = append(a.middlewares, mw)
	}
}

func WithRef[C any](ref URIRef) ActivityOption[C] {
	return func(a *WebActivity[C]) {
		if strings.TrimSpace(ref.ID()) == "" {
			panic("ref must not be nil")
		}
		a.ref = ref
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

func (a *WebActivity[C]) Apply(app *WebApp) {
	if app == nil {
		return
	}
	AddWebActivity(app.builder, a)
}

func (a *WebActivity[C]) register(r *Registry) {
	RegisterWebActivity(r, a)
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
		req = withActivityMeta(req, ActivityMeta{
			ID:      id,
			Pattern: pattern,
		})
		req = withAssetManifest(req, r.assets)
		req = withDevState(req, r.devState)
		rq := newRequest(req, map[string]string{}, req.URL.Query())
		decoded := a.decode(rq)
		if rq.Failed() {
			http.Error(w, rq.Error().Error(), http.StatusBadRequest)
			return
		}
		ctx := NewContext(w, req)
		ctx.renderError = r.errorHandler
		hctx := &handlerContext[C]{runtime: ctx, data: decoded, req: rq}

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
		if title := strings.TrimSpace(resolveTitle(a.title, hctx)); title != "" {
			result = wrapPageResult(result, title)
		}
		if uri, status, ok := RedirectState(ctx); ok {
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
	uriBindingsMu.Lock()
	r.byActivity[reflect.ValueOf(a).Pointer()] = pattern
	if strings.TrimSpace(a.ref.ID()) != "" {
		r.byRef[a.ref.ID()] = pattern
	}
	uriBindingsMu.Unlock()
}

func Register[P any, I any](r *Registry, def *activity.Definition[P, I], resolver activity.Resolver[P, I]) {
	if r == nil {
		panic("registry is nil")
	}
	if def == nil {
		panic("activity definition is nil")
	}

	id := prefixedID(r.idPrefix, def.ID())
	pattern := prefixedPath(r.pathPrefix, def.Pattern())
	if registeredPattern, exists := r.byID[id]; exists {
		panic(fmt.Sprintf("activity id '%s' already registered for pattern '%s'", id, registeredPattern))
	}
	if registeredID, exists := r.byPath[pattern]; exists {
		panic(fmt.Sprintf("activity pattern '%s' already used by '%s'", pattern, registeredID))
	}

	handler := func(w http.ResponseWriter, req *http.Request) {
		req = withAssetManifest(req, r.assets)
		req = withDevState(req, r.devState)
		pathParams := map[string]string{}
		for _, name := range def.PathParamNames() {
			pathParams[name] = chi.URLParam(req, name)
		}

		params, err := def.DecodeParams(pathParams, req.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		input, err := decodeInput[I](req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx := NewContext(w, req)
		ctx.renderError = r.errorHandler
		result := resolver(params).Handle(ctx, input)

		if uri, status, ok := RedirectState(ctx); ok {
			http.Redirect(w, req, uri, status)
			return
		}

		if err := renderResult(w, req, result); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	if isNoInput[I]() {
		r.router.Get(pattern, handler)
	} else {
		r.router.Post(pattern, handler)
	}

	r.byID[id] = pattern
	r.byPath[pattern] = id
}

func RegisterActivity[P any, I any](r *Registry, a *activity.Activity[P, I]) {
	if r == nil {
		panic("registry is nil")
	}
	if a == nil {
		panic("activity is nil")
	}

	id := prefixedID(r.idPrefix, a.ID())
	pattern := prefixedPath(r.pathPrefix, a.Pattern())
	if registeredPattern, exists := r.byID[id]; exists {
		panic(fmt.Sprintf("activity id '%s' already registered for pattern '%s'", id, registeredPattern))
	}
	if registeredID, exists := r.byPath[pattern]; exists {
		panic(fmt.Sprintf("activity pattern '%s' already used by '%s'", pattern, registeredID))
	}

	handler := func(w http.ResponseWriter, req *http.Request) {
		req = withAssetManifest(req, r.assets)
		req = withDevState(req, r.devState)
		pathParams := map[string]string{}
		for _, name := range a.PathParamNames() {
			pathParams[name] = chi.URLParam(req, name)
		}

		params, err := a.DecodeParams(pathParams, req.URL.Query())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		input, err := decodeInput[I](req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ctx := NewContext(w, req)
		ctx.renderError = r.errorHandler
		result := a.Handle(ctx, params, input)
		if uri, status, ok := RedirectState(ctx); ok {
			http.Redirect(w, req, uri, status)
			return
		}
		if err := renderResult(w, req, result); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}

	if isNoInput[I]() {
		r.router.Get(pattern, handler)
	} else {
		r.router.Post(pattern, handler)
	}

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
		req = withAssetManifest(req, r.assets)
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
		byActivity:   r.byActivity,
		byRef:        r.byRef,
		globalMW:     r.globalMW,
	}
}

func resolveTitle(title TitleFunc, ctx activity.Context) string {
	if title == nil {
		return ""
	}
	return title(ctx)
}

func wrapPageResult(result activity.Result, title string) activity.Result {
	switch typed := result.(type) {
	case nil:
		return Page{Title: title}
	case Page:
		if strings.TrimSpace(typed.Title) == "" {
			typed.Title = title
		}
		return typed
	case *Page:
		if typed == nil {
			return Page{Title: title}
		}
		if strings.TrimSpace(typed.Title) == "" {
			typed.Title = title
		}
		return typed
	default:
		return Page{Title: title, Body: result}
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

func (r *Registry) SetAssets(manifest AssetManifest) {
	if r == nil {
		return
	}
	r.assets = manifest
}

func (r *Registry) RegisterDevEndpoints(state *DevState) {
	if r == nil || state == nil {
		return
	}
	r.router.Get("/__stack/dev/events", state.ServeHTTP)
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
	return HTTPRequest(c)
}

func (c *RuntimeContext) ResponseWriter() http.ResponseWriter {
	return ResponseWriter(c)
}

func (c *RuntimeContext) Error(err error) activity.Result {
	if c == nil {
		return err.Error()
	}
	return c.renderError(c, err)
}

func RedirectTo[P any, I any](ctx activity.Context, instance activity.Instance[P, I]) {
	if ctx == nil {
		return
	}
	ctx.RedirectToURI(instance.URI())
}

func RedirectToWithStatus[P any, I any](ctx activity.Context, instance activity.Instance[P, I], statusCode int) {
	if ctx == nil {
		return
	}
	ctx.RedirectToURIWithStatus(instance.URI(), statusCode)
}

func RedirectState(ctx activity.Context) (string, int, bool) {
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

func HTTPRequest(ctx activity.Context) *http.Request {
	rctx, ok := runtimeFromActivityContext(ctx)
	if !ok {
		return nil
	}
	return rctx.request
}

func ResponseWriter(ctx activity.Context) http.ResponseWriter {
	rctx, ok := runtimeFromActivityContext(ctx)
	if !ok {
		return nil
	}
	return rctx.responseWriter
}

func (r *Registry) SetErrorHandler(handler ErrorHandler) {
	if r == nil || handler == nil {
		return
	}
	r.errorHandler = handler
}

func (c *handlerContext[C]) Data() C {
	if c == nil {
		var zero C
		return zero
	}
	return c.data
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
// It checks path params first, then query string, for each candidate name.
func (c *handlerContext[C]) Resolve(names []string) (string, bool) {
	if c == nil || c.req == nil {
		return "", false
	}
	for _, name := range names {
		if v, ok := c.req.pathParams[name]; ok && v != "" {
			return v, true
		}
	}
	for _, name := range names {
		if v := c.req.query.Get(name); v != "" {
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
		if page, ok := typed.(*Page); ok {
			if state := devStateFromContext(r.Context()); state != nil {
				page.LiveReloadURL = devLiveReloadScriptURL()
			}
		}
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

// RenderResult writes an activity result using the stack renderer so page assets,
// layout wrappers, and dev reload hooks are applied consistently.
func RenderResult(w http.ResponseWriter, r *http.Request, result activity.Result) error {
	return renderResult(w, r, result)
}

func defaultErrorHandler(_ *RuntimeContext, err error) activity.Result {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	})
}

func decodeInput[I any](r *http.Request) (I, error) {
	var out I
	if isNoInput[I]() {
		return out, nil
	}
	if err := r.ParseForm(); err != nil {
		return out, err
	}
	v := reflect.ValueOf(&out).Elem()
	if err := decodeStruct(v, formLookup(r.PostForm)); err != nil {
		return out, err
	}
	return out, nil
}

type lookupFunc = func(keys []string) (string, bool)

func formLookup(values url.Values) lookupFunc {
	return func(keys []string) (string, bool) {
		for _, key := range keys {
			if value := values.Get(key); value != "" {
				return value, true
			}
		}
		return "", false
	}
}

func decodeStruct(v reflect.Value, lookup lookupFunc) error {
	if !v.IsValid() {
		return nil
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		t := v.Type()
		for idx := 0; idx < t.NumField(); idx++ {
			field := t.Field(idx)
			if field.PkgPath != "" {
				continue
			}
			target := v.Field(idx)
			keys := fieldKeys(field)
			raw, ok := lookup(keys)
			if !ok {
				continue
			}
			if err := setFromString(target, raw); err != nil {
				return fmt.Errorf("field %s: %w", field.Name, err)
			}
		}
		return nil
	default:
		raw, ok := lookup([]string{"value"})
		if !ok {
			return nil
		}
		return setFromString(v, raw)
	}
}

func fieldKeys(field reflect.StructField) []string {
	keys := []string{}
	for _, tag := range []string{"activity", "form", "json"} {
		if value, ok := tagValue(field, tag); ok {
			keys = append(keys, value)
		}
	}
	defaults := []string{field.Name, lowerFirst(field.Name), toSnake(field.Name), toKebab(field.Name)}
	for _, candidate := range defaults {
		if !contains(keys, candidate) {
			keys = append(keys, candidate)
		}
	}
	return keys
}

func setFromString(v reflect.Value, raw string) error {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(raw)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		v.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(raw, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetInt(parsed)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		parsed, err := strconv.ParseUint(raw, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetUint(parsed)
	case reflect.Float32, reflect.Float64:
		parsed, err := strconv.ParseFloat(raw, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetFloat(parsed)
	default:
		return fmt.Errorf("unsupported field type %s", v.Type().String())
	}
	return nil
}

func tagValue(field reflect.StructField, tag string) (string, bool) {
	value := field.Tag.Get(tag)
	if value == "" || value == "-" {
		return "", false
	}
	parts := strings.Split(value, ",")
	if len(parts) == 0 || parts[0] == "" {
		return "", false
	}
	return parts[0], true
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	if len(s) == 1 {
		return strings.ToLower(s)
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func toSnake(s string) string {
	return splitWords(s, "_")
}

func toKebab(s string) string {
	return splitWords(s, "-")
}

func splitWords(s string, sep string) string {
	if s == "" {
		return s
	}
	var out strings.Builder
	for idx, r := range s {
		if idx > 0 && r >= 'A' && r <= 'Z' {
			out.WriteString(sep)
		}
		out.WriteRune(r)
	}
	return strings.ToLower(out.String())
}

func isNoInput[I any]() bool {
	t := reflect.TypeOf((*I)(nil)).Elem()
	return t == reflect.TypeOf(activity.NoInput{})
}

func newRequest(raw *http.Request, pathParams map[string]string, query url.Values) *Request {
	return &Request{
		raw:        raw,
		pathParams: pathParams,
		query:      query,
	}
}

func (r *Request) Fail(err error) {
	if r == nil || err == nil {
		return
	}
	r.failed = true
	r.errs = append(r.errs, err)
}

func (r *Request) Failed() bool {
	if r == nil {
		return false
	}
	return r.failed
}

func (r *Request) Error() error {
	if r == nil {
		return nil
	}
	return errors.Join(r.errs...)
}

func (r *Request) IntParam(key InputKey, validators ...IntValidator) int {
	if key.name == "" {
		r.Fail(fmt.Errorf("input key must not be empty"))
		return 0
	}
	raw := ""
	if value, ok := r.pathParams[key.name]; ok {
		raw = value
	}
	if raw == "" {
		raw = r.query.Get(key.name)
	}
	if raw == "" {
		r.Fail(fmt.Errorf("missing param '%s'", key.name))
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		r.Fail(fmt.Errorf("invalid int param '%s': %w", key.name, err))
		return 0
	}
	for _, validate := range validators {
		if validate == nil {
			continue
		}
		if err := validate(parsed); err != nil {
			r.Fail(fmt.Errorf("invalid param '%s': %w", key.name, err))
		}
	}
	return parsed
}

func (r *Request) IntField(key InputKey, validators ...IntValidator) int {
	if key.name == "" {
		r.Fail(fmt.Errorf("input key must not be empty"))
		return 0
	}
	if r.raw == nil {
		r.Fail(fmt.Errorf("request is nil"))
		return 0
	}
	if r.form == nil {
		if err := r.raw.ParseForm(); err != nil {
			r.Fail(fmt.Errorf("parse form failed: %w", err))
			return 0
		}
		r.form = r.raw.PostForm
	}
	raw := r.form.Get(key.name)
	if raw == "" {
		r.Fail(fmt.Errorf("missing field '%s'", key.name))
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		r.Fail(fmt.Errorf("invalid int field '%s': %w", key.name, err))
		return 0
	}
	for _, validate := range validators {
		if validate == nil {
			continue
		}
		if err := validate(parsed); err != nil {
			r.Fail(fmt.Errorf("invalid field '%s': %w", key.name, err))
		}
	}
	return parsed
}

package activity

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

type Result = any

type Context interface {
	Request() *http.Request
	ResponseWriter() http.ResponseWriter
	RedirectToURI(uri string)
	RedirectToURIWithStatus(uri string, statusCode int)
	Error(err error) Result
}

type NoInput struct{}

type Executor[P any, I any] func(ctx Context, input I) Result
type Middleware[P any, I any] func(next Executor[P, I]) Executor[P, I]
type Handler[I any] func(ctx Context, input I) Result
type Decorator[I any] func(next Handler[I]) Handler[I]

type Resolver[P any, I any] func(params P) Instance[P, I]
type Option[P any, I any] func(*Activity[P, I])

type Definition[P any, I any] struct {
	id           string
	pattern      string
	placeholders []string
	middlewares  []Middleware[P, I]
}

type Instance[P any, I any] struct {
	definition  *Definition[P, I]
	params      P
	executor    Executor[P, I]
	middlewares []Middleware[P, I]
}

type Activity[P any, I any] struct {
	definition *Definition[P, I]
	executor   Executor[P, I]
}

func As[P any, I any](id string) *Definition[P, I] {
	if strings.TrimSpace(id) == "" {
		panic("activity id must not be empty")
	}
	pattern := PathFromID(id)

	return &Definition[P, I]{
		id:           id,
		pattern:      pattern,
		placeholders: parsePlaceholders(pattern),
	}
}

func Def[P any, I any](id string, handler Executor[P, I], opts ...Option[P, I]) *Activity[P, I] {
	a := &Activity[P, I]{
		definition: As[P, I](id),
		executor:   handler,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func PathFromID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		panic("activity id must not be empty")
	}
	if strings.EqualFold(id, "root") {
		return "/"
	}
	parts := strings.Split(id, ".")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		clean = append(clean, part)
	}
	if len(clean) == 0 {
		panic("activity id must not be empty")
	}
	return "/" + strings.Join(clean, "/")
}

func WithMiddleware[P any, I any](middleware Middleware[P, I]) Option[P, I] {
	return func(a *Activity[P, I]) {
		a.definition.Use(middleware)
	}
}

func (a *Activity[P, I]) ID() string {
	return a.definition.ID()
}

func (a *Activity[P, I]) Pattern() string {
	return a.definition.Pattern()
}

func (a *Activity[P, I]) PathParamNames() []string {
	return a.definition.PathParamNames()
}

func (a *Activity[P, I]) DecodeParams(pathParams map[string]string, query url.Values) (P, error) {
	return a.definition.DecodeParams(pathParams, query)
}

func (a *Activity[P, I]) URI(params ...P) string {
	var p P
	if len(params) > 0 {
		p = params[0]
	}
	return a.definition.Take(p).Then(a.executor).URI()
}

func (a *Activity[P, I]) Handle(ctx Context, params P, input I, decorators ...Decorator[I]) Result {
	return a.definition.Take(params).Then(a.executor).Handle(ctx, input, decorators...)
}

func (d *Definition[P, I]) ID() string {
	return d.id
}

func (d *Definition[P, I]) Pattern() string {
	return d.pattern
}

func (d *Definition[P, I]) PathParamNames() []string {
	out := make([]string, len(d.placeholders))
	copy(out, d.placeholders)
	return out
}

func (d *Definition[P, I]) Take(params P) Instance[P, I] {
	return Instance[P, I]{
		definition: d,
		params:     params,
	}
}

func (d *Definition[P, I]) DecodeParams(pathParams map[string]string, query url.Values) (P, error) {
	return decodeParams[P](pathParams, query)
}

func (d *Definition[P, I]) Use(middlewares ...Middleware[P, I]) *Definition[P, I] {
	d.middlewares = append(d.middlewares, middlewares...)
	return d
}

func (i Instance[P, I]) Then(executor Executor[P, I]) Instance[P, I] {
	i.executor = executor
	return i
}

func (i Instance[P, I]) Use(middlewares ...Middleware[P, I]) Instance[P, I] {
	i.middlewares = append(i.middlewares, middlewares...)
	return i
}

func (i Instance[P, I]) URI() string {
	if i.definition == nil {
		panic("activity instance has no definition")
	}
	uri, err := buildURI(i.definition.pattern, i.definition.placeholders, i.params)
	if err != nil {
		panic(err)
	}
	return uri
}

func (i Instance[P, I]) Run(ctx Context, input I) Result {
	return i.Handle(ctx, input)
}

func (i Instance[P, I]) Handle(ctx Context, input I, decorators ...Decorator[I]) Result {
	if i.definition == nil {
		return ctx.Error(fmt.Errorf("activity instance has no definition"))
	}
	if i.executor == nil {
		return ctx.Error(fmt.Errorf("activity '%s' has no executor", i.definition.id))
	}
	executor := i.executor
	for idx := len(i.middlewares) - 1; idx >= 0; idx-- {
		executor = i.middlewares[idx](executor)
	}
	for idx := len(i.definition.middlewares) - 1; idx >= 0; idx-- {
		executor = i.definition.middlewares[idx](executor)
	}
	handler := Handler[I](executor)
	for idx := len(decorators) - 1; idx >= 0; idx-- {
		handler = decorators[idx](handler)
	}
	return handler(ctx, input)
}

var placeholderExpr = regexp.MustCompile(`\{([^{}]+)\}`)

func parsePlaceholders(pattern string) []string {
	matches := placeholderExpr.FindAllStringSubmatch(pattern, -1)
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		out = append(out, m[1])
	}
	return out
}

func decodeParams[P any](pathParams map[string]string, query url.Values) (P, error) {
	var out P
	v := reflect.ValueOf(&out).Elem()

	normalizedPath := map[string]string{}
	for key, value := range pathParams {
		normalizedPath[normalize(key)] = value
	}
	normalizedQuery := map[string]string{}
	for key, values := range query {
		if len(values) == 0 {
			continue
		}
		normalizedQuery[normalize(key)] = values[0]
	}

	if err := decodeValue(v, func(keys []string) (string, bool) {
		for _, key := range keys {
			if value, ok := pathParams[key]; ok {
				return value, true
			}
			if value, ok := normalizedPath[normalize(key)]; ok {
				return value, true
			}
		}
		for _, key := range keys {
			if value := query.Get(key); value != "" {
				return value, true
			}
			if value, ok := normalizedQuery[normalize(key)]; ok {
				return value, true
			}
		}
		return "", false
	}); err != nil {
		return out, err
	}

	return out, nil
}

func buildURI[P any](pattern string, placeholders []string, params P) (string, error) {
	values, err := exportFieldValues(reflect.ValueOf(params))
	if err != nil {
		return "", err
	}

	replaced := pattern
	used := make(map[string]struct{}, len(placeholders))
	for _, name := range placeholders {
		entry, ok := findValue(values, name)
		if !ok {
			return "", fmt.Errorf("missing parameter '%s' for pattern '%s'", name, pattern)
		}
		replaced = strings.ReplaceAll(replaced, "{"+name+"}", url.PathEscape(entry.value))
		used[normalize(name)] = struct{}{}
	}

	q := url.Values{}
	for _, entry := range values {
		if _, ok := used[entry.norm]; ok {
			continue
		}
		if entry.value == "" {
			continue
		}
		q.Set(entry.key, entry.value)
	}

	if encoded := q.Encode(); encoded != "" {
		return replaced + "?" + encoded, nil
	}
	return replaced, nil
}

type lookupFunc = func(keys []string) (string, bool)

func decodeValue(v reflect.Value, lookup lookupFunc) error {
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
		return decodeStruct(v, lookup)
	default:
		value, ok := lookup([]string{"value"})
		if !ok {
			return nil
		}
		return setFromString(v, value)
	}
}

func decodeStruct(v reflect.Value, lookup lookupFunc) error {
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

type exportedValue struct {
	key   string
	norm  string
	value string
}

func exportFieldValues(v reflect.Value) ([]exportedValue, error) {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil, nil
		}
		v = v.Elem()
	}

	if !v.IsValid() {
		return nil, nil
	}
	if v.Kind() != reflect.Struct {
		return []exportedValue{{key: "value", norm: "value", value: fmt.Sprint(v.Interface())}}, nil
	}

	t := v.Type()
	out := make([]exportedValue, 0, t.NumField())
	for idx := 0; idx < t.NumField(); idx++ {
		field := t.Field(idx)
		if field.PkgPath != "" {
			continue
		}
		value := v.Field(idx)
		if !value.IsValid() {
			continue
		}
		entryKey := defaultKey(field)
		out = append(out, exportedValue{
			key:   entryKey,
			norm:  normalize(entryKey),
			value: fmt.Sprint(value.Interface()),
		})
	}
	return out, nil
}

func findValue(values []exportedValue, name string) (exportedValue, bool) {
	n := normalize(name)
	for _, v := range values {
		if v.norm == n {
			return v, true
		}
	}
	return exportedValue{}, false
}

func fieldKeys(field reflect.StructField) []string {
	keys := []string{}
	for _, tag := range []string{"activity", "uri", "path", "form", "json"} {
		if value, ok := tagValue(field, tag); ok {
			keys = append(keys, value)
		}
	}

	defaults := []string{
		field.Name,
		lowerFirst(field.Name),
		toSnake(field.Name),
		toKebab(field.Name),
	}
	for _, candidate := range defaults {
		if !contains(keys, candidate) {
			keys = append(keys, candidate)
		}
	}
	return keys
}

func defaultKey(field reflect.StructField) string {
	for _, tag := range []string{"uri", "path", "query", "json"} {
		if value, ok := tagValue(field, tag); ok {
			return value
		}
	}
	return lowerFirst(field.Name)
}

func contains(values []string, candidate string) bool {
	for _, value := range values {
		if value == candidate {
			return true
		}
	}
	return false
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

func normalize(value string) string {
	value = strings.ToLower(value)
	value = strings.ReplaceAll(value, "_", "")
	value = strings.ReplaceAll(value, "-", "")
	return value
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

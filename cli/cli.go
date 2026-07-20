package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/cleanstartup/stack/activity"
	"github.com/cleanstartup/stack/param"
)

type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type RuntimeContext struct {
	stdout strings.Builder
	stderr strings.Builder
}

type Registry struct {
	commands map[string]func(args []string, pp pathParams) Result
	help     map[string]string
	byID     map[string]string
	mounts   map[string]*Registry
	dynamic  *dynamicRoute
}

// dynamicRoute holds the single dynamic path segment mounted at a registry
// level, e.g. "{selector}" -> sub-registry. Only one dynamic segment name is
// allowed per level; static mounts and commands are always matched first.
type dynamicRoute struct {
	param    string
	registry *Registry
}

// pathParams carries values captured from dynamic path segments (e.g.
// "{selector}") down to the Invocation of the eventually matched command.
type pathParams map[string]string

func (p pathParams) with(name, value string) pathParams {
	out := make(pathParams, len(p)+1)
	for k, v := range p {
		out[k] = v
	}
	out[name] = value
	return out
}

// dynamicSegmentName reports the param name for a dynamic path segment like
// "{selector}", mirroring the {name} convention chi uses for web routes.
func dynamicSegmentName(segment string) (string, bool) {
	if len(segment) < 3 || segment[0] != '{' || segment[len(segment)-1] != '}' {
		return "", false
	}
	name := strings.TrimSpace(segment[1 : len(segment)-1])
	if name == "" {
		return "", false
	}
	return name, true
}

type ParamKey struct {
	names []string
}

type Invocation struct {
	rawArgs    []string
	flags      map[string]string
	pathParams pathParams
	failed     bool
	errs       []error
}

type IntValidator func(int) error
type StringValidator func(string) error
type StringParamOption func(*stringParamConfig)
type DecodeFunc[C any] func(inv *Invocation) C
type Context[C any] interface {
	activity.Context
	Data() C
	Stdout(format string, values ...any)
	Stderr(format string, values ...any)
}
type ActivityHandler[C any] func(ctx Context[C]) Result
type ActivityMiddleware[C any] func(next ActivityHandler[C]) ActivityHandler[C]
type ActivityOption[C any] func(a *CliActivity[C])

type CliActivity[C any] struct {
	id               string
	help             string
	decode           DecodeFunc[C]
	handler          ActivityHandler[C]
	middlewares      []ActivityMiddleware[C]
	crossMiddlewares []activity.ContextMiddleware
}

type handlerContext[C any] struct {
	runtime *RuntimeContext
	data    C
	inv     *Invocation
}

type stringParamConfig struct {
	prompt     string
	required   bool
	validators []StringValidator
}

func NewRegistry() *Registry {
	return &Registry{
		commands: map[string]func(args []string, pp pathParams) Result{},
		help:     map[string]string{},
		byID:     map[string]string{},
		mounts:   map[string]*Registry{},
	}
}

var defaultRegistry = NewRegistry()

func Reset() { defaultRegistry = NewRegistry() }

func RegisterDefault[C any](a *CliActivity[C]) { RegisterActivity(defaultRegistry, a) }

func MountDefault(path string, r *Registry) { defaultRegistry.Mount(path, r) }

func DefaultGroup(path string) *Registry { return defaultRegistry.Group(path) }

func Run(args []string) int { return defaultRegistry.Run(args) }

func Execute(args []string) Result { return defaultRegistry.Execute(args) }

func Param(name string, aliases ...string) ParamKey {
	name = strings.TrimSpace(name)
	if name == "" {
		panic("param name must not be empty")
	}
	out := []string{name}
	for _, alias := range aliases {
		alias = strings.TrimSpace(alias)
		if alias == "" {
			continue
		}
		out = append(out, alias)
	}
	return ParamKey{names: out}
}

func Activity[C any](id string, decode DecodeFunc[C], handler ActivityHandler[C], opts ...ActivityOption[C]) *CliActivity[C] {
	if strings.TrimSpace(id) == "" {
		panic("activity id must not be empty")
	}
	if decode == nil {
		panic("decode function must not be nil")
	}
	if handler == nil {
		panic("handler function must not be nil")
	}
	a := &CliActivity[C]{
		id:      id,
		decode:  decode,
		handler: handler,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

func Simple(id string, handler func(ctx activity.Context) Result, opts ...ActivityOption[struct{}]) *CliActivity[struct{}] {
	if handler == nil {
		panic("handler function must not be nil")
	}
	return Activity(
		id,
		func(_ *Invocation) struct{} { return struct{}{} },
		func(ctx Context[struct{}]) Result {
			return handler(ctx)
		},
		opts...,
	)
}

func WithMiddleware[C any](mw ActivityMiddleware[C]) ActivityOption[C] {
	return func(a *CliActivity[C]) {
		a.middlewares = append(a.middlewares, mw)
	}
}

// WithCrossMiddleware attaches transport-agnostic middlewares to this activity.
// They run at the activity.Context level, after typed middlewares, and work
// identically in web and CLI targets.
func WithCrossMiddleware[C any](mw ...activity.ContextMiddleware) ActivityOption[C] {
	return func(a *CliActivity[C]) {
		a.crossMiddlewares = append(a.crossMiddlewares, mw...)
	}
}

func WithHelp[C any](text string) ActivityOption[C] {
	return func(a *CliActivity[C]) {
		if a == nil {
			return
		}
		a.help = strings.TrimSpace(text)
	}
}

func (a *CliActivity[C]) ID() string {
	if a == nil {
		return ""
	}
	return a.id
}

func RegisterActivity[C any](r *Registry, a *CliActivity[C]) {
	if r == nil {
		panic("registry is nil")
	}
	if a == nil {
		panic("cli activity is nil")
	}
	if name, exists := r.byID[a.id]; exists {
		panic(fmt.Sprintf("activity id '%s' already registered for command '%s'", a.id, name))
	}
	if _, exists := r.commands[a.id]; exists {
		panic(fmt.Sprintf("command '%s' already registered", a.id))
	}

	r.commands[a.id] = func(args []string, pp pathParams) Result {
		inv := newInvocation(args)
		inv.pathParams = pp
		decoded := a.decode(inv)
		if inv.Failed() {
			return Error(inv.Error().Error())
		}
		ctx := &RuntimeContext{}
		hctx := &handlerContext[C]{runtime: ctx, data: decoded, inv: inv}

		exec := a.handler
		for idx := len(a.middlewares) - 1; idx >= 0; idx-- {
			exec = a.middlewares[idx](exec)
		}
		run := activity.ApplyMiddlewares(
			func(ac activity.Context) activity.Result {
				return exec(ac.(Context[C]))
			},
			a.crossMiddlewares,
		)
		raw := run(hctx)
		result, _ := raw.(Result)
		return ctx.merge(result)
	}
	r.help[a.id] = strings.TrimSpace(a.help)
	r.byID[a.id] = a.id
}

func (r *Registry) Mount(path string, mounted *Registry) {
	if r == nil || mounted == nil {
		return
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return
	}
	if name, ok := dynamicSegmentName(path); ok {
		if r.dynamic != nil && r.dynamic.param != name {
			panic(fmt.Sprintf("dynamic segment '{%s}' conflicts with existing dynamic segment '{%s}' at this level", name, r.dynamic.param))
		}
		r.dynamic = &dynamicRoute{param: name, registry: mounted}
		return
	}
	r.mounts[path] = mounted
}

func (r *Registry) Group(path string) *Registry {
	if r == nil {
		panic("registry is nil")
	}
	child := NewRegistry()
	r.Mount(path, child)
	return child
}

// GetOrCreateGroup returns the existing sub-registry for name, or creates one.
// Use this when multiple activities share the same group name.
func (r *Registry) GetOrCreateGroup(name string) *Registry {
	if r == nil {
		panic("registry is nil")
	}
	if paramName, ok := dynamicSegmentName(name); ok {
		if r.dynamic != nil && r.dynamic.param == paramName {
			return r.dynamic.registry
		}
		return r.Group(name)
	}
	if sub, ok := r.mounts[name]; ok {
		return sub
	}
	return r.Group(name)
}

func (r *Registry) Run(args []string) int {
	result := r.Execute(args)
	if result.Stdout != "" {
		fmt.Print(result.Stdout)
	}
	if result.Stderr != "" {
		fmt.Print(result.Stderr)
	}
	return result.ExitCode
}

func (r *Registry) Execute(args []string) Result {
	return r.execute(args, nil)
}

// execute matches static mounts and commands before falling back to a
// dynamic path segment ("{name}"), threading captured path param values
// down to whichever command ultimately handles the request.
func (r *Registry) execute(args []string, pp pathParams) Result {
	if r == nil {
		return Error("registry is nil")
	}
	if len(args) == 0 {
		return r.Help()
	}
	if isHelpRequest(args) {
		return r.Help()
	}

	if mounted, ok := r.mounts[args[0]]; ok {
		if len(args) == 1 {
			return mounted.Help()
		}
		if isHelpRequest(args[1:]) {
			return mounted.Help()
		}
		return mounted.execute(args[1:], pp)
	}

	if cmd, ok := r.commands[args[0]]; ok {
		if containsHelpFlag(args[1:]) {
			return r.commandHelp(args[0])
		}
		return cmd(args[1:], pp)
	}

	if r.dynamic != nil {
		next := pp.with(r.dynamic.param, args[0])
		if len(args) == 1 {
			return r.dynamic.registry.Help()
		}
		if isHelpRequest(args[1:]) {
			return r.dynamic.registry.Help()
		}
		return r.dynamic.registry.execute(args[1:], next)
	}

	help := r.Help()
	if help.Stdout == "" {
		return Error(fmt.Sprintf("unknown command '%s'", args[0]))
	}
	return Result{
		ExitCode: 1,
		Stderr:   fmt.Sprintf("unknown command '%s'\n\n%s", args[0], strings.TrimSpace(help.Stdout)),
	}
}

func Done() Result {
	return Result{ExitCode: 0}
}

func Text(value string) Result {
	return Result{ExitCode: 0, Stdout: value}
}

func Textf(format string, values ...any) Result {
	return Text(fmt.Sprintf(format, values...))
}

func Error(message string) Result {
	return Result{ExitCode: 1, Stderr: message}
}

func (c *RuntimeContext) Request() *http.Request {
	return nil
}

func (c *RuntimeContext) ResponseWriter() http.ResponseWriter {
	return nil
}

func (c *RuntimeContext) RedirectToURI(_ string) {}

func (c *RuntimeContext) RedirectToURIWithStatus(_ string, _ int) {}

func (c *RuntimeContext) Error(err error) activity.Result {
	if err == nil {
		return Done()
	}
	return Error(err.Error())
}

func (c *RuntimeContext) writeStdout(format string, values ...any) {
	if c == nil {
		return
	}
	fmt.Fprintf(&c.stdout, format, values...)
}

func (c *RuntimeContext) writeStderr(format string, values ...any) {
	if c == nil {
		return
	}
	fmt.Fprintf(&c.stderr, format, values...)
}

func (c *RuntimeContext) merge(result Result) Result {
	if c == nil {
		return result
	}
	result.Stdout = c.stdout.String() + result.Stdout
	result.Stderr = c.stderr.String() + result.Stderr
	return result
}

func newInvocation(args []string) *Invocation {
	return &Invocation{rawArgs: args, flags: parseFlags(args)}
}

func (i *Invocation) Fail(err error) {
	if i == nil || err == nil {
		return
	}
	i.failed = true
	i.errs = append(i.errs, err)
}

func (i *Invocation) Failed() bool {
	if i == nil {
		return false
	}
	return i.failed
}

func (i *Invocation) Error() error {
	if i == nil || len(i.errs) == 0 {
		return nil
	}
	if len(i.errs) == 1 {
		return i.errs[0]
	}
	return fmt.Errorf("%v", i.errs)
}

func (i *Invocation) Arg(index int) string {
	if i == nil || index < 0 {
		return ""
	}
	args := i.Args()
	if index >= len(args) {
		return ""
	}
	return args[index]
}

func (i *Invocation) Args() []string {
	if i == nil {
		return nil
	}
	var args []string
	for _, arg := range i.rawArgs {
		arg = strings.TrimSpace(arg)
		if arg == "" || strings.HasPrefix(arg, "-") {
			continue
		}
		args = append(args, arg)
	}
	return args
}

func (c *handlerContext[C]) Data() C {
	if c == nil {
		var zero C
		return zero
	}
	return c.data
}

func (c *handlerContext[C]) Stdout(format string, values ...any) {
	if c == nil || c.runtime == nil {
		return
	}
	c.runtime.writeStdout(format, values...)
}

func (c *handlerContext[C]) Stderr(format string, values ...any) {
	if c == nil || c.runtime == nil {
		return
	}
	c.runtime.writeStderr(format, values...)
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

// Resolve implements param.Resolver so activity.Param[T] works in CLI handlers.
func (c *handlerContext[C]) Resolve(names []string) (string, bool) {
	if c == nil || c.inv == nil {
		return "", false
	}
	return c.inv.lookupStringParam(ParamKey{names: names})
}

// Prompt implements param.Prompter for interactive CLI params.
func (c *handlerContext[C]) Prompt(text string) (string, error) {
	return promptString(os.Stdin, os.Stdout, text)
}

var _ param.Resolver = (*handlerContext[struct{}])(nil)
var _ param.Prompter = (*handlerContext[struct{}])(nil)

func (i *Invocation) IntParam(key ParamKey, validators ...IntValidator) int {
	raw, ok := i.lookupStringParam(key)
	if !ok {
		i.Fail(fmt.Errorf("missing param '%s'", key.PrimaryName()))
		return 0
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		i.Fail(fmt.Errorf("invalid int param '%s': %w", key.PrimaryName(), err))
		return 0
	}
	for _, validate := range validators {
		if validate == nil {
			continue
		}
		if err := validate(parsed); err != nil {
			i.Fail(fmt.Errorf("invalid param '%s': %w", key.PrimaryName(), err))
		}
	}
	return parsed
}

func (i *Invocation) StringParam(key ParamKey, opts ...StringParamOption) string {
	cfg := stringParamConfig{}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(&cfg)
	}

	value, ok := i.lookupStringParam(key)
	if !ok && cfg.prompt != "" {
		prompted, err := promptString(os.Stdin, os.Stdout, cfg.prompt)
		if err != nil {
			i.Fail(err)
			return ""
		}
		value = prompted
		ok = true
	}
	if !ok {
		if cfg.required {
			i.Fail(fmt.Errorf("missing param '%s'", key.PrimaryName()))
		}
		return ""
	}
	if cfg.required && strings.TrimSpace(value) == "" {
		i.Fail(fmt.Errorf("missing param '%s'", key.PrimaryName()))
		return ""
	}
	for _, validate := range cfg.validators {
		if validate == nil {
			continue
		}
		if err := validate(value); err != nil {
			i.Fail(fmt.Errorf("invalid param '%s': %w", key.PrimaryName(), err))
		}
	}
	return value
}

func (i *Invocation) lookupStringParam(key ParamKey) (string, bool) {
	if i == nil {
		return "", false
	}
	for _, name := range key.names {
		if value, ok := i.pathParams[name]; ok {
			return value, true
		}
	}
	for _, name := range key.names {
		if value, ok := i.flags[name]; ok {
			return value, true
		}
	}
	return "", false
}

func Prompt(text string) StringParamOption {
	return func(cfg *stringParamConfig) {
		if cfg == nil {
			return
		}
		cfg.prompt = text
	}
}

func Required() StringParamOption {
	return func(cfg *stringParamConfig) {
		if cfg == nil {
			return
		}
		cfg.required = true
	}
}

func ValidateString(validate StringValidator) StringParamOption {
	return func(cfg *stringParamConfig) {
		if cfg == nil || validate == nil {
			return
		}
		cfg.validators = append(cfg.validators, validate)
	}
}

func (k ParamKey) PrimaryName() string {
	if len(k.names) == 0 {
		return ""
	}
	return k.names[0]
}

func parseFlags(args []string) map[string]string {
	out := map[string]string{}
	for idx := 0; idx < len(args); idx++ {
		current := args[idx]
		if strings.HasPrefix(current, "--") {
			nameValue := strings.TrimPrefix(current, "--")
			if nameValue == "" {
				continue
			}
			if eq := strings.Index(nameValue, "="); eq >= 0 {
				out[nameValue[:eq]] = nameValue[eq+1:]
				continue
			}
			if idx+1 < len(args) && !strings.HasPrefix(args[idx+1], "-") {
				out[nameValue] = args[idx+1]
				idx++
				continue
			}
			out[nameValue] = "true"
			continue
		}
		if strings.HasPrefix(current, "-") {
			nameValue := strings.TrimPrefix(current, "-")
			if nameValue == "" {
				continue
			}
			if eq := strings.Index(nameValue, "="); eq >= 0 {
				out[nameValue[:eq]] = nameValue[eq+1:]
				continue
			}
			if idx+1 < len(args) && !strings.HasPrefix(args[idx+1], "-") {
				out[nameValue] = args[idx+1]
				idx++
				continue
			}
			out[nameValue] = "true"
		}
	}
	return out
}

func promptString(in io.Reader, out io.Writer, prompt string) (string, error) {
	if strings.TrimSpace(prompt) != "" {
		if _, err := io.WriteString(out, prompt); err != nil {
			return "", err
		}
	}
	reader := bufio.NewReader(in)
	value, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func isHelpRequest(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "help", "--help", "-h", "-help":
		return true
	}
	return false
}

func containsHelpFlag(args []string) bool {
	for _, arg := range args {
		switch arg {
		case "--help", "-h", "-help":
			return true
		}
	}
	return false
}

func (r *Registry) Help() Result {
	if r == nil {
		return Error("registry is nil")
	}
	var lines []string
	lines = append(lines, "Usage: <command> [subcommand] [flags]")
	if len(r.mounts) > 0 || r.dynamic != nil {
		lines = append(lines, "")
		lines = append(lines, "Subcommands:")
		for _, name := range sortedKeys(r.mounts) {
			lines = append(lines, "  "+name)
		}
		if r.dynamic != nil {
			lines = append(lines, "  <"+r.dynamic.param+">")
		}
	}
	if len(r.commands) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Commands:")
		for _, name := range sortedKeys(r.commands) {
			lines = append(lines, formatHelpEntry(name, r.help[name]))
		}
	}
	lines = append(lines, "")
	lines = append(lines, "Use --help on any command to show help.")
	return Text(strings.Join(lines, "\n"))
}

func (r *Registry) commandHelp(name string) Result {
	name = strings.TrimSpace(name)
	if name == "" {
		return r.Help()
	}
	summary := strings.TrimSpace(r.help[name])
	if summary == "" {
		summary = "Use --help to show the available commands."
	}
	return Text(strings.Join([]string{
		"Usage: " + name + " [flags]",
		"",
		summary,
	}, "\n"))
}

func formatHelpEntry(name, summary string) string {
	name = strings.TrimSpace(name)
	summary = strings.TrimSpace(summary)
	if summary == "" {
		return "  " + name
	}
	return "  " + name + " - " + summary
}

func sortedKeys[V any](m map[string]V) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

package chi

import (
	"context"
	"net/http"
	"strings"
	"sync"
)

type Router interface {
	http.Handler
	Get(pattern string, handler http.HandlerFunc)
	Post(pattern string, handler http.HandlerFunc)
	Mount(pattern string, handler http.Handler)
}

type contextKey struct{}

type params map[string]string

type route struct {
	method  string
	pattern string
	handler http.Handler
	mount   bool
}

type Mux struct {
	mu     sync.RWMutex
	routes []route
}

func NewRouter() *Mux {
	return &Mux{}
}

func (m *Mux) Get(pattern string, handler http.HandlerFunc) {
	m.add("GET", pattern, handler, false)
}

func (m *Mux) Post(pattern string, handler http.HandlerFunc) {
	m.add("POST", pattern, handler, false)
}

func (m *Mux) Mount(pattern string, handler http.Handler) {
	m.add("*", pattern, handler, true)
}

func (m *Mux) add(method, pattern string, handler http.Handler, mount bool) {
	if m == nil || handler == nil {
		return
	}
	pattern = normalizePath(pattern)
	m.mu.Lock()
	m.routes = append(m.routes, route{method: method, pattern: pattern, handler: handler, mount: mount})
	m.mu.Unlock()
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m == nil {
		http.NotFound(w, r)
		return
	}
	method := r.Method
	path := normalizePath(r.URL.Path)
	m.mu.RLock()
	routes := append([]route(nil), m.routes...)
	m.mu.RUnlock()
	for _, rt := range routes {
		if rt.method != "*" && rt.method != method {
			continue
		}
		if rt.mount {
			if path == rt.pattern || strings.HasPrefix(path, rt.pattern+"/") {
				req := r.Clone(r.Context())
				trimmed := strings.TrimPrefix(path, rt.pattern)
				if trimmed == "" {
					trimmed = "/"
				}
				req.URL.Path = trimmed
				rt.handler.ServeHTTP(w, req)
				return
			}
			continue
		}
		if matched, values := matchPattern(rt.pattern, path); matched {
			req := r.Clone(withParams(r.Context(), values))
			rt.handler.ServeHTTP(w, req)
			return
		}
	}
	http.NotFound(w, r)
}

func URLParam(r *http.Request, key string) string {
	if r == nil {
		return ""
	}
	value, _ := r.Context().Value(contextKey{}).(params)[key]
	return value
}

func withParams(ctx context.Context, values map[string]string) context.Context {
	return context.WithValue(ctx, contextKey{}, params(values))
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
		if path == "" {
			return "/"
		}
	}
	return path
}

func matchPattern(pattern, path string) (bool, map[string]string) {
	pattern = normalizePath(pattern)
	path = normalizePath(path)
	if pattern == path {
		return true, map[string]string{}
	}
	patternParts := splitPath(pattern)
	pathParts := splitPath(path)
	if len(patternParts) != len(pathParts) {
		return false, nil
	}
	values := map[string]string{}
	for idx := range patternParts {
		pp := patternParts[idx]
		sp := pathParts[idx]
		if strings.HasPrefix(pp, "{") && strings.HasSuffix(pp, "}") && len(pp) > 2 {
			values[pp[1:len(pp)-1]] = sp
			continue
		}
		if pp != sp {
			return false, nil
		}
	}
	return true, values
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

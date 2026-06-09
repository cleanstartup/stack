package web

import (
	"strings"

	"github.com/cleanstartup/stack/activity"
)

// Deprecated: use NewApp.
func NewHugo(parts ...Part) *WebApp {
	app := newWebApp(TargetHugo)
	app.Apply(parts...)
	return app
}

// Deprecated: use NewHugo or the hugo package facade.
func NewHugoWithDefaults(baseDir string, parts ...Part) *WebApp {
	app := newWebApp(TargetHugo)
	app.baseDir = strings.TrimSpace(baseDir)
	app.moduleDir = moduleRoot(baseDir)
	app.Apply(parts...)
	return app
}

// Deprecated: use activity.Ref or activity.RootRef.
func Ref(id string) URIRef { return activity.Ref(id) }

// Deprecated: use activity.RootRef.
func RootRef() URIRef { return activity.RootRef() }

// Deprecated: use URI(ref) or activity.Ref directly.
func URIByRef(ref URIRef) string { return URI(ref) }

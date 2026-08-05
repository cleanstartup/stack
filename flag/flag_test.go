package flag_test

import (
	"net/http"
	"testing"

	"github.com/cleanstartup/stack/flag"
	"github.com/cleanstartup/stack/way2go/activity"
)

// fakeContext is a minimal activity.Context stub for testing flag resolution
// without pulling in a transport (web/cli).
type fakeContext struct{}

func (fakeContext) Request() *http.Request                             { return nil }
func (fakeContext) ResponseWriter() http.ResponseWriter                { return nil }
func (fakeContext) RedirectToURI(uri string)                           {}
func (fakeContext) RedirectToURIWithStatus(uri string, statusCode int) {}
func (fakeContext) Error(err error) activity.Result                    { return err }

type fakeResolver struct {
	value map[string]bool
}

func (r fakeResolver) Resolve(_ activity.Context, name string) (bool, bool) {
	v, ok := r.value[name]
	return v, ok
}

func TestEnabledFallsBackToDefaultWithoutResolver(t *testing.T) {
	flag.Use(nil)
	f := flag.Bool("payments-enabled", flag.WithDefault(true))
	if !flag.Enabled(fakeContext{}, f) {
		t.Fatalf("expected default true when no resolver is registered")
	}
}

func TestEnabledFallsBackToDefaultWhenResolverHasNoValue(t *testing.T) {
	flag.Use(fakeResolver{value: map[string]bool{}})
	defer flag.Use(nil)

	f := flag.Bool("payments-enabled", flag.WithDefault(true))
	if !flag.Enabled(fakeContext{}, f) {
		t.Fatalf("expected default true when resolver has no value for the flag")
	}
}

func TestEnabledUsesResolverValueWhenFound(t *testing.T) {
	flag.Use(fakeResolver{value: map[string]bool{"payments-enabled": true}})
	defer flag.Use(nil)

	f := flag.Bool("payments-enabled", flag.WithDefault(false))
	if !flag.Enabled(fakeContext{}, f) {
		t.Fatalf("expected resolver value true to win over default false")
	}
}

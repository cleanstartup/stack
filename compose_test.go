package stack

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/cleanstartup/stack/plugin"
	"github.com/cleanstartup/stack/webasset"
)

// fakeProducer implements only plugin.Builder, per the design's "plugins
// opt in per stage" model — proves the Build-only case works without
// needing Dev/Test/Clean.
type fakeProducer struct {
	assets []plugin.Asset
	err    error
}

func (f fakeProducer) Build(context.Context, plugin.StageContext) ([]plugin.Asset, error) {
	return f.assets, f.err
}

// spyPart is a minimal webasset.Part that records whether it was applied —
// used to observe whether a Module ingredient's Apply(*WebApp) actually ran.
type spyPart struct{ applied *bool }

func (s spyPart) Apply(*webasset.WebApp) { *s.applied = true }

func TestFlattenBareProducerContribution(t *testing.T) {
	producer := fakeProducer{assets: []plugin.Asset{{Path: "a.css", ContentType: "text/css"}}}
	contributions, err := flattenIngredients(context.Background(), plugin.StageContext{}, nil, []Ingredient{producer})
	if err != nil {
		t.Fatalf("flattenIngredients: %v", err)
	}
	if len(contributions) != 1 || contributions[0].Mount != "" {
		t.Fatalf("got %+v, want one contribution with empty Mount (target default policy)", contributions)
	}
	if len(contributions[0].Assets) != 1 || contributions[0].Assets[0].Path != "a.css" {
		t.Fatalf("got assets %+v", contributions[0].Assets)
	}
}

func TestFlattenMountedProducerContribution(t *testing.T) {
	producer := fakeProducer{assets: []plugin.Asset{{Path: "docs/index.html", ContentType: "text/html"}}}
	contributions, err := flattenIngredients(context.Background(), plugin.StageContext{}, nil, []Ingredient{
		Mount(producer, "/docs"),
	})
	if err != nil {
		t.Fatalf("flattenIngredients: %v", err)
	}
	if len(contributions) != 1 || contributions[0].Mount != "/docs" {
		t.Fatalf("got %+v, want one contribution mounted at /docs", contributions)
	}
}

func TestFlattenBareModuleAppliesWhenAppPresent(t *testing.T) {
	var applied bool
	module := Bundle("ns", spyPart{applied: &applied})
	app := webasset.NewApp()

	if _, err := flattenIngredients(context.Background(), plugin.StageContext{}, app, []Ingredient{module}); err != nil {
		t.Fatalf("flattenIngredients: %v", err)
	}
	if !applied {
		t.Fatal("bare Module ingredient was not applied to the WebApp target's app")
	}
}

func TestFlattenBareModuleNoopWhenAppNil(t *testing.T) {
	var applied bool
	module := Bundle("ns", spyPart{applied: &applied})

	if _, err := flattenIngredients(context.Background(), plugin.StageContext{}, nil, []Ingredient{module}); err != nil {
		t.Fatalf("flattenIngredients: %v", err)
	}
	if applied {
		t.Fatal("bare Module ingredient was applied even though the target has no app (Website/CLI)")
	}
}

func TestFlattenMountedModuleNoopWhenAppNil(t *testing.T) {
	module := Bundle("ns")
	if _, err := flattenIngredients(context.Background(), plugin.StageContext{}, nil, []Ingredient{
		Mount(module, "/docs"),
	}); err != nil {
		t.Fatalf("flattenIngredients: %v, want accepted no-op for a nil-app (Website/CLI) target", err)
	}
}

func TestFlattenMountedHandlerErrorsWhenAppNil(t *testing.T) {
	handler := http.NotFoundHandler()
	_, err := flattenIngredients(context.Background(), plugin.StageContext{}, nil, []Ingredient{
		Mount(handler, "/auth"),
	})
	if err == nil {
		t.Fatal("want an error mounting a raw http.Handler on a target with no live registrar (Website/CLI)")
	}
}

func TestFlattenDedupSamePathErrors(t *testing.T) {
	first := fakeProducer{assets: []plugin.Asset{{Path: "shared.css", ContentType: "text/css"}}}
	second := fakeProducer{assets: []plugin.Asset{{Path: "shared.css", ContentType: "text/css"}}}

	_, err := flattenIngredients(context.Background(), plugin.StageContext{}, nil, []Ingredient{
		Mount(first, "/a"),
		Mount(second, "/b"),
	})
	if err == nil {
		t.Fatal("want an error for two producer edges contributing the same Asset.Path")
	}
}

func TestFlattenOrderPreserved(t *testing.T) {
	p1 := fakeProducer{assets: []plugin.Asset{{Path: "one.css", ContentType: "text/css"}}}
	p2 := fakeProducer{assets: []plugin.Asset{{Path: "two.css", ContentType: "text/css"}}}
	p3 := fakeProducer{assets: []plugin.Asset{{Path: "three.css", ContentType: "text/css"}}}

	contributions, err := flattenIngredients(context.Background(), plugin.StageContext{}, nil, []Ingredient{p1, p2, p3})
	if err != nil {
		t.Fatalf("flattenIngredients: %v", err)
	}
	if len(contributions) != 3 {
		t.Fatalf("got %d contributions, want 3", len(contributions))
	}
	wantOrder := []string{"one.css", "two.css", "three.css"}
	for i, want := range wantOrder {
		if got := contributions[i].Assets[0].Path; got != want {
			t.Fatalf("contribution[%d].Assets[0].Path = %q, want %q (wiring order)", i, got, want)
		}
	}
}

func TestFlattenUnsupportedIngredientErrors(t *testing.T) {
	_, err := flattenIngredients(context.Background(), plugin.StageContext{}, nil, []Ingredient{42})
	if err == nil {
		t.Fatal("want an error for an ingredient of an unsupported type")
	}
}

func TestFlattenProducerBuildErrorPropagates(t *testing.T) {
	wantErr := errors.New("boom")
	producer := fakeProducer{err: wantErr}
	_, err := flattenIngredients(context.Background(), plugin.StageContext{}, nil, []Ingredient{producer})
	if !errors.Is(err, wantErr) {
		t.Fatalf("got err %v, want it to wrap %v", err, wantErr)
	}
}

package stack

import (
	"context"

	"github.com/cleanstartup/stack/plugin"
)

// CLIAppTarget is the CLI target kind (D-L): it never links asset tooling
// or a browser bundle. Consuming a CLIApp target never touches an asset's
// bytes — that follows from what's composed, not from a flag.
type CLIAppTarget struct {
	module      Module
	ingredients []Ingredient
}

var _ plugin.TargetKind = (*CLIAppTarget)(nil)

// CLIApp composes a CLI target from a Module plus Ingredients.
func CLIApp(module Module, ingredients ...Ingredient) *CLIAppTarget {
	return &CLIAppTarget{module: module, ingredients: ingredients}
}

func (t *CLIAppTarget) Build(ctx context.Context, stageCtx plugin.StageContext) error {
	contributions, err := flattenIngredients(ctx, stageCtx, nil, t.ingredients)
	if err != nil {
		return err
	}
	return t.Consume(ctx, contributions)
}

// Consume implements plugin.TargetKind as a literal no-op: a CLI target
// composes no asset pipeline, so it never reads an Asset's Path.
func (t *CLIAppTarget) Consume(ctx context.Context, contributions []plugin.Contribution) error {
	return nil
}

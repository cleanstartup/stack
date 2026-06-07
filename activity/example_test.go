package activity_test

import (
	"fmt"

	"github.com/cleanstartup/stack/activity"
)

type accountShowParams struct {
	AccountID string
}

func ExampleDefinition_takeThenURIAndRun() {
	accountShowDef := activity.As[accountShowParams, activity.NoInput](
		"account.show",
	)

	show := func(params accountShowParams) activity.Instance[accountShowParams, activity.NoInput] {
		return accountShowDef.Take(params).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
			return "Account " + params.AccountID
		})
	}

	fmt.Println(show(accountShowParams{AccountID: "42"}).URI())
	result := show(accountShowParams{AccountID: "42"}).Run(testCtx{}, activity.NoInput{})
	fmt.Println(result)

	// Output:
	// /account/show?accountID=42
	// Account 42
}

func ExampleInstance_handle() {
	accountShowDef := activity.As[accountShowParams, activity.NoInput](
		"account.show",
	)

	show := func(params accountShowParams) activity.Instance[accountShowParams, activity.NoInput] {
		return accountShowDef.Take(params).Then(func(ctx activity.Context, input activity.NoInput) activity.Result {
			return "Account " + params.AccountID
		})
	}

	sessionDecorator := func(next activity.Handler[activity.NoInput]) activity.Handler[activity.NoInput] {
		return func(ctx activity.Context, input activity.NoInput) activity.Result {
			result := next(ctx, input)
			return "session:" + result.(string)
		}
	}

	result := show(accountShowParams{AccountID: "42"}).Handle(testCtx{}, activity.NoInput{}, sessionDecorator)
	fmt.Println(result)

	// Output:
	// session:Account 42
}

package activity

// ContextHandler is the transport-agnostic handler signature — receives an
// activity.Context and returns a result. Used by cross-target middlewares.
type ContextHandler func(Context) Result

// ContextMiddleware wraps a ContextHandler with cross-cutting logic.
// Implementations receive an activity.Context, so they work identically
// across web and CLI targets.
//
// Example:
//
//	func Track() activity.ContextMiddleware {
//	    return func(next activity.ContextHandler) activity.ContextHandler {
//	        return func(ctx activity.Context) activity.Result {
//	            result := next(ctx)
//	            // record event here
//	            return result
//	        }
//	    }
//	}
type ContextMiddleware func(next ContextHandler) ContextHandler

// ApplyMiddlewares wraps handler with middlewares in reverse order so the
// first middleware in the slice is the outermost wrapper.
func ApplyMiddlewares(handler ContextHandler, middlewares []ContextMiddleware) ContextHandler {
	for idx := len(middlewares) - 1; idx >= 0; idx-- {
		handler = middlewares[idx](handler)
	}
	return handler
}

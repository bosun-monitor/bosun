// +build go1.7

package miniprofiler

import (
	"context"
	"net/http"
)

type ctxKey int

// contextKey can be used to retreive the profiler instance from the request's context
const contextKey ctxKey = 0

// ContextHandler is an alternate handler that passes the profiler on the http
// request's context, rather than as function arguments.
// This approach is more compatible with standard net/http Handlers.
type ContextHandler struct {
	f http.HandlerFunc
}

// NewContextHandler creates a ContextHandler to wrap the given http.HandlerFunc.
// A profiler will be added to the request Context, and can be retreived with
// miniprofiler.GetTimer(r)
func NewContextHandler(f http.HandlerFunc) http.Handler {
	return ContextHandler{
		f: f,
	}
}

func (h ContextHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := NewProfile(w, r, FuncName(h.f))
	ctx := context.WithValue(r.Context(), contextKey, p)
	h.f(w, r.WithContext(ctx))
	p.Finalize()
}

// GetTimer will retreive the timer from the given http request's context.
// If the request has not been wrapped by a ContextHandler, nil will be returned.
func GetTimer(r *http.Request) Timer {
	val := r.Context().Value(contextKey)
	if val == nil {
		return nil
	}
	return val.(*Profile)
}

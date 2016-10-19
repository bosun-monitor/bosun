package web

import (
	"net/http"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/gorilla/mux"

	"bosun.org/collect"
	"bosun.org/opentsdb"
)

// custom middlewares for bosun. Must match  alice.Constructor signature (func(http.Handler) http.Handler)

var miniprofilerMiddleware = func(next http.Handler) http.Handler {
	return miniprofiler.NewContextHandler(next.ServeHTTP)
}

var endpointStatsMiddleware = func(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//metric for http vs https
		proto := "http"
		if r.TLS != nil {
			proto = "https"
		}
		collect.Add("bosun.http_protocol", opentsdb.TagSet{"proto": proto}, 1)

		//if we use gorilla named routes, we can add stats and timings per route
		routeName := ""
		if route := mux.CurrentRoute(r); route != nil {
			routeName = route.GetName()
		}
		if routeName == "" {
			routeName = "unknown"
		}
		t := collect.StartTimer("bosun.http_routes", opentsdb.TagSet{"route": routeName})
		next.ServeHTTP(w, r)
		t()
	})
}

package web

import (
	"encoding/json"
	"net/http"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/tsaf/expr"
)

func Expr(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	e, err := expr.New(r.FormValue("q"))
	if err != nil {
		serveError(w, err)
		return
	}
	res, err := e.Execute(tsdbHost, t)
	if err != nil {
		serveError(w, err)
		return
	}
	b, err := json.Marshal(res)
	if err != nil {
		serveError(w, err)
		return
	}
	w.Write(b)
}

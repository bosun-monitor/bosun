package web

import (
	"html/template"
	"log"
	"net/http"

	"github.com/StackExchange/tsaf/search"
	"github.com/gorilla/mux"
)

var TSDBHttp string
var templates *template.Template
var router = mux.NewRouter()

func Listen(addr, dir, tsdbhttp string) error {
	TSDBHttp = tsdbhttp
	var err error
	templates, err = template.New("").ParseFiles(
		dir + "/templates/chart.html",
	)
	if err != nil {
		log.Fatal(err)
	}
	router.HandleFunc("/", Index)
	router.HandleFunc("/api/chart", Chart)
	router.HandleFunc("/api/metric", UniqueMetrics)
	router.HandleFunc("/api/metric/{tagk}/{tagv}", MetricsByTagPair)
	router.HandleFunc("/api/tagk/{metric}", TagKeysByMetric)
	router.HandleFunc("/api/tagv/{tagk}", TagValuesByTagKey)
	router.HandleFunc("/api/tagv/{tagk}/{metric}", TagValuesByMetricTagKey)
	router.HandleFunc("/api/expr", Expr)
	http.Handle("/", router)
	http.Handle("/static/", http.FileServer(http.Dir(dir)))
	log.Println("web listening on", addr)
	return http.ListenAndServe(addr, nil)
}

func Index(w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "chart.html", struct {
		Metric, Tagv search.QMap
		Tagk         search.SMap
	}{
		search.Metric,
		search.Tagv,
		search.Tagk,
	})
}

func serveError(w http.ResponseWriter, err error) {
	serveError(w, err)
}

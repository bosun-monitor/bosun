package web

import (
	"html/template"
	"log"
	"net/http"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/tsaf/search"
	"github.com/gorilla/mux"
)

var TSDBHttp string
var templates *template.Template
var router = mux.NewRouter()

func init() {
	miniprofiler.Position = "bottomleft"
}

func Listen(addr, dir, tsdbhttp string) error {
	TSDBHttp = tsdbhttp
	var err error
	templates, err = template.New("").ParseFiles(
		dir + "/templates/chart.html",
	)
	if err != nil {
		log.Fatal(err)
	}
	router.Handle("/", miniprofiler.NewHandler(Index))
	router.Handle("/api/chart", miniprofiler.NewHandler(Chart))
	router.Handle("/api/metric", miniprofiler.NewHandler(UniqueMetrics))
	router.Handle("/api/metric/{tagk}/{tagv}", miniprofiler.NewHandler(MetricsByTagPair))
	router.Handle("/api/tagk/{metric}", miniprofiler.NewHandler(TagKeysByMetric))
	router.Handle("/api/tagv/{tagk}", miniprofiler.NewHandler(TagValuesByTagKey))
	router.Handle("/api/tagv/{tagk}/{metric}", miniprofiler.NewHandler(TagValuesByMetricTagKey))
	router.Handle("/api/expr", miniprofiler.NewHandler(Expr))
	http.Handle("/", router)
	http.Handle("/static/", http.FileServer(http.Dir(dir)))
	log.Println("TSAF web listening on:", addr)
	log.Println("TSAF web directory:", dir)
	return http.ListenAndServe(addr, nil)
}

func Index(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	templates.ExecuteTemplate(w, "chart.html", struct {
		Metric, Tagv search.QMap
		Tagk         search.SMap
		Includes     template.HTML
	}{
		search.Metric,
		search.Tagv,
		search.Tagk,
		t.Includes(),
	})
}

func serveError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

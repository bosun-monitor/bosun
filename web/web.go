package web

import (
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/tsaf/sched"
	"github.com/gorilla/mux"
)

var (
	tsdbHost  string
	templates *template.Template
	router    = mux.NewRouter()
	schedule  = sched.DefaultSched
)

func init() {
	miniprofiler.Position = "bottomleft"
}

func Listen(addr, dir, host string) error {
	tsdbHost = host
	var err error
	templates, err = template.New("").Funcs(funcs).ParseFiles(
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
	err := templates.ExecuteTemplate(w, "chart.html", struct {
		Includes template.HTML
		Schedule *sched.Schedule
	}{
		t.Includes(),
		schedule,
	})
	if err != nil {
		fmt.Println(err)
	}
}

func serveError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

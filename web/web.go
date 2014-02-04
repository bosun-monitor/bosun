package web

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/gorilla/mux"

	"github.com/StackExchange/tsaf/sched"
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
		dir + "/templates/index.html",
	)
	if err != nil {
		log.Fatal(err)
	}
	router.Handle("/api/alerts", miniprofiler.NewHandler(Alerts))
	router.Handle("/api/chart", miniprofiler.NewHandler(Chart))
	router.Handle("/api/metric", miniprofiler.NewHandler(UniqueMetrics))
	router.Handle("/api/metric/{tagk}/{tagv}", miniprofiler.NewHandler(MetricsByTagPair))
	router.Handle("/api/tagk/{metric}", miniprofiler.NewHandler(TagKeysByMetric))
	router.Handle("/api/tagv/{tagk}", miniprofiler.NewHandler(TagValuesByTagKey))
	router.Handle("/api/tagv/{tagk}/{metric}", miniprofiler.NewHandler(TagValuesByMetricTagKey))
	router.Handle("/api/expr", miniprofiler.NewHandler(Expr))
	http.Handle("/", miniprofiler.NewHandler(Index))
	http.Handle("/api/", router)
	http.Handle("/static/", http.FileServer(http.Dir(dir)))
	http.Handle("/partials/", http.FileServer(http.Dir(dir)))
	log.Println("TSAF web listening on:", addr)
	log.Println("TSAF web directory:", dir)
	return http.ListenAndServe(addr, nil)
}

func Index(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	err := templates.ExecuteTemplate(w, "index.html", struct {
		Includes template.HTML
	}{
		t.Includes(),
	})
	if err != nil {
		serveError(w, err)
	}
}

func serveError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func Alerts(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	b, err := json.Marshal(schedule)
	if err != nil {
		serveError(w, err)
		return
	}
	w.Write(b)
}

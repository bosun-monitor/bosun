package web

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/gorilla/mux"

	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/tsaf/sched"
)

var (
	tsdbHost  opentsdb.Host
	templates *template.Template
	router    = mux.NewRouter()
	schedule  = sched.DefaultSched
)

func init() {
	miniprofiler.Position = "bottomleft"
}

func Listen(addr, dir, host string) error {
	tsdbHost = opentsdb.Host(host)
	var err error
	templates, err = template.New("").ParseFiles(
		dir + "/templates/index.html",
	)
	if err != nil {
		log.Fatal(err)
	}
	router.Handle("/api/acknowledge/{alert}/{group}", JSON(Acknowledge))
	router.Handle("/api/alerts", JSON(Alerts))
	router.Handle("/api/expr", JSON(Expr))
	router.Handle("/api/metric", JSON(UniqueMetrics))
	router.Handle("/api/metric/{tagk}/{tagv}", JSON(MetricsByTagPair))
	router.Handle("/api/query", JSON(Query))
	router.Handle("/api/tagk/{metric}", JSON(TagKeysByMetric))
	router.Handle("/api/tagv/{tagk}", JSON(TagValuesByTagKey))
	router.Handle("/api/tagv/{tagk}/{metric}", JSON(TagValuesByMetricTagKey))
	http.Handle("/", miniprofiler.NewHandler(Index))
	http.Handle("/api/", router)
	http.Handle("/partials/", http.FileServer(http.Dir(dir)))
	http.Handle("/static/", http.FileServer(http.Dir(dir)))
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

func JSON(h func(miniprofiler.Timer, http.ResponseWriter, *http.Request) (interface{}, error)) http.Handler {
	return miniprofiler.NewHandler(func(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
		d, err := h(t, w, r)
		if err != nil {
			serveError(w, err)
			return
		}
		b, err := json.Marshal(d)
		if err != nil {
			serveError(w, err)
			return
		}
		w.Write(b)
	})
}

func Alerts(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule, nil
}

func Acknowledge(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	vars := mux.Vars(r)
	ak := sched.AlertKey{
		Name:  vars["alert"],
		Group: vars["group"],
	}
	log.Println("ACK", ak)
	schedule.Acknowledge(ak)
	return nil, nil
}

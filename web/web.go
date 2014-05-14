package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/StackExchange/tsaf/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/tsaf/_third_party/github.com/gorilla/mux"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/sched"
)

var (
	templates *template.Template
	router    = mux.NewRouter()
	schedule  = sched.DefaultSched
)

func init() {
	miniprofiler.Position = "bottomleft"
}

func Listen(addr, dir, host string) error {
	var err error
	templates, err = template.New("").ParseFiles(
		dir + "/templates/index.html",
	)
	if err != nil {
		log.Fatal(err)
	}
	router.Handle("/api/action", JSON(Action))
	router.Handle("/api/alerts", JSON(Alerts))
	router.Handle("/api/config", miniprofiler.NewHandler(Config))
	router.Handle("/api/template", JSON(Template))
	router.Handle("/api/config_test", miniprofiler.NewHandler(ConfigTest))
	router.Handle("/api/egraph", JSON(ExprGraph))
	router.Handle("/api/expr", JSON(Expr))
	router.Handle("/api/graph", JSON(Graph))
	router.Handle("/api/metric", JSON(UniqueMetrics))
	router.Handle("/api/metric/{tagk}/{tagv}", JSON(MetricsByTagPair))
	router.Handle("/api/rule", JSON(Rule))
	router.Handle("/api/silence/clear", JSON(SilenceClear))
	router.Handle("/api/silence/get", JSON(SilenceGet))
	router.Handle("/api/silence/set", JSON(SilenceSet))
	router.Handle("/api/tagk/{metric}", JSON(TagKeysByMetric))
	router.Handle("/api/tagv/{tagk}", JSON(TagValuesByTagKey))
	router.Handle("/api/tagv/{tagk}/{metric}", JSON(TagValuesByMetricTagKey))
	http.Handle("/", miniprofiler.NewHandler(Index))
	http.Handle("/api/", router)
	fs := http.FileServer(http.Dir(dir))
	http.Handle("/partials/", fs)
	http.Handle("/static/", fs)
	static := http.FileServer(http.Dir(filepath.Join(dir, "static")))
	http.Handle("/favicon.ico", static)
	log.Println("TSAF web listening on:", addr)
	log.Println("TSAF web directory:", dir)
	return http.ListenAndServe(addr, nil)
}

func Index(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/graph" {
		r.ParseForm()
		if _, present := r.Form["png"]; present {
			if _, err := Graph(t, w, r); err != nil {
				serveError(w, err)
			}
			return
		}
	}
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
		if d == nil {
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

func Action(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var data map[string]string
	j := json.NewDecoder(r.Body)
	if err := j.Decode(&data); err != nil {
		return nil, err
	}
	var at sched.ActionType
	switch data["type"] {
	case "ack":
		at = sched.ActionAcknowledge
	case "close":
		at = sched.ActionClose
	case "forget":
		at = sched.ActionForget
	}
	err := schedule.Action(data["user"], data["message"], at, sched.AlertKey(data["key"]))
	return nil, err
}

func SilenceGet(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.Silence, nil
}

var silenceLayouts = []string{
	"2006-01-02 15:04:05 MST",
	"2006-01-02 15:04:05 -0700",
	"2006-01-02 15:04 MST",
	"2006-01-02 15:04 -0700",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
}

func SilenceSet(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var start, end time.Time
	var err error
	var data map[string]string
	j := json.NewDecoder(r.Body)
	if err := j.Decode(&data); err != nil {
		return nil, err
	}
	if s := data["start"]; s != "" {
		for _, layout := range silenceLayouts {
			start, err = time.Parse(layout, s)
			if err == nil {
				break
			}
		}
		if start.IsZero() {
			return nil, fmt.Errorf("unrecognized start time format: %s", s)
		}
	}
	if s := data["end"]; s != "" {
		for _, layout := range silenceLayouts {
			end, err = time.Parse(layout, s)
			if err == nil {
				break
			}
		}
		if end.IsZero() {
			return nil, fmt.Errorf("unrecognized end time format: %s", s)
		}
	}
	if start.IsZero() {
		start = time.Now().UTC()
	}
	if end.IsZero() {
		d, err := time.ParseDuration(data["duration"])
		if err != nil {
			return nil, err
		}
		end = start.Add(d)
	}
	return schedule.AddSilence(start, end, data["alert"], data["tags"], len(data["confirm"]) > 0, data["edit"])
}

func SilenceClear(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var data map[string]string
	j := json.NewDecoder(r.Body)
	if err := j.Decode(&data); err != nil {
		return nil, err
	}
	return nil, schedule.ClearSilence(data["id"])
}

func ConfigTest(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	_, err := conf.New("test", r.FormValue("config_text"))
	if err != nil {
		fmt.Fprint(w, err.Error())
	}
}

func Config(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, schedule.Conf.RawText)
}

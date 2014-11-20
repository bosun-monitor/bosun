package web

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bosun-monitor/bosun/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"github.com/bosun-monitor/bosun/_third_party/github.com/bosun-monitor/collect"
	"github.com/bosun-monitor/bosun/_third_party/github.com/bosun-monitor/metadata"
	"github.com/bosun-monitor/bosun/_third_party/github.com/bosun-monitor/opentsdb"
	"github.com/bosun-monitor/bosun/_third_party/github.com/gorilla/mux"
	"github.com/bosun-monitor/bosun/conf"
	"github.com/bosun-monitor/bosun/expr"
	"github.com/bosun-monitor/bosun/sched"
)

//go:generate esc -o web/static.go -pkg web -prefix web/static web/static/

var (
	templates *template.Template
	router    = mux.NewRouter()
	schedule  = sched.DefaultSched
)

const tsdbFormat = "2006/01/02-15:04"

func init() {
	miniprofiler.Position = "bottomleft"
	miniprofiler.StartHidden = true
}

func Listen(listenAddr string, devMode bool, tsdbHost *url.URL) error {
	var err error
	webFS := FS(devMode)
	if devMode {
		log.Println("using local web assets")
	}
	index, err := webFS.Open("/templates/index.html")
	if err != nil {
		log.Fatal(err)
	}
	b, err := ioutil.ReadAll(index)
	if err != nil {
		log.Fatal(err)
	}
	templates, err = template.New("").Parse(string(b))
	if err != nil {
		log.Fatal(err)
	}
	router.HandleFunc("/api/", APIRedirect)
	router.Handle("/api/action", JSON(Action))
	router.Handle("/api/alerts", JSON(Alerts))
	router.Handle("/api/config", miniprofiler.NewHandler(Config))
	router.Handle("/api/config_test", miniprofiler.NewHandler(ConfigTest))
	router.Handle("/api/egraph/{bs}.svg", JSON(ExprGraph))
	router.Handle("/api/expr", JSON(Expr))
	router.Handle("/api/graph", JSON(Graph))
	router.Handle("/api/health", JSON(HealthCheck))
	router.Handle("/api/host", JSON(Host))
	router.Handle("/api/metadata/get", JSON(GetMetadata))
	router.Handle("/api/metadata/metrics", JSON(MetadataMetrics))
	router.Handle("/api/metadata/put", JSON(PutMetadata))
	router.Handle("/api/metric", JSON(UniqueMetrics))
	router.Handle("/api/metric/{tagk}/{tagv}", JSON(MetricsByTagPair))
	router.Handle("/api/rule", JSON(Rule))
	router.Handle("/api/silence/clear", JSON(SilenceClear))
	router.Handle("/api/silence/get", JSON(SilenceGet))
	router.Handle("/api/silence/set", JSON(SilenceSet))
	router.Handle("/api/status", JSON(Status))
	router.Handle("/api/tagk/{metric}", JSON(TagKeysByMetric))
	router.Handle("/api/tagv/{tagk}", JSON(TagValuesByTagKey))
	router.Handle("/api/tagv/{tagk}/{metric}", JSON(TagValuesByMetricTagKey))
	router.Handle("/api/templates", JSON(Templates))
	router.Handle("/api/put", Relay(tsdbHost))
	router.Handle("/api/run", JSON(Run))
	http.Handle("/", miniprofiler.NewHandler(Index))
	http.Handle("/api/", router)
	fs := http.FileServer(webFS)
	http.Handle("/partials/", fs)
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.Handle("/favicon.ico", fs)
	log.Println("bosun web listening on:", listenAddr)
	log.Println("tsdb host:", tsdbHost)
	return http.ListenAndServe(listenAddr, nil)
}

type relayProxy struct {
	*httputil.ReverseProxy
}

type passthru struct {
	io.ReadCloser
	buf bytes.Buffer
}

func (p *passthru) Read(b []byte) (int, error) {
	n, err := p.ReadCloser.Read(b)
	p.buf.Write(b[:n])
	return n, err
}

type relayWriter struct {
	http.ResponseWriter
	code int
}

func (rw *relayWriter) WriteHeader(code int) {
	rw.code = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rp *relayProxy) ServeHTTP(responseWriter http.ResponseWriter, r *http.Request) {
	clean := func(s string) string {
		return opentsdb.MustReplace(s, "_")
	}

	reader := &passthru{ReadCloser: r.Body}
	r.Body = reader
	w := &relayWriter{ResponseWriter: responseWriter}
	rp.ReverseProxy.ServeHTTP(w, r)

	body := reader.buf.Bytes()
	if r, err := gzip.NewReader(bytes.NewReader(body)); err == nil {
		body, _ = ioutil.ReadAll(r)
		r.Close()
	}
	var dp opentsdb.DataPoint
	var mdp opentsdb.MultiDataPoint
	if err := json.Unmarshal(body, &mdp); err == nil {
	} else if err = json.Unmarshal(body, &dp); err == nil {
		mdp = opentsdb.MultiDataPoint{&dp}
	}
	if len(mdp) > 0 {
		ra := strings.Split(r.RemoteAddr, ":")[0]
		tags := opentsdb.TagSet{"remote": clean(ra)}
		collect.Add("search.puts_relayed", tags, 1)
		collect.Add("search.datapoints_relayed", tags, int64(len(mdp)))
		schedule.Search.Index(mdp)
	}
	tags := opentsdb.TagSet{"path": clean(r.URL.Path), "remote": clean(strings.Split(r.RemoteAddr, ":")[0])}
	collect.Add("relay.bytes", tags, int64(reader.buf.Len()))
	tags["status"] = strconv.Itoa(w.code)
	collect.Add("relay.response", tags, 1)
}

func Relay(dest *url.URL) http.Handler {
	return &relayProxy{ReverseProxy: httputil.NewSingleHostReverseProxy(dest)}
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
	err := templates.Execute(w, struct {
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
		var tw io.Writer = w
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			tw = gz
		}
		if cb := r.FormValue("callback"); cb != "" {
			w.Header().Add("Content-Type", "application/javascript")
			tw.Write([]byte(cb + "("))
			if err := json.NewEncoder(tw).Encode(d); err != nil {
				log.Println(err)
			}
			tw.Write([]byte(")"))
			return
		}
		w.Header().Add("Content-Type", "application/json")
		if err := json.NewEncoder(tw).Encode(d); err != nil {
			log.Println(err)
		}
	})
}

type Health struct {
	// RuleCheck is true if last check happened within the check frequency window.
	RuleCheck bool
}

func HealthCheck(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var h Health
	h.RuleCheck = schedule.LastCheck.After(time.Now().Add(-schedule.Conf.CheckFrequency))
	return h, nil
}

func PutMetadata(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	d := json.NewDecoder(r.Body)
	var ms []metadata.Metasend
	if err := d.Decode(&ms); err != nil {
		return nil, err
	}
	for _, m := range ms {
		schedule.PutMetadata(metadata.Metakey{
			Metric: m.Metric,
			Tags:   m.Tags.Tags(),
			Name:   m.Name,
		}, m.Value)
	}
	w.WriteHeader(204)
	return nil, nil
}

func GetMetadata(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	tags := make(opentsdb.TagSet)
	r.ParseForm()
	vals := r.Form["tagv"]
	for i, k := range r.Form["tagk"] {
		if len(vals) <= i {
			return nil, fmt.Errorf("unpaired tagk/tagv")
		}
		tags[k] = vals[i]
	}
	return schedule.GetMetadata(r.FormValue("metric"), tags), nil
}

func MetadataMetrics(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.MetadataMetrics(), nil
}

func Alerts(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.MarshalGroups(r.FormValue("filter"))
}

func Status(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	r.ParseForm()
	m := make(map[string]*sched.State)
	for _, k := range r.Form["ak"] {
		ak, err := expr.ParseAlertKey(k)
		if err != nil {
			return nil, err
		}
		st := schedule.Status(ak)
		if st == nil {
			return nil, fmt.Errorf("unknown alert key: %v", k)
		}
		m[k] = st
	}
	return m, nil
}

func Action(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var data struct {
		Type    string
		User    string
		Message string
		Keys    []string
	}
	j := json.NewDecoder(r.Body)
	if err := j.Decode(&data); err != nil {
		return nil, err
	}
	var at sched.ActionType
	switch data.Type {
	case "ack":
		at = sched.ActionAcknowledge
	case "close":
		at = sched.ActionClose
	case "forget":
		at = sched.ActionForget
	}
	errs := make(MultiError)
	r.ParseForm()
	for _, key := range data.Keys {
		ak, err := expr.ParseAlertKey(key)
		if err != nil {
			return nil, err
		}
		err = schedule.Action(data.User, data.Message, at, ak)
		if err != nil {
			errs[key] = err
		}
	}
	if len(errs) != 0 {
		return nil, errs
	}
	return nil, nil
}

type MultiError map[string]error

func (m MultiError) Error() string {
	return fmt.Sprint(map[string]error(m))
}

func SilenceGet(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.Silence, nil
}

var silenceLayouts = []string{
	tsdbFormat,
	tsdbFormat + ":05",
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
		d, err := opentsdb.ParseDuration(data["duration"])
		if err != nil {
			return nil, err
		}
		end = start.Add(time.Duration(d))
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

func Templates(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.Conf.AlertTemplateStrings()
}

func APIRedirect(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "http://bosun.org/api.html", 302)
}

func Run(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.Check(t, time.Now())
}

func Host(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.Host(r.FormValue("filter")), nil
}

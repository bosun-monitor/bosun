package web

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"text/template/parse"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/collect"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/metadata"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/_third_party/github.com/gorilla/mux"
	"github.com/StackExchange/bosun/conf"
	"github.com/StackExchange/bosun/expr"
	"github.com/StackExchange/bosun/sched"
)

var (
	templates *template.Template
	router    = mux.NewRouter()
	schedule  = sched.DefaultSched
)

func init() {
	miniprofiler.Position = "bottomleft"
	miniprofiler.StartHidden = true
}

func Listen(addr, dir, host, relayListen string) error {
	var err error
	templates, err = template.New("").ParseFiles(
		dir + "/templates/index.html",
	)
	if err != nil {
		log.Fatal(err)
	}
	RelayHTTP(relayListen, host, JSON(PutMetadata))
	router.Handle("/api/action", JSON(Action))
	router.Handle("/api/alerts", JSON(Alerts))
	router.Handle("/api/alerts/details", JSON(AlertDetails))
	router.Handle("/api/config", miniprofiler.NewHandler(Config))
	router.Handle("/api/config_test", miniprofiler.NewHandler(ConfigTest))
	router.Handle("/api/egraph/{bs}.svg", JSON(ExprGraph))
	router.Handle("/api/expr", JSON(Expr))
	router.Handle("/api/graph", JSON(Graph))
	router.Handle("/api/health", JSON(HealthCheck))
	router.Handle("/api/metadata/get", JSON(GetMetadata))
	router.Handle("/api/metadata/put", JSON(PutMetadata))
	router.Handle("/api/metric", JSON(UniqueMetrics))
	router.Handle("/api/metric/{tagk}/{tagv}", JSON(MetricsByTagPair))
	router.Handle("/api/rule", JSON(Rule))
	router.Handle("/api/silence/clear", JSON(SilenceClear))
	router.Handle("/api/silence/get", JSON(SilenceGet))
	router.Handle("/api/silence/set", JSON(SilenceSet))
	router.Handle("/api/tagk/{metric}", JSON(TagKeysByMetric))
	router.Handle("/api/tagv/{tagk}", JSON(TagValuesByTagKey))
	router.Handle("/api/tagv/{tagk}/{metric}", JSON(TagValuesByMetricTagKey))
	router.Handle("/api/templates", JSON(Templates))
	router.HandleFunc("/api/put", Relay(host, JSON(PutMetadata)))
	http.Handle("/", miniprofiler.NewHandler(Index))
	http.Handle("/api/", router)
	fs := http.FileServer(http.Dir(dir))
	http.Handle("/partials/", fs)
	http.Handle("/static/", fs)
	static := http.FileServer(http.Dir(filepath.Join(dir, "static")))
	http.Handle("/favicon.ico", static)
	log.Println("bosun web listening on:", addr)
	log.Println("bosun web directory:", dir)
	return http.ListenAndServe(addr, nil)
}

func RelayHTTP(addr, dest string, metaHandler http.Handler) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", Relay(dest, metaHandler))
	log.Println("OpenTSDB relay listening on:", addr)
	log.Println("OpenTSDB destination:", dest)
	go func() { log.Fatal(http.ListenAndServe(addr, mux)) }()
}

var client = &http.Client{
	Timeout: time.Minute,
}

func Relay(dest string, metaHandler http.Handler) func(http.ResponseWriter, *http.Request) {
	clean := func(s string) string {
		return opentsdb.MustReplace(s, "_")
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/metadata/put" {
			metaHandler.ServeHTTP(w, r)
			return
		}
		orig, _ := ioutil.ReadAll(r.Body)
		if r.URL.Path == "/api/put" {
			var body []byte
			if r, err := gzip.NewReader(bytes.NewReader(orig)); err == nil {
				body, _ = ioutil.ReadAll(r)
				r.Close()
			} else {
				body = orig
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
		}
		durl := url.URL{
			Scheme: "http",
			Host:   dest,
		}
		durl.Path = r.URL.Path
		durl.RawQuery = r.URL.RawQuery
		durl.Fragment = r.URL.Fragment
		req, err := http.NewRequest(r.Method, durl.String(), bytes.NewReader(orig))
		if err != nil {
			log.Println("relay NewRequest err:", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		req.Header = r.Header
		req.TransferEncoding = r.TransferEncoding
		req.ContentLength = r.ContentLength
		resp, err := client.Do(req)
		tags := opentsdb.TagSet{"path": clean(r.URL.Path), "remote": clean(strings.Split(r.RemoteAddr, ":")[0])}
		if err != nil {
			log.Println("relay Do err:", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			collect.Add("relay.do_err", tags, 1)
			return
		}
		b, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		tags["status"] = strconv.Itoa(resp.StatusCode)
		collect.Add("relay.response", tags, 1)
		w.WriteHeader(resp.StatusCode)
		w.Write(b)
	}
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

type Health struct {
	// RuleCheck is true if last check happened within the check frequency window.
	RuleCheck bool
}

func HealthCheck(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var h Health
	h.RuleCheck = schedule.CheckStart.After(time.Now().Add(-schedule.Conf.CheckFrequency))
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

func Alerts(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule, nil
}

func AlertDetails(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	r.ParseForm()
	states := make(sched.States)
	for _, v := range r.Form["key"] {
		k := expr.AlertKey(v)
		s := schedule.Status(k)
		if s == nil {
			return nil, fmt.Errorf("unknown key: %v", v)
		}
		states[k] = s
	}
	return states, nil
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
		err := schedule.Action(data.User, data.Message, at, expr.AlertKey(key))
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

func nilFunc() {}

var builtins = template.FuncMap{
	"and":      nilFunc,
	"call":     nilFunc,
	"html":     nilFunc,
	"index":    nilFunc,
	"js":       nilFunc,
	"len":      nilFunc,
	"not":      nilFunc,
	"or":       nilFunc,
	"print":    nilFunc,
	"printf":   nilFunc,
	"println":  nilFunc,
	"urlquery": nilFunc,
	"eq":       nilFunc,
	"ge":       nilFunc,
	"gt":       nilFunc,
	"le":       nilFunc,
	"lt":       nilFunc,
	"ne":       nilFunc,

	// HTML-specific funcs
	"html_template_attrescaper":     nilFunc,
	"html_template_commentescaper":  nilFunc,
	"html_template_cssescaper":      nilFunc,
	"html_template_cssvaluefilter":  nilFunc,
	"html_template_htmlnamefilter":  nilFunc,
	"html_template_htmlescaper":     nilFunc,
	"html_template_jsregexpescaper": nilFunc,
	"html_template_jsstrescaper":    nilFunc,
	"html_template_jsvalescaper":    nilFunc,
	"html_template_nospaceescaper":  nilFunc,
	"html_template_rcdataescaper":   nilFunc,
	"html_template_urlescaper":      nilFunc,
	"html_template_urlfilter":       nilFunc,
	"html_template_urlnormalizer":   nilFunc,

	// bosun-specific funcs
	"V":       nilFunc,
	"bytes":   nilFunc,
	"replace": nilFunc,
	"short":   nilFunc,
}

func Templates(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	templates := make(map[string]string)
	for name, template := range schedule.Conf.Templates {
		incl := map[string]bool{name: true}
		var parseSection func(*conf.Template) error
		parseTemplate := func(s string) error {
			trees, err := parse.Parse("", s, "", "", builtins)
			if err != nil {
				return err
			}
			for _, node := range trees[""].Root.Nodes {
				switch node := node.(type) {
				case *parse.TemplateNode:
					if incl[node.Name] {
						continue
					}
					incl[node.Name] = true
					if err := parseSection(schedule.Conf.Templates[node.Name]); err != nil {
						return err
					}
				}
			}
			return nil
		}
		parseSection = func(s *conf.Template) error {
			if s.Body != nil {
				if err := parseTemplate(s.Body.Tree.Root.String()); err != nil {
					return err
				}
			}
			if s.Subject != nil {
				if err := parseTemplate(s.Subject.Tree.Root.String()); err != nil {
					return err
				}
			}
			return nil
		}
		if err := parseSection(template); err != nil {
			return nil, err
		}
		delete(incl, name)
		templates[name] = template.Def
		for n := range incl {
			t := schedule.Conf.Templates[n]
			if t == nil {
				continue
			}
			templates[name] += "\n\n" + t.Def
		}
	}
	alerts := make(map[string]string)
	for name, alert := range schedule.Conf.Alerts {
		var add func([]string)
		add = func(macros []string) {
			for _, macro := range macros {
				m := schedule.Conf.Macros[macro]
				add(m.Macros)
				alerts[name] += m.Def + "\n\n"
			}
		}
		add(alert.Macros)
		alerts[name] += alert.Def
	}
	return struct {
		Templates map[string]string
		Alerts    map[string]string
	}{
		templates,
		alerts,
	}, nil
}

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
	cparse "github.com/StackExchange/bosun/conf/parse"
	"github.com/StackExchange/bosun/expr"
	eparse "github.com/StackExchange/bosun/expr/parse"
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

func Listen(listenAddr, webDirectory string, tsdbHost *url.URL) error {
	var err error
	templates, err = template.New("").ParseFiles(
		webDirectory + "/templates/index.html",
	)
	if err != nil {
		log.Fatal(err)
	}
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
	router.Handle("/api/status", JSON(Status))
	router.Handle("/api/tagk/{metric}", JSON(TagKeysByMetric))
	router.Handle("/api/tagv/{tagk}", JSON(TagValuesByTagKey))
	router.Handle("/api/tagv/{tagk}/{metric}", JSON(TagValuesByMetricTagKey))
	router.Handle("/api/templates", JSON(Templates))
	router.Handle("/api/put", Relay(tsdbHost))
	http.Handle("/", miniprofiler.NewHandler(Index))
	http.Handle("/api/", router)
	fs := http.FileServer(http.Dir(webDirectory))
	http.Handle("/partials/", fs)
	http.Handle("/static/", fs)
	static := http.FileServer(http.Dir(filepath.Join(webDirectory, "static")))
	http.Handle("/favicon.ico", static)
	log.Println("bosun web listening on:", listenAddr)
	log.Println("bosun web directory:", webDirectory)
	log.Println("tsdb host:", tsdbHost)
	return http.ListenAndServe(listenAddr, nil)
}

var client *http.Client = &http.Client{
	Transport: &timeoutTransport{
		Transport: &http.Transport{},
	},
	Timeout: time.Minute,
}

type timeoutTransport struct {
	*http.Transport
	Timeout time.Time
}

func (t *timeoutTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if time.Now().After(t.Timeout) {
		t.Transport.CloseIdleConnections()
		t.Timeout = time.Now().Add(time.Minute * 5)
	}
	return t.Transport.RoundTrip(r)
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
		if cb := r.FormValue("callback"); cb != "" {
			w.Header().Add("Content-Type", "application/javascript")
			w.Write([]byte(cb + "("))
			w.Write(b)
			w.Write([]byte(")"))
			return
		}
		w.Header().Add("Content-Type", "application/json")
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
	return schedule.MarshalGroups(r.FormValue("filter"))
}

func Status(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	r.ParseForm()
	m := make(map[string]interface{})
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

func AlertDetails(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	r.ParseForm()
	states := make(sched.States)
	for _, v := range r.Form["key"] {
		k, err := expr.ParseAlertKey(v)
		if err != nil {
			return nil, err
		}
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
		lookups := make(map[string]bool)
		var add func([]string)
		add = func(macros []string) {
			for _, macro := range macros {
				m := schedule.Conf.Macros[macro]
				for _, pair := range m.Pairs {
					n := pair.GetNode()
					if pn, ok := n.(*cparse.PairNode); ok {
						if lookup := conf.LookupNotificationRE.FindStringSubmatch(pn.Val.Text); lookup != nil {
							table := lookup[1]
							if !lookups[table] {
								lookups[table] = true
								l := schedule.Conf.Lookups[table]
								if l != nil {
									alerts[name] += l.Def + "\n\n"
								}
							}
						}
					}
				}
				add(m.Macros)
				alerts[name] += m.Def + "\n\n"
			}
		}
		add(alert.Macros)
		walk := func(n eparse.Node) {
			eparse.Walk(n, func(n eparse.Node) {
				switch n := n.(type) {
				case *eparse.FuncNode:
					if n.Name != "lookup" || len(n.Args) == 0 {
						return
					}
					switch n := n.Args[0].(type) {
					case *eparse.StringNode:
						if lookups[n.Text] {
							return
						}
						lookups[n.Text] = true
						l := schedule.Conf.Lookups[n.Text]
						if l == nil {
							return
						}
						alerts[name] += l.Def + "\n\n"
					}
				}
			})
		}
		if alert.Crit != nil {
			walk(alert.Crit.Tree.Root)
		}
		if alert.Warn != nil {
			walk(alert.Warn.Tree.Root)
		}
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

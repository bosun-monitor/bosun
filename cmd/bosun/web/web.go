package web // import "bosun.org/cmd/bosun/web"

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"bosun.org/_third_party/github.com/MiniProfiler/go/miniprofiler"
	"bosun.org/_third_party/github.com/gorilla/mux"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/sched"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/version"
)

var (
	indexTemplate func() *template.Template
	router        = mux.NewRouter()
	schedule      = sched.DefaultSched
)

const (
	tsdbFormat         = "2006/01/02-15:04"
	tsdbFormatSecs     = tsdbFormat + ":05"
	miniprofilerHeader = "X-Miniprofiler"
)

func init() {
	miniprofiler.Position = "bottomleft"
	miniprofiler.StartHidden = true
	miniprofiler.Enable = func(r *http.Request) bool {
		return r.Header.Get(miniprofilerHeader) != ""
	}
	metadata.AddMetricMeta("bosun.search.puts_relayed", metadata.Counter, metadata.Request,
		"The count of api put requests sent to Bosun for relaying to the backend server.")
	metadata.AddMetricMeta("bosun.search.datapoints_relayed", metadata.Counter, metadata.Item,
		"The count of data points sent to Bosun for relaying to the backend server.")
	metadata.AddMetricMeta("bosun.relay.bytes", metadata.Counter, metadata.BytesPerSecond,
		"Bytes per second relayed from Bosun to the backend server.")
	metadata.AddMetricMeta("bosun.relay.response", metadata.Counter, metadata.PerSecond,
		"HTTP response codes from the backend server for request relayed through Bosun.")
}

func Listen(listenAddr string, devMode bool, tsdbHost string) error {
	if devMode {
		slog.Infoln("using local web assets")
	}
	webFS := FS(devMode)

	indexTemplate = func() *template.Template {
		str := FSMustString(devMode, "/templates/index.html")
		templates, err := template.New("").Parse(str)
		if err != nil {
			slog.Fatal(err)
		}
		return templates
	}

	if !devMode {
		tpl := indexTemplate()
		indexTemplate = func() *template.Template {
			return tpl
		}
	}

	if tsdbHost != "" {
		router.HandleFunc("/api/index", IndexTSDB)
		router.Handle("/api/put", Relay(tsdbHost))
	}
	router.HandleFunc("/api/", APIRedirect)
	router.Handle("/api/action", JSON(Action))
	router.Handle("/api/alerts", JSON(Alerts))
	router.Handle("/api/backup", JSON(Backup))
	router.Handle("/api/config", miniprofiler.NewHandler(Config))
	router.Handle("/api/config_test", miniprofiler.NewHandler(ConfigTest))
	router.Handle("/api/egraph/{bs}.svg", JSON(ExprGraph))
	router.Handle("/api/errors", JSON(ErrorHistory))
	router.Handle("/api/expr", JSON(Expr))
	router.Handle("/api/graph", JSON(Graph))
	router.Handle("/api/health", JSON(HealthCheck))
	router.Handle("/api/host", JSON(Host))
	router.Handle("/api/last", JSON(Last))
	router.Handle("/api/incidents", JSON(Incidents))
	router.Handle("/api/incidents/events", JSON(IncidentEvents))
	router.Handle("/api/metadata/get", JSON(GetMetadata))
	router.Handle("/api/metadata/metrics", JSON(MetadataMetrics))
	router.Handle("/api/metadata/put", JSON(PutMetadata))
	router.Handle("/api/metric", JSON(UniqueMetrics))
	router.Handle("/api/metric/{tagk}/{tagv}", JSON(MetricsByTagPair))
	router.Handle("/api/metric/tagkey", JSON(MetricsWithTagKeys))
	router.Handle("/api/rule", JSON(Rule))
	router.HandleFunc("/api/shorten", Shorten)
	router.Handle("/api/silence/clear", JSON(SilenceClear))
	router.Handle("/api/silence/get", JSON(SilenceGet))
	router.Handle("/api/silence/set", JSON(SilenceSet))
	router.Handle("/api/status", JSON(Status))
	router.Handle("/api/tagk/{metric}", JSON(TagKeysByMetric))
	router.Handle("/api/tagv/{tagk}", JSON(TagValuesByTagKey))
	router.Handle("/api/tagv/{tagk}/{metric}", JSON(TagValuesByMetricTagKey))
	router.HandleFunc("/api/version", Version)
	router.Handle("/api/debug/schedlock", JSON(ScheduleLockStatus))
	http.Handle("/", miniprofiler.NewHandler(Index))
	http.Handle("/api/", router)
	fs := http.FileServer(webFS)
	http.Handle("/partials/", fs)
	http.Handle("/static/", http.StripPrefix("/static/", fs))
	http.Handle("/favicon.ico", fs)
	slog.Infoln("bosun web listening on:", listenAddr)
	slog.Infoln("tsdb host:", tsdbHost)
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
	indexTSDB(r, reader.buf.Bytes())
	tags := opentsdb.TagSet{"path": clean(r.URL.Path), "remote": clean(strings.Split(r.RemoteAddr, ":")[0])}
	collect.Add("relay.bytes", tags, int64(reader.buf.Len()))
	tags["status"] = strconv.Itoa(w.code)
	collect.Add("relay.response", tags, 1)
}

func Relay(dest string) http.Handler {
	return &relayProxy{ReverseProxy: httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   dest,
	})}
}

func indexTSDB(r *http.Request, body []byte) {
	clean := func(s string) string {
		return opentsdb.MustReplace(s, "_")
	}
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
}

func IndexTSDB(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		slog.Error(err)
	}
	indexTSDB(r, body)
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
	r.Header.Set(miniprofilerHeader, "true")
	err := indexTemplate().Execute(w, struct {
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
		buf := new(bytes.Buffer)
		if err := json.NewEncoder(buf).Encode(d); err != nil {
			slog.Error(err)
			serveError(w, err)
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
			buf.WriteTo(tw)
			tw.Write([]byte(")"))
			return
		}
		w.Header().Add("Content-Type", "application/json")
		buf.WriteTo(tw)
	})
}

func Shorten(w http.ResponseWriter, r *http.Request) {
	u := url.URL{
		Scheme: "https",
		Host:   "www.googleapis.com",
		Path:   "/urlshortener/v1/url",
	}
	if schedule.Conf.ShortURLKey != "" {
		u.RawQuery = "key=" + schedule.Conf.ShortURLKey
	}
	j, err := json.Marshal(struct {
		LongURL string `json:"longUrl"`
	}{
		r.Referer(),
	})
	if err != nil {
		serveError(w, err)
		return
	}
	req, err := http.Post(u.String(), "application/json", bytes.NewBuffer(j))
	if err != nil {
		serveError(w, err)
		return
	}
	io.Copy(w, req.Body)
	req.Body.Close()
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
	metric := r.FormValue("metric")
	return schedule.MetadataMetrics(metric), nil
}

func Alerts(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.MarshalGroups(t, r.FormValue("filter"))
}

func Backup(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	data, err := schedule.GetStateFileBackup()
	if err != nil {
		return nil, err
	}
	_, err = w.Write(data)
	return nil, err
}

func IncidentEvents(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	id := r.FormValue("id")
	if id == "" {
		return nil, fmt.Errorf("id must be specified")
	}
	num, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, err
	}
	incident, events, actions, err := schedule.GetIncidentEvents(uint64(num))
	if err != nil {
		return nil, err
	}
	return struct {
		Incident *sched.Incident
		Events   []sched.Event
		Actions  []sched.Action
	}{incident, events, actions}, nil
}

func Incidents(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	alert := r.FormValue("alert")
	toTime := time.Now().UTC()
	fromTime := toTime.Add(-14 * 24 * time.Hour) // 2 weeks

	if from := r.FormValue("from"); from != "" {
		t, err := time.Parse(tsdbFormatSecs, from)
		if err != nil {
			return nil, err
		}
		fromTime = t
	}
	if to := r.FormValue("to"); to != "" {
		t, err := time.Parse(tsdbFormatSecs, to)
		if err != nil {
			return nil, err
		}
		toTime = t
	}
	incidents := schedule.GetIncidents(alert, fromTime, toTime)
	maxIncidents := 200
	if len(incidents) > maxIncidents {
		incidents = incidents[:maxIncidents]
	}
	return incidents, nil
}

func Status(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	r.ParseForm()
	type ExtStatus struct {
		AlertName string
		*sched.State
	}
	m := make(map[string]ExtStatus)
	for _, k := range r.Form["ak"] {
		ak, err := expr.ParseAlertKey(k)
		if err != nil {
			return nil, err
		}
		st := ExtStatus{State: schedule.GetStatus(ak)}
		if st.State == nil {
			return nil, fmt.Errorf("unknown alert key: %v", k)
		}
		st.AlertName = ak.Name()
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
		Notify  bool
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
	successful := []expr.AlertKey{}
	for _, key := range data.Keys {
		ak, err := expr.ParseAlertKey(key)
		if err != nil {
			return nil, err
		}
		err = schedule.Action(data.User, data.Message, at, ak)
		if err != nil {
			errs[key] = err
		} else {
			successful = append(successful, ak)
		}
	}
	if len(errs) != 0 {
		return nil, errs
	}
	if data.Notify && len(successful) != 0 {
		schedule.ActionNotify(at, data.User, data.Message, successful)
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
	tsdbFormatSecs,
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
	return schedule.AddSilence(start, end, data["alert"], data["tags"], data["forget"] == "true", len(data["confirm"]) > 0, data["edit"], data["user"], data["message"])
}

func SilenceClear(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	id := r.FormValue("id")
	return nil, schedule.ClearSilence(id)
}

func ConfigTest(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		serveError(w, err)
		return
	}
	if len(b) == 0 {
		serveError(w, fmt.Errorf("empty config"))
		return
	}
	_, err = conf.New("test", string(b))
	if err != nil {
		fmt.Fprintf(w, err.Error())
	}
}

func Config(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	var text string
	var err error
	if hash := r.FormValue("hash"); hash != "" {
		text, err = schedule.LoadTempConfig(hash)
		if err != nil {
			serveError(w, err)
			return
		}
	} else {
		text = schedule.Conf.RawText
	}
	fmt.Fprint(w, text)
}

func APIRedirect(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "http://bosun.org/api.html", 302)
}

func Host(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.Host(r.FormValue("filter")), nil
}

func Last(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var counter bool
	if r.FormValue("counter") != "" {
		counter = true
	}
	return schedule.Search.GetLast(r.FormValue("metric"), r.FormValue("tagset"), counter)
}

func Version(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, version.GetVersionInfo("bosun"))
}

func ScheduleLockStatus(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	data := struct {
		Process string
		HeldFor string
	}{}
	if holder, since := schedule.GetLockStatus(); holder != "" {
		data.Process = holder
		data.HeldFor = time.Now().Sub(since).String()
	}
	return data, nil
}

func ErrorHistory(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if r.Method == "GET" {
		return schedule.GetErrorHistory(), nil
	}
	data := []struct {
		Alert string    `json:"Alert"`
		Start time.Time `json:"Start"`
	}{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}
	for _, key := range data {
		schedule.ClearErrorLine(key.Alert, key.Start)
	}
	return nil, nil
}

package web // import "bosun.org/cmd/bosun/web"

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"bosun.org/_version"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule"
	"bosun.org/cmd/bosun/database"
	"bosun.org/cmd/bosun/sched"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"

	"github.com/MiniProfiler/go/miniprofiler"
	"github.com/bosun-monitor/annotate/backend"
	"github.com/bosun-monitor/annotate/web"
	"github.com/gorilla/mux"
)

var (
	indexTemplate   func() *template.Template
	router          = mux.NewRouter()
	schedule        = sched.DefaultSched
	InternetProxy   *url.URL
	annotateBackend backend.Backend
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
	router.Handle("/api/config", miniprofiler.NewHandler(Config))
	router.Handle("/api/config_test", miniprofiler.NewHandler(ConfigTest))
	router.Handle("/api/config/alert", miniprofiler.NewHandler(SetAlert)).Methods(http.MethodPost)
	router.Handle("/api/config/alert/{name}", miniprofiler.NewHandler(DeleteAlert)).Methods(http.MethodDelete)
	router.Handle("/api/config/bulkedit", miniprofiler.NewHandler(BulkEdit)).Methods(http.MethodPost)
	router.Handle("/api/config/save", miniprofiler.NewHandler(SaveConfig)).Methods(http.MethodPost)
	router.Handle("/api/config/diff", miniprofiler.NewHandler(DiffConfig)).Methods(http.MethodPost)
	router.Handle("/api/config/running_hash", JSON(ConfigRunningHash))
	router.Handle("/api/egraph/{bs}.{format:svg|png}", JSON(ExprGraph))
	router.Handle("/api/errors", JSON(ErrorHistory))
	router.Handle("/api/expr", JSON(Expr))
	router.Handle("/api/graph", JSON(Graph))
	router.Handle("/api/health", JSON(HealthCheck))
	router.Handle("/api/host", JSON(Host))
	router.Handle("/api/last", JSON(Last))
	router.Handle("/api/quiet", JSON(Quiet))
	router.Handle("/api/incidents", JSON(Incidents))
	router.Handle("/api/incidents/open", JSON(ListOpenIncidents))
	router.Handle("/api/incidents/events", JSON(IncidentEvents))
	router.Handle("/api/metadata/get", JSON(GetMetadata))
	router.Handle("/api/metadata/metrics", JSON(MetadataMetrics))
	router.Handle("/api/metadata/put", JSON(PutMetadata))
	router.Handle("/api/metadata/delete", JSON(DeleteMetadata)).Methods("DELETE")
	router.Handle("/api/metric", JSON(UniqueMetrics))
	router.Handle("/api/metric/{tagk}", JSON(MetricsByTagKey))
	router.Handle("/api/metric/{tagk}/{tagv}", JSON(MetricsByTagPair))
	router.Handle("/api/rule", JSON(Rule))
	router.HandleFunc("/api/shorten", Shorten)
	router.Handle("/api/silence/clear", JSON(SilenceClear))
	router.Handle("/api/silence/get", JSON(SilenceGet))
	router.Handle("/api/silence/set", JSON(SilenceSet))
	router.Handle("/api/status", JSON(Status))
	router.Handle("/api/tagk/{metric}", JSON(TagKeysByMetric))
	router.Handle("/api/tagv/{tagk}", JSON(TagValuesByTagKey))
	router.Handle("/api/tagv/{tagk}/{metric}", JSON(TagValuesByMetricTagKey))
	router.Handle("/api/tagsets/{metric}", JSON(FilteredTagsetsByMetric))
	router.Handle("/api/opentsdb/version", JSON(OpenTSDBVersion))
	router.Handle("/api/annotate", JSON(AnnotateEnabled))

	// Annotations
	if schedule.SystemConf.AnnotateEnabled() {
		var err error
		index := schedule.SystemConf.GetAnnotateIndex()
		if index == "" {
			index = "annotate"
		}
		annotateBackend, err = backend.NewElastic(schedule.SystemConf.GetAnnotateElasticHosts(), index)
		if err != nil {
			return err
		}
		if err := annotateBackend.InitBackend(); err != nil {
			return err
		}
		web.AddRoutes(router, "/api", []backend.Backend{annotateBackend}, false, false)
	}

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

func ResetSchedule() {
	schedule = sched.DefaultSched
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
	return &relayProxy{ReverseProxy: util.NewSingleHostProxy(&url.URL{
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
	if schedule.SystemConf.GetShortURLKey() != "" {
		u.RawQuery = "key=" + schedule.SystemConf.GetShortURLKey()
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

	transport := &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	if InternetProxy != nil {
		transport.Proxy = http.ProxyURL(InternetProxy)
	}
	c := http.Client{Transport: transport}

	req, err := c.Post(u.String(), "application/json", bytes.NewBuffer(j))
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

func Quiet(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.GetQuiet(), nil
}

func HealthCheck(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var h Health
	h.RuleCheck = schedule.LastCheck.After(time.Now().Add(-schedule.SystemConf.GetCheckFrequency()))
	return h, nil
}

func OpenTSDBVersion(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if schedule.SystemConf.GetTSDBContext() != nil {
		return schedule.SystemConf.GetTSDBContext().Version(), nil
	}
	return opentsdb.Version{0, 0}, nil
}

func AnnotateEnabled(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.SystemConf.AnnotateEnabled(), nil
}

func PutMetadata(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	d := json.NewDecoder(r.Body)
	var ms []metadata.Metasend
	if err := d.Decode(&ms); err != nil {
		return nil, err
	}
	for _, m := range ms {
		err := schedule.PutMetadata(metadata.Metakey{
			Metric: m.Metric,
			Tags:   m.Tags.Tags(),
			Name:   m.Name,
		}, m.Value)
		if err != nil {
			return nil, err
		}
	}
	w.WriteHeader(204)
	return nil, nil
}

func DeleteMetadata(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	d := json.NewDecoder(r.Body)
	var ms []struct {
		Tags opentsdb.TagSet
		Name string
	}
	if err := d.Decode(&ms); err != nil {
		return nil, err
	}
	for _, m := range ms {
		err := schedule.DeleteMetadata(m.Tags, m.Name)
		if err != nil {
			return nil, err
		}
	}
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
	return schedule.GetMetadata(r.FormValue("metric"), tags)
}

type MetricMetaTagKeys struct {
	*database.MetricMetadata
	TagKeys []string
}

func MetadataMetrics(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	metric := r.FormValue("metric")
	if metric != "" {
		m, err := schedule.MetadataMetrics(metric)
		if err != nil {
			return nil, err
		}
		keymap, err := schedule.DataAccess.Search().GetTagKeysForMetric(metric)
		if err != nil {
			return nil, err
		}
		var keys []string
		for k := range keymap {
			keys = append(keys, k)
		}
		return &MetricMetaTagKeys{
			MetricMetadata: m,
			TagKeys:        keys,
		}, nil
	}
	all := make(map[string]*MetricMetaTagKeys)
	metrics, err := schedule.DataAccess.Search().GetAllMetrics()
	if err != nil {
		return nil, err
	}
	for metric := range metrics {
		if strings.HasPrefix(metric, "__") {
			continue
		}
		m, err := schedule.MetadataMetrics(metric)
		if err != nil {
			return nil, err
		}
		keymap, err := schedule.DataAccess.Search().GetTagKeysForMetric(metric)
		if err != nil {
			return nil, err
		}
		var keys []string
		for k := range keymap {
			keys = append(keys, k)
		}
		all[metric] = &MetricMetaTagKeys{
			MetricMetadata: m,
			TagKeys:        keys,
		}
	}
	return all, nil
}

func Alerts(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.MarshalGroups(t, r.FormValue("filter"))
}

func IncidentEvents(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	id := r.FormValue("id")
	if id == "" {
		return nil, fmt.Errorf("id must be specified")
	}
	num, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, err
	}
	return schedule.DataAccess.State().GetIncidentState(num)
}

func Incidents(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	// TODO: Incident Search
	return nil, nil
	//	alert := r.FormValue("alert")
	//	toTime := time.Now().UTC()
	//	fromTime := toTime.Add(-14 * 24 * time.Hour) // 2 weeks

	//	if from := r.FormValue("from"); from != "" {
	//		t, err := time.Parse(tsdbFormatSecs, from)
	//		if err != nil {
	//			return nil, err
	//		}
	//		fromTime = t
	//	}
	//	if to := r.FormValue("to"); to != "" {
	//		t, err := time.Parse(tsdbFormatSecs, to)
	//		if err != nil {
	//			return nil, err
	//		}
	//		toTime = t
	//	}
	//	incidents, err := schedule.GetIncidents(alert, fromTime, toTime)
	//	if err != nil {
	//		return nil, err
	//	}
	//	maxIncidents := 200
	//	if len(incidents) > maxIncidents {
	//		incidents = incidents[:maxIncidents]
	//	}
	//	return incidents, nil
}

func Status(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	r.ParseForm()
	type ExtStatus struct {
		AlertName string
		*models.IncidentState
	}
	m := make(map[string]ExtStatus)
	for _, k := range r.Form["ak"] {
		ak, err := models.ParseAlertKey(k)
		if err != nil {
			return nil, err
		}
		var state *models.IncidentState
		if r.FormValue("all") != "" {
			allInc, err := schedule.DataAccess.State().GetAllIncidents(ak)
			if err != nil {
				return nil, err
			}
			if len(allInc) == 0 {
				return nil, fmt.Errorf("No incidents for alert key")
			}
			state = allInc[0]
			allEvents := models.EventsByTime{}
			for _, inc := range allInc {
				for _, e := range inc.Events {
					allEvents = append(allEvents, e)
				}
			}
			sort.Sort(allEvents)
			state.Events = allEvents
		} else {
			state, err = schedule.DataAccess.State().GetLatestIncident(ak)
			if err != nil {
				return nil, err
			}
		}
		st := ExtStatus{IncidentState: state}
		if st.IncidentState == nil {
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
		Ids     []int64
		Notify  bool
	}
	j := json.NewDecoder(r.Body)
	if err := j.Decode(&data); err != nil {
		return nil, err
	}
	var at models.ActionType
	switch data.Type {
	case "ack":
		at = models.ActionAcknowledge
	case "close":
		at = models.ActionClose
	case "forget":
		at = models.ActionForget
	case "forceClose":
		at = models.ActionForceClose
	case "purge":
		at = models.ActionPurge
	case "note":
		at = models.ActionNote
	}
	errs := make(MultiError)
	r.ParseForm()
	successful := []models.AlertKey{}
	for _, key := range data.Keys {
		ak, err := models.ParseAlertKey(key)
		if err != nil {
			return nil, err
		}
		err = schedule.ActionByAlertKey(data.User, data.Message, at, ak)
		if err != nil {
			errs[key] = err
		} else {
			successful = append(successful, ak)
		}
	}
	for _, id := range data.Ids {
		ak, err := schedule.ActionByIncidentId(data.User, data.Message, at, id)
		if err != nil {
			errs[fmt.Sprintf("%v", id)] = err
		} else {
			successful = append(successful, ak)
		}
	}
	if len(errs) != 0 {
		return nil, errs
	}
	if data.Notify && len(successful) != 0 {
		err := schedule.ActionNotify(at, data.User, data.Message, successful)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

type MultiError map[string]error

func (m MultiError) Error() string {
	return fmt.Sprint(map[string]error(m))
}

func SilenceGet(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	endingAfter := time.Now().UTC().Unix()
	if t := r.FormValue("t"); t != "" {
		endingAfter, _ = strconv.ParseInt(t, 10, 64)
	}
	return schedule.DataAccess.Silence().ListSilences(endingAfter)
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
	_, err = rule.NewConf("test", schedule.SystemConf.EnabledBackends(), string(b))
	if err != nil {
		fmt.Fprintf(w, err.Error())
	}
}

func Config(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	var text string
	var err error
	if hash := r.FormValue("hash"); hash != "" {
		text, err = schedule.DataAccess.Configs().GetTempConfig(hash)
		if err != nil {
			serveError(w, err)
			return
		}
	} else {
		text = schedule.RuleConf.GetRawText()
	}
	fmt.Fprint(w, text)
}

func SaveConfig(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	data := struct {
		Config  string
		User    string // should come from auth
		Diff    string
		Message string
		Other   []string
	}{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		serveError(w, err)
		return
	}
	err := schedule.RuleConf.SaveRawText(data.Config, data.Diff, data.User, data.Message, data.Other...)
	if err != nil {
		serveError(w, err)
		return
	}
	fmt.Fprint(w, "save successful")
}

func DiffConfig(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	data := struct {
		Config  string
		User    string // should come from auth
		Message string
		Other   []string
	}{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		serveError(w, err)
		return
	}
	diff, err := schedule.RuleConf.RawDiff(data.Config)
	if err != nil {
		serveError(w, err)
		return
	}
	fmt.Fprint(w, diff)
}

func ConfigRunningHash(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	hash := schedule.RuleConf.GetHash()
	return struct {
		Hash string
	}{
		hash,
	}, nil
}

func BulkEdit(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	bulkEdit := conf.BulkEditRequest{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&bulkEdit); err != nil {
		serveError(w, err)
		return
	}
	err := schedule.RuleConf.BulkEdit(bulkEdit)
	if err != nil {
		serveError(w, err)
		return
	}
	fmt.Fprint(w, "edit successful")
}

func SetAlert(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	var text string
	var err error
	data := struct {
		Name      string
		AlertText string
	}{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		serveError(w, err)
		return
	}
	text, err = schedule.RuleConf.SetAlert(data.Name, data.AlertText)
	if err != nil {
		serveError(w, err)
		return
	}
	fmt.Fprint(w, text)
}

func DeleteAlert(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := vars["name"]
	if name == "" {
		serveError(w, fmt.Errorf("must provide alert name"))
		return
	}
	err := schedule.RuleConf.DeleteAlert(name)
	if err != nil {
		serveError(w, err)
		return
	}
	fmt.Fprint(w, fmt.Sprintf("%v deleted", name))
}

func APIRedirect(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "http://bosun.org/api.html", 302)
}

func Host(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.Host(r.FormValue("filter"))
}

// Last returns the most recent datapoint for a metric+tagset. The metric+tagset
// string should be formated like os.cpu{host=foo}. The tag porition expects the
// that the keys will be in alphabetical order.
func Last(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var counter bool
	if r.FormValue("counter") != "" {
		counter = true
	}
	val, timestamp, err := schedule.Search.GetLast(r.FormValue("metric"), r.FormValue("tagset"), counter)
	return struct {
		Value     float64
		Timestamp int64
	}{
		val,
		timestamp,
	}, err
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
		data, err := schedule.DataAccess.Errors().GetFullErrorHistory()
		if err != nil {
			return nil, err
		}
		type AlertStatus struct {
			Success bool
			Errors  []*models.AlertError
		}
		failingAlerts, err := schedule.DataAccess.Errors().GetFailingAlerts()
		if err != nil {
			return nil, err
		}
		m := make(map[string]*AlertStatus, len(data))
		for a, list := range data {
			m[a] = &AlertStatus{
				Success: !failingAlerts[a],
				Errors:  list,
			}
		}
		return m, nil
	}
	if r.Method == "POST" {
		data := []string{}
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&data); err != nil {
			return nil, err
		}
		for _, key := range data {
			if err := schedule.ClearErrors(key); err != nil {
				return nil, err
			}
		}
	}
	return nil, nil
}

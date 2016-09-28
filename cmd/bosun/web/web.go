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
	"bosun.org/cmd/bosun/web/auth"
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
	reload          func() error
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

func Listen(httpAddr, httpsAddr, certFile, keyFile string, devMode bool, tsdbHost string, reloadFunc func() error, authConfig conf.AuthConf) error {
	if devMode {
		slog.Infoln("using local web assets")
	}
	webFS := FS(devMode)

	if httpAddr == "" && httpsAddr == "" {
		return fmt.Errorf("Either http or https address needs to be specified.")
	}

	indexTemplate = func() *template.Template {
		str := FSMustString(devMode, "/templates/index.html")
		templates, err := template.New("").Parse(str)
		if err != nil {
			slog.Fatal(err)
		}
		return templates
	}

	reload = reloadFunc

	if !devMode {
		tpl := indexTemplate()
		indexTemplate = func() *template.Template {
			return tpl
		}
	}
	provider, tok := buildAuth(authConfig)

	// middlewares everything gets
	baseChain := MiddlewareChain{miniprofileMiddleware, gzipMiddleware, protocolLoggingMiddleware}
	noAuth := baseChain.Extend(authMiddleware(auth.None, provider))
	readAuth := baseChain.Extend(authMiddleware(auth.Reader, provider))
	writeAuth := baseChain.Extend(authMiddleware(auth.Admin, provider))

	if tsdbHost != "" {
		router.HandleFunc("/api/index", IndexTSDB)
		router.Handle("/api/put", Relay(tsdbHost))
	}

	// api routes have their own function signature. Using JSON as an adapter function of sorts
	api := func(route string, f func(miniprofiler.Timer, http.ResponseWriter, *http.Request) (interface{}, error)) *mux.Route {
		return router.Handle(route, noAuth.Build()(jsonWrapper(f)))
	}
	apiR := func(route string, f func(miniprofiler.Timer, http.ResponseWriter, *http.Request) (interface{}, error)) *mux.Route {
		return router.Handle(route, readAuth.Build()(jsonWrapper(f)))
	}
	apiW := func(route string, f func(miniprofiler.Timer, http.ResponseWriter, *http.Request) (interface{}, error)) *mux.Route {
		return router.Handle(route, writeAuth.Build()(jsonWrapper(f)))
	}

	// some routes directly implement http listener
	wrap := noAuth.Build()
	wrapR := readAuth.Build()
	plain := func(route string, h http.HandlerFunc) *mux.Route {
		return router.Handle(route, wrap(h))
	}

	http.Handle("/login/", provider.LoginHandler())
	plain("/api/", apiRedirect)
	apiW("/api/action", action)
	apiR("/api/alerts", alerts)
	apiR("/api/save_enabled", SaveEnabled)
	apiR("/api/config", config)
	apiR("/api/config_test", configTest)
	if schedule.SystemConf.ReloadEnabled() { // Is true of save is enabled
		apiW("/api/reload", Reload).Methods(http.MethodPost)
	}
	if schedule.SystemConf.SaveEnabled() {
		apiW("/api/config/bulkedit", BulkEdit).Methods(http.MethodPost)
		apiW("/api/config/save", SaveConfig).Methods(http.MethodPost)
		apiR("/api/config/diff", DiffConfig).Methods(http.MethodPost)
		apiR("/api/config/running_hash", ConfigRunningHash)
	}
	apiR("/api/egraph/{bs}.{format:svg|png}", ExprGraph)
	apiR("/api/errors", errorHistory)
	apiR("/api/expr", Expr)
	apiR("/api/graph", Graph)
	api("/api/health", healthCheck)
	apiR("/api/host", host)
	apiR("/api/last", last)
	apiR("/api/quiet", quiet)
	apiR("/api/incidents", incidents)
	apiR("/api/incidents/open", ListOpenIncidents)
	apiR("/api/incidents/events", incidentEvents)
	apiR("/api/metadata/get", getMetadata)
	apiR("/api/metadata/metrics", metadataMetrics)
	apiW("/api/metadata/put", putMetadata) //TODO: does this need write access?
	apiW("/api/metadata/delete", deleteMetadata).Methods("DELETE")
	apiR("/api/metric", uniqueMetrics)
	apiR("/api/metric/{tagk}", metricsByTagKey)
	apiR("/api/metric/{tagk}/{tagv}", metricsByTagPair)
	apiR("/api/rule", Rule)
	apiR("/api/shorten", shorten)
	apiW("/api/silence/clear", silenceClear)
	apiR("/api/silence/get", silenceGet)
	apiW("/api/silence/set", silenceSet)
	apiR("/api/status", status)
	apiR("/api/tagk/{metric}", tagKeysByMetric)
	apiR("/api/tagv/{tagk}", tagValuesByTagKey)
	apiR("/api/tagv/{tagk}/{metric}", tagValuesByMetricTagKey)
	apiR("/api/tagsets/{metric}", filteredTagsetsByMetric)
	apiR("/api/opentsdb/version", openTSDBVersion)
	apiR("/api/annotate", annotateEnabled)
	if tok != nil {
		apiW("/api/tokens", tok.ListTokens).Methods("GET")
		apiW("/api/tokens", tok.CreateToken).Methods("POST")
		apiW("/api/tokens", tok.Revoke).Methods("DELETE")
	}
	// Annotations
	if schedule.SystemConf.AnnotateEnabled() {
		index := schedule.SystemConf.GetAnnotateIndex()
		if index == "" {
			index = "annotate"
		}
		annotateBackend = backend.NewElastic(schedule.SystemConf.GetAnnotateElasticHosts(), index)

		go func() {
			for {
				err := annotateBackend.InitBackend()
				if err == nil {
					return
				}
				slog.Warningf("could not initalize annotate backend, will try again: %v", err)
				time.Sleep(time.Second * 30)
			}
		}()
		//TODO: kinda hard to wrap these calls with middleware.
		web.AddRoutes(router, "/api", []backend.Backend{annotateBackend}, false, false)
	}

	plain("/api/version", Version)

	http.Handle("/", wrapR(http.HandlerFunc(index)))

	http.Handle("/api/", router)

	fs := http.FileServer(webFS)

	http.Handle("/partials/", wrap(fs))
	http.Handle("/static/", wrap(http.StripPrefix("/static/", fs)))
	http.Handle("/favicon.ico", wrap(fs))

	slog.Infoln("tsdb host:", tsdbHost)
	errChan := make(chan error, 1)
	if httpAddr != "" {
		go func() {
			slog.Infoln("bosun web listening http on:", httpAddr)
			errChan <- http.ListenAndServe(httpAddr, nil)
		}()
	}
	if httpsAddr != "" {
		go func() {
			slog.Infoln("bosun web listening https on:", httpsAddr)
			if certFile == "" || keyFile == "" {
				errChan <- fmt.Errorf("certFile and keyfile must be specified to use https")
			}
			errChan <- http.ListenAndServeTLS(httpsAddr, certFile, keyFile, nil)
		}()
	}
	return <-errChan
}

func buildAuth(cfg conf.AuthConf) (auth.Provider, *auth.TokenProvider) {
	providers := []auth.Provider{}
	var tok *auth.TokenProvider
	if cfg.LDAPServer != "" {
		grps := make([]*auth.LdapGroup, len(cfg.Groups))
		for i, g := range cfg.Groups {
			grps[i] = &auth.LdapGroup{
				Path:  g.Path,
				Level: auth.Permission(g.AccesLevel),
			}
		}
		p := auth.NewLdap(cfg.LDAPServer, cfg.Domain, grps, cfg.RootSearchPath, cfg.CookieSecret)
		providers = append(providers, p)
	}
	if cfg.TokenSecret != "" {
		tok = auth.NewToken(cfg.TokenSecret, func() auth.TokenDataAccess { return schedule.DataAccess.Tokens() })
		providers = append(providers, tok)
	}
	if len(providers) == 1 {
		return providers[0], tok
	}
	if len(providers) > 1 {
		return auth.MultipleProviders(providers), tok
	}
	return auth.NoAuth{}, nil
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

type appSetings struct {
	SaveEnabled     bool
	AnnotateEnabled bool
	Quiet           bool
	Version         opentsdb.Version
}

type indexVariables struct {
	Includes template.HTML
	Settings string
}

func index(w http.ResponseWriter, r *http.Request) {
	t := miniprofiler.GetTimer(r)
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
	// Set some global settings for the UI to know about. This saves us from
	// having to make an HTTP call to see what features should be enabled
	// in the UI
	openTSDBVersion := opentsdb.Version{0, 0}
	if schedule.SystemConf.GetTSDBContext() != nil {
		openTSDBVersion = schedule.SystemConf.GetTSDBContext().Version()
	}
	settings, err := json.Marshal(appSetings{
		schedule.SystemConf.SaveEnabled(),
		schedule.SystemConf.AnnotateEnabled(),
		schedule.GetQuiet(),
		openTSDBVersion,
	})
	if err != nil {
		serveError(w, err)
		return
	}
	err = indexTemplate().Execute(w, indexVariables{
		t.Includes(),
		string(settings),
	})
	if err != nil {
		serveError(w, err)
	}
}

func serveError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func jsonWrapper(h func(miniprofiler.Timer, http.ResponseWriter, *http.Request) (interface{}, error)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t := miniprofiler.GetTimer(r)
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
		if cb := r.FormValue("callback"); cb != "" {
			w.Header().Add("Content-Type", "application/javascript")
			w.Write([]byte(cb + "("))
			buf.WriteTo(w)
			w.Write([]byte(")"))
			return
		}
		w.Header().Add("Content-Type", "application/json")
		buf.WriteTo(w)
	})
}

func shorten(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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
		return nil, err
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

	resp, err := c.Post(u.String(), "application/json", bytes.NewBuffer(j))
	if err != nil {
		return nil, err
	}
	io.Copy(w, resp.Body)
	resp.Body.Close()
	return nil, nil
}

type Health struct {
	// RuleCheck is true if last check happened within the check frequency window.
	RuleCheck bool
}

func Reload(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	d := json.NewDecoder(r.Body)
	var sane struct {
		Reload bool
	}
	if err := d.Decode(&sane); err != nil {
		return nil, fmt.Errorf("failed to decode post body: %v", err)
	}
	if !sane.Reload {
		return nil, fmt.Errorf("reload must be set to true in post body")
	}
	err := reload()
	if err != nil {
		return nil, fmt.Errorf("failed to reload: %v", err)
	}
	return "reloaded", nil

}

func quiet(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.GetQuiet(), nil
}

func healthCheck(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var h Health
	h.RuleCheck = schedule.LastCheck.After(time.Now().Add(-schedule.SystemConf.GetCheckFrequency()))
	return h, nil
}

func openTSDBVersion(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if schedule.SystemConf.GetTSDBContext() != nil {
		return schedule.SystemConf.GetTSDBContext().Version(), nil
	}
	return opentsdb.Version{0, 0}, nil
}

func annotateEnabled(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.SystemConf.AnnotateEnabled(), nil
}

func putMetadata(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func deleteMetadata(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func getMetadata(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func metadataMetrics(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func alerts(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.MarshalGroups(t, r.FormValue("filter"))
}

func incidentEvents(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func incidents(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func status(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func action(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func silenceGet(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func silenceSet(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func silenceClear(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	id := r.FormValue("id")
	return nil, schedule.ClearSilence(id)
}

func configTest(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("empty config")
	}
	_, err = rule.NewConf("test", schedule.SystemConf.EnabledBackends(), string(b))
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func config(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var text string
	var err error
	if hash := r.FormValue("hash"); hash != "" {
		text, err = schedule.DataAccess.Configs().GetTempConfig(hash)
		if err != nil {
			return nil, err
		}
	} else {
		text = schedule.RuleConf.GetRawText()
	}
	fmt.Fprint(w, text)
	return nil, nil
}

func apiRedirect(w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, "http://bosun.org/api.html", 302)
}

func host(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.Host(r.FormValue("filter"))
}

// last returns the most recent datapoint for a metric+tagset. The metric+tagset
// string should be formated like os.cpu{host=foo}. The tag porition expects the
// that the keys will be in alphabetical order.
func last(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func errorHistory(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

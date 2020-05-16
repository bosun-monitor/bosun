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
	"sort"
	"strconv"
	"strings"
	"time"

	version "bosun.org/_version"
	"bosun.org/annotate/backend"
	"bosun.org/annotate/web"
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
	"github.com/NYTimes/gziphandler"
	"github.com/captncraig/easyauth"
	"github.com/gorilla/mux"
	"github.com/justinas/alice"
)

var (
	indexTemplate func() *template.Template
	router        = mux.NewRouter()
	schedule      = sched.DefaultSched
	//InternetProxy is a url to use as a proxy when communicating with external services.
	//currently only google's shortener.
	InternetProxy *url.URL
	// AnnotateBackend is a client for the annotations store
	AnnotateBackend backend.Backend
	reloadFn        func() error

	tokensEnabled bool
	authEnabled   bool
	startTime     time.Time
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

// Listen starts the web server
func Listen(httpAddr, httpsAddr, certFile, keyFile string, devMode bool, tsdbHost string, reloadFunc func() error, authConfig *conf.AuthConf, st time.Time) error {
	startTime = st
	if devMode {
		slog.Infoln("using local web assets")
	}
	webFS := FS(devMode)

	if httpAddr == "" && httpsAddr == "" {
		return fmt.Errorf("either HTTP or HTTPS address needs to be specified")
	}

	indexTemplate = func() *template.Template {
		str := FSMustString(devMode, "/templates/index.html")
		templates, err := template.New("").Parse(str)
		if err != nil {
			slog.Fatal(err)
		}
		return templates
	}

	reloadFn = reloadFunc

	if !devMode {
		tpl := indexTemplate()
		indexTemplate = func() *template.Template {
			return tpl
		}
	}

	baseChain := alice.New(miniProfilerMiddleware, endpointStatsMiddleware, gziphandler.GzipHandler)

	auth, tokens, err := buildAuth(authConfig)
	if err != nil {
		slog.Fatal(err)
	}

	//helpers to add routes with middleware
	handle := func(route string, h http.Handler, perms easyauth.Role) *mux.Route {
		return router.Handle(route, baseChain.Then(auth.Wrap(h, perms)))
	}
	handleFunc := func(route string, h http.HandlerFunc, perms easyauth.Role) *mux.Route {
		return handle(route, h, perms)
	}

	const (
		GET  = http.MethodGet
		POST = http.MethodPost
	)

	if tsdbHost != "" {
		handleFunc("/api/index", indexTSDBHandler, canPutData).Name("tsdb_index")
		handle("/api/put", RelayToOpenTSDB(tsdbHost), canPutData).Name("tsdb_put")
	}
	router.PathPrefix("/auth/").Handler(auth.LoginHandler())
	handleFunc("/api/", apiRedirect, fullyOpen).Name("api_redir")
	handle("/api/action", wrapJSON(action), canPerformActions).Name("action").Methods(POST)
	handle("/api/alerts", wrapJSON(alerts), canViewDash).Name("alerts").Methods(GET)
	handle("/api/config", wrapJSON(config), canViewConfig).Name("get_config").Methods(GET)

	handle("/api/config_test", wrapJSON(configTest), canViewConfig).Name("config_test").Methods(POST)
	handle("/api/save_enabled", wrapJSON(saveEnabled), fullyOpen).Name("seve_enabled").Methods(GET)

	if schedule.SystemConf.ReloadEnabled() {
		handle("/api/reload", wrapJSON(reload), canSaveConfig).Name("can_save").Methods(POST)
	}

	if schedule.SystemConf.SaveEnabled() {
		handle("/api/config/bulkedit", wrapJSON(bulkEdit), canSaveConfig).Name("bulk_edit").Methods(POST)
		handle("/api/config/save", wrapJSON(saveConfig), canSaveConfig).Name("config_save").Methods(POST)
		handle("/api/config/diff", wrapJSON(diffConfig), canSaveConfig).Name("config_diff").Methods(POST)
		handle("/api/config/running_hash", wrapJSON(configRunningHash), canViewConfig).Name("config_hash").Methods(GET)
	}

	handle("/api/egraph/{bs}.{format:svg|png}", wrapJSON(ExprGraph), canRunTests).Name("expr_graph")
	handle("/api/errors", wrapJSON(errorHistory), canViewDash).Name("errors").Methods(GET, POST)
	handle("/api/expr", wrapJSON(expression), canRunTests).Name("expr").Methods(POST)
	handle("/api/graph", wrapJSON(Graph), canViewDash).Name("graph").Methods(GET)

	handle("/api/health", wrapJSON(healthCheck), fullyOpen).Name("health_check").Methods(GET)
	handle("/api/host", wrapJSON(hostHandler), canViewDash).Name("host").Methods(GET)
	handle("/api/last", wrapJSON(last), canViewDash).Name("last").Methods(GET)
	handle("/api/quiet", wrapJSON(quiet), canViewDash).Name("quiet").Methods(GET)
	handle("/api/incidents/open", wrapJSON(listOpenIncidents), canViewDash).Name("open_incidents").Methods(GET)
	handle("/api/incidents/events", wrapJSON(incidentEvents), canViewDash).Name("incident_events").Methods(GET)
	handle("/api/metadata/get", wrapJSON(getMetadata), canViewDash).Name("meta_get").Methods(GET)
	handle("/api/metadata/metrics", wrapJSON(metadataMetrics), canViewDash).Name("meta_metrics").Methods(GET)
	handle("/api/metadata/put", wrapJSON(putMetadata), canPutData).Name("meta_put").Methods(POST)
	handle("/api/metadata/delete", wrapJSON(deleteMetadata), canPutData).Name("meta_delete").Methods(http.MethodDelete)
	handle("/api/metric", wrapJSON(uniqueMetrics), canViewDash).Name("meta_uniqe_metrics").Methods(GET)
	handle("/api/metric/{tagk}", wrapJSON(metricsByTagKey), canViewDash).Name("meta_metrics_by_tag").Methods(GET)
	handle("/api/metric/{tagk}/{tagv}", wrapJSON(metricsByTagPair), canViewDash).Name("meta_metric_by_tag_pair").Methods(GET)

	handle("/api/rule", wrapJSON(ruleTest), canRunTests).Name("rule_test").Methods(POST)
	handle("/api/rule/notification/test", wrapJSON(testHTTPNotification), canRunTests).Name("rule__notification_test").Methods(POST)
	handle("/api/shorten", wrapJSON(shorten), canViewDash).Name("shorten")
	handle("/s/{id}", wrapJSON(getShortLink), canViewDash).Name("shortlink")
	handle("/api/silence/clear", wrapJSON(silenceClear), canSilence).Name("silence_clear")
	handle("/api/silence/get", wrapJSON(silenceGet), canViewDash).Name("silence_get").Methods(GET)
	handle("/api/silence/set", wrapJSON(silenceSet), canSilence).Name("silence_set")
	handle("/api/status", wrapJSON(status), canViewDash).Name("status").Methods(GET)
	handle("/api/tagk/{metric}", wrapJSON(tagKeysByMetric), canViewDash).Name("search_tkeys_by_metric").Methods(GET)
	handle("/api/tagv/{tagk}", wrapJSON(tagValuesByTagKey), canViewDash).Name("search_tvals_by_metric").Methods(GET)
	handle("/api/tagv/{tagk}/{metric}", wrapJSON(tagValuesByMetricTagKey), canViewDash).Name("search_tvals_by_metrictagkey").Methods(GET)
	handle("/api/tagsets/{metric}", wrapJSON(filteredTagsetsByMetric), canViewDash).Name("search_tagsets_by_metric").Methods(GET)
	handle("/api/opentsdb/version", wrapJSON(openTSDBVersion), fullyOpen).Name("otsdb_version").Methods(GET)
	handle("/api/annotate", wrapJSON(annotateEnabled), fullyOpen).Name("annotate_enabled").Methods(GET)

	// Annotations
	if schedule.SystemConf.AnnotateEnabled() {
		read := baseChain.Append(auth.Wrapper(canViewAnnotations)).ThenFunc
		write := baseChain.Append(auth.Wrapper(canCreateAnnotations)).ThenFunc
		web.AddRoutesWithMiddleware(router, "/api", []backend.Backend{AnnotateBackend}, false, false, read, write)
	}

	//auth specific stuff
	if auth != nil {
		router.PathPrefix("/login").Handler(http.StripPrefix("/login", auth.LoginHandler())).Name("auth")
	}
	if tokens != nil {
		handle("/api/tokens", tokens.AdminHandler(), canManageTokens).Name("tokens")
	}

	router.Handle("/api/version", baseChain.ThenFunc(versionHandler)).Name("version").Methods(GET)
	fs := http.FileServer(webFS)
	router.PathPrefix("/partials/").Handler(baseChain.Then(fs)).Name("partials")
	router.PathPrefix("/static/").Handler(baseChain.Then(http.StripPrefix("/static/", fs))).Name("static")
	router.PathPrefix("/favicon.ico").Handler(baseChain.Then(fs)).Name("favicon")

	var miniprofilerRoutes = http.StripPrefix(miniprofiler.PATH, http.HandlerFunc(miniprofiler.MiniProfilerHandler))
	router.PathPrefix(miniprofiler.PATH).Handler(baseChain.Then(miniprofilerRoutes)).Name("miniprofiler")

	//use default mux for pprof
	router.PathPrefix("/debug/pprof").Handler(http.DefaultServeMux)

	router.PathPrefix("/api").HandlerFunc(http.NotFound)
	//MUST BE LAST!
	router.PathPrefix("/").Handler(baseChain.Then(auth.Wrap(wrapJSON(index), canViewDash))).Name("index")

	slog.Infoln("tsdb host:", tsdbHost)
	errChan := make(chan error, 1)
	if httpAddr != "" {
		go func() {
			slog.Infoln("bosun web listening http on:", httpAddr)
			errChan <- http.ListenAndServe(httpAddr, router)
		}()
	}
	if httpsAddr != "" {
		go func() {
			slog.Infoln("bosun web listening https on:", httpsAddr)
			if certFile == "" || keyFile == "" {
				errChan <- fmt.Errorf("certFile and keyfile must be specified to use https")
			}
			errChan <- http.ListenAndServeTLS(httpsAddr, certFile, keyFile, router)
		}()
	}
	return <-errChan
}

type relayProxy struct {
	*httputil.ReverseProxy
}

// ResetSchedule resets the schedule to the default
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

// RelayToOpenTSDB creates a new handler to relay data to OpenTSDB
//
// This is used to mimic OpenTSDB's /api/put endpoint
func RelayToOpenTSDB(tsdbHost string) http.Handler {
	return &relayProxy{ReverseProxy: util.NewSingleHostProxy(&url.URL{
		Scheme: "http",
		Host:   tsdbHost,
	})}
}

// indexTSDB indexes data points sent through /api/put for searching and autocomplete by Bosun.
// Can be used directly or as part of the relay functionality
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

func indexTSDBHandler(_ http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		slog.Error(err)
	}
	indexTSDB(r, body)
}

type appSetings struct {
	SaveEnabled       bool
	AnnotateEnabled   bool
	Quiet             bool
	Version           opentsdb.Version
	ExampleExpression string

	AuthEnabled   bool
	TokensEnabled bool
	Username      string
	Permissions   easyauth.Role
	Roles         *roleMetadata
}

type indexVariables struct {
	Includes template.HTML
	Settings string
}

func index(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	if r.URL.Path == "/graph" {
		r.ParseForm()
		if _, present := r.Form["png"]; present {
			if _, err := Graph(t, w, r); err != nil {
				return nil, err
			}
			return nil, nil
		}
	}
	r.Header.Set(miniprofilerHeader, "true")
	// Set some global settings for the UI to know about. This saves us from
	// having to make an HTTP call to see what features should be enabled
	// in the UI
	openTSDBVersion := opentsdb.Version{}
	if schedule.SystemConf.GetTSDBContext() != nil {
		openTSDBVersion = schedule.SystemConf.GetTSDBContext().Version()
	}
	u := easyauth.GetUser(r)
	as := &appSetings{
		SaveEnabled:       schedule.SystemConf.SaveEnabled(),
		AnnotateEnabled:   schedule.SystemConf.AnnotateEnabled(),
		Quiet:             schedule.GetQuiet(),
		Version:           openTSDBVersion,
		AuthEnabled:       authEnabled,
		TokensEnabled:     tokensEnabled,
		Roles:             roleDefs,
		ExampleExpression: schedule.SystemConf.GetExampleExpression(),
	}
	if u != nil {
		as.Username = u.Username
		as.Permissions = u.Access
	}
	settings, err := json.Marshal(as)
	if err != nil {
		return nil, err
	}
	err = indexTemplate().Execute(w, indexVariables{
		t.Includes(),
		string(settings),
	})
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func serveError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func wrapJSON(h func(miniprofiler.Timer, http.ResponseWriter, *http.Request) (interface{}, error)) http.Handler {
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
		enc := json.NewEncoder(buf)
		if strings.Contains(r.Header.Get("Accept"), "html") || strings.Contains(r.Host, "localhost") {
			enc.SetIndent("", "  ")
		}
		if err := enc.Encode(d); err != nil {
			slog.Error(err)
			serveError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		buf.WriteTo(w)
	})
}

func shorten(_ miniprofiler.Timer, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	id, err := schedule.DataAccess.Configs().ShortenLink(r.Referer())
	if err != nil {
		return nil, err
	}
	return struct {
		ID string `json:"id"`
	}{schedule.SystemConf.MakeLink(fmt.Sprintf("/s/%d", id), nil)}, nil
}

func getShortLink(t miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	// on any error or bad param, just redirect to index. Otherwise 302 to stored url
	vars := mux.Vars(r)
	idv := vars["id"]
	id, err := strconv.Atoi(idv)
	targetURL := ""
	if err != nil {
		return index(t, w, r)
	}
	targetURL, err = schedule.DataAccess.Configs().GetShortLink(id)
	if err != nil {
		return index(t, w, r)
	}
	http.Redirect(w, r, targetURL, 302)
	return nil, nil
}

type health struct {
	// RuleCheck is true if last check happened within the check frequency window.
	RuleCheck     bool
	Quiet         bool
	UptimeSeconds int64
	StartEpoch    int64
	Notifications notificationStats
}

type notificationStats struct {
	// Post and email notifiaction stats
	PostNotificationsSuccess  int64
	PostNotificationsFailed   int64
	EmailNotificationsSuccess int64
	EmailNotificationsFailed  int64
}

func reload(_ miniprofiler.Timer, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
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
	err := reloadFn()
	if err != nil {
		return nil, fmt.Errorf("failed to reload: %v", err)
	}
	return "reloaded", nil

}

func quiet(miniprofiler.Timer, http.ResponseWriter, *http.Request) (interface{}, error) {
	return schedule.GetQuiet(), nil
}

func healthCheck(miniprofiler.Timer, http.ResponseWriter, *http.Request) (interface{}, error) {
	var h health
	var n notificationStats
	h.RuleCheck = schedule.LastCheck.After(time.Now().Add(-schedule.SystemConf.GetCheckFrequency()))
	h.Quiet = schedule.GetQuiet()
	h.UptimeSeconds = int64(time.Since(startTime).Seconds())
	h.StartEpoch = startTime.Unix()

	//notifications stats
	n.PostNotificationsSuccess = collect.Get("post.sent", nil)
	n.PostNotificationsFailed = collect.Get("post.sent_failed", nil)
	n.EmailNotificationsSuccess = collect.Get("email.sent", nil)
	n.EmailNotificationsFailed = collect.Get("email.sent_failed", nil)

	h.Notifications = n
	return h, nil
}

func openTSDBVersion(miniprofiler.Timer, http.ResponseWriter, *http.Request) (interface{}, error) {
	if schedule.SystemConf.GetTSDBContext() != nil {
		return schedule.SystemConf.GetTSDBContext().Version(), nil
	}
	return opentsdb.Version{Major: 0, Minor: 0}, nil
}

func annotateEnabled(miniprofiler.Timer, http.ResponseWriter, *http.Request) (interface{}, error) {
	return schedule.SystemConf.AnnotateEnabled(), nil
}

func putMetadata(_ miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func deleteMetadata(_ miniprofiler.Timer, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func getMetadata(_ miniprofiler.Timer, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
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

type metricMetaTagKeys struct {
	*database.MetricMetadata
	TagKeys []string
}

func metadataMetrics(_ miniprofiler.Timer, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
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
		return &metricMetaTagKeys{
			MetricMetadata: m,
			TagKeys:        keys,
		}, nil
	}
	all := make(map[string]*metricMetaTagKeys)
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
		all[metric] = &metricMetaTagKeys{
			MetricMetadata: m,
			TagKeys:        keys,
		}
	}
	return all, nil
}

func alerts(t miniprofiler.Timer, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.MarshalGroups(t, r.FormValue("filter"))
}

type extStatus struct {
	AlertName string
	Subject   string
	*models.IncidentState
	*models.RenderedTemplates
}

type extIncidentStatus struct {
	extStatus
	IsActive  bool
	Silence   *models.Silence
	SilenceId string
}

func incidentEvents(t miniprofiler.Timer, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	id := r.FormValue("id")
	if id == "" {
		return nil, fmt.Errorf("id must be specified")
	}
	num, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, err
	}
	state, err := schedule.DataAccess.State().GetIncidentState(num)
	if err != nil {
		return nil, err
	}
	rt, err := schedule.DataAccess.State().GetRenderedTemplates(state.Id)
	if err != nil {
		return nil, err
	}
	st := extIncidentStatus{
		extStatus: extStatus{IncidentState: state, RenderedTemplates: rt, Subject: state.Subject},
		IsActive:  state.IsActive(),
	}
	silence := schedule.GetSilence(t, state.AlertKey)
	if silence != nil {
		st.Silence = silence
		st.SilenceId = silence.ID()
	}
	return st, nil
}

func status(_ miniprofiler.Timer, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	r.ParseForm()
	m := make(map[string]extStatus)
	for _, k := range r.Form["ak"] {
		ak, err := models.ParseAlertKey(k)
		if err != nil {
			return nil, err
		}
		var state *models.IncidentState
		if r.FormValue("all") != "" {
			allInc, err := schedule.DataAccess.State().GetAllIncidentsByAlertKey(ak)
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
			if state == nil {
				return nil, fmt.Errorf("alert key %v wasn't found", k)
			}
		}
		rt, err := schedule.DataAccess.State().GetRenderedTemplates(state.Id)
		if err != nil {
			return nil, err
		}
		st := extStatus{IncidentState: state, RenderedTemplates: rt}
		if st.IncidentState == nil {
			return nil, fmt.Errorf("unknown alert key: %v", k)
		}
		st.AlertName = ak.Name()
		m[k] = st
	}
	return m, nil
}

func getUsername(r *http.Request) string {
	user := easyauth.GetUser(r)
	if user != nil {
		return user.Username
	}
	return "unknown"
}

func userCanOverwriteUsername(r *http.Request) bool {
	user := easyauth.GetUser(r)
	if user != nil {
		return user.Access&canOverwriteUsername != 0
	}
	return false
}

func action(_ miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	var data struct {
		Type    string
		Message string
		Keys    []string
		Ids     []int64
		Notify  bool
		User    string
		Time    *time.Time
	}
	j := json.NewDecoder(r.Body)
	if err := j.Decode(&data); err != nil {
		return nil, err
	}
	var at models.ActionType
	// TODO Make constants in the JS code for these that *match* the names the string Method for ActionType
	switch data.Type {
	case "ack":
		at = models.ActionAcknowledge
	case "close":
		at = models.ActionClose
	case "cancelClose":
		at = models.ActionCancelClose
	case "forget":
		at = models.ActionForget
	case "forceClose":
		at = models.ActionForceClose
	case "purge":
		at = models.ActionPurge
	case "note":
		at = models.ActionNote
	}
	errs := make(multiError)
	r.ParseForm()
	successful := []models.AlertKey{}

	if data.User != "" && !userCanOverwriteUsername(r) {
		http.Error(w, "Not Authorized to set User", 400)
		return nil, nil
	} else if data.User == "" {
		data.User = getUsername(r)
	}

	for _, key := range data.Keys {
		ak, err := models.ParseAlertKey(key)
		if err != nil {
			return nil, err
		}
		err = schedule.ActionByAlertKey(data.User, data.Message, at, data.Time, ak)
		if err != nil {
			errs[key] = err
		} else {
			successful = append(successful, ak)
		}
	}
	for _, id := range data.Ids {
		ak, err := schedule.ActionByIncidentId(data.User, data.Message, at, data.Time, id)
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
	} else {
		slog.Infof("action without notification. user: %s, type: %s, keys: %v, ids: %v", data.User, data.Type, data.Keys, data.Ids)
	}
	return nil, nil
}

type multiError map[string]error

func (m multiError) Error() string {
	return fmt.Sprint(map[string]error(m))
}

func silenceGet(_ miniprofiler.Timer, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func silenceSet(_ miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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
	username := getUsername(r)
	if _, ok := data["user"]; ok && !userCanOverwriteUsername(r) {
		http.Error(w, "Not authorized to set 'user' parameter", 400)
		return nil, nil
	} else if ok {
		username = data["user"]
	}
	return schedule.AddSilence(start, end, data["alert"], data["tags"], data["forget"] == "true", len(data["confirm"]) > 0, data["edit"], username, data["message"])
}

func silenceClear(_ miniprofiler.Timer, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	id := r.FormValue("id")
	return nil, schedule.ClearSilence(id)
}

func configTest(_ miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("empty config")
	}
	_, err = rule.NewConf("test", schedule.SystemConf.EnabledBackends(), schedule.SystemConf.GetRuleVars(), string(b))
	if err != nil {
		fmt.Fprint(w, err.Error())
	}
	return nil, nil
}

func config(_ miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func hostHandler(_ miniprofiler.Timer, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
	return schedule.Host(r.FormValue("filter"))
}

// last returns the most recent datapoint for a metric+tagset. The metric+tagset
// string should be formated like os.cpu{host=foo}. The tag porition expects the
// that the keys will be in alphabetical order.
func last(_ miniprofiler.Timer, _ http.ResponseWriter, r *http.Request) (interface{}, error) {
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

func versionHandler(w http.ResponseWriter, _ *http.Request) {
	io.WriteString(w, version.GetVersionInfo("bosun"))
}

func errorHistory(_ miniprofiler.Timer, w http.ResponseWriter, r *http.Request) (interface{}, error) {
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

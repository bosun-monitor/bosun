package main

//go:generate go run ../../build/generate/generate.go

import (
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"bosun.org/_version"

	"bosun.org/annotate/backend"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/conf/rule"
	"bosun.org/cmd/bosun/database"
	"bosun.org/cmd/bosun/expr"
	"bosun.org/cmd/bosun/ping"
	"bosun.org/cmd/bosun/sched"
	"bosun.org/cmd/bosun/web"
	"bosun.org/collect"
	"bosun.org/graphite"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
	"github.com/facebookgo/httpcontrol"
	elastic6 "github.com/olivere/elastic"
	"gopkg.in/fsnotify.v1"
	elastic2 "gopkg.in/olivere/elastic.v3"
	elastic5 "gopkg.in/olivere/elastic.v5"
)

type bosunHttpTransport struct {
	UserAgent string
	http.RoundTripper
}

func (t *bosunHttpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Add("User-Agent", t.UserAgent)
	}
	req.Header.Add("X-Bosun-Server", util.Hostname)
	return t.RoundTripper.RoundTrip(req)
}

var startTime time.Time

func init() {
	startTime = time.Now().UTC()
	client := &http.Client{
		Transport: &bosunHttpTransport{
			"Bosun/" + version.ShortVersion(),
			&httpcontrol.Transport{
				Proxy:          http.ProxyFromEnvironment,
				RequestTimeout: time.Minute,
				MaxTries:       3,
			},
		},
	}
	http.DefaultClient = client
	opentsdb.DefaultClient = client
	graphite.DefaultClient = client
	collect.DefaultClient = &http.Client{
		Transport: &bosunHttpTransport{
			"Bosun/" + version.ShortVersion(),
			&httpcontrol.Transport{
				RequestTimeout: time.Minute,
			},
		},
	}
	sched.DefaultClient = &http.Client{
		Transport: &bosunHttpTransport{
			"Bosun/" + version.ShortVersion(),
			&httpcontrol.Transport{
				RequestTimeout: time.Second * 5,
			},
		},
	}
}

var (
	flagConf     = flag.String("c", "bosun.toml", "system config file location")
	flagTest     = flag.Bool("t", false, "test for valid config; exits with 0 on success, else 1")
	flagWatch    = flag.Bool("w", false, "watch .go files below current directory and exit; also build typescript files on change")
	flagReadonly = flag.Bool("r", false, "readonly-mode: don't write or relay any OpenTSDB metrics")
	flagQuiet    = flag.Bool("q", false, "quiet-mode: don't send any notifications except from the rule test page")
	flagNoChecks = flag.Bool("n", false, "no-checks: don't run the checks at the run interval")
	flagDev      = flag.Bool("dev", false, "enable dev mode: use local resources; no syslog")
	flagSkipLast = flag.Bool("skiplast", false, "skip loading last datapoints from and to redis: useful for speeding up bosun startup time during development")
	flagVersion  = flag.Bool("version", false, "Prints the version and exits")

	mains []func() // Used to hook up syslog on *nix systems
)

func main() {
	flag.Parse()
	if *flagVersion {
		fmt.Println(version.GetVersionInfo("bosun"))
		os.Exit(0)
	}
	for _, m := range mains {
		m()
	}
	systemConf, err := conf.LoadSystemConfigFile(*flagConf)
	if err != nil {
		slog.Fatalf("couldn't read system configuration: %v", err)
	}

	// Check if ES version is set by getting configs on start-up.
	// Because the current APIs don't return error so calling slog.Fatalf
	// inside these functions (for multiple-es support).
	systemConf.GetElasticContext()
	systemConf.GetAnnotateElasticHosts()

	sysProvider, err := systemConf.GetSystemConfProvider()
	if err != nil {
		slog.Fatal(err)
	}
	ruleConf, err := rule.ParseFile(sysProvider.GetRuleFilePath(), systemConf.EnabledBackends(), systemConf.GetRuleVars())
	if err != nil {
		slog.Fatalf("couldn't read rules: %v", err)
	}
	if *flagTest {
		os.Exit(0)
	}
	var ruleProvider conf.RuleConfProvider = ruleConf

	addrToSendTo := sysProvider.GetHTTPSListen()
	proto := "https"
	if addrToSendTo == "" {
		addrToSendTo = sysProvider.GetHTTPListen()
		proto = "http"
	}
	selfAddress := &url.URL{
		Scheme: proto,
		Host:   addrToSendTo,
	}
	if strings.HasPrefix(selfAddress.Host, ":") {
		selfAddress.Host = "localhost" + selfAddress.Host
	}

	da, err := initDataAccess(sysProvider)
	if err != nil {
		slog.Fatal(err)
	}
	if sysProvider.GetMaxRenderedTemplateAge() != 0 {
		go da.State().CleanupOldRenderedTemplates(time.Hour * 24 * time.Duration(sysProvider.GetMaxRenderedTemplateAge()))
	}
	var annotateBackend backend.Backend
	if sysProvider.AnnotateEnabled() {
		index := sysProvider.GetAnnotateIndex()
		if index == "" {
			index = "annotate"
		}
		config := sysProvider.GetAnnotateElasticHosts()
		switch config.Version {
		case expr.ESV2:
			annotateBackend = backend.NewElastic2([]string(config.Hosts), config.SimpleClient, index, config.ClientOptionFuncs.([]elastic2.ClientOptionFunc))
		case expr.ESV5:
			annotateBackend = backend.NewElastic5([]string(config.Hosts), config.SimpleClient, index, config.ClientOptionFuncs.([]elastic5.ClientOptionFunc))
		case expr.ESV6:
			annotateBackend = backend.NewElastic6([]string(config.Hosts), config.SimpleClient, index, config.ClientOptionFuncs.([]elastic6.ClientOptionFunc))
		}
		go func() {
			for {
				err := annotateBackend.InitBackend()
				if err == nil {
					return
				}
				slog.Warningf("could not initialize annotate backend, will try again: %v", err)
				time.Sleep(time.Second * 30)
			}
		}()
		web.AnnotateBackend = annotateBackend
	}
	if err := sched.Load(sysProvider, ruleProvider, da, annotateBackend, *flagSkipLast, *flagQuiet); err != nil {
		slog.Fatal(err)
	}
	if err := metadata.InitF(false, func(k metadata.Metakey, v interface{}) error { return sched.DefaultSched.PutMetadata(k, v) }); err != nil {
		slog.Fatal(err)
	}
	if sysProvider.GetTSDBHost() != "" {
		relay := web.Relay(sysProvider.GetTSDBHost())
		collect.DirectHandler = relay
		if err := collect.Init(selfAddress, "bosun"); err != nil {
			slog.Fatal(err)
		}
		tsdbHost := &url.URL{
			Scheme: "http",
			Host:   sysProvider.GetTSDBHost(),
		}
		if *flagReadonly {
			rp := util.NewSingleHostProxy(tsdbHost)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/put" {
					w.WriteHeader(204)
					return
				}
				rp.ServeHTTP(w, r)
			}))
			slog.Infoln("readonly relay at", ts.URL, "to", tsdbHost)
			tsdbHost, _ = url.Parse(ts.URL)
			sysProvider.SetTSDBHost(tsdbHost.Host)
		}
	}
	if systemConf.GetPing() {
		go ping.PingHosts(sched.DefaultSched.Search, systemConf.GetPingDuration())
	}
	if sysProvider.GetInternetProxy() != "" {
		web.InternetProxy, err = url.Parse(sysProvider.GetInternetProxy())
		if err != nil {
			slog.Fatalf("InternetProxy error: %s", err)
		}
	}
	var cmdHook conf.SaveHook
	if hookPath := sysProvider.GetCommandHookPath(); hookPath != "" {
		cmdHook, err = conf.MakeSaveCommandHook(hookPath)
		if err != nil {
			slog.Fatal(err)
		}
		ruleProvider.SetSaveHook(cmdHook)
	}
	var reload func() error
	reloading := make(chan bool, 1) // a lock that we can give up acquiring
	reload = func() error {
		select {
		case reloading <- true:
			// Got lock
		default:
			return fmt.Errorf("not reloading, reload in progress")
		}
		defer func() {
			<-reloading
		}()
		newConf, err := rule.ParseFile(sysProvider.GetRuleFilePath(), sysProvider.EnabledBackends(), sysProvider.GetRuleVars())
		if err != nil {
			return err
		}
		newConf.SetSaveHook(cmdHook)
		newConf.SetReload(reload)
		oldSched := sched.DefaultSched
		oldSearch := oldSched.Search
		sched.Close(true)
		sched.Reset()
		newSched := sched.DefaultSched
		newSched.Search = oldSearch
		slog.Infoln("schedule shutdown, loading new schedule")

		// Load does not set the DataAccess or Search if it is already set
		if err := sched.Load(sysProvider, newConf, da, annotateBackend, *flagSkipLast, *flagQuiet); err != nil {
			slog.Fatal(err)
		}
		web.ResetSchedule() // Signal web to point to the new DefaultSchedule
		go func() {
			slog.Infoln("running new schedule")
			if !*flagNoChecks {
				sched.Run()
			}
		}()
		slog.Infoln("config reload complete")
		return nil
	}

	ruleProvider.SetReload(reload)

	go func() {
		slog.Fatal(web.Listen(sysProvider.GetHTTPListen(), sysProvider.GetHTTPSListen(),
			sysProvider.GetTLSCertFile(), sysProvider.GetTLSKeyFile(), *flagDev,
			sysProvider.GetTSDBHost(), reload, sysProvider.GetAuthConf(), startTime))
	}()
	go func() {
		if !*flagNoChecks {
			sched.Run()
		}
	}()

	go func() {
		sc := make(chan os.Signal, 1)
		signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
		killing := false
		for range sc {
			if killing {
				slog.Infoln("Second interrupt: exiting")
				os.Exit(1)
			}
			killing = true
			go func() {
				slog.Infoln("Interrupt: closing down...")
				sched.Close(false)
				slog.Infoln("done")
				os.Exit(0)
			}()
		}
	}()

	if *flagWatch {
		watch(".", "*.go", quit)
		watch(filepath.Join("web", "static", "templates"), "*.html", web.RunEsc)
		base := filepath.Join("web", "static", "js")
		watch(base, "*.ts", web.RunTsc)
	}
	select {}
}

func quit() {
	slog.Error("Exiting")
	os.Exit(0)
}

func initDataAccess(systemConf conf.SystemConfProvider) (database.DataAccess, error) {
	var da database.DataAccess
	if systemConf.GetRedisHost() != "" {
		da = database.NewDataAccess(systemConf.GetRedisHost(), true, systemConf.GetRedisDb(), systemConf.GetRedisPassword())
	} else {
		_, err := database.StartLedis(systemConf.GetLedisDir(), systemConf.GetLedisBindAddr())
		if err != nil {
			return nil, err
		}
		da = database.NewDataAccess(systemConf.GetLedisBindAddr(), false, 0, "")
	}
	err := da.Migrate()
	return da, err
}

func watch(root, pattern string, f func()) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Fatal(err)
	}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if matched, err := filepath.Match(pattern, info.Name()); err != nil {
			slog.Fatal(err)
		} else if !matched {
			return nil
		}
		err = watcher.Add(path)
		if err != nil {
			slog.Fatal(err)
		}
		return nil
	})
	slog.Infoln("watching", pattern, "in", root)
	wait := time.Now()
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if wait.After(time.Now()) {
					continue
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					f()
					wait = time.Now().Add(time.Second * 2)
				}
			case err := <-watcher.Errors:
				slog.Errorln("error:", err)
			}
		}
	}()
}

package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"bosun.org/_third_party/gopkg.in/fsnotify.v1"
	"bosun.org/cmd/bosun/conf"
	"bosun.org/cmd/bosun/sched"
	"bosun.org/cmd/bosun/web"
	"bosun.org/collect"
	"bosun.org/metadata"
)

// These constants should remain in source control as their zero values.
const (
	// VersionDate should be set at build time as a date: 20140721184001.
	VersionDate uint64 = 0
	// VersionID should be set at build time as the most recent commit hash.
	VersionID string = ""
)

var (
	flagConf     = flag.String("c", "dev.conf", "config file location")
	flagTest     = flag.Bool("t", false, "test for valid config; exits with 0 on success, else 1")
	flagWatch    = flag.Bool("w", false, "watch .go files below current directory and exit; also build typescript files on change")
	flagReadonly = flag.Bool("r", false, "readonly-mode: don't write or relay any OpenTSDB metrics")
	flagQuiet    = flag.Bool("q", false, "quiet-mode: don't send any notifications except from the rule test page")
	flagNoChecks = flag.Bool("n", false, "no-checks: don't run the checks at the run interval")
	flagDev      = flag.Bool("dev", false, "enable dev mode: use local resources; no syslog")
	flagVersion  = flag.Bool("version", false, "Prints the version and exits")

	mains []func()
)

func main() {
	flag.Parse()
	if *flagVersion {
		fmt.Printf("bosun version %v (%v)\n", VersionDate, VersionID)
		os.Exit(0)
	}
	for _, m := range mains {
		m()
	}
	runtime.GOMAXPROCS(runtime.NumCPU())
	c, err := conf.ParseFile(*flagConf)
	if err != nil {
		log.Fatal(err)
	}
	if *flagTest {
		os.Exit(0)
	}
	httpListen := &url.URL{
		Scheme: "http",
		Host:   c.HTTPListen,
	}
	if strings.HasPrefix(httpListen.Host, ":") {
		httpListen.Host = "localhost" + httpListen.Host
	}
	if err := metadata.Init(httpListen, false); err != nil {
		log.Fatal(err)
	}
	if err := sched.Load(c); err != nil {
		log.Fatal(err)
	}
	if c.RelayListen != "" {
		go func() {
			mux := http.NewServeMux()
			mux.Handle("/api/", httputil.NewSingleHostReverseProxy(httpListen))
			s := &http.Server{
				Addr:    c.RelayListen,
				Handler: mux,
			}
			log.Fatal(s.ListenAndServe())
		}()
	}
	if c.TSDBHost != "" {
		if err := collect.Init(httpListen, "bosun"); err != nil {
			log.Fatal(err)
		}
		tsdbHost := &url.URL{
			Scheme: "http",
			Host:   c.TSDBHost,
		}
		if *flagReadonly {
			rp := httputil.NewSingleHostReverseProxy(tsdbHost)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/put" {
					w.WriteHeader(204)
					return
				}
				rp.ServeHTTP(w, r)
			}))
			log.Println("readonly relay at", ts.URL, "to", tsdbHost)
			tsdbHost, _ = url.Parse(ts.URL)
			c.TSDBHost = tsdbHost.Host
		}
	}
	if *flagQuiet {
		c.Quiet = true
	}
	go func() { log.Fatal(web.Listen(c.HTTPListen, *flagDev, c.TSDBHost)) }()
	go func() {
		if !*flagNoChecks {
			log.Fatal(sched.Run())
		}
	}()
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		log.Println("Interrupt: closing down...")
		sched.Close()
		log.Println("done")
		os.Exit(1)
	}()
	if *flagWatch {
		watch(".", "*.go", quit)
		watch(filepath.Join("web", "static", "templates"), "*.html", quit)
		base := filepath.Join("web", "static", "js")
		args := []string{
			"--out", filepath.Join(base, "bosun.js"),
		}
		matches, _ := filepath.Glob(filepath.Join(base, "*.ts"))
		sort.Strings(matches)
		args = append(args, matches...)
		tsc := run("tsc", args...)
		watch(base, "*.ts", tsc)
		go tsc()
	}
	select {}
}

func quit() {
	os.Exit(0)
}

func run(name string, arg ...string) func() {
	return func() {
		log.Println("running", name)
		c := exec.Command(name, arg...)
		stdout, err := c.StdoutPipe()
		if err != nil {
			log.Fatal(err)
		}
		stderr, err := c.StderrPipe()
		if err != nil {
			log.Fatal(err)
		}
		if err := c.Start(); err != nil {
			log.Fatal(err)
		}
		go func() { io.Copy(os.Stdout, stdout) }()
		go func() { io.Copy(os.Stderr, stderr) }()
		if err := c.Wait(); err != nil {
			log.Printf("run error: %v: %v", name, err)
		}
		log.Println("run complete:", name)
	}
}

func watch(root, pattern string, f func()) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if matched, err := filepath.Match(pattern, info.Name()); err != nil {
			log.Fatal(err)
		} else if !matched {
			return nil
		}
		err = watcher.Add(path)
		if err != nil {
			log.Fatal(err)
		}
		return nil
	})
	log.Println("watching", pattern, "in", root)
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
				log.Println("error:", err)
			}
		}
	}()
}

//go:generate esc -o web/static.go -pkg web -prefix web/static web/static/

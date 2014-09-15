package main

import (
	"flag"
	"io"
	"log"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/collect"
	"github.com/StackExchange/bosun/_third_party/gopkg.in/fsnotify.v1"
	"github.com/StackExchange/bosun/conf"
	"github.com/StackExchange/bosun/sched"
	"github.com/StackExchange/bosun/web"
)

var (
	flagConf  = flag.String("c", "dev.conf", "config file location")
	flagTest  = flag.Bool("t", false, "Only validate config then exit")
	flagWatch = flag.Bool("w", false, "watch .go files below current directory and exit; also build typescript files on change")
)

func main() {
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())
	c, err := conf.ParseFile(*flagConf)
	if err != nil {
		log.Fatal(err)
	}
	if *flagTest {
		log.Println("Valid Config")
		os.Exit(0)
	}
	if err := collect.Init(c.HttpListen, "bosun"); err != nil {
		log.Fatal(err)
	}
	sched.Load(c)
	tsdbHost := &url.URL{
		Scheme: "http",
		Host:   c.TsdbHost,
	}
	go func() { log.Fatal(web.Listen(c.HttpListen, c.WebDir, tsdbHost)) }()
	go func() { log.Fatal(sched.Run()) }()
	if *flagWatch {
		watch(".", "*.go", quit)
		base := filepath.Join("web", "static", "js")
		args := []string{
			"--out", filepath.Join(base, "bosun.js"),
		}
		matches, _ := filepath.Glob(filepath.Join(base, "*.ts"))
		sort.Strings(matches)
		args = append(args, matches...)
		tsc := run("tsc", args...)
		watch(base, "*.ts", tsc)
		tsc()
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

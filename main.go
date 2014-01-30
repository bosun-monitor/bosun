package main

import (
	"flag"
	"os"
	"path/filepath"
	"time"

	"github.com/StackExchange/slog"
	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/relay"
	"github.com/StackExchange/tsaf/sched"
	"github.com/StackExchange/tsaf/web"
	"github.com/howeyc/fsnotify"
)

var (
	confFile = flag.String("c", "dev.conf", "config file location")
	watch    = flag.Bool("w", false, "watch current directory and exit on changes; for use with an autorestarter")
)

func main() {
	flag.Parse()
	c, err := conf.ParseFile(*confFile)
	if err != nil {
		slog.Fatal(err)
	}
	sched.Load(c)
	go func() { slog.Fatal(relay.RelayHTTP(c.RelayListen, c.TsdbHost)) }()
	go func() { slog.Fatal(web.Listen(c.HttpListen, c.WebDir, c.TsdbHost)) }()
	go func() { slog.Fatal(sched.Run()) }()
	go watcher()
	select {}
}

func watcher() {
	if !*watch {
		return
	}
	time.Sleep(time.Second)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error(err)
		return
	}
	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				slog.Info("file changed, exiting:", ev)
				os.Exit(0)
			case err := <-watcher.Error:
				slog.Error(err)
			}
		}
	}()
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() || (len(path) > 1 && path[0] == '.') {
			return nil
		}
		watcher.Watch(path)
		return nil
	})
}

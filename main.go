package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/relay"
	"github.com/StackExchange/tsaf/sched"
	"github.com/StackExchange/tsaf/_third_party/github.com/howeyc/fsnotify"
	"github.com/StackExchange/tsaf/web"
)

var (
	confFile = flag.String("c", "dev.conf", "config file location")
	test     = flag.Bool("t", false, "Only validate config then exit")
	watch    = flag.Bool("w", false, "watch current directory and exit on changes; for use with an autorestarter")
)

func main() {
	flag.Parse()
	c, err := conf.ParseFile(*confFile)
	if err != nil {
		log.Fatal(err)
	}
	if *test {
		log.Println("Valid Config")
		os.Exit(0)
	}
	sched.Load(c)
	go func() { log.Fatal(relay.RelayHTTP(c.RelayListen, c.TsdbHost)) }()
	go func() { log.Fatal(web.Listen(c.HttpListen, c.WebDir, c.TsdbHost)) }()
	go func() { log.Fatal(sched.Run()) }()
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
		log.Print(err)
		return
	}
	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				log.Print("file changed, exiting:", ev)
				os.Exit(0)
			case err := <-watcher.Error:
				log.Print(err)
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

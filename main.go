package main

import (
	"flag"
	"log"
	"net"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/collect"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/slog"
	"github.com/StackExchange/bosun/_third_party/github.com/howeyc/fsnotify"
	"github.com/StackExchange/bosun/_third_party/github.com/tatsushid/go-fastping"
	"github.com/StackExchange/bosun/conf"
	"github.com/StackExchange/bosun/sched"
	"github.com/StackExchange/bosun/search"
	"github.com/StackExchange/bosun/web"
)

var (
	flagConf  = flag.String("c", "dev.conf", "config file location")
	flagTest  = flag.Bool("t", false, "Only validate config then exit")
	flagPing  = flag.Bool("p", true, "Pings known hosts")
	flagWatch = flag.Bool("w", false, "watch current directory and exit on changes; for use with an autorestarter")
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
	if *flagPing {
		go pingHosts()
	}
	if err := collect.Init(c.RelayListen, "bosun"); err != nil {
		log.Fatal(err)
	}
	sched.Load(c)
	go func() { log.Fatal(web.Listen(c.HttpListen, c.WebDir, c.TsdbHost, c.RelayListen)) }()
	go func() { log.Fatal(sched.Run()) }()
	go watcher()
	select {}
}

func watcher() {
	if !*flagWatch {
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

func pingHosts() {
	hostmap := make(map[string]bool)
	for {
		hosts := search.TagValuesByTagKey("host")
		for _, host := range hosts {
			if _, ok := hostmap[host]; !ok {
				hostmap[host] = true
				go pingHost(host)
			}
		}
		time.Sleep(time.Second * 15)
	}
}

func pingHost(host string) {
	for {
		p := fastping.NewPinger()
		ra, err := net.ResolveIPAddr("ip4:icmp", host)
		if err != nil {
			slog.Error(err)
		}
		p.AddIPAddr(ra)
		p.MaxRTT = time.Second * 5
		timeout := 1
		p.AddHandler("receive", func(addr *net.IPAddr, t time.Duration) {
			collect.Put("ping.rtt", opentsdb.TagSet{"dst_host": host}, float64(t)/float64(time.Millisecond))
			timeout = 0
		})
		if err := p.Run(); err != nil {
			slog.Error(err)
		}
		collect.Put("ping.timeout", opentsdb.TagSet{"dst_host": host}, float64(timeout))
		time.Sleep(time.Second * 15)
	}
}

package main

import (
	"flag"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/StackExchange/scollector/collect"
	"github.com/StackExchange/scollector/collectors"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/slog"
)

var (
	flagFilter = flag.String("f", "", "Filters collectors matching this term. Works with all other arguments.")
	flagTest   = flag.Bool("t", false, "Test - run collectors once, print, and exit.")
	flagList   = flag.Bool("l", false, "List")
	flagPrint  = flag.Bool("p", false, "Print to screen instead of sending to a host")
	host       = flag.String("h", "tsaf", `OpenTSDB host. Ex: "tsdb.example.com". Can optionally specify port: "tsdb.example.com:4000", but will default to 4242 otherwise`)
	colDir     = flag.String("c", "", `Passthrough collector directory. It should contain numbered directories like the OpenTSDB scollector expects. Any executable file in those directories is run every N seconds, where N is the name of the directory. Use 0 for a program that should be run continuously and simply pass data through to OpenTSDB (the program will be restarted if it exits. Data output format is: "metric timestamp value tag1=val1 tag2=val2 ...". Timestamp is in Unix format (seconds since epoch). Tags are optional. A host tag is automatically added, but overridden if specified.`)
	batchSize  = flag.Int("b", 0, "OpenTSDB batch size. Used for debugging bad data.")
	snmp       = flag.String("s", "", "SNMP host to poll of the format: \"community@host[,community@host...]\".")
	fake       = flag.Int("fake", 0, "Generates X fake data points on the test.fake metric per second.")
	debug      = flag.Bool("d", false, "Enables debug output.")

	mains []func()
)

func main() {
	flag.Parse()
	for _, m := range mains {
		m()
	}

	if *colDir != "" {
		collectors.InitPrograms(*colDir)
	}
	if *snmp != "" {
		for _, s := range strings.Split(*snmp, ",") {
			sp := strings.Split(s, "@")
			if len(sp) != 2 {
				slog.Fatal("invalid snmp string:", *snmp)
			}
			collectors.SNMPIfaces(sp[0], sp[1])
			collectors.SNMPCisco(sp[0], sp[1])
		}
	}
	if *fake > 0 {
		collectors.InitFake(*fake)
	}
	collect.Debug = *debug
	c := collectors.Search(*flagFilter)
	for _, col := range c {
		col.Init()
	}
	u := parseHost()
	if *flagTest {
		test(c)
		return
	} else if *flagList {
		list(c)
		return
	} else if *host != "" {
		if u == nil {
			slog.Fatal("invalid host:", *host)
		}
	}
	if *flagPrint {
		collectors.DefaultFreq = time.Second * 3
		slog.Infoln("Set default frequency to", collectors.DefaultFreq)
	}
	cdp := collectors.Run(c)
	if u != nil && !*flagPrint {
		slog.Infoln("OpenTSDB host:", u)
		if err := collect.InitChan(u.Host, "scollector", cdp); err != nil {
			slog.Fatal(err)
		}
		if *batchSize > 0 {
			collect.BatchSize = *batchSize
		}
		go func() {
			const maxMem = 500 * 1024 * 1024 // 500MB
			var m runtime.MemStats
			for _ = range time.Tick(time.Minute) {
				runtime.ReadMemStats(&m)
				if m.Alloc > maxMem {
					panic("memory max reached")
				}
			}
		}()
	} else {
		slog.Infoln("Outputting to screen")
		printPut(cdp)
	}
	select {}
}

func test(cs []collectors.Collector) {
	dpchan := make(chan *opentsdb.DataPoint)
	for _, c := range cs {
		go c.Run(dpchan)
		slog.Infoln("run", c.Name())
	}
	dur := time.Second * 10
	slog.Infoln("running for", dur)
	next := time.After(dur)
Loop:
	for {
		select {
		case dp := <-dpchan:
			slog.Info(dp.Telnet())
		case <-next:
			break Loop
		}
	}
}

func list(cs []collectors.Collector) {
	for _, c := range cs {
		slog.Infoln(c.Name())
	}
}

func parseHost() *url.URL {
	if *host == "" {
		return nil
	}
	u := url.URL{
		Scheme: "http",
		Path:   "/api/put",
	}
	if !strings.Contains(*host, ":") {
		*host += ":4242"
	}
	u.Host = *host
	return &u
}

func printPut(c chan *opentsdb.DataPoint) {
	for dp := range c {
		slog.Info(dp.Telnet())
	}
}

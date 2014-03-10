package main

import (
	"flag"
	"net/url"
	"strings"
	"time"

	"github.com/StackExchange/scollector/collectors"
	"github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/scollector/queue"
	"github.com/StackExchange/slog"
)

var flagFilter = flag.String("f", "", "Filters collectors matching this term. Works with all other arguments.")
var flagTest = flag.Bool("t", false, "Test - run collectors once, print, and exit.")
var flagList = flag.Bool("l", false, "List")
var flagPrint = flag.Bool("p", false, "Print to screen instead of sending to a host")
var host = flag.String("h", "tsa1", `OpenTSDB host. Ex: "tsdb.example.com". Can optionally specify port: "tsdb.example.com:4000", but will default to 4242 otherwise`)
var colDir = flag.String("c", "", `Passthrough collector directory. It should contain numbered directories like the OpenTSDB scollector expects. Any executable file in those directories is run every N seconds, where N is the name of the directory. Use 0 for a program that should be run continuously and simply pass data through to OpenTSDB (the program will be restarted if it exits. Data output format is: "metric timestamp value tag1=val1 tag2=val2 ...". Timestamp is in Unix format (seconds since epoch). Tags are optional. A host tag is automatically added, but overridden if specified.`)
var batchSize = flag.Int("b", 0, "OpenTSDB batch size. Used for debugging bad data.")

var mains []func()

func main() {
	flag.Parse()
	for _, m := range mains {
		m()
	}

	if *colDir != "" {
		collectors.InitPrograms(*colDir)
	}
	if *batchSize > 0 {
		queue.BatchSize = *batchSize
	}
	c := collectors.Search(*flagFilter)
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
		collectors.DEFAULT_FREQ = time.Second * 3
		slog.Infoln("Set default frequency to", collectors.DEFAULT_FREQ)
	}
	cdp := collectors.Run(c)
	if u != nil && !*flagPrint {
		slog.Infoln("OpenTSDB host:", u)
		queue.New(u.String(), cdp)
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
	next := time.After(time.Second * 2)
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

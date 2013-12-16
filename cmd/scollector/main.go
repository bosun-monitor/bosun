package main

import (
	"flag"
	"log"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/StackExchange/tcollector/collectors"
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/tcollector/queue"
)

var l = log.New(os.Stdout, "", log.LstdFlags)

var flagFilter = flag.String("f", "", "Filters collectors matching this term. Works with all other arguments.")
var flagTest = flag.Bool("t", false, "Test - run collectors once, print, and exit.")
var flagList = flag.Bool("l", false, "List")
var host = flag.String("h", "", `OpenTSDB host. Ex: "tsdb.example.com". Can optionally specify port: "tsdb.example.com:4000", but will default to 4242 otherwise. If not specified, will print to screen`)

func main() {
	flag.Parse()
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
			l.Fatal("invalid host:", *host)
		}
	}

	if u == nil {
		collectors.DEFAULT_FREQ = time.Second * 3
		l.Println("Set default frequency to", collectors.DEFAULT_FREQ)
	}
	cdp := collectors.Run(c)
	if u != nil {
		l.Println("OpenTSDB host:", u)
		queue.New(u.String(), cdp)
	} else {
		l.Println("Outputting to screen")
		printPut(cdp)
	}
	select {}
}

func test(cs []collectors.Collector) {
	dpchan := make(chan *opentsdb.DataPoint)
	for _, c := range cs {
		go c.Run(dpchan)
		l.Println("run", c.Name())
	}
	next := time.After(time.Second * 2)
Loop:
	for {
		select {
		case dp := <-dpchan:
			l.Print(dp.Telnet())
		case <-next:
			break Loop
		}
	}
}

func list(cs []collectors.Collector) {
	for _, c := range cs {
		l.Println(c.Name())
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
		l.Print(dp.Telnet())
	}
}

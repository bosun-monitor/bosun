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
var colDir = flag.String("c", "", `Passthrough collector directory. It should contain numbered directories like the OpenTSDB tcollector expects. Any executable file in those directories is run every N seconds, where N is the name of the directory. Use 0 for a program that should be run continuously and simply pass data through to OpenTSDB (the program will be restarted if it exits. Data output format is: "metric timestamp value tag1=val1 tag2=val2 ...". Timestamp is in Unix format (seconds since epoch). Tags are optional. A host tag is automatically added, but overridden if specified.`)
var batchSize = flag.Int("b", 0, "OpenTSDB batch size. Used for debugging bad data.")

func main() {
	flag.Parse()
	if *colDir != "" {
		collectors.InitPrograms(*colDir)
	}
	if *batchSize > 0 {
		queue.MAX_PERSEC = *batchSize
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

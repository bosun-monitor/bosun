package main

import (
	"flag"
	"log"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/StackExchange/tcollector/collectors"
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/tcollector/queue"
)

var l = log.New(os.Stdout, "", log.LstdFlags)

var flagTest = flag.String("test", "", "Test collector matching pattern")
var flagList = flag.Bool("list", false, "List available collectors")
var host = flag.String("host", "", `OpenTSDB host. Ex: "tsdb.example.com". Can optionally specify port: "tsdb.example.com:4000", but will default to 4242 otherwise. If not specified, will print to screen`)

func main() {
	flag.Parse()
	u := parseHost()
	if *flagTest != "" {
		test(*flagTest)
		return
	} else if *flagList {
		list()
		return
	} else if *host != "" {
		if u == nil {
			l.Fatal("invalid host:", *host)
		}
	}

	cdp := collectors.Run()
	if u != nil {
		l.Println("OpenTSDB host:", u)
		queue.New(u.String(), cdp)
	} else {
		l.Println("Outputting to screen")
		printPut(cdp)
	}
	select {}
}

func test(s string) {
	for _, c := range collectors.Search(s) {
		md := c()
		for _, d := range md {
			l.Print(d.Telnet())
		}
	}
}

func list() {
	for _, c := range collectors.Search("") {
		v := runtime.FuncForPC(reflect.ValueOf(c).Pointer())
		l.Println(v.Name())
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

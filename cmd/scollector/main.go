package main

import (
	"flag"
	"fmt"
	"net/url"
	"reflect"
	"runtime"
	"strings"

	"github.com/StackExchange/tcollector/collectors"
	"github.com/StackExchange/tcollector/queue"
)

var flagTest = flag.String("test", "", "Test collector matching pattern")
var flagList = flag.Bool("list", false, "List available collectors")
var host = flag.String("host", "", `Required. OpenTSDB host. Ex: "tsdb.example.com". Can optionally specify port: "tsdb.example.com:4000", but will default to 4242 otherwise.`)

func main() {
	flag.Parse()
	if *flagTest != "" {
		test(*flagTest)
		return
	} else if *flagList {
		list()
		return
	} else if *host == "" {
		fmt.Println("missing -host flag")
		return
	}
	u := parseHost()
	if u == nil {
		fmt.Println("invalid host:", *host)
		return
	}
	fmt.Println("OpenTSDB host:", u)

	cdp := collectors.Run()
	queue.New(u.String(), cdp)
	select {}
}

func test(s string) {
	for _, c := range collectors.Search(s) {
		md := c()
		for _, d := range md {
			fmt.Print(d.Telnet())
		}
	}
}

func list() {
	for _, c := range collectors.Search("") {
		v := runtime.FuncForPC(reflect.ValueOf(c).Pointer())
		fmt.Println(v.Name())
	}
}

func parseHost() *url.URL {
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

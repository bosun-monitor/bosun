package main

import (
	"flag"
	"fmt"
	"reflect"
	"runtime"

	"github.com/StackExchange/tcollector/collectors"
	"github.com/StackExchange/tcollector/opentsdb"
)

var queue []*opentsdb.DataPoint

var flagTest = flag.String("test", "", "Test collector matching pattern")
var flagList = flag.Bool("list", false, "List available collectors")

func main() {
	flag.Parse()
	if *flagTest != "" {
		test(*flagTest)
		return
	} else if *flagList {
		list()
		return
	}
	return

	cdp := collectors.Run()
	go fetch(cdp)
	select {}
}

func fetch(c chan *opentsdb.DataPoint) {
	for dp := range c {
		queue = append(queue, dp)
		fmt.Print(dp.Telnet())
	}
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

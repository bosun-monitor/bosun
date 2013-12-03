package main

import (
	"fmt"

	"github.com/StackExchange/tcollector/collectors"
	"github.com/StackExchange/tcollector/opentsdb"
)

var queue []*opentsdb.DataPoint

func main() {
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

package main

import (
	"log"

	"github.com/StackExchange/tsaf/opentsdb"
	"github.com/StackExchange/tsaf/relay"
	"github.com/StackExchange/tsaf/search"
)

func main() {
	log.Println("running")
	send := relay.TSDBSend("http://ny-devtsdb02.ds.stackexchange.com:4242/api/put")
	dc := make(chan *opentsdb.DataPoint)
	go search.Process(dc)
	extract := search.Extract(dc)
	log.Fatal(relay.Listen(":4241", send, extract))
}

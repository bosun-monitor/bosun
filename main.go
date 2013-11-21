package main

import (
	"log"

	"github.com/StackExchange/tsaf/opentsdb"
	"github.com/StackExchange/tsaf/relay"
	"github.com/StackExchange/tsaf/search"
	"github.com/StackExchange/tsaf/web"
)

var (
	TSDBHost    = "ny-devtsdb02.ds.stackexchange.com:4242"
	RelayListen = ":4242"
	WebListen   = ":8080"
	WebDir      = "web/"

	TSDBHttp = "http://" + TSDBHost + "/"
)

func main() {
	log.Println("running")
	go func() {
		dc := make(chan *opentsdb.DataPoint)
		go search.Process(dc)
		send := relay.TSDBSend(TSDBHttp)
		extract := search.Extract(dc)
		log.Fatal(relay.Listen(RelayListen, send, extract))
	}()
	go log.Fatal(web.Listen(WebListen, WebDir, TSDBHttp))
	select {}
}

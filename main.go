package main

import (
	"log"
	"os"
	"strings"

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

func init() {
	if host, err := os.Hostname(); err == nil && strings.HasPrefix(host, "ny-devtsaf") {
		WebListen = ":80"
	}
}

func main() {
	log.Println("running")
	/*
		go func() {
			send := relay.TSDBSendHTTP(TSDBHttp)
			extract := search.ExtractHTTP()
			log.Fatal(relay.ListenHTTP(RelayListen, send, extract))
		}()
	*/
	go func() {
		send := relay.TSDBSendUDP(TSDBHost)
		extract := search.ExtractTCP()
		log.Fatal(relay.ListenTCP(RelayListen, send, extract))
	}()
	go log.Fatal(web.Listen(WebListen, WebDir, TSDBHttp))
	select {}
}

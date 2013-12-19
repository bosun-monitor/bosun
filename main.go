package main

import (
	"log"
	"os"
	"strings"

	"github.com/StackExchange/tsaf/relay"
	"github.com/StackExchange/tsaf/web"
)

var (
	TSDBHost    = "ny-devtsdb04.ds.stackexchange.com:4242"
	RelayListen = ":4242"
	WebListen   = ":8070"
	WebDir      = "web"

	TSDBHttp = "http://" + TSDBHost + "/"
)

func init() {
	if host, err := os.Hostname(); err == nil && strings.HasPrefix(host, "ny-devtsaf") {
		WebListen = ":80"
	}
}

func main() {
	log.Println("running")
	go func() { log.Fatal(relay.RelayHTTP(RelayListen, TSDBHost)) }()
	go func() { log.Fatal(web.Listen(WebListen, WebDir, TSDBHttp)) }()
	select {}
}

package main

import (
	"flag"
	"log"

	"github.com/StackExchange/tsaf/conf"
	"github.com/StackExchange/tsaf/relay"
	"github.com/StackExchange/tsaf/web"
)

var (
	confFile = flag.String("c", "dev.conf", "config file location")
)

func main() {
	flag.Parse()
	c, err := conf.ParseFile(*confFile)
	if err != nil {
		log.Fatal(err)
	}
	webDir := c.Global.Get("webDir", "web")
	webListen := c.Global.Get("webListen", ":8070")
	relayListen := c.Global.Get("relayListen", ":4242")
	tsdbHost := c.Global["tsdbHost"]
	if tsdbHost == "" {
		log.Fatal("tsaf: no tsdbHost in config file")
	}
	tsdbHttp := "http://" + tsdbHost + "/"
	go func() { log.Fatal(relay.RelayHTTP(relayListen, tsdbHost)) }()
	go func() { log.Fatal(web.Listen(webListen, webDir, tsdbHttp)) }()
	select {}
}

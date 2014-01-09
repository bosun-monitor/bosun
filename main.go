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
	tsdbHttp := "http://" + c.TsdbHost + "/"
	go func() { log.Fatal(relay.RelayHTTP(c.RelayListen, c.TsdbHost)) }()
	go func() { log.Fatal(web.Listen(c.HttpListen, c.WebDir, tsdbHttp)) }()
	select {}
}

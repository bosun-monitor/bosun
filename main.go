package main

import (
	"log"

	"github.com/StackExchange/tsaf/relay"
)

func main() {
	log.Println("running")
	send := relay.TSDBSend("http://ny-devtsdb02.ds.stackexchange.com:4242/api/put")
	log.Fatal(relay.Listen(":4241", send))
}

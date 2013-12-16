package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/StackExchange/tsaf/opentsdb"
)

//const host = "localhost:4242"

const host = "ny-devtsaf01.ds.stackexchange.com:4242"

func main() {
	put := func(d time.Duration, name string, useJson bool) {
		i := 0
		for t := range time.Tick(d) {
			dp := opentsdb.DataPoint{
				"cpu.test",
				t.UTC().Unix(),
				i % 20,
				map[string]string{"host": name},
			}
			if useJson {
				resp, err := http.Post("http://"+host+"/api/put", "application/json", dp.Json())
				if err != nil {
					log.Println("ERROR", name, t, i, err)
				} else {
					log.Println(name, t, i, resp.Status)
					resp.Body.Close()
				}
			} else {
				conn, err := net.Dial("tcp", host)
				if err != nil {
					log.Println("conn error", err)
					continue
				}
				fmt.Fprintf(conn, dp.Telnet())
				go func(t time.Time, i int) {
					conn.SetDeadline(time.Now().Add(time.Second))
					buf := make([]byte, 1024)
					if n, err := conn.Read(buf); n > 0 {
						log.Println(string(buf))
					} else if err != nil {
						if e, ok := err.(net.Error); !ok || !e.Timeout() {
							log.Println("tcp conn err", err)
						} else {
							log.Println(name, t, i, "TELNET1 SENT")
						}
					} else {
						log.Println(name, t, i, "TELNET2 SENT")
					}
					conn.Close()
				}(t, i)
			}
			i++
		}
	}
	go put(time.Second*2, "2-s", false)
	go put(time.Second*3, "3-s", false)
	select {}
}

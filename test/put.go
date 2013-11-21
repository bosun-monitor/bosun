package main

import (
	"log"
	"net/http"
	"time"

	"github.com/StackExchange/tsaf/opentsdb"
)

func main() {
	put := func(d time.Duration, name string) {
		i := 0
		for t := range time.Tick(d) {
			dp := opentsdb.DataPoint{
				"cpu.test",
				t.Unix(),
				i,
				map[string]string{"host": name},
			}
			resp, err := http.Post("http://localhost:4241/api/put", "application/json", dp.Json())
			if err != nil {
				log.Println("ERROR", name, t, i, err)
			} else {
				log.Println(name, t, i, resp.Status)
				resp.Body.Close()
			}
			i++
		}
	}
	go put(time.Second*2, "2s")
	go put(time.Second*3, "3s")
	select {}
}

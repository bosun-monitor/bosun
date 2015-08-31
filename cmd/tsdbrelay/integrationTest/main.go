/*
We are going to simulate a full multi-datacenter cluster:
DC1:
	- tsdbrelay:	:5555
	- opentsdb:		:5556
	- bosun: 		:5557
DC2:
	- tsdbrelay		:6555
	- opentsdb 		:6556
	- bosun:		:6557
*/
package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"bosun.org/cmd/bosun/expr"
	"bosun.org/opentsdb"
)

var (
	relay1, relay2 *os.Process
)

func init() {
	cmd := exec.Command("tsdbrelay", "-l=:5555", "-b=localhost:5557", "-r=localhost:6555", "-t=localhost:5556", "-denormalize=os.cpu__host")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		fatal(err)
	}
	relay1 = cmd.Process
	cmd = exec.Command("tsdbrelay", "-l=:6555", "-b=localhost:6557", "-r=localhost:5555", "-t=localhost:6556", "-denormalize=os.cpu__host")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		fatal(err)
	}
	relay2 = cmd.Process
	dc1BosunMux := http.NewServeMux()
	dc1BosunMux.HandleFunc("/api/index", handleBosun(dc1BosunReceived))
	go func() {
		fatal("DC1-Bosun", http.ListenAndServe(":5557", dc1BosunMux))
	}()

	dc2BosunMux := http.NewServeMux()
	dc2BosunMux.HandleFunc("/api/index", handleBosun(dc2BosunReceived))
	go func() {
		fatal("DC2-Bosun", http.ListenAndServe(":6557", dc2BosunMux))
	}()

	dc1TsdbMux := http.NewServeMux()
	dc1TsdbMux.HandleFunc("/api/put", handleTsdb(dc1TsdbReceived))

	go func() {
		fatal("DC1-TSDB", http.ListenAndServe(":5556", dc1TsdbMux))
	}()

	dc2TsdbMux := http.NewServeMux()
	dc2TsdbMux.HandleFunc("/api/put", handleTsdb(dc2TsdbReceived))
	go func() {
		fatal("DC3-TSDB", http.ListenAndServe(":6556", dc2TsdbMux))
	}()

	ch := make(chan os.Signal)
	signal.Notify(ch, os.Kill, os.Interrupt)
	go func() {
		<-ch
		fatal("INTERRUPT RECIEVED")
	}()
	time.Sleep(2 * time.Second)
}

const (
	localRelayUrl  = "http://localhost:5555/api/put"
	remoteRelayUrl = "http://localhost:6555/api/put"
)

var (
	dc1BosunReceived = map[expr.AlertKey]int{}
	dc2BosunReceived = map[expr.AlertKey]int{}
	dc1TsdbReceived  = map[expr.AlertKey]int{}
	dc2TsdbReceived  = map[expr.AlertKey]int{}
)

func main() {
	// 1. Single data point to local relay
	dp1 := &opentsdb.DataPoint{Metric: "abc", Tags: opentsdb.TagSet{"host": "h1"}}
	buf := encodeMdp([]*opentsdb.DataPoint{dp1})
	resp, err := http.Post(localRelayUrl, "application/json", buf)
	if err != nil {
		fatal(err)
	}
	if resp.StatusCode != 204 {
		fatal("response code not relayed")
	}
	time.Sleep(1 * time.Second)
	check("Bosun DC1", "abc{host=h1}", 1, dc1BosunReceived)
	check("Bosun DC2", "abc{host=h1}", 1, dc2BosunReceived)
	check("Tsdb DC1", "abc{host=h1}", 1, dc1TsdbReceived)
	check("Tsdb DC2", "abc{host=h1}", 1, dc2TsdbReceived)
	log.Println("test 1 ok")
	// 2. Single data point to remote relay. Should be denormalized.
	dp2 := &opentsdb.DataPoint{Metric: "os.cpu", Tags: opentsdb.TagSet{"host": "h1"}}
	buf = encodeMdp([]*opentsdb.DataPoint{dp2})
	resp, err = http.Post(remoteRelayUrl, "application/json", buf)
	if err != nil {
		fatal(err)
	}
	if resp.StatusCode != 204 {
		fatal("response code not relayed")
	}
	time.Sleep(1 * time.Second)
	check("Bosun DC1", "os.cpu{host=h1}", 1, dc1BosunReceived)
	check("Bosun DC2", "os.cpu{host=h1}", 1, dc2BosunReceived)
	check("Tsdb DC1", "os.cpu{host=h1}", 1, dc1TsdbReceived)
	check("Tsdb DC2", "os.cpu{host=h1}", 1, dc2TsdbReceived)
	check("Bosun DC1", "__h1.os.cpu{host=h1}", 1, dc1BosunReceived)
	check("Bosun DC2", "__h1.os.cpu{host=h1}", 1, dc2BosunReceived)
	check("Tsdb DC1", "__h1.os.cpu{host=h1}", 1, dc1TsdbReceived)
	check("Tsdb DC2", "__h1.os.cpu{host=h1}", 1, dc2TsdbReceived)
	log.Println("test 2 ok")
	killAll()
}
func check(node string, ak expr.AlertKey, expected int, data map[expr.AlertKey]int) {
	if data[ak] != expected {
		msg := fmt.Sprintf("Expected %s to see %s %d times, but saw %d.", node, ak, expected, data[ak])
		fatal(msg)
	}
}
func handleBosun(data map[expr.AlertKey]int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		readDps(r.Body, data)
		w.WriteHeader(500)
	}
}

func handleTsdb(data map[expr.AlertKey]int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		readDps(r.Body, data)
		w.WriteHeader(204)
	}
}

func killAll() {
	if relay1 != nil {
		log.Println("Killing relay 1:", relay1.Kill())
	}
	if relay2 != nil {
		log.Println("Killing relay 2:", relay2.Kill())
	}
}
func readDps(r io.Reader, data map[expr.AlertKey]int) {
	gr, err := gzip.NewReader(r)
	if err != nil {
		fatal(err)
	}
	jr := json.NewDecoder(gr)
	mdp := []*opentsdb.DataPoint{}
	err = jr.Decode(&mdp)
	if err != nil {
		fatal(err)
	}
	for _, dp := range mdp {
		ak := expr.NewAlertKey(dp.Metric, dp.Tags)
		n, ok := data[ak]
		if ok {
			data[ak] = n + 1
		} else {
			data[ak] = 1
		}
	}
}

func encodeMdp(mdp []*opentsdb.DataPoint) io.Reader {
	buf := &bytes.Buffer{}
	gw := gzip.NewWriter(buf)
	jw := json.NewEncoder(gw)
	jw.Encode(mdp)
	gw.Close()
	return buf
}

func fatal(i ...interface{}) {
	log.Println(i...)
	killAll()
	log.Fatal("")
}

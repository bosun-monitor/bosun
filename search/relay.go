package search

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/collect"
	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
)

func RelayHTTP(addr, dest string, metaHandler http.Handler) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", Handle(dest, metaHandler))
	log.Println("OpenTSDB relay listening on:", addr)
	log.Println("OpenTSDB destination:", dest)
	go func() { log.Fatal(http.ListenAndServe(addr, mux)) }()
}

var client = &http.Client{
	Timeout: time.Minute,
}

func Handle(dest string, metaHandler http.Handler) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/metadata/put" {
			metaHandler.ServeHTTP(w, r)
			return
		}
		orig, _ := ioutil.ReadAll(r.Body)
		if r.URL.Path == "/api/put" {
			var body []byte
			if r, err := gzip.NewReader(bytes.NewReader(orig)); err == nil {
				body, _ = ioutil.ReadAll(r)
				r.Close()
			} else {
				body = orig
			}
			var dp opentsdb.DataPoint
			var mdp opentsdb.MultiDataPoint
			if err := json.Unmarshal(body, &mdp); err == nil {
			} else if err = json.Unmarshal(body, &dp); err == nil {
				mdp = opentsdb.MultiDataPoint{&dp}
			}
			if len(mdp) > 0 {
				HTTPExtract(mdp)
			}
		}
		durl := url.URL{
			Scheme: "http",
			Host:   dest,
		}
		durl.Path = r.URL.Path
		durl.RawQuery = r.URL.RawQuery
		durl.Fragment = r.URL.Fragment
		req, err := http.NewRequest(r.Method, durl.String(), bytes.NewReader(orig))
		if err != nil {
			log.Println("relay NewRequest err:", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		req.Header = r.Header
		req.TransferEncoding = r.TransferEncoding
		req.ContentLength = r.ContentLength
		resp, err := client.Do(req)
		if err != nil {
			log.Println("relay Do err:", err)
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		b, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 500 {
			collect.Add("relay.err", opentsdb.TagSet{"path": r.URL.Path, "dest": r.URL.Host}, 1)
		}
		w.WriteHeader(resp.StatusCode)
		w.Write(b)
	}
}

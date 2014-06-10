package relay

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
	"github.com/StackExchange/bosun/_third_party/github.com/mreiferson/go-httpclient"
	"github.com/StackExchange/bosun/search"
)

func RelayHTTP(addr, dest string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", Handle(dest))
	log.Println("OpenTSDB relay listening on:", addr)
	log.Println("OpenTSDB destination:", dest)
	return http.ListenAndServe(addr, mux)
}

var client = &http.Client{
	Transport: &httpclient.Transport{
		RequestTimeout: time.Minute,
	},
}

func Handle(dest string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		orig, _ := ioutil.ReadAll(r.Body)
		if r.URL.Path == "/api/put" {
			var reader io.Reader = bytes.NewReader(orig)
			if r, err := gzip.NewReader(reader); err == nil {
				reader = r
				defer r.Close()
			}
			body, _ := ioutil.ReadAll(reader)
			var dp opentsdb.DataPoint
			var mdp opentsdb.MultiDataPoint
			if err := json.Unmarshal(body, &mdp); err == nil {
			} else if err = json.Unmarshal(body, &dp); err == nil {
				mdp = opentsdb.MultiDataPoint{&dp}
			}
			if len(mdp) > 0 {
				search.HTTPExtract(mdp)
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
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		req.Header = r.Header
		req.TransferEncoding = r.TransferEncoding
		req.ContentLength = r.ContentLength
		resp, err := client.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		b, _ := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		w.WriteHeader(resp.StatusCode)
		w.Write(b)
	}
}

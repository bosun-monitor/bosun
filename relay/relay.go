package relay

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/StackExchange/tsaf/search"
	"github.com/StackExchange/tsaf/_third_party/github.com/mreiferson/go-httpclient"
)

func RelayHTTP(addr, dest string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handle(dest, w, r)
	})
	log.Println("OpenTSDB relay listening on:", addr)
	log.Println("OpenTSDB destination:", dest)
	return http.ListenAndServe(addr, mux)
}

var client = &http.Client{
	Transport: &httpclient.Transport{
		RequestTimeout: time.Minute,
	},
}

func handle(dest string, w http.ResponseWriter, r *http.Request) {
	body, _ := ioutil.ReadAll(r.Body)
	search.HTTPExtract(body)
	durl := url.URL{
		Scheme: "http",
		Host:   dest,
	}
	durl.Path = r.URL.Path
	durl.RawQuery = r.URL.RawQuery
	durl.Fragment = r.URL.Fragment
	req, err := http.NewRequest(r.Method, durl.String(), bytes.NewReader(body))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		log.Println(err)
		return
	}
	req.Header = r.Header
	req.TransferEncoding = append(req.TransferEncoding, "identity")
	req.ContentLength = int64(len(body))
	resp, err := client.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		//log.Println(err)
		return
	}
	b, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	w.WriteHeader(resp.StatusCode)
	w.Write(b)
}

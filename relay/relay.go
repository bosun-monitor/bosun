package relay

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

func ListenHTTP(addr string, relays ...func(*http.Request, []byte) error) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("req")
		body, _ := ioutil.ReadAll(r.Body)
		r.Body.Close()
		for i, relay := range relays {
			log.Println("send to relay", i)
			if err := relay(r, body); err != nil {
				log.Println("relay error", i, err)
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
		}
	})
	log.Println(len(relays), "relays listening on", addr)
	return http.ListenAndServe(addr, mux)
}

func TSDBSendHTTP(dest string) func(*http.Request, []byte) error {
	return func(r *http.Request, body []byte) error {
		durl, err := url.Parse(dest)
		if err != nil {
			return err
		}
		durl.Path = r.URL.Path
		durl.RawQuery = r.URL.RawQuery
		durl.Fragment = r.URL.Fragment
		req, err := http.NewRequest(r.Method, durl.String(), bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header = r.Header
		req.TransferEncoding = append(req.TransferEncoding, "identity")
		req.ContentLength = int64(len(body))
		c := new(http.Client)
		resp, err := c.Do(req)
		if err != nil {
			return err
		} else if resp.StatusCode >= 300 {
			err := fmt.Errorf("relay: Bad Send response: %v", resp.Status)
			b, _ := ioutil.ReadAll(resp.Body)
			log.Println("error", err, string(b))
			return err
		}
		log.Println("relayed", resp.Status)
		return nil
	}
}

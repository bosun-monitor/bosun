package relay

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

func Listen(addr string, relays ...func(*http.Request, []byte) error) error {
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
	log.Println("begin listen", len(relays))
	return http.ListenAndServe(addr, mux)
}

func TSDBSend(dest string) func(*http.Request, []byte) error {
	return func(r *http.Request, body []byte) error {
		req, err := http.NewRequest(r.Method, dest, bytes.NewReader(body))
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

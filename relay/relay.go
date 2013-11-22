package relay

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"
)

func ListenTCP(addr string, relays ...func([]byte) string) error {
	ln, err := net.Listen("tcp", addr)
	log.Println("tcp listen on", addr)
	if err != nil {
		return err
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Println("listen tcp accept error", err)
			continue
		}
		go func(conn net.Conn) {
			body, _ := ioutil.ReadAll(conn)
			log.Println("body", string(body))
			for i, relay := range relays {
				log.Println("relay", i)
				if res := relay(body); res != "" {
					log.Println("tcp relay err", i, err)
					conn.SetDeadline(time.Now().Add(time.Second * 5))
					conn.Write([]byte(res))
					break
				}
			}
			if err := conn.Close(); err != nil {
				log.Println("conn close err", err)
			}
		}(conn)
	}
}

func TSDBSendUDP(dest string) func([]byte) string {
	return func(body []byte) string {
		conn, err := net.Dial("tcp", dest)
		if err != nil {
			return err.Error()
		}
		conn.Write(body)
		conn.SetDeadline(time.Now().Add(time.Second * 5))
		status, err := bufio.NewReader(conn).ReadString('\n')
		if e, ok := err.(net.Error); ok && e.Timeout() {
			// no-op
		} else if err != nil {
			return err.Error()
		} else {
			return status
		}
		return ""
	}
}

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

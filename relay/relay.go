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

	"github.com/StackExchange/tsaf/search"
)

func RelayTCP(listen, dest string) error {
	ln, err := net.Listen("tcp", listen)
	log.Println("tcp listen on", listen)
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
			log.Println("new conn", conn.LocalAddr(), conn.RemoteAddr())
			conn.SetDeadline(time.Now().Add(time.Second))
			cb := bufio.NewReader(conn)
			for {
				bt, err := cb.ReadBytes('\n')
				if len(bt) > 0 {
					log.Println("tcp read", string(bt))
					search.TCPExtract(bt)

					dc, err := net.Dial("tcp", dest)
					if err != nil {
						log.Println("dc err", err)
						continue
					}
					dc.SetDeadline(time.Now().Add(time.Second))
					if n, err := dc.Write(bt); err != nil {
						log.Println("dc write err", err)
						continue
					} else {
						log.Println("wrote bytes", n, err)
					}
					br := bufio.NewReader(dc)
					for {
						log.Println("read bytes dc", dest)
						t, err := br.ReadBytes('\n')
						log.Println("read", string(t), err)
						if len(t) > 0 {
							log.Println("br read", string(t))
							if _, err := conn.Write(t); err != nil {
								log.Println("conn write err", err)
							}
						}
						if err != nil {
							log.Println("br err", err)
							break
						}
					}
					dc.Close()
					log.Println("dc close")
				}
				if err != nil {
					log.Println("bt err", err)
					break
				}
			}
			conn.Close()
			log.Println("conn close")
		}(conn)
	}
}

type TCPRelay func(net.Conn, []byte) error

func ListenTCP(addr string, relays ...TCPRelay) error {
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
			log.Println("got tcp", string(body))
			for i := range relays {
				log.Println("relay", i)
				go func(i int) {
					relay := relays[i]
					if err := relay(conn, body); err != nil {
						log.Println("tcp relay error", i, err)
					}
				}(i)
			}
			if err := conn.Close(); err != nil {
				log.Println("conn close err", err)
			}
		}(conn)
	}
}

func TSDBSendTCP(dest string) TCPRelay {
	return func(conn net.Conn, body []byte) error {
		dconn, err := net.Dial("tcp", dest)
		if err != nil {
			return err
		}
		log.Println("writing", string(body))
		dconn.SetDeadline(time.Now().Add(time.Second))
		dconn.Write(body)
		dconn.SetDeadline(time.Now().Add(time.Second))
		status, err := ioutil.ReadAll(dconn)
		log.Println("SE", err, string(status))
		if len(status) > 0 {
			conn.SetDeadline(time.Now().Add(time.Second))
			conn.Write([]byte(status))
		} else if err != nil {
			return err
		}
		return nil
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

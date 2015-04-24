package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
)

var (
	listenAddr  = flag.String("l", ":4242", "Listen address.")
	bosunServer = flag.String("b", "bosun", "Target Bosun server. Can specify port with host:port.")
	tsdbServer  = flag.String("t", "", "Target OpenTSDB server. Can specify port with host:port.")
	logVerbose  = flag.Bool("v", false, "enable verbose logging")
)

func main() {
	flag.Parse()
	if *bosunServer == "" || *tsdbServer == "" {
		log.Fatal("must specify both bosun and tsdb server")
	}
	log.Println("listen on", *listenAddr)
	log.Println("relay to bosun at", *bosunServer)
	log.Println("relay to tsdb at", *tsdbServer)
	tsdbURL := &url.URL{
		Scheme: "http",
		Host:   *tsdbServer,
	}
	bosunURL := &url.URL{
		Scheme: "http",
		Host:   *bosunServer,
	}
	tsdbProxy := httputil.NewSingleHostReverseProxy(tsdbURL)
	http.Handle("/api/put", &relayProxy{
		ReverseProxy: tsdbProxy,
	})
	http.Handle("/api/metadata/put", httputil.NewSingleHostReverseProxy(bosunURL))
	http.Handle("/", tsdbProxy)
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}

func verbose(format string, a ...interface{}) {
	if *logVerbose {
		log.Printf(format, a...)
	}
}

type relayProxy struct {
	*httputil.ReverseProxy
}

type passthru struct {
	io.ReadCloser
	buf bytes.Buffer
}

func (p *passthru) Read(b []byte) (int, error) {
	n, err := p.ReadCloser.Read(b)
	p.buf.Write(b[:n])
	return n, err
}

type relayWriter struct {
	http.ResponseWriter
	code int
}

func (rw *relayWriter) WriteHeader(code int) {
	rw.code = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rp *relayProxy) ServeHTTP(responseWriter http.ResponseWriter, r *http.Request) {
	reader := &passthru{ReadCloser: r.Body}
	r.Body = reader
	w := &relayWriter{ResponseWriter: responseWriter}
	rp.ReverseProxy.ServeHTTP(w, r)
	if w.code != 204 {
		verbose("got status", w.code)
		return
	}
	verbose("relayed to tsdb")
	// Run in a separate go routine so we can end the source's request.
	go func() {
		body := bytes.NewBuffer(reader.buf.Bytes())
		u := &url.URL{
			Scheme: "http",
			Host:   *bosunServer,
			Path:   "/api/index",
		}
		req, err := http.NewRequest(r.Method, u.String(), body)
		if err != nil {
			verbose("%v", err)
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			verbose("bosun relay error: %v", err)
			return
		}
		resp.Body.Close()
		verbose("bosun relay success")
	}()
}

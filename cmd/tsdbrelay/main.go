package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"

	"bosun.org/cmd/tsdbrelay/denormalize"
	"bosun.org/opentsdb"
)

var (
	listenAddr    = flag.String("l", ":4242", "Listen address.")
	bosunServer   = flag.String("b", "bosun", "Target Bosun server. Can specify port with host:port.")
	tsdbServer    = flag.String("t", "", "Target OpenTSDB server. Can specify port with host:port.")
	logVerbose    = flag.Bool("v", false, "enable verbose logging")
	toDenormalize = flag.String("denormalize", "", "List of metrics to denormalize. Comma seperated list of `metric__tagname__tagname` rules. Will be translated to `___metric__tagvalue__tagvalue`")
)

var (
	tsdbPutURL    string
	bosunIndexURL string

	denormalizationRules map[string]*denormalize.DenormalizationRule
)

func main() {
	flag.Parse()
	if *bosunServer == "" || *tsdbServer == "" {
		log.Fatal("must specify both bosun and tsdb server")
	}
	log.Println("listen on", *listenAddr)
	log.Println("relay to bosun at", *bosunServer)
	log.Println("relay to tsdb at", *tsdbServer)
	if *toDenormalize != "" {
		var err error
		denormalizationRules, err = denormalize.ParseDenormalizationRules(*toDenormalize)
		if err != nil {
			log.Fatal(err)
		}
	}

	tsdbURL := &url.URL{
		Scheme: "http",
		Host:   *tsdbServer,
	}

	u := url.URL{
		Scheme: "http",
		Host:   *tsdbServer,
		Path:   "/api/put",
	}
	tsdbPutURL = u.String()
	bosunURL := &url.URL{
		Scheme: "http",
		Host:   *bosunServer,
	}
	u = url.URL{
		Scheme: "http",
		Host:   *bosunServer,
		Path:   "/api/index",
	}
	bosunIndexURL = u.String()
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
	rp.relayRequest(responseWriter, r, true)
}

func (rp *relayProxy) relayRequest(responseWriter http.ResponseWriter, r *http.Request, parse bool) {
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
		req, err := http.NewRequest(r.Method, bosunIndexURL, body)
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
	if parse && denormalizationRules != nil {
		go rp.denormalize(bytes.NewReader(reader.buf.Bytes()))
	}
}

func (rp *relayProxy) denormalize(body io.Reader) {
	gReader, err := gzip.NewReader(body)
	if err != nil {
		verbose("error making gzip reader: %v", err)
		return
	}
	decoder := json.NewDecoder(gReader)
	dps := []*opentsdb.DataPoint{}
	err = decoder.Decode(&dps)
	if err != nil {
		verbose("error decoding data points: %v", err)
		return
	}
	relayDps := []*opentsdb.DataPoint{}
	for _, dp := range dps {
		if rule, ok := denormalizationRules[dp.Metric]; ok {
			if err = rule.Translate(dp); err == nil {
				relayDps = append(relayDps, dp)
			} else {
				verbose(err.Error())
			}
		}
	}
	if len(relayDps) == 0 {
		return
	}
	buf := &bytes.Buffer{}
	gWriter := gzip.NewWriter(buf)
	encoder := json.NewEncoder(gWriter)
	err = encoder.Encode(relayDps)
	if err != nil {
		verbose("error encoding denormalized data points: %v", err)
		return
	}
	if err = gWriter.Close(); err != nil {
		verbose("error zipping denormalized data points: %v", err)
		return
	}
	req, err := http.NewRequest("POST", tsdbPutURL, buf)
	if err != nil {
		verbose("%v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	responseWriter := httptest.NewRecorder()
	rp.relayRequest(responseWriter, req, false)

	verbose("relayed %d denormalized data points. Tsdb response: %d", len(relayDps), responseWriter.Code)
}

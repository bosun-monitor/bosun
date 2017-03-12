package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	_ "expvar"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/facebookgo/httpcontrol"

	version "bosun.org/_version"

	"bosun.org/cmd/tsdbrelay/denormalize"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
)

var (
	listenAddr      = flag.String("l", ":4242", "Listen address.")
	bosunServer     = flag.String("b", "bosun", "Target Bosun server. Can specify port with host:port.")
	secondaryRelays = flag.String("r", "", "Additional relays to send data to. Intended for secondary data center replication. Only response from primary tsdb server wil be relayed to clients.")
	tsdbServer      = flag.String("t", "", "Target OpenTSDB server. Can specify port with host:port.")
	logVerbose      = flag.Bool("v", false, "enable verbose logging")
	toDenormalize   = flag.String("denormalize", "", "List of metrics to denormalize. Comma seperated list of `metric__tagname__tagname` rules. Will be translated to `__tagvalue.tagvalue.metric`")
	flagVersion     = flag.Bool("version", false, "Prints the version and exits.")

	redisHost = flag.String("redis", "", "redis host for aggregating external counters")
	redisDb   = flag.Int("db", 0, "redis db to use for counters")
)

var (
	tsdbPutURL    string
	bosunIndexURL string

	denormalizationRules map[string]*denormalize.DenormalizationRule

	relayPutUrls []string

	tags = opentsdb.TagSet{}
)

type tsdbrelayHTTPTransport struct {
	UserAgent string
	http.RoundTripper
}

func (t *tsdbrelayHTTPTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Add("User-Agent", t.UserAgent)
	}
	return t.RoundTripper.RoundTrip(req)
}

func init() {
	client := &http.Client{
		Transport: &tsdbrelayHTTPTransport{
			"Tsdbrelay/" + version.ShortVersion(),
			&httpcontrol.Transport{
				RequestTimeout: time.Minute,
			},
		},
	}
	http.DefaultClient = client
	collect.DefaultClient = client
}

func main() {
	var err error
	myHost, err = os.Hostname()
	if err != nil || myHost == "" {
		myHost = "tsdbrelay"
	}

	flag.Parse()
	if *flagVersion {
		fmt.Println(version.GetVersionInfo("tsdbrelay"))
		os.Exit(0)
	}
	if *bosunServer == "" || *tsdbServer == "" {
		slog.Fatal("must specify both bosun and tsdb server")
	}
	slog.Infoln(version.GetVersionInfo("tsdbrelay"))
	slog.Infoln("listen on", *listenAddr)
	slog.Infoln("relay to bosun at", *bosunServer)
	slog.Infoln("relay to tsdb at", *tsdbServer)
	if *toDenormalize != "" {
		var err error
		denormalizationRules, err = denormalize.ParseDenormalizationRules(*toDenormalize)
		if err != nil {
			slog.Fatal(err)
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
	if *secondaryRelays != "" {
		for _, rUrl := range strings.Split(*secondaryRelays, ",") {
			u = url.URL{
				Scheme: "http",
				Host:   rUrl,
				Path:   "/api/put",
			}
			relayPutUrls = append(relayPutUrls, u.String())
		}
	}

	tsdbProxy := util.NewSingleHostProxy(tsdbURL)
	bosunProxy := util.NewSingleHostProxy(bosunURL)
	rp := &relayProxy{
		TSDBProxy:  tsdbProxy,
		BosunProxy: bosunProxy,
	}
	http.HandleFunc("/api/put", func(w http.ResponseWriter, r *http.Request) {
		rp.relayPut(w, r, true)
	})
	if *redisHost != "" {
		http.HandleFunc("/api/count", collect.HandleCounterPut(*redisHost, *redisDb))
	}
	http.HandleFunc("/api/metadata/put", func(w http.ResponseWriter, r *http.Request) {
		rp.relayMetadata(w, r)
	})
	http.Handle("/", tsdbProxy)

	collectUrl := &url.URL{
		Scheme: "http",
		Host:   *listenAddr,
		Path:   "/api/put",
	}
	if err = collect.Init(collectUrl, "tsdbrelay"); err != nil {
		slog.Fatal(err)
	}
	if err := metadata.Init(collectUrl, false); err != nil {
		slog.Fatal(err)
	}
	// Make sure these get zeroed out instead of going unknown on restart
	collect.Add("puts.relayed", tags, 0)
	collect.Add("puts.error", tags, 0)
	collect.Add("metadata.relayed", tags, 0)
	collect.Add("metadata.error", tags, 0)
	collect.Add("additional.puts.relayed", tags, 0)
	collect.Add("additional.puts.error", tags, 0)
	metadata.AddMetricMeta("tsdbrelay.puts.relayed", metadata.Counter, metadata.Count, "Number of successful puts relayed to opentsdb target")
	metadata.AddMetricMeta("tsdbrelay.puts.error", metadata.Counter, metadata.Count, "Number of puts that could not be relayed to opentsdb target")
	metadata.AddMetricMeta("tsdbrelay.metadata.relayed", metadata.Counter, metadata.Count, "Number of successful metadata puts relayed to bosun target")
	metadata.AddMetricMeta("tsdbrelay.metadata.error", metadata.Counter, metadata.Count, "Number of metadata puts that could not be relayed to bosun target")
	metadata.AddMetricMeta("tsdbrelay.additional.puts.relayed", metadata.Counter, metadata.Count, "Number of successful puts relayed to additional targets")
	metadata.AddMetricMeta("tsdbrelay.additional.puts.error", metadata.Counter, metadata.Count, "Number of puts that could not be relayed to additional targets")
	slog.Fatal(http.ListenAndServe(*listenAddr, nil))
}

func verbose(format string, a ...interface{}) {
	if *logVerbose {
		slog.Infof(format, a...)
	}
}

type relayProxy struct {
	TSDBProxy  *httputil.ReverseProxy
	BosunProxy *httputil.ReverseProxy
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

var (
	relayHeader  = "X-Relayed-From"
	encHeader    = "Content-Encoding"
	typeHeader   = "Content-Type"
	accessHeader = "X-Access-Token"
	myHost       string
)

func (rp *relayProxy) relayPut(responseWriter http.ResponseWriter, r *http.Request, parse bool) {
	isRelayed := r.Header.Get(relayHeader) != ""
	reader := &passthru{ReadCloser: r.Body}
	r.Body = reader
	w := &relayWriter{ResponseWriter: responseWriter}
	rp.TSDBProxy.ServeHTTP(w, r)
	if w.code/100 != 2 {
		verbose("relayPut got status %d", w.code)
		collect.Add("puts.error", tags, 1)
		return
	}
	verbose("relayed to tsdb")
	collect.Add("puts.relayed", tags, 1)
	// Send to bosun in a separate go routine so we can end the source's request.
	go func() {
		body := bytes.NewBuffer(reader.buf.Bytes())
		req, err := http.NewRequest(r.Method, bosunIndexURL, body)
		if err != nil {
			verbose("bosun connect error: %v", err)
			return
		}
		if access := r.Header.Get(accessHeader); access != "" {
			req.Header.Set(accessHeader, access)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			verbose("bosun relay error: %v", err)
			return
		}
		// Drain up to 512 bytes and close the body to let the Transport reuse the connection
		io.CopyN(ioutil.Discard, resp.Body, 512)
		resp.Body.Close()
		verbose("bosun relay success")
	}()
	// Parse and denormalize datapoints
	if !isRelayed && parse && denormalizationRules != nil {
		go rp.denormalize(bytes.NewReader(reader.buf.Bytes()))
	}

	if !isRelayed && len(relayPutUrls) > 0 {
		go func() {
			for _, relayUrl := range relayPutUrls {
				body := bytes.NewBuffer(reader.buf.Bytes())
				req, err := http.NewRequest(r.Method, relayUrl, body)
				if err != nil {
					verbose("relayPutUrls connect error: %v", err)
					collect.Add("additional.puts.error", tags, 1)
					continue
				}
				if contenttype := r.Header.Get(typeHeader); contenttype != "" {
					req.Header.Set(typeHeader, contenttype)
				}
				if access := r.Header.Get(accessHeader); access != "" {
					req.Header.Set(accessHeader, access)
				}
				if encoding := r.Header.Get(encHeader); encoding != "" {
					req.Header.Set(encHeader, encoding)
				}
				req.Header.Add(relayHeader, myHost)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					verbose("secondary relay error: %v", err)
					collect.Add("additional.puts.error", tags, 1)
					continue
				}
				// Drain up to 512 bytes and close the body to let the Transport reuse the connection
				io.CopyN(ioutil.Discard, resp.Body, 512)
				resp.Body.Close()
				verbose("secondary relay success")
				collect.Add("additional.puts.relayed", tags, 1)
			}
		}()
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
				verbose("error translating points: %v", err.Error())
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
		verbose("error posting denormalized data points: %v", err)
		return
	}
	req.Header.Set(typeHeader, "application/json")
	req.Header.Set(encHeader, "gzip")

	responseWriter := httptest.NewRecorder()
	rp.relayPut(responseWriter, req, false)

	verbose("relayed %d denormalized data points. Tsdb response: %d", len(relayDps), responseWriter.Code)
}

func (rp *relayProxy) relayMetadata(responseWriter http.ResponseWriter, r *http.Request) {
	reader := &passthru{ReadCloser: r.Body}
	r.Body = reader
	w := &relayWriter{ResponseWriter: responseWriter}
	rp.BosunProxy.ServeHTTP(w, r)
	if w.code != 204 {
		verbose("relayMetadata got status %d", w.code)
		collect.Add("metadata.error", tags, 1)
		return
	}
	verbose("relayed metadata to bosun")
	collect.Add("metadata.relayed", tags, 1)
	if r.Header.Get(relayHeader) != "" {
		return
	}
	if len(relayPutUrls) != 0 {
		go func() {
			for _, relayUrl := range relayPutUrls {
				relayUrl = strings.Replace(relayUrl, "/put", "/metadata/put", 1)
				body := bytes.NewBuffer(reader.buf.Bytes())
				req, err := http.NewRequest(r.Method, relayUrl, body)
				if err != nil {
					verbose("metadata relayPutUrls error %v", err)
					continue
				}
				if contenttype := r.Header.Get(typeHeader); contenttype != "" {
					req.Header.Set(typeHeader, contenttype)
				}
				if access := r.Header.Get(accessHeader); access != "" {
					req.Header.Set(accessHeader, access)
				}
				if encoding := r.Header.Get(encHeader); encoding != "" {
					req.Header.Set(encHeader, encoding)
				}
				req.Header.Add(relayHeader, myHost)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					verbose("secondary relay metadata error: %v", err)
					continue
				}
				// Drain up to 512 bytes and close the body to let the Transport reuse the connection
				io.CopyN(ioutil.Discard, resp.Body, 512)
				resp.Body.Close()
				verbose("secondary relay metadata success")
			}
		}()
	}
}

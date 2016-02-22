package main

// TODO:
//  add graceful shutdown (http://www.hydrogen18.com/blog/stop-listening-http-server-go.html)

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"bosun.org/cmd/tsdbrelay/denormalize"
	"bosun.org/collect"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

var (
	bosunTransport *http.Transport
	bosunProxy     *httputil.ReverseProxy

	tsdbTransport *http.Transport
	tsdbProxy     *httputil.ReverseProxy

	denormalizationRules map[string]*denormalize.DenormalizationRule

	// We juggle separate queues here in order to avoid them interfering with
	// one another - one bloated queue won't slow a different one.
	bosunRequestQueue     chan *CountedRequest
	tsdbRequestQueue      chan *CountedRequest
	secondaryRelaysQueues *[]chan *CountedRequest
)

var (
	listenAddr  = flag.String("l", ":4242", "Listen address.")
	bosunServer = flag.String("b", "bosun", "Target Bosun server. Can specify port with host:port.")
	tsdbServer  = flag.String("t", "", "Target OpenTSDB server. Can specify port with host:port.")

	secondaryRelays = flag.String("r", "", "Additional relays to send data to. Intended for secondary data center replication. Only response from primary tsdb server wil be relayed to clients.")
	toDenormalize   = flag.String("denormalize", "", "List of metrics to denormalize. Comma seperated list of `metric__tagname__tagname` rules. Will be translated to `__tagvalue.tagvalue.metric`")

	redisHost = flag.String("redis", "", "redis host for aggregating external counters")
	redisDb   = flag.Int("db", 0, "redis db to use for counters")

	maxRetries       = flag.Int("a", 2, "Maximum number of times to attempt to asynchronously deliver a request")
	maxRequestBuffer = flag.Int("u", 250000, "Maximum asynchronous request buffer size")
	relayPoolSize    = flag.Int("w", 64, "Maximum number of concurrent synchronous relay sessions")
	maxRelayWaitTime = flag.Int("p", 10, "Maximum time to wait for a synchronous relay slot, in seconds")
	workersPerQueue  = flag.Int("q", 6, "Number of worker goroutines to process asynchronous deliveries")

	synchTimeout = flag.Int("relaytimeout", 30, "TSDB/relay HTTP response timeout in seconds")
	bosunTimeout = flag.Int("bosuntimeout", 30, "Bosun HTTP response timeout in seconds")
)

var relayPool chan bool

// RelayResponseWriter is an extension of http.ResponseWriter that permits
// us to capture the return code for later use.
type RelayResponseWriter struct {
	http.ResponseWriter
	ResponseCode int
}

// Pass the call along to http.ResponseWriter.WriteHeader, but save the return
// code too.
func (res *RelayResponseWriter) WriteHeader(code int) {
	res.ResponseCode = code
	res.ResponseWriter.WriteHeader(res.ResponseCode)
}

// CountedRequest is a type which lets us track the number of times
// we've attempted to relay a request.
type CountedRequest struct {
	Request  []byte
	NumTries int
}

// NopResponseWriter is a basic implementation of the ResponseWriter interface
// that does nothing but store a response code. It returns an error if an attempt
// is made to write to the body of the response.
type NopResponseWriter struct {
	ResponseCode int
	wroteStatus  bool
}

// Want a Header? Here you go, have a Header. Headers for everyone.
func (resp *NopResponseWriter) Header() http.Header {
	return make(http.Header)
}

// Return an error indicating that we do not permit bodies. Implicitly set the
// status code to 204 No Content if it hasn't been set otherwise.
func (resp *NopResponseWriter) Write(_ []byte) (n int, err error) {
	if !resp.wroteStatus {
		resp.WriteHeader(http.StatusNoContent)
	}
	return 0, http.ErrBodyNotAllowed
}

// Store the given response code, and record that it's been explicitly set.
func (resp *NopResponseWriter) WriteHeader(status int) {
	resp.ResponseCode = status
	resp.wroteStatus = true
}

// Synchronously relay the given request to the specified proxy, storing the
// server's response in a RelayResponseWriter. This call will block until a
// relay pool slot is available, or the channel receive times out.
func synchRelay(destinationProxy *httputil.ReverseProxy, clientResp *RelayResponseWriter, reqBytes *[]byte) {
	clientReq, err := requestFromBytes(reqBytes)
	if err != nil {
		slog.Error("synchRelay could not grok request bytes: ", reqBytes, " because ", err)
		clientResp.WriteHeader(http.StatusBadRequest)
		return
	}
	slog.Debug("synchRelay got ", clientReq.URL.EscapedPath())
	select {
	case relayPool <- true:
		slog.Debug("synchRelay got a slot for ", clientReq.URL.EscapedPath())
		destinationProxy.ServeHTTP(clientResp, clientReq)
		slog.Debug("synchRelay relayed request ", clientReq.URL.EscapedPath(), " with return code ", clientResp.ResponseCode)
		<-relayPool
	case <-time.After(time.Duration(*maxRelayWaitTime) * time.Second):
		defer collect.Add("puts.queue_timeouts", opentsdb.TagSet{}, 1)
		slog.Warning("synchRelay could not get a slot in time to relay ", clientReq.URL.EscapedPath())
		clientResp.WriteHeader(http.StatusGatewayTimeout)
	}
	return
}

// Retrieve a (byte string) HTTP request from the queue, and forward it to the
// supplied proxy. Monitors the response code, and returns the request to the
// queue if it should be replayed. If delivery of the request has been attempted
// too many times, abandon the request and log a warning.
//
// n.b. that the retry logic does not know anything about backing off or waiting.
func retryingRelayWorker(requestQueue chan *CountedRequest, destinationProxy *httputil.ReverseProxy) {
	slog.Info("rRW starting up")
	defer slog.Info("rRW shutting down")
	throwawayResp := &NopResponseWriter{}
	for request := range requestQueue {
		clientReq, err := requestFromBytes(&request.Request)
		if err != nil {
			slog.Error("rRW could not grok request bytes: ", request.Request, " because ", err)
		}
		slog.Debug("rRW got ", clientReq.URL.EscapedPath(), " with ", request.NumTries, " tries")
		request.NumTries += 1
		destinationProxy.ServeHTTP(throwawayResp, clientReq)
		if shouldRetry(throwawayResp.ResponseCode) && request.NumTries < *maxRetries {
			slog.Warning("rRW must replay ", clientReq.URL.EscapedPath(), ": got response ", throwawayResp.ResponseCode)
			requestQueue <- request
		} else {
			slog.Debug("rRW is done with ", clientReq.URL.EscapedPath(),
				": got response ", throwawayResp.ResponseCode,
				" after ", request.NumTries, " tries")
		}
	}
}

// Does the response's status suggest that we should re-queue the request and
// try it again?
// Codes which suggest possible future success are 500, 502, 503, and 504.
func shouldRetry(respCode int) bool {
	switch respCode {
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

// Take the request and put it on the queue for asynchronous relaying.
func asynchRelay(requestQueue chan *CountedRequest, reqBytes *[]byte) {
	trackedRequest := &CountedRequest{
		Request:  *reqBytes,
		NumTries: 0,
	}
	select {
	case requestQueue <- trackedRequest:
		return
	default:
		slog.Error("async could not put a request into the queue")
		collect.Add("asynch.queue_overflow", opentsdb.TagSet{}, 1)
	}
}

// Generate an http.Request from a byte array (presumably containing an HTTP request)
func requestFromBytes(reqBytes *[]byte) (*http.Request, error) {
	return http.ReadRequest(bufio.NewReader(bytes.NewBuffer(*reqBytes)))
}

// Handle the incoming HTTP request by synchronously relaying it to the TSDB
// server (via tsdbProxy). If the proxy succeeds, asynchronously relay it to
// the /api/index endpoint at Bosun (via bosunProxy), and return the TSDB response
// code to the client. If the relay to TSDB does not succeed, return early.
func putRelay(clientResp http.ResponseWriter, clientReq *http.Request) {
	rtt := collect.StartTimer("puts.rtt", opentsdb.TagSet{})
	defer rtt()
	slog.Info("putRelay handling ", clientReq.URL.EscapedPath(), " for ", clientReq.RemoteAddr)
	defer collect.Add("puts.received", opentsdb.TagSet{}, 1)

	// Extract a copy of the request that we can pass around
	reqBuf := new(bytes.Buffer)
	clientReq.WriteProxy(reqBuf)
	reqStore := reqBuf.Bytes()

	// Munge the response type so we can capture the response code
	synchResp := &RelayResponseWriter{ResponseWriter: clientResp}

	// Relay the request data to tsdbProxy, returning the response in synchResp
	synchRelay(tsdbProxy, synchResp, &reqStore)

	if synchResp.ResponseCode != http.StatusNoContent {
		slog.Warning("putRelay could not relay ", clientReq.URL.EscapedPath(), ": got response code ", synchResp.ResponseCode)
	} else {
		slog.Debug("putRelay succeeded at relaying ", clientReq.URL.EscapedPath())
		defer collect.Add("puts.relayed", opentsdb.TagSet{"status": strconv.Itoa(synchResp.ResponseCode)}, 1)

		// Rewrite the request to change the endpoint, so that Bosun will index
		// it, and not forward it back to us, and then queue it for sending.
		toIndex := bytes.Replace(reqStore[:], []byte("/api/put"), []byte("/api/index"), 1)
		asynchRelay(bosunRequestQueue, &toIndex)

		if clientReq.Header.Get("X-Forwarded-For") == "" && denormalizationRules != nil {
			denormalizeAndQueue(&reqStore)
		}

		// Now send it to secondaries
		if *secondaryRelays != "" {
			for _, q := range *secondaryRelaysQueues {
				asynchRelay(q, &reqStore)
				slog.Debug("putRelay sent to a secondaryRelay")
			}
		}
	}
	// Return the synchronous response to the client.
	clientResp = synchResp
}

func denormalizeAndQueue(reqBytes *[]byte) {
	// Do some gymnastics to get the body of the request out of the byte blob :(
	clientReq, err := requestFromBytes(reqBytes)
	if err != nil {
		slog.Error("dAQ could not grok request bytes: ", reqBytes, " because ", err)
	}

	gz, err := gzip.NewReader(clientReq.Body)
	if err != nil {
		slog.Error("dAQ could not create a gzip reader: ", err)
	}
	decoder := json.NewDecoder(gz)
	dps := []*opentsdb.DataPoint{}
	err = decoder.Decode(&dps)
	if err != nil {
		slog.Debug("dAQ error decoding data points: ", err)
		return
	}

	relayDps := []*opentsdb.DataPoint{}
	for _, dp := range dps {
		if rule, ok := denormalizationRules[dp.Metric]; ok {
			if err = rule.Translate(dp); err == nil {
				relayDps = append(relayDps, dp)
			} else {
				slog.Info(err.Error())
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
		slog.Info("error encoding denormalized data points: %v", err)
		return
	}
	if err = gWriter.Close(); err != nil {
		slog.Info("error zipping denormalized data points: %v", err)
		return
	}

	clientReq.Body.Read(buf.Bytes())         // Read in the re-written body
	clientReq.Write(buf)                     // Write out the whole request
	reqStore := buf.Bytes()                  // Store the content for queueing
	asynchRelay(tsdbRequestQueue, &reqStore) // Queue the request and forget

	slog.Debug("dAQ queued ", len(relayDps), " denormalized data points")
}

// Synchronously relay the POSTed metadata to Bosun, skipping TSDB entirely.
func metadataRelay(clientResp http.ResponseWriter, clientReq *http.Request) {
	slog.Info("metadataRelay handling ", clientReq.URL.EscapedPath())
	collect.Add("metadata.received", opentsdb.TagSet{}, 1)

	synchResp := &RelayResponseWriter{ResponseWriter: clientResp}

	// Extract the request so we can pass it to synchRelay
	reqBuf := new(bytes.Buffer)
	clientReq.WriteProxy(reqBuf)
	reqStore := reqBuf.Bytes()

	synchRelay(bosunProxy, synchResp, &reqStore)

	if synchResp.ResponseCode != http.StatusNoContent {
		slog.Warning("metadataRelay could not relay ", clientReq.URL.EscapedPath(), ": got response code ", synchResp.ResponseCode)
	} else {
		slog.Debug("metadataRelay succeeded at relaying ", clientReq.URL.EscapedPath())
		defer collect.Add("metadata.relayed", opentsdb.TagSet{"status": strconv.Itoa(synchResp.ResponseCode)}, 1)
	}

	clientResp = synchResp
}

func init() {
	flag.Parse()
	if *bosunServer == "" || *tsdbServer == "" {
		slog.Fatal("Must specify both bosun and tsdb servers")
	}
	slog.Info("Listening on ", *listenAddr)
	slog.Info("Relaying to Bosun at ", *bosunServer)
	slog.Info("Relaying to TSDB at ", *tsdbServer)

	relayPool = make(chan bool, *relayPoolSize)

	bosunTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: time.Minute,
		}).Dial,
		ResponseHeaderTimeout: time.Duration(*bosunTimeout) * time.Second,
		MaxIdleConnsPerHost:   55,
	}

	bosunProxy = httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   *bosunServer,
	})
	bosunProxy.Transport = bosunTransport

	bosunRequestQueue = make(chan *CountedRequest, *maxRequestBuffer)

	tsdbTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: time.Minute,
		}).Dial,
		ResponseHeaderTimeout: time.Duration(*synchTimeout) * time.Second,
		MaxIdleConnsPerHost:   55,
	}

	tsdbProxy = httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http",
		Host:   *tsdbServer,
	})
	tsdbProxy.Transport = tsdbTransport

	tsdbRequestQueue = make(chan *CountedRequest, *maxRequestBuffer)

	if *toDenormalize != "" {
		var err error
		denormalizationRules, err = denormalize.ParseDenormalizationRules(*toDenormalize)
		if err != nil {
			slog.Fatal("error parsing denormalization rules on init: ", err)
		}
	}

	if err := collect.Init(&url.URL{Scheme: "http", Host: *listenAddr, Path: "/api/put"}, "tsdbrelay"); err != nil {
		slog.Fatal(err)
	}
	collect.Set("asynch.queuelength", opentsdb.TagSet{"queue": "bosun"}, func() interface{} { return len(bosunRequestQueue) })
	collect.Set("asynch.queuelength", opentsdb.TagSet{"queue": "tsdb"}, func() interface{} { return len(tsdbRequestQueue) })
	collect.Set("synch.active_workers", opentsdb.TagSet{}, func() interface{} { return len(relayPool) })
}

func main() {
	defer close(bosunRequestQueue)
	defer close(tsdbRequestQueue)

	if *secondaryRelays != "" {
		slog.Info("Planning to use secondary relays: ", *secondaryRelays)
		secondaryHosts := strings.Split(*secondaryRelays, ",")
		queuesArr := make([]chan *CountedRequest, len(secondaryHosts))
		for i, h := range secondaryHosts {
			slog.Debug("Setting up secondary relay number ", i, " to ", h)
			secondaryQueue := make(chan *CountedRequest, *maxRequestBuffer)
			queuesArr[i] = secondaryQueue
			secondaryTransport := &http.Transport{
				Dial: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: time.Minute,
				}).Dial,
				ResponseHeaderTimeout: time.Duration(*synchTimeout) * time.Second,
				MaxIdleConnsPerHost:   55,
			}
			secondaryProxy := httputil.NewSingleHostReverseProxy(&url.URL{
				Scheme: "http",
				Host:   h,
			})
			secondaryProxy.Transport = secondaryTransport

			go retryingRelayWorker(secondaryQueue, secondaryProxy)
			collect.Set("asynch.queuelength", opentsdb.TagSet{"queue": h}, func() interface{} { return len(secondaryQueue) })
		}
		secondaryRelaysQueues = &queuesArr
		slog.Debug("Created ", len(*secondaryRelaysQueues), " secondary queues")
	}

	createWorkersFor(*workersPerQueue, bosunRequestQueue, bosunProxy)
	createWorkersFor(*workersPerQueue, tsdbRequestQueue, tsdbProxy)

	http.HandleFunc("/api/put", putRelay)
	http.HandleFunc("/api/metadata/put", metadataRelay)
	if *redisHost != "" {
		http.HandleFunc("/api/count", collect.HandleCounterPut(*redisHost, *redisDb))
	}
	http.Handle("/", tsdbProxy)
	slog.Fatal(http.ListenAndServe(*listenAddr, nil))
}

func createWorkersFor(num int, q chan *CountedRequest, p *httputil.ReverseProxy) {
	for i := 0; i < num; i++ {
		go retryingRelayWorker(q, p)
	}
}

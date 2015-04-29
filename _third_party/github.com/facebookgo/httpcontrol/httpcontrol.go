// Package httpcontrol allows a HTTP transport supporting connection pooling,
// timeouts & retries.
//
// This Transport is built on top of the standard library transport and
// augments it with additional features.
package httpcontrol

import (
	"bytes"
	"container/heap"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"bosun.org/_third_party/github.com/facebookgo/pqueue"
	"syscall"
)

// Stats for a RoundTrip.
type Stats struct {
	// The RoundTrip request.
	Request *http.Request

	// May not always be available.
	Response *http.Response

	// Will be set if the RoundTrip resulted in an error. Note that these are
	// RoundTrip errors and we do not care about the HTTP Status.
	Error error

	// Each duration is independent and the sum of all of them is the total
	// request duration. One or more durations may be zero.
	Duration struct {
		Header, Body time.Duration
	}

	Retry struct {
		// Will be incremented for each retry. The initial request will have this
		// set to 0, and the first retry to 1 and so on.
		Count uint

		// Will be set if and only if an error was encountered and a retry is
		// pending.
		Pending bool
	}
}

// A human readable representation often useful for debugging.
func (s *Stats) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s %s", s.Request.Method, s.Request.URL)

	if s.Response != nil {
		fmt.Fprintf(&buf, " got response with status %s", s.Response.Status)
	}

	return buf.String()
}

// Transport is an implementation of RoundTripper that supports http, https,
// and http proxies (for either http or https with CONNECT). Transport can
// cache connections for future re-use, provides various timeouts, retry logic
// and the ability to track request statistics.
type Transport struct {

	// Proxy specifies a function to return a proxy for a given
	// *http.Request. If the function returns a non-nil error, the
	// request is aborted with the provided error.
	// If Proxy is nil or returns a nil *url.URL, no proxy is used.
	Proxy func(*http.Request) (*url.URL, error)

	// TLSClientConfig specifies the TLS configuration to use with
	// tls.Client. If nil, the default configuration is used.
	TLSClientConfig *tls.Config

	// DisableKeepAlives, if true, prevents re-use of TCP connections
	// between different HTTP requests.
	DisableKeepAlives bool

	// DisableCompression, if true, prevents the Transport from
	// requesting compression with an "Accept-Encoding: gzip"
	// request header when the Request contains no existing
	// Accept-Encoding value. If the Transport requests gzip on
	// its own and gets a gzipped response, it's transparently
	// decoded in the Response.Body. However, if the user
	// explicitly requested gzip it is not automatically
	// uncompressed.
	DisableCompression bool

	// MaxIdleConnsPerHost, if non-zero, controls the maximum idle
	// (keep-alive) to keep per-host.  If zero,
	// http.DefaultMaxIdleConnsPerHost is used.
	MaxIdleConnsPerHost int

	// Timeout is the maximum amount of time a dial will wait for
	// a connect to complete.
	//
	// The default is no timeout.
	//
	// With or without a timeout, the operating system may impose
	// its own earlier timeout. For instance, TCP timeouts are
	// often around 3 minutes.
	DialTimeout time.Duration

	// ResponseHeaderTimeout, if non-zero, specifies the amount of
	// time to wait for a server's response headers after fully
	// writing the request (including its body, if any). This
	// time does not include the time to read the response body.
	ResponseHeaderTimeout time.Duration

	// RequestTimeout, if non-zero, specifies the amount of time for the entire
	// request. This includes dialing (if necessary), the response header as well
	// as the entire body.
	RequestTimeout time.Duration

	// MaxTries, if non-zero, specifies the number of times we will retry on
	// failure. Retries are only attempted for temporary network errors or known
	// safe failures.
	MaxTries uint

	// Stats allows for capturing the result of a request and is useful for
	// monitoring purposes.
	Stats func(*Stats)

	transport    *http.Transport
	startOnce    sync.Once
	closeMonitor chan bool
	pqMutex      sync.Mutex
	pq           pqueue.PriorityQueue
}

var knownFailureSuffixes = []string{
	syscall.ECONNREFUSED.Error(),
	syscall.ECONNRESET.Error(),
	syscall.ETIMEDOUT.Error(),
	"no such host",
	"remote error: handshake failure",
	io.ErrUnexpectedEOF.Error(),
	io.EOF.Error(),
}

func shouldRetryError(err error) bool {
	if neterr, ok := err.(net.Error); ok {
		if neterr.Temporary() {
			return true
		}
	}

	s := err.Error()
	for _, suffix := range knownFailureSuffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

// Start the Transport.
func (t *Transport) start() {
	dialer := &net.Dialer{Timeout: t.DialTimeout}
	t.transport = &http.Transport{
		Dial:                  dialer.Dial,
		Proxy:                 t.Proxy,
		TLSClientConfig:       t.TLSClientConfig,
		DisableKeepAlives:     t.DisableKeepAlives,
		DisableCompression:    t.DisableCompression,
		MaxIdleConnsPerHost:   t.MaxIdleConnsPerHost,
		ResponseHeaderTimeout: t.ResponseHeaderTimeout,
	}
	t.closeMonitor = make(chan bool)
	t.pq = pqueue.New(16)
	go t.monitor()
}

// Close the Transport.
func (t *Transport) Close() error {
	// This ensures we were actually started. The alternative is to
	// have a mutex to check if we have started, which loses the benefit of the
	// sync.Once.
	t.startOnce.Do(t.start)

	t.transport.CloseIdleConnections()
	t.closeMonitor <- true
	<-t.closeMonitor
	return nil
}

func (t *Transport) monitor() {
	ticker := time.NewTicker(25 * time.Millisecond)
	for {
		select {
		case <-t.closeMonitor:
			ticker.Stop()
			close(t.closeMonitor)
			return
		case n := <-ticker.C:
			now := n.UnixNano()
			for {
				t.pqMutex.Lock()
				item, _ := t.pq.PeekAndShift(now)
				t.pqMutex.Unlock()

				if item == nil {
					break
				}

				req := item.Value.(*http.Request)
				t.CancelRequest(req)
			}
		}
	}
}

// CancelRequest cancels an in-flight request by closing its connection.
func (t *Transport) CancelRequest(req *http.Request) {
	t.transport.CancelRequest(req)
}

func (t *Transport) tries(req *http.Request, try uint) (*http.Response, error) {
	startTime := time.Now()
	deadline := int64(math.MaxInt64)
	if t.RequestTimeout != 0 {
		deadline = startTime.Add(t.RequestTimeout).UnixNano()
	}
	item := &pqueue.Item{Value: req, Priority: deadline}
	t.pqMutex.Lock()
	heap.Push(&t.pq, item)
	t.pqMutex.Unlock()
	res, err := t.transport.RoundTrip(req)
	headerTime := time.Now()
	if err != nil {
		t.pqMutex.Lock()
		if item.Index != -1 {
			heap.Remove(&t.pq, item.Index)
		}
		t.pqMutex.Unlock()

		var stats *Stats
		if t.Stats != nil {
			stats = &Stats{
				Request:  req,
				Response: res,
				Error:    err,
			}
			stats.Duration.Header = headerTime.Sub(startTime)
			stats.Retry.Count = try
		}

		if try < t.MaxTries && req.Method == "GET" && shouldRetryError(err) {
			if t.Stats != nil {
				stats.Retry.Pending = true
				t.Stats(stats)
			}
			return t.tries(req, try+1)
		}

		if t.Stats != nil {
			t.Stats(stats)
		}
		return nil, err
	}

	res.Body = &bodyCloser{
		ReadCloser: res.Body,
		res:        res,
		item:       item,
		transport:  t,
		startTime:  startTime,
		headerTime: headerTime,
	}
	return res, nil
}

// RoundTrip implements the RoundTripper interface.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.startOnce.Do(t.start)
	return t.tries(req, 0)
}

type bodyCloser struct {
	io.ReadCloser
	res        *http.Response
	item       *pqueue.Item
	transport  *Transport
	startTime  time.Time
	headerTime time.Time
}

func (b *bodyCloser) Close() error {
	err := b.ReadCloser.Close()
	closeTime := time.Now()
	b.transport.pqMutex.Lock()
	if b.item.Index != -1 {
		heap.Remove(&b.transport.pq, b.item.Index)
	}
	b.transport.pqMutex.Unlock()
	if b.transport.Stats != nil {
		stats := &Stats{
			Request:  b.res.Request,
			Response: b.res,
		}
		stats.Duration.Header = b.headerTime.Sub(b.startTime)
		stats.Duration.Body = closeTime.Sub(b.startTime) - stats.Duration.Header
		b.transport.Stats(stats)
	}
	return err
}

// A Flag configured Transport instance.
func TransportFlag(name string) *Transport {
	t := &Transport{TLSClientConfig: &tls.Config{}}
	flag.BoolVar(
		&t.TLSClientConfig.InsecureSkipVerify,
		name+".insecure-tls",
		false,
		name+" skip tls certificate verification",
	)
	flag.BoolVar(
		&t.DisableKeepAlives,
		name+".disable-keepalive",
		false,
		name+" disable keep-alives",
	)
	flag.BoolVar(
		&t.DisableCompression,
		name+".disable-compression",
		false,
		name+" disable compression",
	)
	flag.IntVar(
		&t.MaxIdleConnsPerHost,
		name+".max-idle-conns-per-host",
		http.DefaultMaxIdleConnsPerHost,
		name+" max idle connections per host",
	)
	flag.DurationVar(
		&t.DialTimeout,
		name+".dial-timeout",
		2*time.Second,
		name+" dial timeout",
	)
	flag.DurationVar(
		&t.ResponseHeaderTimeout,
		name+".response-header-timeout",
		3*time.Second,
		name+" response header timeout",
	)
	flag.DurationVar(
		&t.RequestTimeout,
		name+".request-timeout",
		30*time.Second,
		name+" request timeout",
	)
	flag.UintVar(
		&t.MaxTries,
		name+".max-tries",
		0,
		name+" max retries for known safe failures",
	)
	return t
}

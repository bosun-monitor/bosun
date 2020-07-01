package graphite

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
)

// RoundTripFunc .
type RoundTripFunc func(req *http.Request) *http.Response

// RoundTrip .
func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

//NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(fn),
	}
}

func TestUrlParsePanic(t *testing.T) {
	// url.Parse has unexpected behavior when not specifying a scheme
	// This would cause a panic in Query when using ip addresses with no scheme
	// e.g. 127.0.0.1:8080

	tests := []struct {
		host string
		URL  url.URL
	}{
		{"localhost:8080", url.URL{Scheme: "http", Host: "localhost:8080", Path: "/render/", RawQuery: "format=json"}},
		{"127.0.0.1:8080", url.URL{Scheme: "http", Host: "127.0.0.1:8080", Path: "/render/", RawQuery: "format=json"}},
		{"https://graphite.skyscanner.net:8080", url.URL{Scheme: "https", Host: "graphite.skyscanner.net:8080", Path: "/render/", RawQuery: "format=json"}}}

	r := Request{}
	stockResponse := http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewBufferString(`OK`)),
		Header:     make(http.Header),
	}

	for _, test := range tests {
		DefaultClient = NewTestClient(func(req *http.Request) *http.Response {
			assert.Equal(t, test.URL, *req.URL)
			return &stockResponse

		})
		r.Query(test.host, nil)

	}

}

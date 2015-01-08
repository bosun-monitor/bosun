// Package graphite defines structures for interacting with a Graphite server.
package graphite // import "bosun.org/graphite"

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Request holds query objects. Currently only absolute times are supported.
type Request struct {
	Start     *time.Time
	End       *time.Time
	Targets   []string
	MaxPoints int
}

type Response []Series

type Series struct {
	Datapoints []DataPoint
	Target     string
}

type DataPoint []json.Number

func (r *Request) Query(host string) (Response, error) {
	v := url.Values{
		"format": []string{"json"},
		"target": r.Targets,
	}
	if r.MaxPoints > 0 {
		v.Set("maxDataPoints", fmt.Sprintf("%d", r.MaxPoints))
	}
	if r.Start != nil {
		v.Add("from", fmt.Sprint(r.Start.Unix()))
	}
	if r.End != nil {
		v.Add("until", fmt.Sprint(r.End.Unix()))
	}
	u := url.URL{
		Scheme:   "http",
		Host:     host,
		Path:     "/render/",
		RawQuery: v.Encode(),
	}
	resp, err := DefaultClient.Get(u.String())
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Response{}, errors.New(resp.Status)
	}
	var series Response
	err = json.NewDecoder(resp.Body).Decode(&series)
	return series, err
}

// DefaultClient is the default HTTP client for requests.
var DefaultClient = &http.Client{
	Timeout: time.Minute,
}

// Context is the interface for querying a Graphite server.
type Context interface {
	Query(*Request) (Response, error)
}

// Host is a simple Graphite Context with no additional features.
type Host string

// Query performs a request to a Graphite server.
func (h Host) Query(r *Request) (Response, error) {
	return r.Query(string(h))
}

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
	Start   *time.Time
	End     *time.Time
	Targets []string
}

type Response []Series

type Series struct {
	Datapoints []DataPoint
	Target     string
}

type DataPoint []json.Number

func (r *Request) CacheKey() string {
	targets, _ := json.Marshal(r.Targets)
	return fmt.Sprintf("graphite-%d-%d-%s", r.Start.Unix(), r.End.Unix(), targets)
}

func (r *Request) Query(host string) (Response, error) {
	v := url.Values{
		"format": []string{"json"},
		"target": r.Targets,
	}
	if r.Start != nil {
		v.Add("from", fmt.Sprint(r.Start.Unix()))
	}
	if r.End != nil {
		v.Add("until", fmt.Sprint(r.End.Unix()))
	}
	u1, err := url.Parse(host)
	if err != nil {
		return nil, err
	}
	if u1.Host == "" && u1.Scheme != "" && u1.Path != "" {
		u1.Host = host
		u1.Scheme = "http"
	} else if u1.Scheme == "" || u1.Host = "" || u1.Path != "" {
		return nil, fmt.Errorf("bad URL format: %v", host)
	}
	u := url.URL{
		Scheme:   u1.Scheme,
		Host:     u1.Host,
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

// Package graphite defines structures for interacting with a Graphite server.
package graphite // import "bosun.org/graphite"

import (
	"bosun.org/opentsdb"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

//request structs

// Request holds query objects:
// unlike the opentsdb package, this is not a 100% complete graphite package
// and so we only support unix timestamps as from/until
type Request struct {
	Start   *uint32
	End     *uint32
	Targets []string
}

type Query struct {
	Target string
	Format []string
	Tags   opentsdb.TagSet
}

type Response struct {
	Series []Series
}

// response types
type Series struct {
	Datapoints []DataPoint
	Target     string
}
type DataPoint []json.Number

func ParseQuery(query, format string) (q *Query, err error) {
	q = new(Query)
	q.Target = query
	q.Tags, q.Format = ParseFormat(format)
	return
}

// convert a format like "..host..core" into
// a tagset with host and core keys, and the structured repr.
func ParseFormat(format string) (opentsdb.TagSet, []string) {
	nodes := strings.Split(format, ".")
	ts := make(opentsdb.TagSet)
	for _, node := range nodes {
		if node != "" {
			ts[node] = ""
		}
	}
	return ts, nodes
}

func (r *Request) Query(host string) (Response, error) {
	v := url.Values{}
	v.Set("format", "json")
	// note that r.Start and r.End should != nil at this point.
	// if not, a panic is in order.
	v.Set("from", fmt.Sprintf("%d", *r.Start))
	v.Set("until", fmt.Sprintf("%d", *r.End))
	for _, target := range r.Targets {
		v.Add("target", target)
	}
	url := fmt.Sprintf("%s/render/?%s", host, v.Encode())
	resp, err := DefaultClient.Get(url)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Response{}, errors.New(resp.Status)
	}
	decoder := json.NewDecoder(resp.Body)
	var series []Series
	err = decoder.Decode(&series)
	return Response{series}, err
}

func ParseTime(i uint32) (time.Time, error) {
	return time.Unix(int64(i), 0).UTC(), nil
}

// SetTime adjusts the start and end time of the request to assume t is now.
// Relative times ("1m-ago") are changed to absolute times. Existing absolute
// times are adjusted by the difference between time.Now() and t.
// note that r.Start should != nil at this point. if not, a panic is in order.
func (r *Request) SetTime(t time.Time) error {
	diff := -time.Since(t)
	start, err := ParseTime(*r.Start)
	if err != nil {
		return err
	}
	newStart := uint32(start.Add(diff).Unix())
	r.Start = &newStart
	newEnd := uint32(t.UTC().Unix())
	if r.End != nil {
		end, err := ParseTime(*r.End)
		if err != nil {
			return err
		}
		newEnd = uint32(end.Add(diff).Unix())
	}
	r.End = &newEnd
	return nil
}

// DefaultClient is the default http client for requests.
var DefaultClient = &http.Client{
	Timeout: time.Minute,
}

// Context is the interface for querying a graphite server
type Context interface {
	Query(*Request) (Response, error)
}

// Host is a simple Graphite Context with no additional features.
type Host string

// Query performs the request to the graphite server
func (h Host) Query(r *Request) (Response, error) {
	return r.Query(string(h))
}

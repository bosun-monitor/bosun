package opentsdb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var l = log.New(os.Stdout, "", log.LstdFlags)

type ResponseSet []*Response

type Point float64

type Response struct {
	Metric        string           `json:"metric"`
	Tags          TagSet           `json:"tags"`
	AggregateTags []string         `json:"aggregateTags"`
	DPS           map[string]Point `json:"dps"`
}

type DataPoint struct {
	Metric    string      `json:"metric"`
	Timestamp int64       `json:"timestamp"`
	Value     interface{} `json:"value"`
	Tags      TagSet      `json:"tags"`
}

func (d *DataPoint) Telnet() string {
	m := ""
	d.clean()
	for k, v := range d.Tags {
		m += fmt.Sprintf(" %s=%s", k, v)
	}
	return fmt.Sprintf("put %s %d %v%s\n", d.Metric, d.Timestamp, d.Value, m)
}

func (m MultiDataPoint) Json() ([]byte, error) {
	var md MultiDataPoint
	for _, d := range m {
		err := d.clean()
		if err != nil {
			l.Println(err, "Removing Datapoint", d)
			continue
		}
		md = append(md, d)
	}
	return json.Marshal(md)
}

type MultiDataPoint []*DataPoint

type TagSet map[string]string

func (t TagSet) Equal(o TagSet) bool {
	if len(t) != len(o) {
		return false
	}
	for k, v := range t {
		if ov, ok := o[k]; !ok || ov != v {
			return false
		}
	}
	return true
}

// Subset returns true if all k=v pairs in o are in t.
func (t TagSet) Subset(o TagSet) bool {
	for k, v := range o {
		if tv, ok := t[k]; !ok || tv != v {
			return false
		}
	}
	return true
}

// String converts t to an OpenTSDB-style {a=b,c=b} string, alphabetized by key.
func (t TagSet) String() string {
	var keys []string
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b := bytes.NewBufferString("{")
	for i, k := range keys {
		if i > 0 {
			fmt.Fprint(b, ",")
		}
		fmt.Fprintf(b, "%s=%s", k, t[k])
	}
	fmt.Fprint(b, "}")
	return b.String()
}

func (d *DataPoint) clean() error {
	err := d.Tags.clean()
	if err != nil {
		return err
	}
	om := d.Metric
	d.Metric, err = Clean(d.Metric)
	if err != nil {
		return fmt.Errorf("%s. Orginal: [%s] Cleaned: [%s]", err.Error(), om, d.Metric)
	}
	if sv, ok := d.Value.(string); ok {
		if i, err := strconv.ParseInt(sv, 10, 64); err == nil {
			d.Value = i
		} else if f, err := strconv.ParseFloat(sv, 64); err == nil {
			d.Value = f
		} else {
			return fmt.Errorf("Unparseable number %v", sv)
		}
	}
	return nil
}

func (t TagSet) clean() error {
	for k, v := range t {
		kc, err := Clean(k)
		if err != nil {
			return fmt.Errorf("%s. Orginal: [%s] Cleaned: [%s]", err.Error(), k, kc)
		}
		vc, err := Clean(v)
		if err != nil {
			return fmt.Errorf("%s. Orginal: [%s] Cleaned: [%s]", err.Error(), v, vc)
		}
		delete(t, k)
		t[kc] = vc
	}
	return nil
}

// Clean removes characters from s that are invalid for OpenTSDB metric and tag
// values.
// See: http://opentsdb.net/docs/build/html/user_guide/writing.html#metrics-and-tags
func Clean(s string) (string, error) {
	var c string
	if len(s) == 0 {
		// This one is perhaps better checked earlier in the pipeline, but since
		// it makes sense to check that the resulting cleaned tag is not Zero length here I'm including it
		// It also might be the case that this just shouldn't be happening and this is yet another side
		// effect of WMI turning to Garbage....
		return s, errors.New("Metric/Tagk/Tagv Cleaning Passed a Zero Length String")
	}
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' || r == '/' {
			c += string(r)
		}
		s = s[size:]
	}

	if len(c) == 0 {
		return c, errors.New("Cleaning Metric/Tagk/Tagv resulted in a Zero Length String")
	}
	return c, nil
}

type Request struct {
	Start             interface{} `json:"start"`
	End               interface{} `json:"end,omitempty"`
	Queries           []*Query    `json:"queries"`
	NoAnnotations     bool        `json:"noAnnotations,omitempty"`
	GlobalAnnotations bool        `json:"globalAnnotations,omitempty"`
	MsResolution      bool        `json:"msResolution,omitempty"`
	ShowTSUIDs        bool        `json:"showTSUIDs,omitempty"`
}

type Query struct {
	Aggregator  string `json:"aggregator"`
	Metric      string `json:"metric"`
	Rate        bool   `json:"rate,omitempty"`
	RateOptions `json:"rateOptions,omitempty"`
	Downsample  string `json:"downsample,omitempty"`
	Tags        TagSet `json:"tags,omitempty"`
}

type RateOptions struct {
	Counter    bool `json:"counter,omitempty"`
	CounterMax int  `json:"counterMax,omitempty"`
	ResetValue int  `json:"resetValue,omitempty"`
}

// ParsesRequest parses OpenTSDB requests of the form: start=1h-ago&m=avg:cpu.
func ParseRequest(req string) (*Request, error) {
	v, err := url.ParseQuery(req)
	if err != nil {
		return nil, err
	}
	r := Request{}
	if s := v.Get("start"); s == "" {
		return nil, fmt.Errorf("tsdb: missing start: %s", req)
	} else {
		r.Start = s
	}
	for _, m := range v["m"] {
		q, err := ParseQuery(m)
		if err != nil {
			return nil, err
		}
		r.Queries = append(r.Queries, q)
	}
	if len(r.Queries) == 0 {
		return nil, fmt.Errorf("tsdb: missing m: %s", req)
	}
	return &r, nil
}

var qRE = regexp.MustCompile(`^(\w+):(?:(\w+-\w+):)?(?:(rate):)?([\w./]+)(?:\{([\w./,=*-|]+)\})?$`)

// ParseQuery parses OpenTSDB queries of the form: avg:rate:cpu{k=v}.
func ParseQuery(query string) (*Query, error) {
	q := Query{}
	m := qRE.FindStringSubmatch(query)
	if m == nil {
		return nil, fmt.Errorf("tsdb: bad query format: %s", query)
	}
	q.Aggregator = m[1]
	q.Downsample = m[2]
	q.Rate = m[3] == "rate"
	q.Metric = m[4]
	if m[5] != "" {
		tags, err := ParseTags(m[5])
		if err != nil {
			return nil, err
		}
		q.Tags = tags
	}
	return &q, nil
}

// ParseTags parses OpenTSDB tagk=tagv pairs of the form: k=v,m=o.
func ParseTags(t string) (TagSet, error) {
	ts := make(TagSet)
	for _, v := range strings.Split(t, ",") {
		sp := strings.SplitN(v, "=", 2)
		if len(sp) != 2 {
			return nil, fmt.Errorf("tsdb: bad tag: %s", v)
		}
		ts[sp[0]] = sp[1]
	}
	return ts, nil
}

func (q Query) String() string {
	s := q.Aggregator + ":"
	if q.Downsample != "" {
		s += q.Downsample + ":"
	}
	if q.Rate {
		s += "rate:"
	}
	s += q.Metric
	if len(q.Tags) > 0 {
		s += "{"
		first := true
		for k, v := range q.Tags {
			if first {
				first = false
			} else {
				s += ","
			}
			s += k + "=" + v
		}
		s += "}"
	}
	return s
}

func (r Request) String() string {
	v := make(url.Values)
	for _, q := range r.Queries {
		v.Add("m", q.String())
	}
	v.Add("start", fmt.Sprint(r.Start))
	if e := fmt.Sprint(r.End); r.End != nil && e != "" {
		v.Add("end", e)
	}
	return v.Encode()
}

// Query performs a v2 OpenTSDB request to the given host. host should be of the
// form hostname:port.
func (r Request) Query(host string) (ResponseSet, error) {
	u := url.URL{
		Scheme: "http",
		Host:   host,
		Path:   "/api/query",
	}
	b, err := json.Marshal(&r)
	if err != nil {
		return nil, err
	}
	resp, err := http.Post(u.String(), "application/json", bytes.NewReader(b))
	defer resp.Body.Close()
	if err != nil {
		return nil, err
	} else if resp.StatusCode != http.StatusOK {
		b, err = ioutil.ReadAll(resp.Body)
		fmt.Println(string(b))
		return nil, fmt.Errorf("tsdb: %s", resp.Status)
	}
	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var tr ResponseSet
	if err := json.Unmarshal(b, &tr); err != nil {
		return nil, err
	}
	return tr, nil
}

package opentsdb

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/slog"
)

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
			slog.Infoln(err, "Removing Datapoint", d)
			continue
		}
		md = append(md, d)
	}
	return json.Marshal(md)
}

type MultiDataPoint []*DataPoint

type TagSet map[string]string

func (t TagSet) Copy() TagSet {
	n := make(TagSet)
	for k, v := range t {
		n[k] = v
	}
	return n
}

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
	return fmt.Sprintf("{%s}", t.Tags())
}

// Tags is identical to String() but without { and }.
func (t TagSet) Tags() string {
	var keys []string
	for k := range t {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	b := &bytes.Buffer{}
	for i, k := range keys {
		if i > 0 {
			fmt.Fprint(b, ",")
		}
		fmt.Fprintf(b, "%s=%s", k, t[k])
	}
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

func RequestFromJSON(b []byte) (*Request, error) {
	var r Request
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	r.Start = TryParseAbsTime(r.Start)
	r.End = TryParseAbsTime(r.End)
	return &r, nil
}

type Query struct {
	Aggregator  string      `json:"aggregator"`
	Metric      string      `json:"metric"`
	Rate        bool        `json:"rate,omitempty"`
	RateOptions RateOptions `json:"rateOptions,omitempty"`
	Downsample  string      `json:"downsample,omitempty"`
	Tags        TagSet      `json:"tags,omitempty"`
}

type RateOptions struct {
	Counter    bool  `json:"counter,omitempty"`
	CounterMax int64 `json:"counterMax,omitempty"`
	ResetValue int64 `json:"resetValue,omitempty"`
}

// ParsesRequest parses OpenTSDB requests of the form: start=1h-ago&m=avg:cpu.
func ParseRequest(req string) (*Request, error) {
	v, err := url.ParseQuery(req)
	if err != nil {
		return nil, err
	}
	r := Request{}
	if s := v.Get("start"); s == "" {
		return nil, fmt.Errorf("opentsdb: missing start: %s", req)
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
		return nil, fmt.Errorf("opentsdb: missing m: %s", req)
	}
	return &r, nil
}

var qRE = regexp.MustCompile(`^(\w+):(?:(\w+-\w+):)?(?:(rate.*):)?([\w./]+)(?:\{([\w./,=*-|]+)\})?$`)

// ParseQuery parses OpenTSDB queries of the form: avg:rate:cpu{k=v}.
func ParseQuery(query string) (q *Query, err error) {
	q = new(Query)
	m := qRE.FindStringSubmatch(query)
	if m == nil {
		return nil, fmt.Errorf("opentsdb: bad query format: %s", query)
	}
	q.Aggregator = m[1]
	q.Downsample = m[2]
	q.Rate = strings.HasPrefix(m[3], "rate")
	if q.Rate && len(m[3]) > 4 {
		s := m[3][4:]
		if !strings.HasSuffix(s, "}") || !strings.HasPrefix(s, "{") {
			err = fmt.Errorf("opentsdb: invalid rate options")
			return
		}
		sp := strings.Split(s[1:len(s)-1], ",")
		q.RateOptions.Counter = sp[0] == "counter"
		if len(sp) > 1 {
			if sp[1] != "" {
				if q.RateOptions.CounterMax, err = strconv.ParseInt(sp[1], 10, 64); err != nil {
					return
				}
			}
		}
		if len(sp) > 2 {
			if q.RateOptions.ResetValue, err = strconv.ParseInt(sp[2], 10, 64); err != nil {
				return
			}
		}
	}
	q.Metric = m[4]
	if m[5] != "" {
		tags, e := ParseTags(m[5])
		if e != nil {
			err = e
			return
		}
		q.Tags = tags
	}
	return
}

// ParseTags parses OpenTSDB tagk=tagv pairs of the form: k=v,m=o.
func ParseTags(t string) (TagSet, error) {
	ts := make(TagSet)
	for _, v := range strings.Split(t, ",") {
		sp := strings.SplitN(v, "=", 2)
		if len(sp) != 2 {
			return nil, fmt.Errorf("opentsdb: bad tag: %s", v)
		}
		sp[0] = strings.TrimSpace(sp[0])
		sp[1] = strings.TrimSpace(sp[1])
		if _, present := ts[sp[0]]; present {
			return nil, fmt.Errorf("opentsdb: duplicated tag: %s", v)
		}
		ts[sp[0]] = sp[1]
	}
	return ts, nil
}

var groupRE = regexp.MustCompile("{[^}]+}")

// ReplaceTags replaces all tag-like strings with tags from the given
// group. For example, given the string "test.metric{host=*}" and a TagSet
// with host=test.com, this returns "test.metric{host=test.com}".
func ReplaceTags(text string, group TagSet) string {
	return groupRE.ReplaceAllStringFunc(text, func(s string) string {
		tags, err := ParseTags(s[1 : len(s)-1])
		if err != nil {
			return s
		}
		for k := range tags {
			if group[k] != "" {
				tags[k] = group[k]
			}
		}
		return fmt.Sprintf("{%s}", tags.Tags())
	})
}

func (q Query) String() string {
	s := q.Aggregator + ":"
	if q.Downsample != "" {
		s += q.Downsample + ":"
	}
	if q.Rate {
		s += "rate"
		if q.RateOptions.Counter {
			s += "{counter"
			if q.RateOptions.CounterMax != 0 {
				s += ","
				s += strconv.FormatInt(q.RateOptions.CounterMax, 10)
			}
			if q.RateOptions.ResetValue != 0 {
				if q.RateOptions.CounterMax == 0 {
					s += ","
				}
				s += ","
				s += strconv.FormatInt(q.RateOptions.ResetValue, 10)
			}
			s += "}"
		}
		s += ":"
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

func (r *Request) String() string {
	v := make(url.Values)
	for _, q := range r.Queries {
		v.Add("m", q.String())
	}
	if start, err := CanonicalTime(r.Start); err == nil {
		v.Add("start", start)
	}
	if end, err := CanonicalTime(r.End); err == nil {
		v.Add("end", end)
	}
	return v.Encode()
}

// Search returns a string suitable for OpenTSDB's `/` route.
func (r *Request) Search() string {
	// OpenTSDB uses the URL hash, not search parameters, to do this. The values are
	// not URL encoded. So it's the same as a url.Values just left as normal
	// strings.
	v, err := url.ParseQuery(r.String())
	if err != nil {
		return ""
	}
	buf := &bytes.Buffer{}
	for k, values := range v {
		for _, value := range values {
			fmt.Fprintf(buf, "%s=%s&", k, value)
		}
	}
	return buf.String()
}

const TSDBTimeFormat = "2006/01/02-15:04:05"

// CanonicalTime converts v to a string for use with OpenTSDB's `/` route.
func CanonicalTime(v interface{}) (string, error) {
	if s, ok := v.(string); ok {
		if strings.HasSuffix(s, "-ago") {
			return s, nil
		}
	}
	t, err := ParseTime(v)
	if err != nil {
		return "", err
	}
	return t.Format(TSDBTimeFormat), nil
}

func TryParseAbsTime(v interface{}) interface{} {
	if s, ok := v.(string); ok {
		d, err := ParseAbsTime(s)
		if err == nil {
			return d.Unix()
		}
	}
	return v
}

// ParseAbsTime returns the time of s, which must be of any non-relative (not
// "X-ago") format supported by OpenTSDB.
func ParseAbsTime(s string) (time.Time, error) {
	var t time.Time
	t_formats := [4]string{
		"2006/01/02-15:04:05",
		"2006/01/02-15:04",
		"2006/01/02-15",
		"2006/01/02",
	}
	for _, f := range t_formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return t, err
	}
	return time.Unix(i, 0), nil
}

// ParseTime returns the time of v, which can be of any format supported by
// OpenTSDB.
func ParseTime(v interface{}) (time.Time, error) {
	now := time.Now().UTC()
	switch i := v.(type) {
	case string:
		if i != "" {
			if strings.HasSuffix(i, "-ago") {
				s := strings.TrimSuffix(i, "-ago")
				d, err := ParseDuration(s)
				if err != nil {
					return now, err
				}
				return now.Add(time.Duration(-d)), nil
			} else {
				return ParseAbsTime(i)
			}
		} else {
			return now, nil
		}
	case int64:
		return time.Unix(i, 0).UTC(), nil
	case float64:
		return time.Unix(int64(i), 0).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("type must be string or int64, got: %v", v)
	}
}

// GetDuration returns the duration from the request's start to end.
func GetDuration(r *Request) (Duration, error) {
	var t Duration
	if v, ok := r.Start.(string); ok && v == "" {
		return t, errors.New("start time must be provided")
	}
	start, err := ParseTime(r.Start)
	if err != nil {
		return t, err
	}
	var end time.Time
	if r.End != nil {
		end, err = ParseTime(r.End)
		if err != nil {
			return t, err
		}
	} else {
		end = time.Now()
	}
	t = Duration(end.Sub(start))
	return t, nil
}

// AutoDownsample sets the avg downsample aggregator to produce l points.
func (r *Request) AutoDownsample(l int) error {
	if l == 0 {
		return errors.New("opentsdb: target length must be > 0")
	}
	cd, err := GetDuration(r)
	if err != nil {
		return err
	}
	d := cd / Duration(l)
	ds := ""
	if d > Duration(time.Second)*15 {
		ds = fmt.Sprintf("%ds-avg", int64(d.Seconds()))
	}
	for _, q := range r.Queries {
		q.Downsample = ds
	}
	return nil
}

// SetTime adjusts the start and end time of the request to assume t is now.
// Relative times ("1m-ago") are changed to absolute times. Existing absolute
// times are adjusted by the difference between time.Now() and t.
func (r *Request) SetTime(t time.Time) error {
	diff := -time.Since(t)
	start, err := ParseTime(r.Start)
	if err != nil {
		return err
	}
	r.Start = start.Add(diff).Format(TSDBTimeFormat)
	if r.End != nil {
		end, err := ParseTime(r.End)
		if err != nil {
			return err
		}
		r.End = end.Add(diff).Format(TSDBTimeFormat)
	} else {
		r.End = t.UTC().Format(TSDBTimeFormat)
	}
	return nil
}

// Query performs a v2 OpenTSDB request to the given host. host should be of the
// form hostname:port. Can return a RequestError.
func (r *Request) Query(host string) (ResponseSet, error) {
	resp, err := r.QueryResponse(host)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	j := json.NewDecoder(resp.Body)
	var tr ResponseSet
	if err := j.Decode(&tr); err != nil {
		return nil, err
	}
	return tr, nil
}

func (r *Request) QueryResponse(host string) (*http.Response, error) {
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
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		e := RequestError{Request: string(b)}
		defer resp.Body.Close()
		j := json.NewDecoder(resp.Body)
		if err := j.Decode(&e); err == nil {
			return nil, &e
		}
		return nil, fmt.Errorf("opentsdb: %s", b)
	}
	return resp, nil
}

type RequestError struct {
	Request string
	Err     struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Details string `json:"details"`
	} `json:"error"`
}

func (r *RequestError) Error() string {
	return fmt.Sprintf("opentsdb: %s: %s", r.Request, r.Err.Message)
}

type Context interface {
	Query(*Request) (ResponseSet, error)
}

type Host string

func (h Host) Query(r *Request) (ResponseSet, error) {
	return r.Query(string(h))
}

type Cache struct {
	Host string
	// Limit limits response size in bytes
	Limit int64
	// FilterTags removes tagks from results if that tagk was not in the request
	FilterTags bool
	cache      map[string]*cacheResult
}

type cacheResult struct {
	ResponseSet
	Err error
}

func NewCache(host string, limit int64) *Cache {
	return &Cache{
		Host:       host,
		Limit:      limit,
		FilterTags: true,
		cache:      make(map[string]*cacheResult),
	}
}

func (c *Cache) Query(r *Request) (tr ResponseSet, err error) {
	b, err := json.Marshal(&r)
	if err != nil {
		return nil, err
	}
	s := string(b)
	if v, ok := c.cache[s]; ok {
		return v.ResponseSet, v.Err
	}
	defer func() {
		c.cache[s] = &cacheResult{tr, err}
	}()
	resp, err := r.QueryResponse(c.Host)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	lr := &io.LimitedReader{R: resp.Body, N: c.Limit}
	j := json.NewDecoder(lr)
	err = j.Decode(&tr)
	if lr.N == 0 {
		err = fmt.Errorf("TSDB response too large: limited to %E bytes", float64(c.Limit))
		return
	}
	if err != nil {
		return
	}
	if c.FilterTags {
		FilterTags(r, tr)
	}
	return
}

// FilterTags removes tagks in tr not present in r. Does nothing in the event of
// multiple queries in the request.
func FilterTags(r *Request, tr ResponseSet) {
	if len(r.Queries) != 1 {
		return
	}
	for _, resp := range tr {
		for k := range resp.Tags {
			if _, present := r.Queries[0].Tags[k]; !present {
				delete(resp.Tags, k)
			}
		}
	}
}

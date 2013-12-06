package opentsdb

import (
	"encoding/json"
	"fmt"
	"unicode"
	"unicode/utf8"
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
	for _, d := range m {
		d.clean()
	}
	return json.Marshal(m)
}

type MultiDataPoint []*DataPoint

type TagSet map[string]string

func (d *DataPoint) clean() {
	d.Tags.clean()
	d.Metric = Clean(d.Metric)
}

func (t TagSet) clean() {
	for k, v := range t {
		kc := Clean(k)
		vc := Clean(v)
		delete(t, k)
		t[kc] = vc
	}
}

// Clean removes characters from s that are invalid for OpenTSDB metric and tag
// values.
// See: http://opentsdb.net/docs/build/html/user_guide/writing.html#metrics-and-tags
func Clean(s string) string {
	var c string
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' || r == '/' {
			c += string(r)
		}
		s = s[size:]
	}
	return c
}

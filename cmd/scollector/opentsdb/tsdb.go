package opentsdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
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

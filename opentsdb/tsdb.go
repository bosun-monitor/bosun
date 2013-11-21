package opentsdb

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
)

type ResponseSet []*Response

type Point float64

type Response struct {
	Metric        string            `json:"metric"`
	Tags          map[string]string `json:"tags"`
	AggregateTags []string          `json:"aggregateTags"`
	DPS           map[string]Point  `json:"dps"`
}

type DataPoint struct {
	Metric    string            `json:"metric"`
	Timestamp int64             `json:"timestamp"`
	Value     interface{}       `json:"value"`
	Tags      map[string]string `json:"tags"`
}

func (d *DataPoint) Telnet() string {
	m := ""
	for k, v := range d.Tags {
		m += fmt.Sprintf(" %s=%s", k, v)
	}
	return fmt.Sprintf("put %s %d %v%s\n", d.Metric, d.Timestamp, d.Value, m)
}

func (d *DataPoint) Json() io.Reader {
	b, err := json.Marshal(d)
	if err != nil {
		log.Fatal(err)
	}
	return bytes.NewReader(b)
}

type MultiDataPoint []*DataPoint

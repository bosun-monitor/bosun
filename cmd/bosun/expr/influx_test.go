package expr

import (
	"fmt"
	"testing"
	"time"

	"bosun.org/opentsdb"
	"github.com/influxdata/influxdb/client/v2"
)

const influxTimeFmt = time.RFC3339Nano

func TestInfluxQueryDuration(t *testing.T) {
	type influxTest struct {
		query  string
		gbi    string // group by interval
		expect string // empty for error
	}
	date := time.Date(2000, time.January, 1, 2, 0, 0, 0, time.UTC)
	dur := time.Hour
	end := date.Format(influxTimeFmt)
	start := date.Add(-dur).Format(influxTimeFmt)
	tests := []influxTest{
		{
			"select * from a", "",
			fmt.Sprintf("SELECT * FROM a WHERE time >= '%s' AND time <= '%s' fill(none)", start, end),
		},
		{
			"select * from a WHERE value > 0", "",
			fmt.Sprintf("SELECT * FROM a WHERE value > 0 AND time >= '%s' AND time <= '%s' fill(none)", start, end),
		},
		{
			"select * from a WHERE value > 0", "15m",
			fmt.Sprintf("SELECT * FROM a WHERE value > 0 AND time >= '%s' AND time <= '%s' GROUP BY time(15m) fill(none)", start, end),
		},
		{
			"select NON_NEGATIVE_DERIVATIVE(SUM(value)) from a WHERE value > 0", "15m",
			fmt.Sprintf("SELECT non_negative_derivative(sum(value)) FROM a WHERE value > 0 AND time >= '%s' AND time <= '%s' GROUP BY time(15m) fill(none)", start, end),
		},
		{
			"select * from a WHERE time > 0 fill(none)", "",
			"",
		},
	}
	for _, test := range tests {
		q, err := influxQueryDuration(date, test.query, dur.String(), "", test.gbi)
		if err != nil && test.expect != "" {
			t.Errorf("%v: unexpected error: %v", test.query, err)
		} else if q != test.expect {
			t.Errorf("%v: \n\texpected: %v\n\tgot: %v", test.query, test.expect, q)
		}
	}
}

func TestInfluxQuery(t *testing.T) {
	e := State{
		now: time.Date(2015, time.February, 25, 0, 0, 0, 0, time.UTC),
		Backends: &Backends{
			InfluxConfig: client.HTTPConfig{},
		},
		BosunProviders: &BosunProviders{
			Squelched: func(tags opentsdb.TagSet) bool {
				return false
			},
		},
	}
	_, err := InfluxQuery(&e, "db", "select * from alh limit 10", "1n", "", "")
	if err == nil {
		t.Fatal("Should have received an error from InfluxQuery")
	}
}

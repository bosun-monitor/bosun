package sched

import (
	"testing"
	"time"

	"bosun.org/models"
	"bosun.org/opentsdb"
)

// Crit returns {a=b},{a=c}, but {a=b} is ignored by dependency expression.
// Result should be {a=c} only.
func TestDependency_Simple(t *testing.T) {
	defer setup()()
	testSched(t, &schedTest{
		conf: `alert a {
			crit = avg(q("avg:c{a=*}", "5m", "")) > 0
			depends = avg(q("avg:d{a=*}", "5m", "")) > 0
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("avg:c{a=*}", ` + window5Min + `)`: {
				{
					Metric: "c",
					Tags:   opentsdb.TagSet{"a": "b"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
				{
					Metric: "c",
					Tags:   opentsdb.TagSet{"a": "c"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
			},
			`q("avg:d{a=*}", ` + window5Min + `)`: {
				{
					Metric: "d",
					Tags:   opentsdb.TagSet{"a": "b"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
				{
					Metric: "d",
					Tags:   opentsdb.TagSet{"a": "c"},
					DPS:    map[string]opentsdb.Point{"0": 0},
				},
			},
		},
		state: map[schedState]bool{
			{"a{a=c}", "critical"}: true,
		},
	})
}

// Crit and depends don't have same tag sets.
func TestDependency_Overlap(t *testing.T) {
	defer setup()()
	testSched(t, &schedTest{
		conf: `alert a {
			crit = avg(q("avg:c{a=*,b=*}", "5m", "")) > 0
			depends = avg(q("avg:d{a=*,d=*}", "5m", "")) > 0
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("avg:c{a=*,b=*}", ` + window5Min + `)`: {
				{
					Metric: "c",
					Tags:   opentsdb.TagSet{"a": "b", "b": "r"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
				{
					Metric: "c",
					Tags:   opentsdb.TagSet{"a": "b", "b": "z"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
				{
					Metric: "c",
					Tags:   opentsdb.TagSet{"a": "c", "b": "q"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
			},
			`q("avg:d{a=*,d=*}", ` + window5Min + `)`: {
				{
					Metric: "d",
					Tags:   opentsdb.TagSet{"a": "b", "d": "q"}, //this matches first and second datapoints from crit.
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
			},
		},
		state: map[schedState]bool{
			{"a{a=c,b=q}", "critical"}: true,
		},
	})
}

func TestDependency_OtherAlert(t *testing.T) {
	defer setup()()
	testSched(t, &schedTest{
		conf: `alert a {
			crit = avg(q("avg:a{host=*,cpu=*}", "5m", "")) > 0
		}
		alert b{
			depends = alert("a","crit")
			crit = avg(q("avg:b{host=*}", "5m", "")) > 0
		}
		alert c{
			crit = avg(q("avg:b{host=*}", "5m", "")) > 0
		}
		alert d{
			#b will be unevaluated because of a.
			depends = alert("b","crit")
			crit = avg(q("avg:b{host=*}", "5m", "")) > 0
		}
		`,
		queries: map[string]opentsdb.ResponseSet{
			`q("avg:a{cpu=*,host=*}", ` + window5Min + `)`: {
				{
					Metric: "a",
					Tags:   opentsdb.TagSet{"host": "ny01", "cpu": "0"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
			},
			`q("avg:b{host=*}", ` + window5Min + `)`: {
				{
					Metric: "b",
					Tags:   opentsdb.TagSet{"host": "ny01"},
					DPS:    map[string]opentsdb.Point{"0": 1},
				},
			},
		},
		state: map[schedState]bool{
			{"a{cpu=0,host=ny01}", "critical"}: true,
			{"c{host=ny01}", "critical"}:       true,
		},
	})
}

func TestDependency_OtherAlert_Unknown(t *testing.T) {
	defer setup()()

	testSched(t, &schedTest{
		conf: `alert a {
			warn = avg(q("avg:a{host=*}", "5m", "")) > 0
		}

	alert os.cpu {
    	depends = alert("a", "warn")
    	warn = avg(q("avg:os.cpu{host=*}", "5m", "")) > 5
	}
		`,
		queries: map[string]opentsdb.ResponseSet{
			`q("avg:a{host=*}", ` + window5Min + `)`: {
				{
					Metric: "a",
					Tags:   opentsdb.TagSet{"host": "ny01"},
					DPS:    map[string]opentsdb.Point{"0": 0},
				},
				//no results for ny02. Goes unkown here.
			},
			`q("avg:os.cpu{host=*}", ` + window5Min + `)`: {
				{
					Metric: "os.cpu",
					Tags:   opentsdb.TagSet{"host": "ny01"},
					DPS:    map[string]opentsdb.Point{"0": 10},
				},
				{
					Metric: "os.cpu",
					Tags:   opentsdb.TagSet{"host": "ny02"},
					DPS:    map[string]opentsdb.Point{"0": 10},
				},
			},
		},
		state: map[schedState]bool{
			{"a{host=ny02}", "unknown"}:      true,
			{"os.cpu{host=ny01}", "warning"}: true,
		},
		touched: map[models.AlertKey]time.Time{
			"a{host=ny02}": queryTime.Add(-10 * time.Minute),
		},
	})
}

func TestDependency_OtherAlert_UnknownChain(t *testing.T) {
	defer setup()()
	ab := models.AlertKey("a{host=b}")
	bb := models.AlertKey("b{host=b}")
	cb := models.AlertKey("c{host=b}")

	s := testSched(t, &schedTest{
		conf: `
		alert a {
			warn = avg(q("avg:a{host=*}", "5m", "")) && 0
		}

		alert b {
			depends = alert("a", "warn")
			warn = avg(q("avg:b{host=*}", "5m", "")) > 0 
		}

		alert c {
			depends = alert("b", "warn")
			warn = avg(q("avg:b{host=*}", "5m", "")) > 0
		}
		`,
		queries: map[string]opentsdb.ResponseSet{
			`q("avg:a{host=*}", ` + window5Min + `)`: {},
			`q("avg:b{host=*}", ` + window5Min + `)`: {{
				Metric: "b",
				Tags:   opentsdb.TagSet{"host": "b"},
				DPS:    map[string]opentsdb.Point{"0": 0},
			}},
		},
		state: map[schedState]bool{
			{string(ab), "unknown"}: true,
		},
		touched: map[models.AlertKey]time.Time{
			ab: queryTime.Add(-time.Hour),
			bb: queryTime,
			cb: queryTime,
		},
	})
	check := func(ak models.AlertKey, expec bool) {
		_, uneval, err := s.DataAccess.State().GetUnknownAndUnevalAlertKeys(ak.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, ak2 := range uneval {
			if ak2 == ak {
				if !expec {
					t.Fatalf("Should not be unevaluated: %s", ak)
				} else {
					return
				}
			}
		}
		if expec {
			t.Fatalf("Should be unevaluated: %s", ak)
		}
	}
	check(ab, false)
	check(bb, true)
	check(cb, true)
}

func TestDependency_Blocks_Unknown(t *testing.T) {
	defer setup()()
	testSched(t, &schedTest{
		conf: `alert a {
			depends = avg(q("avg:b{host=*}", "5m", "")) > 0
			warn = avg(q("avg:a{host=*}", "5m", "")) > 0
		}`,
		queries: map[string]opentsdb.ResponseSet{
			`q("avg:a{host=*}", ` + window5Min + `)`: {
				//no results for a. Goes unkown here.
			},
			`q("avg:b{host=*}", ` + window5Min + `)`: {
				{
					Metric: "os.cpu",
					Tags:   opentsdb.TagSet{"host": "ny01"},
					DPS:    map[string]opentsdb.Point{"0": 10},
				},
			},
		},
		state: map[schedState]bool{},
		touched: map[models.AlertKey]time.Time{
			"a{host=ny01}": queryTime.Add(-10 * time.Minute),
		},
	})
}

func TestDependency_AlertFunctionHasNoResults(t *testing.T) {
	defer setup()()

	testSched(t, &schedTest{
		conf: `
alert a {
    warn = max(rename(q("sum:bosun.ping.timeout{dst_host=*,host=*}", "5m", ""), "host=source,dst_host=host"))
}

alert b {
	depends = alert("a", "warn")
	warn = avg(q("avg:os.cpu{host=*}", "5m", "")) < -100
}

alert c {
    depends = alert("b", "warn")
    warn = avg(q("avg:rate{counter,,1}:os.cpu{host=*}", "5m", ""))
}
`,
		queries: map[string]opentsdb.ResponseSet{
			`q("sum:bosun.ping.timeout{dst_host=*,host=*}", ` + window5Min + `)`: {
				{
					Metric: "bosun.ping.timeout",
					Tags:   opentsdb.TagSet{"host": "bosun01", "dst_host": "ny01"},
					DPS:    map[string]opentsdb.Point{"0": 1}, //ping fails
				},
			},
			`q("avg:os.cpu{host=*}", ` + window5Min + `)`:                  {}, //no other data
			`q("avg:rate{counter,,1}:os.cpu{host=*}", ` + window5Min + `)`: {},
		},
		state: map[schedState]bool{
			{"a{host=ny01,source=bosun01}", "warning"}: true,
		},
		touched: map[models.AlertKey]time.Time{
			"a{host=ny01,source=bosun01}": queryTime.Add(-5 * time.Minute),
			"b{host=ny01}":                queryTime.Add(-10 * time.Minute),
			"c{host=ny01}":                queryTime.Add(-10 * time.Minute),
		},
	})
}

package expr

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/graphite"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
)

// Graphite defines functions for use with a Graphite backend.
var Graphite = map[string]parse.Func{
	"graphiteBand": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeScalar},
		Return: models.TypeSeriesSet,
		Tags:   graphiteTagQuery,
		F:      GraphiteBand,
	},
	"graphite": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeSeriesSet,
		Tags:   graphiteTagQuery,
		F:      GraphiteQuery,
	},
}

func parseGraphiteResponse(req *graphite.Request, s *graphite.Response, formatTags []string) ([]*Result, error) {
	const parseErrFmt = "graphite ParseError (%s): %s"
	if len(*s) == 0 {
		return nil, fmt.Errorf(parseErrFmt, req.URL, "empty response")
	}
	seen := make(map[string]bool)
	results := make([]*Result, 0)
	for _, res := range *s {
		// build tag set
		tags := make(opentsdb.TagSet)
		if len(formatTags) == 1 && formatTags[0] == "" {
			tags["key"] = res.Target
		} else {
			nodes := strings.Split(res.Target, ".")
			if len(nodes) < len(formatTags) {
				msg := fmt.Sprintf("returned target '%s' does not match format '%s'", res.Target, strings.Join(formatTags, ","))
				return nil, fmt.Errorf(parseErrFmt, req.URL, msg)
			}
			for i, key := range formatTags {
				if len(key) > 0 {
					tags[key] = nodes[i]
				}
			}
		}
		if !tags.Valid() {
			msg := fmt.Sprintf("returned target '%s' would make an invalid tag '%s'", res.Target, tags.String())
			return nil, fmt.Errorf(parseErrFmt, req.URL, msg)
		}
		if ts := tags.String(); !seen[ts] {
			seen[ts] = true
		} else {
			return nil, fmt.Errorf(parseErrFmt, req.URL, fmt.Sprintf("More than 1 series identified by tagset '%v'", ts))
		}
		// build data
		dps := make(Series)
		for _, dp := range res.Datapoints {
			if len(dp) != 2 {
				return nil, fmt.Errorf(parseErrFmt, req.URL, fmt.Sprintf("Datapoint has != 2 fields: %v", dp))
			}
			if len(dp[0].String()) == 0 {
				// none value. skip this record
				continue
			}
			val, err := dp[0].Float64()
			if err != nil {
				msg := fmt.Sprintf("value '%s' cannot be decoded to Float64: %s", dp[0], err.Error())
				return nil, fmt.Errorf(parseErrFmt, req.URL, msg)
			}
			unixTS, err := dp[1].Int64()
			if err != nil {
				msg := fmt.Sprintf("timestamp '%s' cannot be decoded to Int64: %s", dp[1], err.Error())
				return nil, fmt.Errorf(parseErrFmt, req.URL, msg)
			}
			t := time.Unix(unixTS, 0)
			dps[t] = val
		}
		results = append(results, &Result{
			Value: dps,
			Group: tags,
		})
	}
	return results, nil
}

func GraphiteBand(e *State, T miniprofiler.Timer, query, duration, period, format string, num float64) (r *Results, err error) {
	r = new(Results)
	r.IgnoreOtherUnjoined = true
	r.IgnoreUnjoined = true
	T.Step("graphiteBand", func(T miniprofiler.Timer) {
		var d, p opentsdb.Duration
		d, err = opentsdb.ParseDuration(duration)
		if err != nil {
			return
		}
		p, err = opentsdb.ParseDuration(period)
		if err != nil {
			return
		}
		if num < 1 || num > 100 {
			err = fmt.Errorf("expr: Band: num out of bounds")
		}
		req := &graphite.Request{
			Targets: []string{query},
		}
		now := e.now
		req.End = &now
		st := e.now.Add(-time.Duration(d))
		req.Start = &st
		for i := 0; i < int(num); i++ {
			now = now.Add(time.Duration(-p))
			req.End = &now
			st := now.Add(time.Duration(-d))
			req.Start = &st
			var s graphite.Response
			s, err = timeGraphiteRequest(e, T, req)
			if err != nil {
				return
			}
			formatTags := strings.Split(format, ".")
			var results []*Result
			results, err = parseGraphiteResponse(req, &s, formatTags)
			if err != nil {
				return
			}
			if i == 0 {
				r.Results = results
			} else {
				// different graphite requests might return series with different id's.
				// i.e. a different set of tagsets.  merge the data of corresponding tagsets
				for _, result := range results {
					updateKey := -1
					for j, existing := range r.Results {
						if result.Group.Equal(existing.Group) {
							updateKey = j
							break
						}
					}
					if updateKey == -1 {
						// result tagset is new
						r.Results = append(r.Results, result)
						updateKey = len(r.Results) - 1
					}
					for k, v := range result.Value.(Series) {
						r.Results[updateKey].Value.(Series)[k] = v
					}
				}
			}
		}
	})
	if err != nil {
		return nil, fmt.Errorf("graphiteBand: %v", err)
	}
	return
}

func GraphiteQuery(e *State, T miniprofiler.Timer, query string, sduration, eduration, format string) (r *Results, err error) {
	sd, err := opentsdb.ParseDuration(sduration)
	if err != nil {
		return
	}
	ed := opentsdb.Duration(0)
	if eduration != "" {
		ed, err = opentsdb.ParseDuration(eduration)
		if err != nil {
			return
		}
	}
	st := e.now.Add(-time.Duration(sd))
	et := e.now.Add(-time.Duration(ed))
	req := &graphite.Request{
		Targets: []string{query},
		Start:   &st,
		End:     &et,
	}
	s, err := timeGraphiteRequest(e, T, req)
	if err != nil {
		return nil, err
	}
	formatTags := strings.Split(format, ".")
	r = new(Results)
	results, err := parseGraphiteResponse(req, &s, formatTags)
	if err != nil {
		return nil, err
	}
	r.Results = results

	return
}

func graphiteTagQuery(args []parse.Node) (parse.Tags, error) {
	t := make(parse.Tags)
	n := args[3].(*parse.StringNode)
	for _, s := range strings.Split(n.Text, ".") {
		if s != "" {
			t[s] = struct{}{}
		}
	}
	return t, nil
}

func timeGraphiteRequest(e *State, T miniprofiler.Timer, req *graphite.Request) (resp graphite.Response, err error) {
	e.graphiteQueries = append(e.graphiteQueries, *req)
	b, _ := json.MarshalIndent(req, "", "  ")
	T.StepCustomTiming("graphite", "query", string(b), func() {
		key := req.CacheKey()
		getFn := func() (interface{}, error) {
			return e.GraphiteContext.Query(req)
		}
		var val interface{}
		var hit bool
		val, err, hit = e.Cache.Get(key, getFn)
		collectCacheHit(e.Cache, "graphite", hit)
		resp = val.(graphite.Response)
	})
	return
}

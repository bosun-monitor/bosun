package opentsdb

import (
	"fmt"
	"strings"

	"time"

	"github.com/influxdata/influxdb/influxql"
)

// Example Requests for Reference , DELETE Later
// {
//   "start": "1h-ago",
//   "queries": [
//     {
//       "aggregator": "sum",
//       "metric": "os.net.bytes",
//       "rate": true,
//       "rateOptions": {
//         "counter": true,
//         "resetValue": 1
//       },
//       "downsample": "5m-max",
//       "filters": [
//         {
//           "type": "literal_or",
//           "tagk": "host",
//           "filter": "ny-nexus01",
//           "groupBy": true
//         },
//         {
//           "type": "wildcard",
//           "tagk": "iname",
//           "filter": "*",
//           "groupBy": true
//         }
//       ]
//     }
//   ]
// }

// Absolute time and end time:
// {
//   "start": 1495114436,
//   "end": 1495116236,
//   "queries": [
//     {
//       "aggregator": "sum",
//       "metric": "os.net.bytes",
//       "rate": true,
//       "rateOptions": {
//         "counter": true,
//         "resetValue": 1
//       },
//       "downsample": "5m-max",
//       "filters": [
//         {
//           "type": "literal_or",
//           "tagk": "host",
//           "filter": "ny-nexus01",
//           "groupBy": true
//         },
//         {
//           "type": "wildcard",
//           "tagk": "iname",
//           "filter": "*",
//           "groupBy": true
//         }
//       ]
//     }
//   ]
// }

// Order of operations to follow:
// Rate Calculation
// Where
// Downsample
// Groupby with Aggregation

// FluxQLQueryString Generates a InfluxQL Query string from an OpenTSDB request
// For now, will error if there is more than one query in the request
func (r Request) FluxQLQueryString() (string, error) {
	if len(r.Queries) != 1 {
		return "", fmt.Errorf("OpenTSDB to Influx: only requests with single queries are currently supported")
	}
	oq := r.Queries[0]
	var build string
	var where string
	//gbFilters, whereFilters := oq.Filters.ByGroupBy()
	if oq.Rate {
		// Make subquery
		if oq.RateOptions.Counter {
			var err error
			where, err = oq.Filters.WhereClause(r)
			if err != nil {
				return "", err
			}
			build = fmt.Sprintf(`SELECT NON_NEGATIVE_DERIVATIVE(*) FROM "%v" %v GROUP BY *`, oq.Metric, where)
		} else {
			return "", fmt.Errorf("only counter rate queries are supported right now")
			// derivative in sub query
			// Attach Where filters
		}
	} else {
		return "", fmt.Errorf("only rate queries supported right now")
	}
	if oq.Downsample != "" {
		f, w, err := oq.FluxDSTransform()
		if err != nil {
			return "", err
		}
		build = fmt.Sprintf("SELECT %v(*) from (%v) %v GROUP BY *,time(%v)", f, build, where, w)
	}
	agg, err := tsdbToFluxFunc(oq.Aggregator)
	if err != nil {
		return "", err
	}
	gbc, err := oq.Filters.GroupByClause()
	if err != nil {
		return "", err
	}
	build = fmt.Sprintf("SELECT %v(*) FROM (%v) %v", agg, build, gbc)
	return build, nil
}

func (q Query) FluxDSTransform() (string, string, error) {
	sp := strings.Split(q.Downsample, "-")
	if len(sp) != 2 {
		return "", "", fmt.Errorf("unexpected downsample string: %v", q.Downsample)
	}
	win := sp[0] // might have some transforms TODO
	function, err := tsdbToFluxFunc(sp[1])
	if err != nil {
		return "", "", err
	}
	return function, win, nil
}

func tsdbToFluxFunc(f string) (string, error) {
	var function string
	switch f {
	case "avg":
		function = "MEAN"
	case "max":
		function = "MAX"
	case "min":
		function = "MIN"
	case "sum":
		function = "SUM"
	default:
		return "", fmt.Errorf("unsupported downsampling function %v", f)
	}
	return function, nil
}

func (filters Filters) GroupByClause() (string, error) {
	var gb []string
	for _, filter := range filters {
		if filter.GroupBy {
			gb = append(gb, filter.TagK)
		}
	}
	if len(gb) != 0 {
		return fmt.Sprintf("GROUP BY %v", strings.Join(gb, ",")), nil
	}
	return "", nil
}

func (filters Filters) WhereClause(r Request) (string, error) {
	conditions := []string{}
	for _, filter := range filters {
		fieldKey := filter.TagK
		var operator influxql.Token
		var value = filter.Filter
		switch filter.Type {
		case "wildcard":
			operator = influxql.EQREGEX
			value = fmt.Sprintf("/%v/", strings.Replace(filter.Filter, "*", ".*", -1))
		case "iwildcard":
			operator = influxql.EQREGEX
			value = fmt.Sprintf("/(?i)%v/", strings.Replace(filter.Filter, "*", ".*", -1))
		default:
			return "", fmt.Errorf("filter type %v not implemented", filter.Type)
		}
		condition := fmt.Sprintf(`%v %v %v`, fieldKey, operator.String(), value)
		conditions = append(conditions, condition)
	}
	tagConditions := strings.Join(conditions, fmt.Sprintf(" %v ", influxql.AND.String()))
	start, err := ParseTime(r.Start)
	if err != nil {
		return "", err
	}
	end, err := ParseTime(r.End)
	if err != nil {
		return "", err
	}
	timeCondition := fmt.Sprintf("time %v '%v' %v time %v '%v'", influxql.GTE, start.Format(time.RFC3339), influxql.AND, influxql.LTE, end.Format(time.RFC3339))
	return fmt.Sprintf("WHERE %v %v %v", tagConditions, influxql.AND, timeCondition), nil
}

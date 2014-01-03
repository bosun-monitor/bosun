package expr

import "github.com/StackExchange/tsaf/expr/parse"

const (
	DefDuration = "1h"
	DefPeriod   = "1w"
	DefNum      = 8
)

var Builtins = map[string]parse.Func{
	"avg": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		[]interface{}{DefDuration},
		avg,
	},
	"band": {
		[]parse.FuncType{parse.TYPE_QUERY, parse.TYPE_STRING, parse.TYPE_STRING, parse.TYPE_NUMBER},
		parse.TYPE_SERIES,
		[]interface{}{DefDuration, DefPeriod, DefNum},
		band,
	},
	"dev": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		[]interface{}{DefDuration},
		dev,
	},
	"recent": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		[]interface{}{DefDuration},
		recent,
	},
}

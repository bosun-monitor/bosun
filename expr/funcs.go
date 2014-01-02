package expr

import "github.com/StackExchange/tsaf/expr/parse"

var Builtins = map[string]parse.Func{
	"avg": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		1,
	},
	"band": {
		[]parse.FuncType{parse.TYPE_QUERY, parse.TYPE_STRING, parse.TYPE_STRING, parse.TYPE_NUMBER},
		parse.TYPE_SERIES,
		3,
	},
	"dev": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		1,
	},
	"recent": {
		[]parse.FuncType{parse.TYPE_SERIES, parse.TYPE_STRING},
		parse.TYPE_NUMBER,
		1,
	},
}

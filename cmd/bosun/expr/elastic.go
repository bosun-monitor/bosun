package expr

import (
	"strings"

	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
)

func elasticTagQuery(args []parse.Node) (parse.Tags, error) {
	n := args[1].(*parse.StringNode)
	t := make(parse.Tags)
	for _, s := range strings.Split(n.Text, ",") {
		t[s] = struct{}{}
	}
	return t, nil
}

// ElasticFuncs are specific functions that query an elasticsearch instance.
// They are only loaded when the elastic hosts are set in the config file
var Elastic = map[string]parse.Func{
	// Funcs for querying elastic
	"escount": {
		Args:          []models.FuncType{models.TypeESIndexer, models.TypeString, models.TypeESQuery, models.TypeString, models.TypeString, models.TypeString},
		Return:        models.TypeSeriesSet,
		Tags:          elasticTagQuery,
		F:             ESCount,
		PrefixEnabled: true,
	},
	"esstat": {
		Args:          []models.FuncType{models.TypeESIndexer, models.TypeString, models.TypeESQuery, models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return:        models.TypeSeriesSet,
		Tags:          elasticTagQuery,
		F:             ESStat,
		PrefixEnabled: true,
	},

	// Funcs to create elastic index names (ESIndexer type)
	"esindices": {
		Args:     []models.FuncType{models.TypeString, models.TypeString},
		VArgs:    true,
		VArgsPos: 1,
		Return:   models.TypeESIndexer,
		F:        ESIndicies,
	},
	"esdaily": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeESIndexer,
		F:      ESDaily,
	},
	"esmonthly": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeESIndexer,
		F:      ESMonthly,
	},
	"esls": {
		Args:   []models.FuncType{models.TypeString},
		Return: models.TypeESIndexer,
		F:      ESLS,
	},

	// Funcs for generate elastic queries (ESQuery Type) to further filter results
	"esall": {
		Args:   []models.FuncType{},
		Return: models.TypeESQuery,
		F:      ESAll,
	},
	"esregexp": {
		Args:   []models.FuncType{models.TypeString, models.TypeString},
		Return: models.TypeESQuery,
		F:      ESRegexp,
	},
	"esquery": {
		Args:   []models.FuncType{models.TypeString, models.TypeString},
		Return: models.TypeESQuery,
		F:      ESQueryString,
	},
	"esexists": {
		Args:   []models.FuncType{models.TypeString},
		Return: models.TypeESQuery,
		F:      ESExists,
	},
	"esand": {
		Args:   []models.FuncType{models.TypeESQuery},
		VArgs:  true,
		Return: models.TypeESQuery,
		F:      ESAnd,
	},
	"esor": {
		Args:   []models.FuncType{models.TypeESQuery},
		VArgs:  true,
		Return: models.TypeESQuery,
		F:      ESOr,
	},
	"esnot": {
		Args:   []models.FuncType{models.TypeESQuery},
		Return: models.TypeESQuery,
		F:      ESNot,
	},
	"esgt": {
		Args:   []models.FuncType{models.TypeString, models.TypeScalar},
		Return: models.TypeESQuery,
		F:      ESGT,
	},
	"esgte": {
		Args:   []models.FuncType{models.TypeString, models.TypeScalar},
		Return: models.TypeESQuery,
		F:      ESGTE,
	},
	"eslt": {
		Args:   []models.FuncType{models.TypeString, models.TypeScalar},
		Return: models.TypeESQuery,
		F:      ESLT,
	},
	"eslte": {
		Args:   []models.FuncType{models.TypeString, models.TypeScalar},
		Return: models.TypeESQuery,
		F:      ESLTE,
	},
}

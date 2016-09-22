package expr

import (
	"fmt"
	"strings"

	"bosun.org/cmd/bosun/expr/parse"
	"bosun.org/models"
	"bosun.org/opentsdb"
	"github.com/MiniProfiler/go/miniprofiler"
)

type SQLConfig struct {
	DBKey      string
	Tags       []string
	TimeField  string
	TimeMarker string
	ValueField string
}

func (s SQLConfig) Type() models.FuncType { return models.TypeSQLConfig }
func (s SQLConfig) Value() interface{}    { return s }

var SQLFuncs = map[string]parse.Func{
	"sqlConf": {
		Args:   []models.FuncType{models.TypeString, models.TypeString, models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeSQLConfig,
		F:      sqlConf,
	},
	"sql": {
		Args:   []models.FuncType{models.TypeSQLConfig, models.TypeString, models.TypeString, models.TypeString},
		Return: models.TypeSeriesSet,
		F:      sqlQuery,
		Tags:   tagSql,
	},
}

func tagSql(args []parse.Node) (parse.Tags, error) {
	fnode := args[0].(*parse.FuncNode).Args[1].(*parse.StringNode).Text
	txt := strings.Split(fnode, ",")

	t := make(parse.Tags, len(txt))
	for _, tag := range txt {
		t[tag] = struct{}{}
	}
	return t, nil
}

func sqlConf(e *State, T miniprofiler.Timer, dbKey, tags, timeField, timeMarker, valueField string) (*Results, error) {
	var r Results
	if timeMarker == "" {
		timeMarker = "%T"
	}
	if valueField == "" {
		valueField = "value"
	}
	conf := SQLConfig{
		DBKey:      dbKey,
		Tags:       strings.Split(tags, ","),
		TimeField:  timeField,
		TimeMarker: timeMarker,
		ValueField: valueField,
	}
	r.Results = append(r.Results, &Result{Value: conf})
	return &r, nil
}

func sqlQuery(e *State, T miniprofiler.Timer, conf SQLConfig, query, start, end string) (*Results, error) {

	timeWhere := fmt.Sprintf("(%s < '' AND %s > '')", conf.TimeField, conf.TimeField)
	query = strings.Replace(query, conf.TimeMarker, timeWhere, 1)

	data := map[string]Series{}

	var r Results
	for t, d := range data {
		tags, _ := opentsdb.ParseTags(t)
		r.Results = append(r.Results, &Result{Group: tags, Value: d})
	}
	return &r, nil
}

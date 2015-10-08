package collectors

import (
	"fmt"
	"testing"

	"bosun.org/cmd/scollector/conf"
)

func TestMySQL(t *testing.T) {
	// test bad name
	myConf := conf.MySQL{DBName: "bad name"}
	err := MySQL(myConf)
	e_error := fmt.Errorf("invalid MySQL DBName")
	if err == nil || err.Error() != e_error.Error() {
		t.Error("MySQL expected", e_error, "got", err)
	}

	// test good name
	myConf = conf.MySQL{DBName: "good_name"}
	err = MySQL(myConf)
	e_error = nil
	if err != e_error {
		t.Error("MySQL expected", e_error, "got", err)
	}

	// test without database name
	myConf = conf.MySQL{}
	err = MySQL(myConf)
	e_error = nil
	if err != e_error {
		t.Error("MySQL expected", e_error, "got", err)
	}
}

func TestMySQLParseResults(t *testing.T) {
	// test one row
	myConf := conf.MySQL{DBName: "good_name"}
	query := MySQLQuery{name: "good_metric"}
	results := make([][]string, 0, 2)
	row := []string{"1234"}
	results = append(results, row)
	md, err := MySQLParseResults(myConf, &query, &results)
	e_error := error(nil)
	if err != e_error {
		t.Fatal("MySQLParseResults expected", e_error, "got", err)
	}
	// md[0]
	mdTest := md[0]
	e_string := "mysql.good_metric"
	if mdTest.Metric != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", mdTest.Metric)
	}
	e_string = "1234"
	if mdTest.Value != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", mdTest.Metric)
	}
	tag, ok := mdTest.Tags["db_name"]
	e_bool := true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	e_string = "good_name"
	if tag != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", tag)
	}
	tag, ok = mdTest.Tags["host"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}

	// test two rows with diffrent tags
	results = make([][]string, 0, 2)
	row = []string{"1234", "name", "one"}
	results = append(results, row)
	row = []string{"12345", "var", "two"}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("MySQLParseResults expected", e_error, "got", err)
	}
	// md[0]
	mdTest = md[0]
	e_string = "mysql.good_metric"
	if mdTest.Metric != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", mdTest.Metric)
	}
	e_string = "1234"
	if mdTest.Value != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", mdTest.Metric)
	}
	tag, ok = mdTest.Tags["db_name"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	e_string = "good_name"
	if tag != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", tag)
	}
	tag, ok = mdTest.Tags["host"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	tag, ok = mdTest.Tags["name"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	e_string = "one"
	if tag != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", tag)
	}
	tag, ok = mdTest.Tags["var"]
	e_bool = false
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	// md[1]
	mdTest = md[1]
	e_string = "mysql.good_metric"
	if mdTest.Metric != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", mdTest.Metric)
	}
	e_string = "12345"
	if mdTest.Value != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", mdTest.Metric)
	}
	tag, ok = mdTest.Tags["db_name"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	e_string = "good_name"
	if tag != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", tag)
	}
	tag, ok = mdTest.Tags["host"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	tag, ok = mdTest.Tags["name"]
	e_bool = false
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	tag, ok = mdTest.Tags["var"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	e_string = "two"
	if tag != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", tag)
	}

	// test one row without database name
	myConf = conf.MySQL{}
	query = MySQLQuery{name: "good_metric"}
	results = make([][]string, 0, 2)
	row = []string{"1234"}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("MySQLParseResults expected", e_error, "got", err)
	}
	// md[0]
	mdTest = md[0]
	e_string = "mysql.good_metric"
	if mdTest.Metric != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", mdTest.Metric)
	}
	e_string = "1234"
	if mdTest.Value != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", mdTest.Metric)
	}
	tag, ok = mdTest.Tags["db_name"]
	e_bool = false
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	tag, ok = mdTest.Tags["host"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}

	// test invalid timestamp
	myConf = conf.MySQL{DBName: "good_name"}
	query = MySQLQuery{name: "good_metric", hasTime: true}
	results = make([][]string, 0, 2)
	row = []string{"1234", "abc", "name", "one"}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = fmt.Errorf("invalid timestamp from query")
	if err == nil || err.Error() != e_error.Error() {
		t.Error("MySQLParseResults expected", e_error, "got", err)
	}

	// test valid timestamp
	query = MySQLQuery{name: "good_metric", hasTime: true}
	results = make([][]string, 0, 2)
	row = []string{"1234", "12345678", "name", "one"}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("MySQLParseResults expected", e_error, "got", err)
	}
	// md[0]
	mdTest = md[0]
	e_string = "mysql.good_metric"
	if mdTest.Metric != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", mdTest.Metric)
	}
	e_string = "1234"
	if mdTest.Value != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", mdTest.Metric)
	}
	tag, ok = mdTest.Tags["db_name"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	e_string = "good_name"
	if tag != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", tag)
	}
	tag, ok = mdTest.Tags["host"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	tag, ok = mdTest.Tags["name"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	e_string = "one"
	if tag != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", tag)
	}
	e_int64 := int64(12345678)
	if mdTest.Timestamp != e_int64 {
		t.Error("MySQLParseResults expected", e_int64, "got", mdTest.Timestamp)
	}

	// test too little columns without timestamp
	query = MySQLQuery{name: "good_metric"}
	results = make([][]string, 0, 2)
	row = []string{}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = fmt.Errorf("invalid number of columns from query")
	if err == nil || err.Error() != e_error.Error() {
		t.Error("MySQLParseResults expected", e_error, "got", err)
	}

	// test too little columns with timestamp
	query = MySQLQuery{name: "good_metric", hasTime: true}
	results = make([][]string, 0, 2)
	row = []string{"1234"}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = fmt.Errorf("invalid number of columns from query")
	if err == nil || err.Error() != e_error.Error() {
		t.Error("MySQLParseResults expected", e_error, "got", err)
	}

	// test 5 tags without timestamp
	query = MySQLQuery{name: "good_metric"}
	results = make([][]string, 0, 2)
	row = []string{"1234", "one", "one", "two", "two", "three", "three", "four", "four", "five", "five"}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("MySQLParseResults expected", e_error, "got", err)
	}

	// test 5 tags without timestamp with extra column
	query = MySQLQuery{name: "good_metric"}
	results = make([][]string, 0, 2)
	row = []string{"1234", "one", "one", "two", "two", "three", "three", "four", "four", "five", "five", "six"}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("MySQLParseResults expected", e_error, "got", err)
	}

	// test 6 tags without timestamp
	query = MySQLQuery{name: "good_metric"}
	results = make([][]string, 0, 2)
	row = []string{"1234", "one", "one", "two", "two", "three", "three", "four", "four", "five", "five", "six", "six"}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = fmt.Errorf("more than 5 tags from query")
	if err == nil || err.Error() != e_error.Error() {
		t.Error("MySQLParseResults expected", e_error, "got", err)
	}

	// test 5 tags with timestamp
	query = MySQLQuery{name: "good_metric", hasTime: true}
	results = make([][]string, 0, 2)
	row = []string{"1234", "12345678", "one", "one", "two", "two", "three", "three", "four", "four", "five", "five"}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("MySQLParseResults expected", e_error, "got", err)
	}

	// test 5 tags with timestamp with extra column
	query = MySQLQuery{name: "good_metric", hasTime: true}
	results = make([][]string, 0, 2)
	row = []string{"1234", "12345678", "one", "one", "two", "two", "three", "three", "four", "four", "five", "five", "six"}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("MySQLParseResults expected", e_error, "got", err)
	}

	// test 6 tags with timestamp
	query = MySQLQuery{name: "good_metric", hasTime: true}
	results = make([][]string, 0, 2)
	row = []string{"1234", "12345678", "one", "one", "two", "two", "three", "three", "four", "four", "five", "five", "six", "six"}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = fmt.Errorf("more than 5 tags from query")
	if err == nil || err.Error() != e_error.Error() {
		t.Error("MySQLParseResults expected", e_error, "got", err)
	}

	// test regex on tag
	query = MySQLQuery{name: "good_metric"}
	results = make([][]string, 0, 2)
	row = []string{"1234", "tag % name", "value # name"}
	results = append(results, row)
	md, err = MySQLParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("MySQLParseResults expected", e_error, "got", err)
	}
	// md[0]
	mdTest = md[0]
	e_string = "mysql.good_metric"
	if mdTest.Metric != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", mdTest.Metric)
	}
	e_string = "1234"
	if mdTest.Value != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", mdTest.Metric)
	}
	tag, ok = mdTest.Tags["db_name"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	e_string = "good_name"
	if tag != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", tag)
	}
	tag, ok = mdTest.Tags["host"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	tag, ok = mdTest.Tags["tag_name"]
	e_bool = true
	if ok != e_bool {
		t.Error("MySQLParseResults expected", e_bool, "got", ok)
	}
	e_string = "value_name"
	if tag != e_string {
		t.Error("MySQLParseResults expected", e_string, "got", tag)
	}

}

package collectors

import (
	"fmt"
	"testing"

	"bosun.org/cmd/scollector/conf"
)

func TestDatabaseParseResults(t *testing.T) {
	// test one row
	myConf := conf.Database{Type: "mysql", DBName: "good_name"}
	query := DatabaseQuery{name: "good_metric"}
	results := make([][]string, 0, 2)
	row := []string{"1234"}
	results = append(results, row)
	md, err := DatabaseParseResults(myConf, &query, &results)
	e_error := error(nil)
	if err != e_error {
		t.Fatal("DatabaseParseResults expected:", e_error, "got:", err)
	}
	// md[0]
	mdTest := md[0]
	e_string := "mysql.good_metric"
	if mdTest.Metric != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", mdTest.Metric)
	}
	e_string = "1234"
	if mdTest.Value != e_string {
		t.Error("DatabaseParseResults expected", e_string, "got:", mdTest.Metric)
	}
	tag, ok := mdTest.Tags["db_name"]
	e_bool := true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	e_string = "good_name"
	if tag != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", tag)
	}
	tag, ok = mdTest.Tags["host"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}

	// test two rows with diffrent tags
	results = make([][]string, 0, 2)
	row = []string{"1234", "name", "one"}
	results = append(results, row)
	row = []string{"12345", "var", "two"}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("DatabaseParseResults expected:", e_error, "got:", err)
	}
	// md[0]
	mdTest = md[0]
	e_string = "mysql.good_metric"
	if mdTest.Metric != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", mdTest.Metric)
	}
	e_string = "1234"
	if mdTest.Value != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", mdTest.Metric)
	}
	tag, ok = mdTest.Tags["db_name"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	e_string = "good_name"
	if tag != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", tag)
	}
	tag, ok = mdTest.Tags["host"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	tag, ok = mdTest.Tags["name"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	e_string = "one"
	if tag != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", tag)
	}
	tag, ok = mdTest.Tags["var"]
	e_bool = false
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	// md[1]
	mdTest = md[1]
	e_string = "mysql.good_metric"
	if mdTest.Metric != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", mdTest.Metric)
	}
	e_string = "12345"
	if mdTest.Value != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", mdTest.Metric)
	}
	tag, ok = mdTest.Tags["db_name"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	e_string = "good_name"
	if tag != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", tag)
	}
	tag, ok = mdTest.Tags["host"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	tag, ok = mdTest.Tags["name"]
	e_bool = false
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	tag, ok = mdTest.Tags["var"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	e_string = "two"
	if tag != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", tag)
	}

	// test one row without database name
	myConf = conf.Database{Type: "mysql"}
	query = DatabaseQuery{name: "good_metric"}
	results = make([][]string, 0, 2)
	row = []string{"1234"}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("DatabaseParseResults expected:", e_error, "got:", err)
	}
	// md[0]
	mdTest = md[0]
	e_string = "mysql.good_metric"
	if mdTest.Metric != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", mdTest.Metric)
	}
	e_string = "1234"
	if mdTest.Value != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", mdTest.Metric)
	}
	tag, ok = mdTest.Tags["db_name"]
	e_bool = false
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	tag, ok = mdTest.Tags["host"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}

	// test invalid timestamp
	myConf = conf.Database{Type: "mysql", DBName: "good_name"}
	query = DatabaseQuery{name: "good_metric", hasTime: true}
	results = make([][]string, 0, 2)
	row = []string{"1234", "abc", "name", "one"}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = fmt.Errorf("invalid timestamp")
	if err == nil || err.Error() != e_error.Error() {
		t.Error("DatabaseParseResults expected:", e_error, "got:", err)
	}

	// test valid timestamp
	query = DatabaseQuery{name: "good_metric", hasTime: true}
	results = make([][]string, 0, 2)
	row = []string{"1234", "12345678", "name", "one"}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("DatabaseParseResults expected:", e_error, "got:", err)
	}
	// md[0]
	mdTest = md[0]
	e_string = "mysql.good_metric"
	if mdTest.Metric != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", mdTest.Metric)
	}
	e_string = "1234"
	if mdTest.Value != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", mdTest.Metric)
	}
	tag, ok = mdTest.Tags["db_name"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	e_string = "good_name"
	if tag != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", tag)
	}
	tag, ok = mdTest.Tags["host"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	tag, ok = mdTest.Tags["name"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	e_string = "one"
	if tag != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", tag)
	}
	e_int64 := int64(12345678)
	if mdTest.Timestamp != e_int64 {
		t.Error("DatabaseParseResults expected:", e_int64, "got:", mdTest.Timestamp)
	}

	// test too little columns without timestamp
	query = DatabaseQuery{name: "good_metric"}
	results = make([][]string, 0, 2)
	row = []string{}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = fmt.Errorf("invalid number of columns")
	if err == nil || err.Error() != e_error.Error() {
		t.Error("DatabaseParseResults expected:", e_error, "got:", err)
	}

	// test too little columns with timestamp
	query = DatabaseQuery{name: "good_metric", hasTime: true}
	results = make([][]string, 0, 2)
	row = []string{"1234"}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = fmt.Errorf("invalid number of columns")
	if err == nil || err.Error() != e_error.Error() {
		t.Error("DatabaseParseResults expected:", e_error, "got:", err)
	}

	// test 5 tags without timestamp
	query = DatabaseQuery{name: "good_metric"}
	results = make([][]string, 0, 2)
	row = []string{"1234", "one", "one", "two", "two", "three", "three", "four", "four", "five", "five"}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("DatabaseParseResults expected:", e_error, "got:", err)
	}

	// test 5 tags without timestamp with extra column
	query = DatabaseQuery{name: "good_metric"}
	results = make([][]string, 0, 2)
	row = []string{"1234", "one", "one", "two", "two", "three", "three", "four", "four", "five", "five", "six"}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("DatabaseParseResults expected:", e_error, "got:", err)
	}

	// test 6 tags without timestamp
	query = DatabaseQuery{name: "good_metric"}
	results = make([][]string, 0, 2)
	row = []string{"1234", "one", "one", "two", "two", "three", "three", "four", "four", "five", "five", "six", "six"}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Error("DatabaseParseResults expected:", e_error, "got:", err)
	}

	// test 5 tags with timestamp
	query = DatabaseQuery{name: "good_metric", hasTime: true}
	results = make([][]string, 0, 2)
	row = []string{"1234", "12345678", "one", "one", "two", "two", "three", "three", "four", "four", "five", "five"}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("DatabaseParseResults expected:", e_error, "got:", err)
	}

	// test 5 tags with timestamp with extra column
	query = DatabaseQuery{name: "good_metric", hasTime: true}
	results = make([][]string, 0, 2)
	row = []string{"1234", "12345678", "one", "one", "two", "two", "three", "three", "four", "four", "five", "five", "six"}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("DatabaseParseResults expected:", e_error, "got:", err)
	}

	// test 6 tags with timestamp
	query = DatabaseQuery{name: "good_metric", hasTime: true}
	results = make([][]string, 0, 2)
	row = []string{"1234", "12345678", "one", "one", "two", "two", "three", "three", "four", "four", "five", "five", "six", "six"}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Error("DatabaseParseResults expected:", e_error, "got:", err)
	}

	// test regex on tag
	query = DatabaseQuery{name: "good_metric"}
	results = make([][]string, 0, 2)
	row = []string{"1234", "tag % name", "value # name"}
	results = append(results, row)
	md, err = DatabaseParseResults(myConf, &query, &results)
	e_error = nil
	if err != e_error {
		t.Fatal("DatabaseParseResults expected:", e_error, "got:", err)
	}
	// md[0]
	mdTest = md[0]
	e_string = "mysql.good_metric"
	if mdTest.Metric != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", mdTest.Metric)
	}
	e_string = "1234"
	if mdTest.Value != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", mdTest.Metric)
	}
	tag, ok = mdTest.Tags["db_name"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	e_string = "good_name"
	if tag != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", tag)
	}
	tag, ok = mdTest.Tags["host"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	tag, ok = mdTest.Tags["tag_name"]
	e_bool = true
	if ok != e_bool {
		t.Error("DatabaseParseResults expected:", e_bool, "got:", ok)
	}
	e_string = "value_name"
	if tag != e_string {
		t.Error("DatabaseParseResults expected:", e_string, "got:", tag)
	}

}

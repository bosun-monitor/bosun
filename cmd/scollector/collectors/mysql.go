package collectors

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "bosun.org/_third_party/github.com/go-sql-driver/mysql"
	"bosun.org/cmd/scollector/conf"
	"bosun.org/collect"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
)

type MySQLQuery struct {
	name        string
	query       string
	description string
	hasTime     bool
	interval    time.Duration
	next        time.Time
}

func MySQL(c conf.MySQL) error {
	return MySQLAddCollector(c)
}

var MySQLQueryVars struct {
}

func MySQLAddCollector(c conf.MySQL) error {
	if c.DBName != "" {
		cleaned, _ := opentsdb.Clean(c.DBName)
		if c.DBName != cleaned {
			return fmt.Errorf("invalid MySQL DBName")
		}
	}
	if c.InstId < 1 {
		c.InstId = 1
	}
	if c.MaxOpenConns < 1 {
		c.MaxOpenConns = 2
	}
	if c.GetQueriesInterval < 1 {
		c.GetQueriesInterval = 30
	}
	if c.Username == "" {
		c.Username = "root"
	}
	if c.Protocol == "" {
		c.Protocol = "tcp"
	}
	if c.Address == "" {
		c.Address = "127.0.0.1"
	}
	if c.Port < 1 {
		c.Port = 3306
	}
	if c.DBOfTable == "" {
		c.DBOfTable = c.Username
	}
	if c.TableName == "" {
		c.TableName = "scollectorqueries"
	}

	var tags opentsdb.TagSet
	if c.DBName == "" {
		tags = opentsdb.TagSet{"inst_id": strconv.Itoa(c.InstId)}
	} else {
		tags = opentsdb.TagSet{"db_name": c.DBName, "inst_id": strconv.Itoa(c.InstId)}
	}

	collectors = append(collectors, &ContinuousCollector{
		F: func(collectorStatsChan chan<- *ContinuousCollectorStats) {
			MySQLCollect(c, collectorStatsChan)
		},
		name: "mysql",
		tags: tags,
	})

	return nil
}

func MySQLCollect(c conf.MySQL, collectorStatsChan chan<- *ContinuousCollectorStats) {
	var db *sql.DB
	var err error

	if c.Password == "" {
		db, err = sql.Open("mysql", c.Username+"@"+c.Protocol+"("+c.Address+":"+strconv.Itoa(c.Port)+")/"+c.DBOfTable)
	} else {
		db, err = sql.Open("mysql", c.Username+":"+c.Password+"@"+c.Protocol+"("+c.Address+":"+strconv.Itoa(c.Port)+")/"+c.DBOfTable)
	}

	if err != nil {
		slog.Errorf("%v: %v", "mysql open", err)
		return
	}

	defer db.Close()

	// check if can connect to database, if fail, check again every minute until connected
	for {
		err = db.Ping()
		if err == nil {
			break
		}
		slog.Errorf("%v: %v", "mysql ping", err)
		next := time.After(time.Minute)
		select {
		case <-next:
		case <-ContinuousCollectorVars.quit:
			return
		}
	}

	var waitGroup sync.WaitGroup
	queryChan := make(chan *MySQLQuery, 1)
	queries := MySQLGetQueries(c, db, nil)
	nextGetQueries := time.Now().Add(time.Duration(c.GetQueriesInterval) * time.Minute)

	waitGroup.Add(c.MaxOpenConns)
	for i := 0; i < c.MaxOpenConns; i++ {
		go MySQLRunQueryWorker(c, db, waitGroup, queryChan, collectorStatsChan)
	}

	for {
		if time.Now().After(nextGetQueries) {
			queries = MySQLGetQueries(c, db, queries)
			nextGetQueries = time.Now().Add(time.Duration(c.GetQueriesInterval) * time.Minute)
		}
		nextTime := nextGetQueries
		// maybe change to a priority queue instead of checking each slice?
		for i := range queries {
			if time.Now().After(queries[i].next) {
				queryChan <- &queries[i]
				queries[i].next = time.Now().Add(queries[i].interval)
			}
			if queries[i].next.Before(nextTime) {
				nextTime = queries[i].next
			}
		}
		next := time.After(nextTime.Sub(time.Now()))
		select {
		case <-next:
		case <-ContinuousCollectorVars.quit:
			close(queryChan)
			break
		}
	}

	waitGroup.Wait()
}

func MySQLGetQueries(c conf.MySQL, db *sql.DB, oldQueries []MySQLQuery) []MySQLQuery {
	results, err := MySQLRunQuery(db, "select name, query, description, hastime, queryinterval from "+c.TableName)

	if err != nil {
		slog.Errorf("%v: %v", "mysql run get queries", err)
		return nil
	}

	re := regexp.MustCompile("[^a-zA-Z0-9_.-]")
	queries := make([]MySQLQuery, 0, len(*results))

	for _, result := range *results {

		if re.MatchString(result[0]) {
			slog.Warningf("%v: %v", "mysql invalid metric name", result[0])
			continue
		}

		query := MySQLQuery{name: result[0], query: result[1], description: result[2]}
		if result[3] != "0" {
			query.hasTime = true
		}
		interval, _ := strconv.Atoi(result[4])
		if interval < 1 {
			query.interval = DefaultFreq
		} else {
			query.interval = time.Duration(interval) * time.Second
		}
		queries = append(queries, query)
	}

	if oldQueries == nil {
		return queries
	}

	for i := range queries {
		for _, query := range oldQueries {
			if queries[i].query == query.query {
				queries[i].next = query.next
				break
			}
		}
	}

	return queries
}

func MySQLRunQueryWorker(c conf.MySQL, db *sql.DB, waitGroup sync.WaitGroup, queryChan <-chan *MySQLQuery, collectorStatsChan chan<- *ContinuousCollectorStats) {
	var md opentsdb.MultiDataPoint

	defer waitGroup.Done()

	for query := range queryChan {
		result := 0
		timeStart := time.Now()

		results, err := MySQLRunQuery(db, query.query)

		if err == nil {
			md, err = MySQLParseResults(c, query, results)
		}

		if err != nil {
			slog.Warningf("%v: %v: %v", "mysql", err, query.query)
			result = 1
		}

		if !collect.DisableDefaultCollectors {
			collectorStatsChan <- &ContinuousCollectorStats{duration: time.Since(timeStart).Seconds(), result: result}
		}

		for _, dp := range md {
			ContinuousCollectorVars.dpChan <- dp
		}
		md = nil
	}
}

func MySQLRunQuery(db *sql.DB, query string) (*[][]string, error) {
	rows, err := db.Query(query)

	if err != nil {
		return nil, err
	}

	defer rows.Close()

	columns, err := rows.Columns()

	if err != nil {
		return nil, err
	}

	numColumns := len(columns)

	rawValues := make([]sql.RawBytes, numColumns)
	scanArgs := make([]interface{}, numColumns)
	for i := range rawValues {
		scanArgs[i] = &rawValues[i]
	}
	values := make([][]string, 0, 2)

	for rows.Next() {
		err = rows.Err()

		if err != nil {
			return nil, err
		}

		err = rows.Scan(scanArgs...)

		if err != nil {
			return nil, err
		}

		row := make([]string, 0, numColumns)
		for i := range rawValues {
			if rawValues[i] == nil {
				row = append(row, "_null_")
			} else {
				row = append(row, string(rawValues[i]))
			}
		}
		values = append(values, row)
	}

	return &values, nil
}

func MySQLParseResults(c conf.MySQL, query *MySQLQuery, results *[][]string) (opentsdb.MultiDataPoint, error) {
	if len(*results) < 1 {
		return nil, nil
	}

	numTagLoops := len((*results)[0]) - 1
	if query.hasTime {
		numTagLoops--
	}
	if numTagLoops < 0 {
		return nil, fmt.Errorf("invalid number of columns from query")
	}
	numTagLoops = (numTagLoops - (numTagLoops % 2)) / 2
	if numTagLoops > 5 {
		return nil, fmt.Errorf("more than 5 tags from query")
	}

	var md opentsdb.MultiDataPoint
	var name string
	var value string
	var tagsBase opentsdb.TagSet
	if c.DBName == "" {
		tagsBase = opentsdb.TagSet{"inst_id": strconv.Itoa(c.InstId)}
	} else {
		tagsBase = opentsdb.TagSet{"db_name": c.DBName, "inst_id": strconv.Itoa(c.InstId)}
	}

	for _, result := range *results {
		if result[0] == "" {
			continue
		}

		tags := tagsBase.Copy()
		for i := 0; i < numTagLoops; i++ {
			if query.hasTime {
				name = strings.Replace(result[(i*2)+2], " ", "_", -1)
				value = strings.Replace(result[(i*2)+3], " ", "_", -1)
			} else {
				name = strings.Replace(result[(i*2)+1], " ", "_", -1)
				value = strings.Replace(result[(i*2)+2], " ", "_", -1)
			}
			name, _ = opentsdb.Clean(name)
			value, _ = opentsdb.Clean(value)
			name = ContinuousCollectorVars.reTwoOrMoreUnderscore.ReplaceAllString(name, "_")
			value = ContinuousCollectorVars.reTwoOrMoreUnderscore.ReplaceAllString(value, "_")

			if name == "" || value == "" {
				continue
			}
			tags = tags.Merge(opentsdb.TagSet{name: value})
		}

		if query.hasTime {
			timestamp, _ := strconv.ParseInt(result[1], 10, 64)
			if timestamp < 1 {
				return nil, fmt.Errorf("invalid timestamp from query")
			}
			AddTS(&md, "mysql."+query.name, timestamp, result[0], tags, metadata.Unknown, metadata.None, query.description)

		} else {
			Add(&md, "mysql."+query.name, result[0], tags, metadata.Unknown, metadata.None, query.description)
		}
	}

	return md, nil
}

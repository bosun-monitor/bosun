package collectors

import (
	"database/sql"
	"fmt"
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

type DatabaseQuery struct {
	name        string
	query       string
	description string
	hasTime     bool
	interval    time.Duration
	next        time.Time
}

var DatabaseVars struct {
	vaildDatabases map[string]bool
}

func init() {
	DatabaseVars.vaildDatabases = map[string]bool{"mysql": true}
}

func Database(c conf.Database) error {
	return DatabaseAddCollector(c)
}

func DatabaseAddCollector(c conf.Database) error {
	_, found := DatabaseVars.vaildDatabases[c.Type]
	if !found {
		return fmt.Errorf("%v: %v", "invalid Database Type", c.Type)
	}
	if c.DBName != "" {
		cleaned, _ := opentsdb.Clean(c.DBName)
		if c.DBName != cleaned {
			return fmt.Errorf("%v: %v", "invalid Database DBName", c.DBName)
		}
	}
	if c.InstId < 1 {
		c.InstId = 1
	}
	if c.MaxOpenConns < 1 {
		c.MaxOpenConns = 2
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

	var tags opentsdb.TagSet
	if c.DBName == "" {
		tags = opentsdb.TagSet{"inst_id": strconv.Itoa(c.InstId)}
	} else {
		tags = opentsdb.TagSet{"db_name": c.DBName, "inst_id": strconv.Itoa(c.InstId)}
	}

	collectors = append(collectors, &ContinuousCollector{
		F: func(collectorStatsChan chan<- *ContinuousCollectorStats) {
			DatabaseCollect(c, collectorStatsChan)
		},
		name: c.Type,
		tags: tags,
	})

	return nil
}

func DatabaseCollect(c conf.Database, collectorStatsChan chan<- *ContinuousCollectorStats) {
	queries := DatabaseGetQueries(c)

	if len(queries) < 1 {
		slog.Errorf("%v: %v: %v", "Database", c.Type, "no queries")
	}

	db, err := DatabaseConnect(c)
	if err != nil {
		return
	}

	defer db.Close()

	var waitGroup sync.WaitGroup
	queryChan := make(chan *DatabaseQuery, 1)

	waitGroup.Add(c.MaxOpenConns)
	for i := 0; i < c.MaxOpenConns; i++ {
		go func() {
			DatabaseRunQueryWorker(c, db, queryChan, collectorStatsChan)
			waitGroup.Done()
		}()
	}

Loop:
	for {
		nextTime := queries[0].next
		// maybe change to a priority queue instead of checking each slice?
		for i := range queries {
			if time.Now().After(queries[i].next) {
				queryChan <- &queries[i]
				queries[i].next = time.Now().Add(queries[i].interval)
				if queries[i].next.Before(nextTime) {
					nextTime = queries[i].next
				}
			}
		}
		next := time.After(nextTime.Sub(time.Now()))
		select {
		case <-next:
		case <-ContinuousCollectorVars.quit:
			break Loop
		}
	}

	close(queryChan)
	waitGroup.Wait()
}

func DatabaseGetQueries(c conf.Database) []DatabaseQuery {
	queries := make([]DatabaseQuery, 0, len(c.Query))

	for _, query := range c.Query {
		cleaned, _ := opentsdb.Clean(query.Name)

		if query.Name != cleaned {
			slog.Warningf("%v: %v: %v: %v", "Database", c.Type, "invaid metric name", query.Name)
			continue
		}

		databaseQuery := DatabaseQuery{name: query.Name, query: query.Query, description: query.Description, hasTime: query.HasTime}

		if query.Interval < 1 {
			databaseQuery.interval = DefaultFreq
		} else {
			databaseQuery.interval = time.Duration(query.Interval) * time.Second
		}

		queries = append(queries, databaseQuery)
	}

	return queries
}

func DatabaseConnect(c conf.Database) (*sql.DB, error) {
	var db *sql.DB
	var err error

	if c.Type == "mysql" {
		if c.Password == "" {
			db, err = sql.Open("mysql", c.Username+"@"+c.Protocol+"("+c.Address+":"+strconv.Itoa(c.Port)+")/")
		} else {
			db, err = sql.Open("mysql", c.Username+":"+c.Password+"@"+c.Protocol+"("+c.Address+":"+strconv.Itoa(c.Port)+")/")
		}
	}

	if err != nil {
		slog.Errorf("%v: %v: %v: %v", "Database", c.Type, "connect open error", err)
		return nil, fmt.Errorf("%v: %v", "connect open error", err)
	}

	// check if can connect to database, if fail, check again every minute until connected
	for {
		err = db.Ping()
		if err == nil {
			break
		}
		slog.Errorf("%v: %v: %v: %v", "Database", c.Type, "connect ping error", err)
		next := time.After(time.Minute)
		select {
		case <-next:
		case <-ContinuousCollectorVars.quit:
			db.Close()
			return nil, fmt.Errorf("connect while quitting")
		}
	}

	return db, nil
}

func DatabaseRunQueryWorker(c conf.Database, db *sql.DB, queryChan <-chan *DatabaseQuery, collectorStatsChan chan<- *ContinuousCollectorStats) {
	var md opentsdb.MultiDataPoint

	for query := range queryChan {
		result := 0
		timeStart := time.Now()

		results, err := DatabaseRunQuery(db, query.query)

		if err == nil {
			md, err = DatabaseParseResults(c, query, results)
		}

		if err != nil {
			slog.Warningf("%v: %v: %v: %v: %v: %v", "Database", c.Type, "query error", err, "for query", query.query)
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

func DatabaseRunQuery(db *sql.DB, query string) (*[][]string, error) {
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

func DatabaseParseResults(c conf.Database, query *DatabaseQuery, results *[][]string) (opentsdb.MultiDataPoint, error) {
	if len(*results) < 1 {
		return nil, nil
	}

	numTagLoops := len((*results)[0]) - 1
	if query.hasTime {
		numTagLoops--
	}
	if numTagLoops < 0 {
		return nil, fmt.Errorf("invalid number of columns")
	}
	numTagLoops = (numTagLoops - (numTagLoops % 2)) / 2

	var md opentsdb.MultiDataPoint
	var tagName string
	var tagValue string
	var tagsBase opentsdb.TagSet
	baseName := c.Type + "."
	if c.DBName == "" {
		tagsBase = opentsdb.TagSet{"inst_id": strconv.Itoa(c.InstId)}
	} else {
		tagsBase = opentsdb.TagSet{"db_name": c.DBName, "inst_id": strconv.Itoa(c.InstId)}
	}

	for _, result := range *results {
		if result[0] == "" {
			continue
		}

		// Check result[0] is vaild metric value?

		tags := tagsBase.Copy()
		for i := 0; i < numTagLoops; i++ {
			if query.hasTime {
				tagName = strings.Replace(result[(i*2)+2], " ", "_", -1)
				tagValue = strings.Replace(result[(i*2)+3], " ", "_", -1)
			} else {
				tagName = strings.Replace(result[(i*2)+1], " ", "_", -1)
				tagValue = strings.Replace(result[(i*2)+2], " ", "_", -1)
			}
			tagName, _ = opentsdb.Clean(tagName)
			tagValue, _ = opentsdb.Clean(tagValue)
			tagName = ContinuousCollectorVars.reTwoOrMoreUnderscore.ReplaceAllString(tagName, "_")
			tagValue = ContinuousCollectorVars.reTwoOrMoreUnderscore.ReplaceAllString(tagValue, "_")

			if tagName == "" || tagValue == "" {
				continue
			}
			tags = tags.Merge(opentsdb.TagSet{tagName: tagValue})
		}

		// Add metadata tag set type to configuration file?

		if query.hasTime {
			timestamp, _ := strconv.ParseInt(result[1], 10, 64)
			if timestamp < 1 {
				return nil, fmt.Errorf("invalid timestamp")
			}
			AddTS(&md, baseName+query.name, timestamp, result[0], tags, metadata.Unknown, metadata.None, query.description)
		} else {
			Add(&md, baseName+query.name, result[0], tags, metadata.Unknown, metadata.None, query.description)
		}
	}

	return md, nil
}

package collectors

import (
	"database/sql"
	"fmt"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	_ "github.com/lib/pq"
)

var postgresqlMeta = map[string]MetricMeta{
	"xact_commit": {
		Metric:   "commits",
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of transactions in this database that have been committed.",
	},
	"xact_rollbacks": {
		Metric:   "rollbacks",
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of transactions in this database that have been rolled back.",
	},
	"blks_read": {
		Metric:   "blksread",
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of disk blocks read in this database.",
	},
	"blks_hit": {
		Metric:   "blkshit",
		RateType: metadata.Counter,
		Unit:     metadata.Operation,
		Desc:     "Number of times disk blocks were found already in the buffer cache, so that a read was not necessary (this only includes hits in the PostgreSQL buffer cache, not the operating system's file system cache)",
	},
	"numbackends": {
		Metric:   "backends",
		RateType: metadata.Gauge,
		Unit:     metadata.Count,
		Desc:     "Number of backends currently connected to this database.",
	},
	"conflicts": {
		Metric:   "conflicts",
		RateType: metadata.Counter,
		Unit:     metadata.Count,
		Desc:     "Number of queries canceled due to conflicts with recovery in this database (standby only).",
	},
	"deadlocks": {
		Metric:   "deadlocks",
		RateType: metadata.Counter,
		Unit:     metadata.Count,
		Desc:     "Number of deadlocks detected in this database.",
	},
	"locks": {
		Metric:   "locks",
		RateType: metadata.Counter,
		Unit:     metadata.Event,
		Desc:     "Transactions locks.",
	},
	"size": {
		Metric:   "dbsize",
		RateType: metadata.Gauge,
		Unit:     metadata.KBytes,
		Desc:     "Database size.",
	},
}

func init() {
	registerInit(func(c *conf.Conf) {
		for _, p := range c.Postgresql {
			i := p
			collectors = append(collectors,
				&IntervalCollector{
					F: func() (opentsdb.MultiDataPoint, error) {
						return c_postgresql(i.ConnectionString, i.Name)
					},
					name: fmt.Sprintf("postgresql-%s", i.Name),
				})
		}
	})
}

func c_postgresql(s string, n string) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	db, err := postgresqlConnect(s)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	for _, i := range listDbs(db) {
		getDbStats(db, &md, i, n)
	}
	if isMaster(db, &md, n) {
		checkSlaves(db, &md, n)
	}
	return md, nil
}

func postgresqlConnect(s string) (*sql.DB, error) {
	db, err := sql.Open("postgres", s)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	return db, nil
}

func listDbs(db *sql.DB) []string {
	databases, err := db.Query("SELECT datname FROM pg_database WHERE datistemplate = false;")
	if err != nil {
		return nil
	}
	var dbs []string
	for databases.Next() {
		var datname string
		err := databases.Scan(&datname)
		if err != nil {
			slog.Errorln(err)
		}
		dbs = append(dbs, datname)
	}
	return dbs
}

func getDbStats(db *sql.DB, md *opentsdb.MultiDataPoint, datname string, pn string) (*opentsdb.MultiDataPoint, error) {
	rows, err := db.Query(`SELECT db.xact_commit, db.xact_rollback, db.blks_read,
		db.blks_hit, db.numbackends, db.conflicts, db.deadlocks,
		stat_locks.locks, db_size.pg_database_size AS size FROM pg_stat_database AS db,
		(SELECT COUNT(*) AS locks FROM pg_locks) AS stat_locks,
		(SELECT * FROM pg_database_size($1)) AS db_size WHERE db.datname = $1`, datname)
	if err != nil {
		return nil, err
	}
	m := make(map[string]int)
	for rows.Next() {
		var commits, rollbacks, blksread, blkshit, backends, conflicts, deadlocks, locks, size int
		err := rows.Scan(&commits, &rollbacks, &blksread, &blkshit, &backends, &conflicts, &deadlocks, &locks, &size)
		if err != nil {
			slog.Errorln(err)
		}
		m["xact_commit"] = commits
		m["xact_rollback"] = rollbacks
		m["blks_read"] = blksread
		m["blks_hit"] = blkshit
		m["numbackends"] = backends
		m["conflicts"] = conflicts
		m["deadlocks"] = deadlocks
		m["locks"] = locks
		m["size"] = size
		for k, v := range m {
			if n, ok := postgresqlMeta[k]; ok {
				n.TagSet = opentsdb.TagSet{"database": datname}
				if pn != "" {
					n.TagSet["postgresql"] = pn
				}
				Add(md, "postgresql."+n.Metric, v, n.TagSet, n.RateType, n.Unit, n.Desc)
			}
		}
	}
	return md, nil
}

func isMaster(db *sql.DB, md *opentsdb.MultiDataPoint, pn string) bool {
	rows, err := db.Query("select pg_is_in_recovery()")
	if err != nil {
		slog.Errorln(err)
		return false
	}
	defer rows.Close()
	var recovery bool
	tags := opentsdb.TagSet{}
	if pn != "" {
		tags["postgresql"] = pn
	}
	for rows.Next() {
		err := rows.Scan(&recovery)
		if err != nil {
			slog.Errorln(err)
		}
		Add(md, "postgresql.master", !recovery, tags, metadata.Gauge, metadata.Bool, "Whether current instance is master (or standalone server) or standby.")
	}
	return !recovery
}

func checkSlaves(db *sql.DB, md *opentsdb.MultiDataPoint, pn string) (*opentsdb.MultiDataPoint, error) {
	rows, err := db.Query("SELECT pg_xlog_location_diff(pg_current_xlog_location(), flush_location) AS xlog_diff, client_addr FROM pg_stat_replication")
	if err != nil {
		slog.Errorln(err)
	}
	defer rows.Close()
	var diff int
	var client string
	for rows.Next() {
		err := rows.Scan(&diff, &client)
		if err != nil {
			slog.Errorln(err)
		}
		tags := opentsdb.TagSet{"standby": client}
		if pn != "" {
			tags["postgresql"] = pn
		}
		Add(md, "postgresql.xlog.diff", diff, tags, metadata.Gauge, metadata.Bytes, "Difference between pg_current_xlog_location at master server and flushed location at standby.")
	}
	return md, nil
}

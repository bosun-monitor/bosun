package collectors

import (
	"github.com/bosun-monitor/scollector/metadata"
	"github.com/bosun-monitor/scollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	c := &IntervalCollector{
		F: c_mssql,
	}
	c.init = wmiInit(c, func() interface{} { return &[]Win32_PerfRawData_MSSQLSERVER_SQLServerGeneralStatistics{} }, `WHERE Name <> '_Total'`, &sqlQuery)
	collectors = append(collectors, c)
}

var (
	sqlQuery string
)

func c_mssql() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var err error
	add := func(f func() (opentsdb.MultiDataPoint, error)) {
		dps, e := f()
		if e != nil {
			err = e
		}
		md = append(md, dps...)
	}
	add(c_mssql_general)
	add(c_mssql_statistics)
	add(c_mssql_locks)
	add(c_mssql_databases)
	return md, err
}

func c_mssql_general() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerGeneralStatistics
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	if err := queryWmi(q, &dst); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "mssql.user_connections", v.UserConnections, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.connection_resets", v.ConnectionResetPersec, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.logins", v.LoginsPersec, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.logouts", v.LogoutsPersec, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.mars_deadlocks", v.MarsDeadlocks, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.proc_blocked", v.Processesblocked, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.temptables_created", v.TempTablesCreationRate, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.temptables_to_destroy", v.TempTablesForDestruction, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.transactions", v.Transactions, nil, metadata.Unknown, metadata.None, "")
	}
	return md, nil
}

type Win32_PerfRawData_MSSQLSERVER_SQLServerGeneralStatistics struct {
	ConnectionResetPersec    uint64
	LoginsPersec             uint64
	LogoutsPersec            uint64
	MarsDeadlocks            uint64
	Processesblocked         uint64
	TempTablesCreationRate   uint64
	TempTablesForDestruction uint64
	Transactions             uint64
	UserConnections          uint64
}

func c_mssql_statistics() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerSQLStatistics
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "mssql.autoparam_attempts", v.AutoParamAttemptsPersec, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.autoparam_failed", v.FailedAutoParamsPersec, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.autoparam_forced", v.ForcedParameterizationsPersec, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.autoparam_safe", v.SafeAutoParamsPersec, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.autoparam_unsafe", v.UnsafeAutoParamsPersec, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.batches", v.BatchRequestsPersec, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.guided_plans", v.GuidedplanexecutionsPersec, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.misguided_plans", v.MisguidedplanexecutionsPersec, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.compilations", v.SQLCompilationsPersec, nil, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.recompilations", v.SQLReCompilationsPersec, nil, metadata.Unknown, metadata.None, "")
	}
	return md, nil
}

type Win32_PerfRawData_MSSQLSERVER_SQLServerSQLStatistics struct {
	AutoParamAttemptsPersec       uint64
	BatchRequestsPersec           uint64
	FailedAutoParamsPersec        uint64
	ForcedParameterizationsPersec uint64
	GuidedplanexecutionsPersec    uint64
	MisguidedplanexecutionsPersec uint64
	SafeAutoParamsPersec          uint64
	SQLCompilationsPersec         uint64
	SQLReCompilationsPersec       uint64
	UnsafeAutoParamsPersec        uint64
}

func c_mssql_locks() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerLocks
	var q = wmi.CreateQuery(&dst, `WHERE Name = 'Page' OR Name = 'Extent' OR Name = 'Object' or Name = 'Database'`)
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "mssql.lock_wait_time", v.AverageWaitTimems, opentsdb.TagSet{"type": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.lock_requests", v.LockRequestsPersec, opentsdb.TagSet{"type": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.lock_timeouts", v.LockTimeoutsPersec, opentsdb.TagSet{"type": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.lock_timeouts0", v.LockTimeoutstimeout0Persec, opentsdb.TagSet{"type": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.lock_waits", v.LockWaitsPersec, opentsdb.TagSet{"type": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.deadlocks", v.NumberofDeadlocksPersec, opentsdb.TagSet{"type": v.Name}, metadata.Unknown, metadata.None, "")
	}
	return md, nil
}

type Win32_PerfRawData_MSSQLSERVER_SQLServerLocks struct {
	AverageWaitTimems          uint64
	LockRequestsPersec         uint64
	LockTimeoutsPersec         uint64
	LockTimeoutstimeout0Persec uint64
	LockWaitsPersec            uint64
	Name                       string
	NumberofDeadlocksPersec    uint64
}

func c_mssql_databases() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerDatabases
	var q = wmi.CreateQuery(&dst, "")
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "mssql.active_transactions", v.ActiveTransactions, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.backup_restore_throughput", v.BackupPerRestoreThroughputPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.bulkcopy_rows", v.BulkCopyRowsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.bulkcopy_throughput", v.BulkCopyThroughputPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.commit_table_entries", v.Committableentries, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.data_files_size", v.DataFilesSizeKB*1024, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.dbcc_logical_scan_bytes", v.DBCCLogicalScanBytesPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.group_commit_time", v.GroupCommitTimePersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_bytes_flushed", v.LogBytesFlushedPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_cache_hit_ratio", v.LogCacheHitRatio, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_cache_hit_ratio_base", v.LogCacheHitRatio_Base, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_cache_reads", v.LogCacheReadsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_files_size", v.LogFilesSizeKB*1024, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_files_used_size", v.LogFilesUsedSizeKB*1024, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_flushes", v.LogFlushesPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_flush_waits", v.LogFlushWaitsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_flush_wait_time", v.LogFlushWaitTime, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_flush_write_time_ms", v.LogFlushWriteTimems, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_growths", v.LogGrowths, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_pool_cache_misses", v.LogPoolCacheMissesPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_pool_disk_reads", v.LogPoolDiskReadsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_pool_requests", v.LogPoolRequestsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_shrinks", v.LogShrinks, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.log_truncations", v.LogTruncations, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.percent_log_used", v.PercentLogUsed, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.repl_pending_xacts", v.ReplPendingXacts, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.repl_trans_rate", v.ReplTransRate, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.shrink_data_movement_bytes", v.ShrinkDataMovementBytesPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.tracked_transactions", v.TrackedtransactionsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.transactions", v.TransactionsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
		Add(&md, "mssql.write_transactions", v.WriteTransactionsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Unknown, metadata.None, "")
	}
	return md, nil
}

type Win32_PerfRawData_MSSQLSERVER_SQLServerDatabases struct {
	ActiveTransactions               uint64
	BackupPerRestoreThroughputPersec uint64
	BulkCopyRowsPersec               uint64
	BulkCopyThroughputPersec         uint64
	Committableentries               uint64
	DataFilesSizeKB                  uint64
	DBCCLogicalScanBytesPersec       uint64
	GroupCommitTimePersec            uint64
	LogBytesFlushedPersec            uint64
	LogCacheHitRatio                 uint64
	LogCacheHitRatio_Base            uint64
	LogCacheReadsPersec              uint64
	LogFilesSizeKB                   uint64
	LogFilesUsedSizeKB               uint64
	LogFlushesPersec                 uint64
	LogFlushWaitsPersec              uint64
	LogFlushWaitTime                 uint64
	LogFlushWriteTimems              uint64
	LogGrowths                       uint64
	LogPoolCacheMissesPersec         uint64
	LogPoolDiskReadsPersec           uint64
	LogPoolRequestsPersec            uint64
	LogShrinks                       uint64
	LogTruncations                   uint64
	Name                             string
	PercentLogUsed                   uint64
	ReplPendingXacts                 uint64
	ReplTransRate                    uint64
	ShrinkDataMovementBytesPersec    uint64
	TrackedtransactionsPersec        uint64
	TransactionsPersec               uint64
	WriteTransactionsPersec          uint64
}

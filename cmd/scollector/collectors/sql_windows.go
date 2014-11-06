package collectors

import (
	"github.com/StackExchange/wmi"
	"github.com/bosun-monitor/scollector/metadata"
	"github.com/bosun-monitor/scollector/opentsdb"
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
		Add(&md, "mssql.user_connections", v.UserConnections, nil, metadata.Gauge, metadata.Count, descMSSQLUserConnections)
		Add(&md, "mssql.connection_resets", v.ConnectionResetPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLConnectionResetPersec)
		Add(&md, "mssql.logins", v.LoginsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLLoginsPersec)
		Add(&md, "mssql.logouts", v.LogoutsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLLogoutsPersec)
		Add(&md, "mssql.mars_deadlocks", v.MarsDeadlocks, nil, metadata.Counter, metadata.Count, descMSSQLMarsDeadlocks)
		Add(&md, "mssql.proc_blocked", v.Processesblocked, nil, metadata.Gauge, metadata.Count, descMSSQLProcessesblocked)
		Add(&md, "mssql.temptables_created", v.TempTablesCreationRate, nil, metadata.Counter, metadata.PerSecond, descMSSQLTempTablesCreationRate)
		Add(&md, "mssql.temptables_to_destroy", v.TempTablesForDestruction, nil, metadata.Gauge, metadata.Count, descMSSQLTempTablesForDestruction)
		Add(&md, "mssql.transactions", v.Transactions, nil, metadata.Gauge, metadata.Count, descMSSQLTransactions)

	}
	return md, nil
}

const (
	descMSSQLUserConnections          = "Number of users connected to the system."
	descMSSQLConnectionResetPersec    = "Total number of connection resets per second."
	descMSSQLLoginsPersec             = "Total number of logins started per second."
	descMSSQLLogoutsPersec            = "Total number of logouts started per second."
	descMSSQLMarsDeadlocks            = "Number of Mars Deadlocks detected."
	descMSSQLProcessesblocked         = "Number of currently blocked processes."
	descMSSQLTempTablesCreationRate   = "Number of temporary tables/table variables created/sec"
	descMSSQLTempTablesForDestruction = "Number of temporary tables/table variables waiting to be destroyed by the cleanup system thread"
	descMSSQLTransactions             = "Number of transaction enlistments (local, dtc, and bound)."
)

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
		Add(&md, "mssql.autoparam_attempts", v.AutoParamAttemptsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLAutoParamAttemptsPersec)
		Add(&md, "mssql.autoparam_failed", v.FailedAutoParamsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLFailedAutoParamsPersec)
		Add(&md, "mssql.autoparam_forced", v.ForcedParameterizationsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLForcedParameterizationsPersec)
		Add(&md, "mssql.autoparam_safe", v.SafeAutoParamsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLSafeAutoParamsPersec)
		Add(&md, "mssql.autoparam_unsafe", v.UnsafeAutoParamsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLUnsafeAutoParamsPersec)
		Add(&md, "mssql.batches", v.BatchRequestsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLBatchRequestsPersec)
		Add(&md, "mssql.guided_plans", v.GuidedplanexecutionsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLGuidedplanexecutionsPersec)
		Add(&md, "mssql.misguided_plans", v.MisguidedplanexecutionsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLMisguidedplanexecutionsPersec)
		Add(&md, "mssql.compilations", v.SQLCompilationsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLSQLCompilationsPersec)
		Add(&md, "mssql.recompilations", v.SQLReCompilationsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLSQLReCompilationsPersec)
	}
	return md, nil
}

const (
	descMSSQLAutoParamAttemptsPersec       = "Number of auto-parameterization attempts."
	descMSSQLFailedAutoParamsPersec        = "Number of failed auto-parameterizations."
	descMSSQLForcedParameterizationsPersec = "Number of statements parameterized by forced parameterization per second."
	descMSSQLSafeAutoParamsPersec          = "Number of safe auto-parameterizations."
	descMSSQLUnsafeAutoParamsPersec        = "Number of unsafe auto-parameterizations."
	descMSSQLBatchRequestsPersec           = "Number of SQL batch requests received by server."
	descMSSQLGuidedplanexecutionsPersec    = "Number of plan executions per second in which the query plan has been generated by using a plan guide."
	descMSSQLMisguidedplanexecutionsPersec = "Number of plan executions per second in which a plan guide could not be honored during plan generation. The plan guide was disregarded and normal compilation was used to generate the executed plan."
	descMSSQLSQLCompilationsPersec         = "Number of SQL compilations."
	descMSSQLSQLReCompilationsPersec       = "Number of SQL re-compiles."
)

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
		Add(&md, "mssql.lock_wait_time", v.AverageWaitTimems, opentsdb.TagSet{"type": v.Name}, metadata.Counter, metadata.MilliSecond, descMSSQLAverageWaitTimems)
		Add(&md, "mssql.lock_requests", v.LockRequestsPersec, opentsdb.TagSet{"type": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLLockRequestsPersec)
		Add(&md, "mssql.lock_timeouts", v.LockTimeoutsPersec, opentsdb.TagSet{"type": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLLockTimeoutsPersec)
		Add(&md, "mssql.lock_timeouts0", v.LockTimeoutstimeout0Persec, opentsdb.TagSet{"type": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLLockTimeoutstimeout0Persec)
		Add(&md, "mssql.lock_waits", v.LockWaitsPersec, opentsdb.TagSet{"type": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLLockWaitsPersec)
		Add(&md, "mssql.deadlocks", v.NumberofDeadlocksPersec, opentsdb.TagSet{"type": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLNumberofDeadlocksPersec)

	}
	return md, nil
}

const (
	descMSSQLAverageWaitTimems          = "The average amount of wait time (milliseconds) for each lock request that resulted in a wait."
	descMSSQLLockRequestsPersec         = "Number of new locks and lock conversions requested from the lock manager."
	descMSSQLLockTimeoutsPersec         = "Number of lock requests that timed out. This includes requests for NOWAIT locks."
	descMSSQLLockTimeoutstimeout0Persec = "Number of lock requests that timed out. This does not include requests for NOWAIT locks."
	descMSSQLLockWaitsPersec            = "Number of lock requests that could not be satisfied immediately and required the caller to wait before being granted the lock."
	descMSSQLNumberofDeadlocksPersec    = "Number of lock requests that resulted in a deadlock."
)

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
		Add(&md, "mssql.active_transactions", v.ActiveTransactions, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.Count, descMSSQLActiveTransactions)
		Add(&md, "mssql.backup_restore_throughput", v.BackupPerRestoreThroughputPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLBackupPerRestoreThroughputPersec)
		Add(&md, "mssql.bulkcopy_rows", v.BulkCopyRowsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLBulkCopyRowsPersec)
		Add(&md, "mssql.bulkcopy_throughput", v.BulkCopyThroughputPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.KBytes, descMSSQLBulkCopyThroughputPersec)
		Add(&md, "mssql.commit_table_entries", v.Committableentries, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.Count, descMSSQLCommittableentries)
		Add(&md, "mssql.data_files_size", v.DataFilesSizeKB*1024, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.KBytes, descMSSQLDataFilesSizeKB)
		Add(&md, "mssql.dbcc_logical_scan_bytes", v.DBCCLogicalScanBytesPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.BytesPerSecond, descMSSQLDBCCLogicalScanBytesPersec)
		Add(&md, "mssql.group_commit_time", v.GroupCommitTimePersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLGroupCommitTimePersec)
		Add(&md, "mssql.log_bytes_flushed", v.LogBytesFlushedPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLLogBytesFlushedPersec)
		Add(&md, "mssql.log_cache_hit_ratio", v.LogCacheHitRatio, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.Pct, descMSSQLLogCacheHitRatio)
		Add(&md, "mssql.log_cache_hit_ratio_base", v.LogCacheHitRatio_Base, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.None, descMSSQLLogCacheHitRatio_Base)
		Add(&md, "mssql.log_cache_reads", v.LogCacheReadsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLLogCacheReadsPersec)
		Add(&md, "mssql.log_files_size", v.LogFilesSizeKB*1024, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.None, descMSSQLLogFilesSizeKB)
		Add(&md, "mssql.log_files_used_size", v.LogFilesUsedSizeKB*1024, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.KBytes, descMSSQLLogFilesUsedSizeKB)
		Add(&md, "mssql.log_flushes", v.LogFlushesPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.None, descMSSQLLogFlushesPersec)
		Add(&md, "mssql.log_flush_waits", v.LogFlushWaitsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLLogFlushWaitsPersec)
		Add(&md, "mssql.log_flush_wait_time", v.LogFlushWaitTime, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.MilliSecond, descMSSQLLogFlushWaitTime)
		Add(&md, "mssql.log_flush_write_time_ms", v.LogFlushWriteTimems, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.MilliSecond, descMSSQLLogFlushWriteTimems)
		Add(&md, "mssql.log_growths", v.LogGrowths, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.Counter, descMSSQLLogGrowths)
		Add(&md, "mssql.log_pool_cache_misses", v.LogPoolCacheMissesPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLLogPoolCacheMissesPersec)
		Add(&md, "mssql.log_pool_disk_reads", v.LogPoolDiskReadsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLLogPoolDiskReadsPersec)
		Add(&md, "mssql.log_pool_requests", v.LogPoolRequestsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLLogPoolRequestsPersec)
		Add(&md, "mssql.log_shrinks", v.LogShrinks, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.Count, descMSSQLLogShrinks)
		Add(&md, "mssql.log_truncations", v.LogTruncations, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.Count, descMSSQLLogTruncations)
		Add(&md, "mssql.percent_log_used", v.PercentLogUsed, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.Pct, descMSSQLPercentLogUsed)
		Add(&md, "mssql.repl_pending_xacts", v.ReplPendingXacts, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.Count, descMSSQLReplPendingXacts)
		Add(&md, "mssql.repl_trans_rate", v.ReplTransRate, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLReplTransRate)
		Add(&md, "mssql.shrink_data_movement_bytes", v.ShrinkDataMovementBytesPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.BytesPerSecond, descMSSQLShrinkDataMovementBytesPersec)
		Add(&md, "mssql.tracked_transactions", v.TrackedtransactionsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLTrackedtransactionsPersec)
		Add(&md, "mssql.transactions", v.TransactionsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLTransactionsPersec)
		Add(&md, "mssql.write_transactions", v.WriteTransactionsPersec, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.PerSecond, descMSSQLWriteTransactionsPersec)

	}
	return md, nil
}

const (
	descMSSQLActiveTransactions               = "Number of active update transactions for the database."
	descMSSQLBackupPerRestoreThroughputPersec = "Read/write throughput for backup/restore of a database."
	descMSSQLBulkCopyRowsPersec               = "Number of rows bulk copied."
	descMSSQLBulkCopyThroughputPersec         = "KiloBytes bulk copied."
	descMSSQLCommittableentries               = "The size of the in-memory part of the commit table for the database."
	descMSSQLDataFilesSizeKB                  = "The cumulative size of all the data files in the database."
	descMSSQLDBCCLogicalScanBytesPersec       = "Logical read scan rate for DBCC commands"
	descMSSQLGroupCommitTimePersec            = "Group stall time (microseconds) per second."
	descMSSQLLogBytesFlushedPersec            = "Total number of log bytes flushed."
	descMSSQLLogCacheHitRatio                 = "Percentage of log cache reads that were satisfied from the log cache."
	descMSSQLLogCacheHitRatio_Base            = "Percentage of log cache reads that were satisfied from the log cache."
	descMSSQLLogCacheReadsPersec              = "Reads performed through the log manager cache."
	descMSSQLLogFilesSizeKB                   = "The cumulative size of all the log files in the database."
	descMSSQLLogFilesUsedSizeKB               = "The cumulative used size of all the log files in the database."
	descMSSQLLogFlushesPersec                 = "Number of log flushes."
	descMSSQLLogFlushWaitsPersec              = "Number of commits waiting on log flush."
	descMSSQLLogFlushWaitTime                 = "Total wait time (milliseconds)."
	descMSSQLLogFlushWriteTimems              = "Milliseconds it took to perform the writes of log flushes completed in the last second"
	descMSSQLLogGrowths                       = "Total number of log growths for this database."
	descMSSQLLogPoolCacheMissesPersec         = "Log block cache misses from log pool."
	descMSSQLLogPoolDiskReadsPersec           = "Log disk reads via log pool."
	descMSSQLLogPoolRequestsPersec            = "Log block requests performed through log pool."
	descMSSQLLogShrinks                       = "Total number of log shrinks for this database."
	descMSSQLLogTruncations                   = "Total number of log truncations for this database."
	descMSSQLPercentLogUsed                   = "The percent of space in the log that is in use."
	descMSSQLReplPendingXacts                 = "Number of pending replication transactions in the database."
	descMSSQLReplTransRate                    = "Replication transaction rate (replicated transactions/sec.)."
	descMSSQLShrinkDataMovementBytesPersec    = "The rate data is being moved by Autoshrink, DBCC SHRINKDATABASE or SHRINKFILE."
	descMSSQLTrackedtransactionsPersec        = "Number of committed transactions recorded in the commit table for the database."
	descMSSQLTransactionsPersec               = "Number of transactions started for the database."
	descMSSQLWriteTransactionsPersec          = "Number of transactions which wrote to the database in the last second."
)

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

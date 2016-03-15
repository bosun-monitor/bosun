package collectors

import (
	"fmt"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
	"github.com/StackExchange/wmi"
)

func init() {
	c := &IntervalCollector{
		F: c_mssql,
	}
	c.init = wmiInit(c, func() interface{} { return &[]Win32_Service{} }, `WHERE Name Like 'MSSQL$%' or Name = 'MSSQLSERVER'`, &sqlQuery)
	collectors = append(collectors, c)

	var dstCluster []MSCluster_Cluster
	var q = wmi.CreateQuery(&dstCluster, ``)
	if err := queryWmiNamespace(q, &dstCluster, rootMSCluster); err != nil {
		sqlClusterName = "None"
	} else if len(dstCluster) != 1 {
		sqlClusterName = "Unknown"
	} else {
		sqlClusterName = dstCluster[0].Name
	}

	c_replica_db := &IntervalCollector{
		F: c_mssql_replica_db,
	}
	c_replica_db.init = wmiInit(c_replica_db, func() interface{} { return &[]Win32_PerfRawData_MSSQLSERVER_SQLServerDatabaseReplica{} }, `WHERE Name <> '_Total'`, &sqlAGDBQuery)
	collectors = append(collectors, c_replica_db)

	c_replica_server := &IntervalCollector{
		F: c_mssql_replica_server,
	}
	c_replica_server.init = wmiInit(c_replica_server, func() interface{} { return &[]Win32_PerfRawData_MSSQLSERVER_SQLServerAvailabilityReplica{} }, `WHERE Name <> '_Total'`, &sqlAGQuery)
	collectors = append(collectors, c_replica_server)

	c_replica_votes := &IntervalCollector{
		F:        c_mssql_replica_votes,
		Interval: time.Minute * 5,
	}
	c_replica_votes.init = wmiInitNamespace(c_replica_votes, func() interface{} { return &[]MSCluster_Node{} }, fmt.Sprintf("WHERE Name = '%s'", util.Hostname), &sqlAGVotes, rootMSCluster)
	collectors = append(collectors, c_replica_votes)

	c_replica_resources := &IntervalCollector{
		F:        c_mssql_replica_resources,
		Interval: time.Minute,
	}
	c_replica_resources.init = wmiInitNamespace(c_replica_resources, func() interface{} { return &[]MSCluster_Resource{} }, ``, &sqlAGResources, rootMSCluster)
	collectors = append(collectors, c_replica_resources)
}

const (
	rootMSCluster string = "root\\MSCluster"
)

var (
	sqlClusterName string
	sqlQuery       string
	sqlAGDBQuery   string
	sqlAGQuery     string
	sqlAGVotes     string
	sqlAGResources string
)

func c_mssql() (opentsdb.MultiDataPoint, error) {
	var err error    
	var svc_dst []Win32_Service
	var svc_q = wmi.CreateQuery(&svc_dst, `WHERE Name Like 'MSSQL$%' or Name = 'MSSQLSERVER'`)
	err = queryWmi(svc_q, &svc_dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	add := func(f func([]Win32_Service) (opentsdb.MultiDataPoint, error)) {
		dps, e := f(svc_dst)
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

func c_mssql_general(svc_dst []Win32_Service) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	for _, w := range svc_dst {
		var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerGeneralStatistics
		var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
		var label = "mssqlserver"
		if w.Name != `MSSQLSERVER` {
            q = instance_wmiquery(w.Name,q)  
		    label = strings.ToLower(w.Name[6:len(w.Name)])
		}  
		if err := queryWmi(q, &dst); err != nil {
			return nil, err
		}
		for _, v := range dst {
            		tags := opentsdb.TagSet{"instance": label} 
			Add(&md, "mssql.user_connections", v.UserConnections, tags, metadata.Gauge, metadata.Count, descMSSQLUserConnections)
			Add(&md, "mssql.connection_resets", v.ConnectionResetPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLConnectionResetPersec)
			Add(&md, "mssql.logins", v.LoginsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLoginsPersec)
			Add(&md, "mssql.logouts", v.LogoutsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogoutsPersec)
			Add(&md, "mssql.mars_deadlocks", v.MarsDeadlocks, tags, metadata.Counter, metadata.Count, descMSSQLMarsDeadlocks)
			Add(&md, "mssql.proc_blocked", v.Processesblocked, tags, metadata.Gauge, metadata.Count, descMSSQLProcessesblocked)
			Add(&md, "mssql.temptables_created", v.TempTablesCreationRate, tags, metadata.Counter, metadata.PerSecond, descMSSQLTempTablesCreationRate)
			Add(&md, "mssql.temptables_to_destroy", v.TempTablesForDestruction, tags, metadata.Gauge, metadata.Count, descMSSQLTempTablesForDestruction)
			Add(&md, "mssql.transactions_total", v.Transactions, tags, metadata.Gauge, metadata.Count, descMSSQLTransactions)

		}
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
	descMSSQLTempTablesCreationRate   = "Number of temporary tables/table variables created/sec."
	descMSSQLTempTablesForDestruction = "Number of temporary tables/table variables waiting to be destroyed by the cleanup system thread."
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

func c_mssql_statistics(svc_dst []Win32_Service) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	for _, w := range svc_dst {
		var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerSQLStatistics
		var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
		var label = "mssqlserver"
		if w.Name != `MSSQLSERVER` {
            q = instance_wmiquery(w.Name,q)  
		    label = strings.ToLower(w.Name[6:len(w.Name)])
		}  
		err := queryWmi(q, &dst)
		if err != nil {
			return nil, err
		}
		for _, v := range dst {
            		tags := opentsdb.TagSet{"instance": label}  
			Add(&md, "mssql.autoparam_attempts", v.AutoParamAttemptsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLAutoParamAttemptsPersec)
			Add(&md, "mssql.autoparam_failed", v.FailedAutoParamsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLFailedAutoParamsPersec)
			Add(&md, "mssql.autoparam_forced", v.ForcedParameterizationsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLForcedParameterizationsPersec)
			Add(&md, "mssql.autoparam_safe", v.SafeAutoParamsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLSafeAutoParamsPersec)
			Add(&md, "mssql.autoparam_unsafe", v.UnsafeAutoParamsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLUnsafeAutoParamsPersec)
			Add(&md, "mssql.batches", v.BatchRequestsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLBatchRequestsPersec)
			Add(&md, "mssql.guided_plans", v.GuidedplanexecutionsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLGuidedplanexecutionsPersec)
			Add(&md, "mssql.misguided_plans", v.MisguidedplanexecutionsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLMisguidedplanexecutionsPersec)
			Add(&md, "mssql.compilations", v.SQLCompilationsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLSQLCompilationsPersec)
			Add(&md, "mssql.recompilations", v.SQLReCompilationsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLSQLReCompilationsPersec)
		}
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

func c_mssql_locks(svc_dst []Win32_Service) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	for _, w := range svc_dst {
		var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerLocks
		var q = wmi.CreateQuery(&dst, `WHERE Name = 'Page' OR Name = 'Extent' OR Name = 'Object' or Name = 'Database'`)
		var label = "mssqlserver"
		if w.Name != `MSSQLSERVER` {
            q = instance_wmiquery(w.Name,q)  
		    label = strings.ToLower(w.Name[6:len(w.Name)])
		}  
		err := queryWmi(q, &dst)
		if err != nil {
			return nil, err
		}
		for _, v := range dst {
			tags := opentsdb.TagSet{"instance": label,"type": v.Name}
			Add(&md, "mssql.lock_wait_time", v.AverageWaitTimems, tags, metadata.Counter, metadata.MilliSecond, descMSSQLAverageWaitTimems)
			Add(&md, "mssql.lock_requests", v.LockRequestsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLockRequestsPersec)
			Add(&md, "mssql.lock_timeouts", v.LockTimeoutsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLockTimeoutsPersec)
			Add(&md, "mssql.lock_timeouts0", v.LockTimeoutstimeout0Persec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLockTimeoutstimeout0Persec)
			Add(&md, "mssql.lock_waits", v.LockWaitsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLockWaitsPersec)
			Add(&md, "mssql.deadlocks", v.NumberofDeadlocksPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLNumberofDeadlocksPersec)
		}
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

func c_mssql_databases(svc_dst []Win32_Service) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	for _, w := range svc_dst {
		var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerDatabases
		var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
		var label = "mssqlserver"
		if w.Name != `MSSQLSERVER` {
            q = instance_wmiquery(w.Name,q)  
		    label = strings.ToLower(w.Name[6:len(w.Name)])
		}  
		err := queryWmi(q, &dst)
		if err != nil {
			return nil, err
		}
		for _, v := range dst {
			tags := opentsdb.TagSet{"instance": label,"db": v.Name}
			Add(&md, "mssql.active_transactions", v.ActiveTransactions, tags, metadata.Gauge, metadata.Count, descMSSQLActiveTransactions)
			Add(&md, "mssql.backup_restore_throughput", v.BackupPerRestoreThroughputPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLBackupPerRestoreThroughputPersec)
			Add(&md, "mssql.bulkcopy_rows", v.BulkCopyRowsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLBulkCopyRowsPersec)
			Add(&md, "mssql.bulkcopy_throughput", v.BulkCopyThroughputPersec, tags, metadata.Counter, metadata.KBytes, descMSSQLBulkCopyThroughputPersec)
			Add(&md, "mssql.commit_table_entries", v.Committableentries, tags, metadata.Gauge, metadata.Count, descMSSQLCommittableentries)
			Add(&md, "mssql.data_files_size", v.DataFilesSizeKB*1024, tags, metadata.Gauge, metadata.Bytes, descMSSQLDataFilesSizeKB)
			Add(&md, "mssql.dbcc_logical_scan_bytes", v.DBCCLogicalScanBytesPersec, tags, metadata.Counter, metadata.BytesPerSecond, descMSSQLDBCCLogicalScanBytesPersec)
			//Add(&md, "mssql.group_commit_time", v.GroupCommitTimePersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLGroupCommitTimePersec)
			Add(&md, "mssql.log_bytes_flushed", v.LogBytesFlushedPersec, tags, metadata.Counter, metadata.BytesPerSecond, descMSSQLLogBytesFlushedPersec)
			Add(&md, "mssql.log_cache_hit_ratio", v.LogCacheHitRatio, tags, metadata.Counter, metadata.Pct, descMSSQLLogCacheHitRatio)
			Add(&md, "mssql.log_cache_hit_ratio_base", v.LogCacheHitRatio_Base, tags, metadata.Counter, metadata.None, descMSSQLLogCacheHitRatio_Base)
			Add(&md, "mssql.log_cache_reads", v.LogCacheReadsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogCacheReadsPersec)
			Add(&md, "mssql.log_files_size", v.LogFilesSizeKB*1024, tags, metadata.Gauge, metadata.Bytes, descMSSQLLogFilesSizeKB)
			Add(&md, "mssql.log_files_used_size", v.LogFilesUsedSizeKB*1024, tags, metadata.Gauge, metadata.Bytes, descMSSQLLogFilesUsedSizeKB)
			Add(&md, "mssql.log_flushes", v.LogFlushesPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogFlushesPersec)
			Add(&md, "mssql.log_flush_waits", v.LogFlushWaitsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogFlushWaitsPersec)
			Add(&md, "mssql.log_flush_wait_time", v.LogFlushWaitTime, tags, metadata.Counter, metadata.MilliSecond, descMSSQLLogFlushWaitTime)
			//Add(&md, "mssql.log_flush_write_time_ms", v.LogFlushWriteTimems, tags, metadata.Counter, metadata.MilliSecond, descMSSQLLogFlushWriteTimems)
			Add(&md, "mssql.log_growths", v.LogGrowths, tags, metadata.Gauge, metadata.Count, descMSSQLLogGrowths)
			//Add(&md, "mssql.log_pool_cache_misses", v.LogPoolCacheMissesPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogPoolCacheMissesPersec)
			//Add(&md, "mssql.log_pool_disk_reads", v.LogPoolDiskReadsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogPoolDiskReadsPersec)
			//Add(&md, "mssql.log_pool_requests", v.LogPoolRequestsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogPoolRequestsPersec)
			Add(&md, "mssql.log_shrinks", v.LogShrinks, tags, metadata.Gauge, metadata.Count, descMSSQLLogShrinks)
			Add(&md, "mssql.log_truncations", v.LogTruncations, tags, metadata.Gauge, metadata.Count, descMSSQLLogTruncations)
			Add(&md, "mssql.percent_log_used", v.PercentLogUsed, tags, metadata.Gauge, metadata.Pct, descMSSQLPercentLogUsed)
			Add(&md, "mssql.repl_pending_xacts", v.ReplPendingXacts, tags, metadata.Gauge, metadata.Count, descMSSQLReplPendingXacts)
			Add(&md, "mssql.repl_trans_rate", v.ReplTransRate, tags, metadata.Counter, metadata.PerSecond, descMSSQLReplTransRate)
			Add(&md, "mssql.shrink_data_movement_bytes", v.ShrinkDataMovementBytesPersec, tags, metadata.Counter, metadata.BytesPerSecond, descMSSQLShrinkDataMovementBytesPersec)
			Add(&md, "mssql.tracked_transactions", v.TrackedtransactionsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLTrackedtransactionsPersec)
			Add(&md, "mssql.transactions", v.TransactionsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLTransactionsPersec)
			Add(&md, "mssql.write_transactions", v.WriteTransactionsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLWriteTransactionsPersec)
		}
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
	descMSSQLDBCCLogicalScanBytesPersec       = "Logical read scan rate for DBCC commands."
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
	descMSSQLLogFlushWriteTimems              = "Milliseconds it took to perform the writes of log flushes completed in the last second."
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
	//GroupCommitTimePersec            uint64
	LogBytesFlushedPersec uint64
	LogCacheHitRatio      uint64
	LogCacheHitRatio_Base uint64
	LogCacheReadsPersec   uint64
	LogFilesSizeKB        uint64
	LogFilesUsedSizeKB    uint64
	LogFlushesPersec      uint64
	LogFlushWaitsPersec   uint64
	LogFlushWaitTime      uint64
	//LogFlushWriteTimems              uint64
	LogGrowths uint64
	//LogPoolCacheMissesPersec         uint64
	//LogPoolDiskReadsPersec uint64
	//LogPoolRequestsPersec            uint64
	LogShrinks                    uint64
	LogTruncations                uint64
	Name                          string
	PercentLogUsed                uint64
	ReplPendingXacts              uint64
	ReplTransRate                 uint64
	ShrinkDataMovementBytesPersec uint64
	TrackedtransactionsPersec     uint64
	TransactionsPersec            uint64
	WriteTransactionsPersec       uint64
}

func c_mssql_replica_db() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerDatabaseReplica
	if err := queryWmi(sqlAGDBQuery, &dst); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		tags := opentsdb.TagSet{"db": v.Name}
		//see http://technet.microsoft.com/en-us/library/dn135338%28v=sql.110%29.aspx
		Add(&md, "mssql.replica.bytes_db", v.FileBytesReceivedPersec, opentsdb.TagSet{"db": v.Name, "type": "filestream_received"}, metadata.Counter, metadata.BytesPerSecond, descMSSQLReplicaFileBytesReceivedPersec)
		Add(&md, "mssql.replica.bytes_db", v.LogBytesReceivedPersec, opentsdb.TagSet{"db": v.Name, "type": "log_received"}, metadata.Counter, metadata.BytesPerSecond, descMSSQLReplicaLogBytesReceivedPersec)
		Add(&md, "mssql.replica.bytes_db", v.RedoneBytesPersec, opentsdb.TagSet{"db": v.Name, "type": "log_redone"}, metadata.Counter, metadata.BytesPerSecond, descMSSQLReplicaRedoneBytesPersec)
		Add(&md, "mssql.replica.mirrored_transactions", v.MirroredWriteTransactionsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLReplicaMirroredWriteTransactionsPersec)
		Add(&md, "mssql.replica.redo_blocked", v.RedoblockedPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLReplicaRedoblockedPersec)
		Add(&md, "mssql.replica.delay_ack", v.TransactionDelay, opentsdb.TagSet{"db": v.Name}, metadata.Counter, metadata.MilliSecond, descMSSQLReplicaTransactionDelay)
		Add(&md, "mssql.replica.recovery", v.LogSendQueue*1024, opentsdb.TagSet{"db": v.Name, "type": "sending"}, metadata.Gauge, metadata.Bytes, descMSSQLReplicaLogSendQueue)
		Add(&md, "mssql.replica.recovery", v.RecoveryQueue*1024, opentsdb.TagSet{"db": v.Name, "type": "received"}, metadata.Gauge, metadata.Bytes, descMSSQLReplicaRecoveryQueue)
		Add(&md, "mssql.replica.recovery", v.RedoBytesRemaining*1024, opentsdb.TagSet{"db": v.Name, "type": "redo"}, metadata.Gauge, metadata.Bytes, descMSSQLReplicaRedoBytesRemaining)
		Add(&md, "mssql.replica.recovery", v.TotalLogrequiringundo*1024, opentsdb.TagSet{"db": v.Name, "type": "undo_total"}, metadata.Gauge, metadata.Bytes, descMSSQLReplicaTotalLogrequiringundo)
		Add(&md, "mssql.replica.recovery", v.Logremainingforundo*1024, opentsdb.TagSet{"db": v.Name, "type": "undo_remaining"}, metadata.Gauge, metadata.Bytes, descMSSQLReplicaLogremainingforundo)
	}
	return md, nil
}

const (
	descMSSQLReplicaFileBytesReceivedPersec         = "Amount of filestream data received by the availability replica for the database."
	descMSSQLReplicaLogBytesReceivedPersec          = "Amount of logs received by the availability replica for the database."
	descMSSQLReplicaLogremainingforundo             = "The amount of log in bytes remaining to finish the undo phase."
	descMSSQLReplicaLogSendQueue                    = "Amount of logs in bytes that is waiting to be sent to the database replica."
	descMSSQLReplicaMirroredWriteTransactionsPersec = "Number of transactions which wrote to the mirrored database in the last second, that waited for log to be sent to the mirror."
	descMSSQLReplicaRecoveryQueue                   = "Total number of hardened log in bytes that is waiting to be redone on the secondary."
	descMSSQLReplicaRedoblockedPersec               = "Number of times redo gets blocked in the last second."
	descMSSQLReplicaRedoBytesRemaining              = "The amount of log in bytes remaining to be redone to finish the reverting phase."
	descMSSQLReplicaRedoneBytesPersec               = "Amount of log records redone in the last second to catch up the database replica."
	descMSSQLReplicaTotalLogrequiringundo           = "The amount of log in bytes that need to be undone."
	descMSSQLReplicaTransactionDelay                = "Number of milliseconds transaction termination waited for acknowledgement per second."
)

type Win32_PerfRawData_MSSQLSERVER_SQLServerDatabaseReplica struct {
	FileBytesReceivedPersec         uint64
	LogBytesReceivedPersec          uint64
	Logremainingforundo             uint64
	LogSendQueue                    uint64
	MirroredWriteTransactionsPersec uint64
	Name                            string
	RecoveryQueue                   uint64
	RedoblockedPersec               uint64
	RedoBytesRemaining              uint64
	RedoneBytesPersec               uint64
	TotalLogrequiringundo           uint64
	TransactionDelay                uint64
}

func c_mssql_replica_server() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerAvailabilityReplica
	if err := queryWmi(sqlAGQuery, &dst); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		//split name into AvailibilityGroup and Destination. Name is in 'Group:Destination' format
		s := strings.Split(v.Name, ":")
		if len(s) != 2 {
			return nil, fmt.Errorf("Invalid Availibility Group Name: '%s'", v.Name)
		}
		destination := strings.ToLower(s[1])
		//see http://technet.microsoft.com/en-us/library/ff878472(v=sql.110).aspx
		//also https://livedemo.customers.na.apm.ibmserviceengage.com/help/index.jsp?topic=%2Fcom.ibm.koq.doc%2Fattr_koqadbst.htm
		Add(&md, "mssql.replica.bytes_ag", v.BytesReceivedfromReplicaPersec, opentsdb.TagSet{"group": s[0], "destination": destination, "type": "received"}, metadata.Counter, metadata.BytesPerSecond, descMSSQLReplicaBytesReceivedfromReplicaPersec)
		Add(&md, "mssql.replica.bytes_ag", v.BytesSenttoReplicaPersec, opentsdb.TagSet{"group": s[0], "destination": destination, "type": "sent_replica"}, metadata.Counter, metadata.BytesPerSecond, descMSSQLReplicaBytesSenttoReplicaPersec)
		Add(&md, "mssql.replica.bytes_ag", v.BytesSenttoTransportPersec, opentsdb.TagSet{"group": s[0], "destination": destination, "type": "sent_transport"}, metadata.Counter, metadata.BytesPerSecond, descMSSQLReplicaBytesSenttoTransportPersec)
		Add(&md, "mssql.replica.delay_flow", v.FlowControlTimemsPersec, opentsdb.TagSet{"group": s[0], "destination": destination}, metadata.Counter, metadata.MilliSecond, descMSSQLReplicaFlowControlTimemsPersec)
		Add(&md, "mssql.replica.messages", v.FlowControlPersec, opentsdb.TagSet{"group": s[0], "destination": destination, "type": "flow_control"}, metadata.Counter, metadata.PerSecond, descMSSQLReplicaFlowControlPersec)
		Add(&md, "mssql.replica.messages", v.ReceivesfromReplicaPersec, opentsdb.TagSet{"group": s[0], "destination": destination, "type": "received"}, metadata.Counter, metadata.PerSecond, descMSSQLReplicaReceivesfromReplicaPersec)
		Add(&md, "mssql.replica.messages", v.ResentMessagesPersec, opentsdb.TagSet{"group": s[0], "destination": destination, "type": "resent"}, metadata.Counter, metadata.PerSecond, descMSSQLReplicaResentMessagesPersec)
		Add(&md, "mssql.replica.messages", v.SendstoReplicaPersec, opentsdb.TagSet{"group": s[0], "destination": destination, "type": "sent_replica"}, metadata.Counter, metadata.PerSecond, descMSSQLReplicaSendstoReplicaPersec)
		Add(&md, "mssql.replica.messages", v.SendstoTransportPersec, opentsdb.TagSet{"group": s[0], "destination": destination, "type": "sent_transport"}, metadata.Counter, metadata.PerSecond, descMSSQLReplicaSendstoTransportPersec)

	}
	return md, nil
}

const (
	descMSSQLReplicaBytesReceivedfromReplicaPersec = "Total bytes receieved from the availability replica."
	descMSSQLReplicaBytesSenttoReplicaPersec       = "Total bytes sent to the availabilty replica."
	descMSSQLReplicaBytesSenttoTransportPersec     = "Total bytes sent to transport for the availabilty replica."
	descMSSQLReplicaFlowControlPersec              = "Number of flow control initiated in the last second."
	descMSSQLReplicaFlowControlTimemsPersec        = "Time in milliseconds messages waited on flow control in the last second."
	descMSSQLReplicaReceivesfromReplicaPersec      = "Total receives from the availability replica."
	descMSSQLReplicaResentMessagesPersec           = "Number of messages being resent in the last second."
	descMSSQLReplicaSendstoReplicaPersec           = "Total sends to the availability replica."
	descMSSQLReplicaSendstoTransportPersec         = "Total sends to transport for the availability replica."
)

type Win32_PerfRawData_MSSQLSERVER_SQLServerAvailabilityReplica struct {
	BytesReceivedfromReplicaPersec uint64
	BytesSenttoReplicaPersec       uint64
	BytesSenttoTransportPersec     uint64
	FlowControlPersec              uint64
	FlowControlTimemsPersec        uint64
	Name                           string
	ReceivesfromReplicaPersec      uint64
	ResentMessagesPersec           uint64
	SendstoReplicaPersec           uint64
	SendstoTransportPersec         uint64
}

func c_mssql_replica_votes() (opentsdb.MultiDataPoint, error) {
	var dst []MSCluster_Node
	if err := queryWmiNamespace(sqlAGVotes, &dst, rootMSCluster); err != nil {
		return nil, err
	}

	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "mssql.replica.votes", v.NodeWeight, opentsdb.TagSet{"cluster": sqlClusterName, "type": "standard"}, metadata.Gauge, metadata.Count, descMSSQLReplicaNodeWeight)
		Add(&md, "mssql.replica.votes", v.DynamicWeight, opentsdb.TagSet{"cluster": sqlClusterName, "type": "dynamic"}, metadata.Gauge, metadata.Count, descMSSQLReplicaDynamicWeight)
		Add(&md, "mssql.replica.cluster_state", v.State, opentsdb.TagSet{"cluster": sqlClusterName}, metadata.Gauge, metadata.StatusCode, descMSSQLReplicaClusterState)
	}
	return md, nil
}

const (
	descMSSQLReplicaNodeWeight    = "The current vote weight of the node."
	descMSSQLReplicaDynamicWeight = "The vote weight of the node when adjusted by the dynamic quorum feature."
	descMSSQLReplicaClusterState  = "StateUnknown (-1), Up (0), Down (1), Paused (2), Joining (3)."
)

type MSCluster_Node struct {
	Name          string
	NodeWeight    uint32
	DynamicWeight uint32
	State         uint32
}

type MSCluster_Cluster struct {
	Name string
}

func c_mssql_replica_resources() (opentsdb.MultiDataPoint, error) {
	var dst []MSCluster_Resource
	//Only report metrics for resources owned by this node
	var q = wmi.CreateQuery(&dst, fmt.Sprintf("WHERE OwnerNode = '%s'", util.Hostname))
	if err := queryWmiNamespace(q, &dst, rootMSCluster); err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "mssql.replica.resource_state", v.State, opentsdb.TagSet{"group": v.OwnerGroup, "type": v.Type, "name": v.Name}, metadata.Gauge, metadata.StatusCode, descMSSQLReplicaResourceState)
	}
	return md, nil
}

const (
	descMSSQLReplicaResourceState = "StateUnknown (-1), TBD (0), Initializing (1), Online (2), Offline (3), Failed(4), Pending(128), Online Pending (129), Offline Pending (130)."
)

type MSCluster_Resource struct {
	Name       string
	OwnerGroup string
	OwnerNode  string
	Type       string
	State      uint32
}

func instance_wmiquery(instancename string, wmiquery string) (string) {
    var newname = strings.Replace(strings.Replace(instancename,`$`,"",1),`_`,"",-1)
    return strings.Replace(wmiquery,`MSSQLSERVER_SQLServer`,newname + `_` + newname,1)
}
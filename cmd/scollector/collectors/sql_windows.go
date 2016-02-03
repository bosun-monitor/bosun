package collectors

import (
	"fmt"
	"strings"
	"time"

	"bosun.org/_third_party/github.com/StackExchange/wmi"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
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
    add(c_mssql_memory)
    add(c_mssql_buffer)
	return md, err
}

func c_mssql_general(svc_dst []Win32_Service) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerGeneralStatistics
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)	
    for _, w := range svc_dst {
        var label = ""
        if w.Name != `MSSQLSERVER` {
            q = strings.Replace(strings.Replace(q,`MSSQLSERVER_SQLServer`,w.Name + `_` + w.Name,1),`$`,"",2)
            label = strings.ToLower(w.Name[6:len(w.Name)]) + "."
        }  
        err := queryWmi(q, &dst)  
        if  err != nil {
            return nil, err
        }
        for _, v := range dst {
            Add(&md, "mssql." + label + "user_connections", v.UserConnections, nil, metadata.Gauge, metadata.Count, descMSSQLUserConnections)
            Add(&md, "mssql." + label + "connection_resets", v.ConnectionResetPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLConnectionResetPersec)
            Add(&md, "mssql." + label + "logins", v.LoginsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLLoginsPersec)
            Add(&md, "mssql." + label + "logouts", v.LogoutsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLLogoutsPersec)
            Add(&md, "mssql." + label + "mars_deadlocks", v.MarsDeadlocks, nil, metadata.Counter, metadata.Count, descMSSQLMarsDeadlocks)
            Add(&md, "mssql." + label + "proc_blocked", v.Processesblocked, nil, metadata.Gauge, metadata.Count, descMSSQLProcessesblocked)
            Add(&md, "mssql." + label + "temptables_created", v.TempTablesCreationRate, nil, metadata.Counter, metadata.PerSecond, descMSSQLTempTablesCreationRate)
            Add(&md, "mssql." + label + "temptables_to_destroy", v.TempTablesForDestruction, nil, metadata.Gauge, metadata.Count, descMSSQLTempTablesForDestruction)
            Add(&md, "mssql." + label + "transactions_total", v.Transactions, nil, metadata.Gauge, metadata.Count, descMSSQLTransactions)
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
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerSQLStatistics
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
    for _, w := range svc_dst {
        var label = ""
        if w.Name != `MSSQLSERVER` {
            q = strings.Replace(strings.Replace(q,`MSSQLSERVER_SQLServer`,w.Name + `_` + w.Name,1),`$`,"",2)
            label = strings.ToLower(w.Name[6:len(w.Name)]) + "."
        }  
        err := queryWmi(q, &dst)
        if err != nil {
            return nil, err
        }
        for _, v := range dst {
            Add(&md, "mssql." + label + "autoparam_attempts", v.AutoParamAttemptsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLAutoParamAttemptsPersec)
            Add(&md, "mssql." + label + "autoparam_failed", v.FailedAutoParamsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLFailedAutoParamsPersec)
            Add(&md, "mssql." + label + "autoparam_forced", v.ForcedParameterizationsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLForcedParameterizationsPersec)
            Add(&md, "mssql." + label + "autoparam_safe", v.SafeAutoParamsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLSafeAutoParamsPersec)
            Add(&md, "mssql." + label + "autoparam_unsafe", v.UnsafeAutoParamsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLUnsafeAutoParamsPersec)
            Add(&md, "mssql." + label + "batches", v.BatchRequestsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLBatchRequestsPersec)
            Add(&md, "mssql." + label + "guided_plans", v.GuidedplanexecutionsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLGuidedplanexecutionsPersec)
            Add(&md, "mssql." + label + "misguided_plans", v.MisguidedplanexecutionsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLMisguidedplanexecutionsPersec)
            Add(&md, "mssql." + label + "compilations", v.SQLCompilationsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLSQLCompilationsPersec)
            Add(&md, "mssql." + label + "recompilations", v.SQLReCompilationsPersec, nil, metadata.Counter, metadata.PerSecond, descMSSQLSQLReCompilationsPersec)
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
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerLocks
	var q = wmi.CreateQuery(&dst, `WHERE Name = 'Page' OR Name = 'Extent' OR Name = 'Object' or Name = 'Database'`)
    for _, w := range svc_dst {
        var label = ""
        if w.Name != `MSSQLSERVER` {
            q = strings.Replace(strings.Replace(q,`MSSQLSERVER_SQLServer`,w.Name + `_` + w.Name,1),`$`,"",2)
            label = strings.ToLower(w.Name[6:len(w.Name)]) + "."
        } 
        err := queryWmi(q, &dst)
        if err != nil {
            return nil, err
        }
        for _, v := range dst {
            tags := opentsdb.TagSet{"type": v.Name}
            Add(&md, "mssql." + label + "lock_wait_time", v.AverageWaitTimems, tags, metadata.Counter, metadata.MilliSecond, descMSSQLAverageWaitTimems)
            Add(&md, "mssql." + label + "lock_requests", v.LockRequestsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLockRequestsPersec)
            Add(&md, "mssql." + label + "lock_timeouts", v.LockTimeoutsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLockTimeoutsPersec)
            Add(&md, "mssql." + label + "lock_timeouts0", v.LockTimeoutstimeout0Persec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLockTimeoutstimeout0Persec)
            Add(&md, "mssql." + label + "lock_waits", v.LockWaitsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLockWaitsPersec)
            Add(&md, "mssql." + label + "deadlocks", v.NumberofDeadlocksPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLNumberofDeadlocksPersec)

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
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerDatabases
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
    for _, w := range svc_dst {
        var label = ""
        if w.Name != `MSSQLSERVER` {
            q = strings.Replace(strings.Replace(q,`MSSQLSERVER_SQLServer`,w.Name + `_` + w.Name,1),`$`,"",2)
            label = strings.ToLower(w.Name[6:len(w.Name)]) + "."
        } 
        err := queryWmi(q, &dst)
        if err != nil {
            return nil, err
        }
        for _, v := range dst {
            tags := opentsdb.TagSet{"db": v.Name}
            Add(&md, "mssql." + label + "active_transactions", v.ActiveTransactions, tags, metadata.Gauge, metadata.Count, descMSSQLActiveTransactions)
            Add(&md, "mssql." + label + "backup_restore_throughput", v.BackupPerRestoreThroughputPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLBackupPerRestoreThroughputPersec)
            Add(&md, "mssql." + label + "bulkcopy_rows", v.BulkCopyRowsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLBulkCopyRowsPersec)
            Add(&md, "mssql." + label + "bulkcopy_throughput", v.BulkCopyThroughputPersec, tags, metadata.Counter, metadata.KBytes, descMSSQLBulkCopyThroughputPersec)
            Add(&md, "mssql." + label + "commit_table_entries", v.Committableentries, tags, metadata.Gauge, metadata.Count, descMSSQLCommittableentries)
            Add(&md, "mssql." + label + "data_files_size", v.DataFilesSizeKB*1024, tags, metadata.Gauge, metadata.Bytes, descMSSQLDataFilesSizeKB)
            Add(&md, "mssql." + label + "dbcc_logical_scan_bytes", v.DBCCLogicalScanBytesPersec, tags, metadata.Counter, metadata.BytesPerSecond, descMSSQLDBCCLogicalScanBytesPersec)
            //Add(&md, "mssql.group_commit_time", v.GroupCommitTimePersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLGroupCommitTimePersec)
            Add(&md, "mssql." + label + "log_bytes_flushed", v.LogBytesFlushedPersec, tags, metadata.Counter, metadata.BytesPerSecond, descMSSQLLogBytesFlushedPersec)
            Add(&md, "mssql." + label + "log_cache_hit_ratio", v.LogCacheHitRatio, tags, metadata.Counter, metadata.Pct, descMSSQLLogCacheHitRatio)
            Add(&md, "mssql." + label + "log_cache_hit_ratio_base", v.LogCacheHitRatio_Base, tags, metadata.Counter, metadata.None, descMSSQLLogCacheHitRatio_Base)
            Add(&md, "mssql." + label + "log_cache_reads", v.LogCacheReadsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogCacheReadsPersec)
            Add(&md, "mssql." + label + "log_files_size", v.LogFilesSizeKB*1024, tags, metadata.Gauge, metadata.Bytes, descMSSQLLogFilesSizeKB)
            Add(&md, "mssql." + label + "log_files_used_size", v.LogFilesUsedSizeKB*1024, tags, metadata.Gauge, metadata.Bytes, descMSSQLLogFilesUsedSizeKB)
            Add(&md, "mssql." + label + "log_flushes", v.LogFlushesPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogFlushesPersec)
            Add(&md, "mssql." + label + "log_flush_waits", v.LogFlushWaitsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogFlushWaitsPersec)
            Add(&md, "mssql." + label + "log_flush_wait_time", v.LogFlushWaitTime, tags, metadata.Counter, metadata.MilliSecond, descMSSQLLogFlushWaitTime)
            //Add(&md, "mssql.log_flush_write_time_ms", v.LogFlushWriteTimems, tags, metadata.Counter, metadata.MilliSecond, descMSSQLLogFlushWriteTimems)
            Add(&md, "mssql." + label + "log_growths", v.LogGrowths, tags, metadata.Gauge, metadata.Count, descMSSQLLogGrowths)
            //Add(&md, "mssql.log_pool_cache_misses", v.LogPoolCacheMissesPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogPoolCacheMissesPersec)
            //Add(&md, "mssql.log_pool_disk_reads", v.LogPoolDiskReadsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogPoolDiskReadsPersec)
            //Add(&md, "mssql.log_pool_requests", v.LogPoolRequestsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLLogPoolRequestsPersec)
            Add(&md, "mssql." + label + "log_shrinks", v.LogShrinks, tags, metadata.Gauge, metadata.Count, descMSSQLLogShrinks)
            Add(&md, "mssql." + label + "log_truncations", v.LogTruncations, tags, metadata.Gauge, metadata.Count, descMSSQLLogTruncations)
            Add(&md, "mssql." + label + "percent_log_used", v.PercentLogUsed, tags, metadata.Gauge, metadata.Pct, descMSSQLPercentLogUsed)
            Add(&md, "mssql." + label + "repl_pending_xacts", v.ReplPendingXacts, tags, metadata.Gauge, metadata.Count, descMSSQLReplPendingXacts)
            Add(&md, "mssql." + label + "repl_trans_rate", v.ReplTransRate, tags, metadata.Counter, metadata.PerSecond, descMSSQLReplTransRate)
            Add(&md, "mssql." + label + "shrink_data_movement_bytes", v.ShrinkDataMovementBytesPersec, tags, metadata.Counter, metadata.BytesPerSecond, descMSSQLShrinkDataMovementBytesPersec)
            Add(&md, "mssql." + label + "tracked_transactions", v.TrackedtransactionsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLTrackedtransactionsPersec)
            Add(&md, "mssql." + label + "transactions", v.TransactionsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLTransactionsPersec)
            Add(&md, "mssql." + label + "write_transactions", v.WriteTransactionsPersec, tags, metadata.Counter, metadata.PerSecond, descMSSQLWriteTransactionsPersec)
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


func c_mssql_memory(svc_dst []Win32_Service) (opentsdb.MultiDataPoint, error) {
    var md opentsdb.MultiDataPoint
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerMemoryManager
	var q = wmi.CreateQuery(&dst, `WHERE Name <> '_Total'`)
    for _, w := range svc_dst {
        var label = ""
        if w.Name != `MSSQLSERVER` {
            q = strings.Replace(strings.Replace(q,`MSSQLSERVER_SQLServer`,w.Name + `_` + w.Name,1),`$`,"",2)
            label = strings.ToLower(w.Name[6:len(w.Name)]) + "."
        } 
        err := queryWmi(q, &dst)
        if err != nil {
            return nil, err
        }
        for _, v := range dst {
            Add(&md, "mssql." + label + "connectionmemorykb", v.ConnectionMemoryKB, nil, metadata.Gauge,  metadata.KBytes, descMSSQLConnectionMemoryKB)
            Add(&md, "mssql." + label + "databasecachememorykb", v.DatabaseCacheMemoryKB, nil, metadata.Gauge,  metadata.Count, descMSSQLDatabaseCacheMemoryKB)
            Add(&md, "mssql." + label + "externalbenefitofmemory", v.Externalbenefitofmemory, nil, metadata.Gauge,  metadata.None, descMSSQLExternalbenefitofmemory)
            Add(&md, "mssql." + label + "freememorykb", v.FreeMemoryKB, nil, metadata.Gauge,  metadata.PerSecond, descMSSQLFreeMemoryKB)
            Add(&md, "mssql." + label + "grantedworkspacememorykb", v.GrantedWorkspaceMemoryKB, nil, metadata.Gauge,  metadata.Count, descMSSQLGrantedWorkspaceMemoryKB)
            Add(&md, "mssql." + label + "lockblocks", v.LockBlocks, nil, metadata.Gauge,  metadata.Count, descMSSQLLockBlocks)
            Add(&md, "mssql." + label + "lockblocksallocated", v.LockBlocksAllocated, nil, metadata.Gauge,  metadata.Count, descMSSQLLockBlocksAllocated)
            Add(&md, "mssql." + label + "lockmemorykb", v.LockMemoryKB, nil, metadata.Gauge,  metadata.Count, descMSSQLLockMemoryKB)
            Add(&md, "mssql." + label + "lockownerblocks", v.LockOwnerBlocks, nil, metadata.Gauge,  metadata.Count, descMSSQLLockOwnerBlocks)
            Add(&md, "mssql." + label + "lockownerblocksallocated", v.LockOwnerBlocksAllocated, nil, metadata.Gauge,  metadata.PerSecond, descMSSQLLockOwnerBlocksAllocated)
            Add(&md, "mssql." + label + "logpoolmemorykb", v.LogPoolMemoryKB, nil, metadata.Gauge,  metadata.PerSecond, descMSSQLLogPoolMemoryKB)
            Add(&md, "mssql." + label + "maximumworkspacememorykb", v.MaximumWorkspaceMemoryKB, nil, metadata.Gauge,  metadata.Count, descMSSQLMaximumWorkspaceMemoryKB)
            Add(&md, "mssql." + label + "memorygrantsoutstanding", v.MemoryGrantsOutstanding, nil, metadata.Gauge,  metadata.PerSecond, descMSSQLMemoryGrantsOutstanding)
            Add(&md, "mssql." + label + "memorygrantspending", v.MemoryGrantsPending, nil, metadata.Gauge,  metadata.PerSecond, descMSSQLMemoryGrantsPending)
            Add(&md, "mssql." + label + "optimizermemorykb", v.OptimizerMemoryKB, nil, metadata.Gauge,  metadata.Count, descMSSQLOptimizerMemoryKB)
            Add(&md, "mssql." + label + "reservedservermemorykb", v.ReservedServerMemoryKB, nil, metadata.Gauge,  metadata.PerSecond, descMSSQLReservedServerMemoryKB)
            Add(&md, "mssql." + label + "sqlcachememorykb", v.SQLCacheMemoryKB, nil, metadata.Gauge,  metadata.Count, descMSSQLSQLCacheMemoryKB)
            Add(&md, "mssql." + label + "stolenservermemorykb", v.StolenServerMemoryKB, nil, metadata.Gauge,  metadata.PerSecond, descMSSQLStolenServerMemoryKB)
            Add(&md, "mssql." + label + "targetservermemorykb", v.TargetServerMemoryKB, nil, metadata.Gauge,  metadata.PerSecond, descMSSQLTargetServerMemoryKB)
            Add(&md, "mssql." + label + "totalservermemorykb", v.TotalServerMemoryKB, nil, metadata.Gauge,  metadata.PerSecond, descMSSQLTotalServerMemoryKB)
        }
	}
	return md, nil
}

const (
    descMSSQLConnectionMemoryKB = "Connection Memory (KB)"
    descMSSQLDatabaseCacheMemoryKB = "Database Cache Memory (KB)"
    descMSSQLExternalbenefitofmemory = "External benefit of memory"
    descMSSQLFreeMemoryKB = "Free Memory (KB)"
    descMSSQLGrantedWorkspaceMemoryKB = "Granted Workspace Memory (KB)"
    descMSSQLLockBlocks = "Lock Blocks"
    descMSSQLLockBlocksAllocated = "Lock Blocks Allocated"
    descMSSQLLockMemoryKB = "Lock Memory (KB)"
    descMSSQLLockOwnerBlocks = "Lock Owner Blocks"
    descMSSQLLockOwnerBlocksAllocated = "Lock Owner Blocks Allocated"
    descMSSQLLogPoolMemoryKB = "Log Pool Memory (KB)"
    descMSSQLMaximumWorkspaceMemoryKB = "Maximum Workspace Memory (KB)"
    descMSSQLMemoryGrantsOutstanding = "Memory Grants Outstanding"
    descMSSQLMemoryGrantsPending = "Memory Grants Pending"
    descMSSQLOptimizerMemoryKB = "Optimizer Memory (KB)"
    descMSSQLReservedServerMemoryKB = "Reserved Server Memory (KB)"
    descMSSQLSQLCacheMemoryKB = "SQL Cache Memory (KB)"
    descMSSQLStolenServerMemoryKB = "Stolen Server Memory (KB)"
    descMSSQLTargetServerMemoryKB = "Target Server Memory (KB)"
    descMSSQLTotalServerMemoryKB = "Total Server Memory (KB)"
)

type Win32_PerfRawData_MSSQLSERVER_SQLServerMemoryManager struct {
    ConnectionMemoryKB	uint64
    DatabaseCacheMemoryKB	uint64
    Externalbenefitofmemory	uint64
    FreeMemoryKB	uint64
    GrantedWorkspaceMemoryKB	uint64
    LockBlocks	uint64
    LockBlocksAllocated	uint64
    LockMemoryKB	uint64
    LockOwnerBlocks	uint64
    LockOwnerBlocksAllocated	uint64
    LogPoolMemoryKB	uint64
    MaximumWorkspaceMemoryKB	uint64
    MemoryGrantsOutstanding	uint64
    MemoryGrantsPending	uint64
    OptimizerMemoryKB	uint64
    ReservedServerMemoryKB	uint64
    SQLCacheMemoryKB	uint64
    StolenServerMemoryKB	uint64
    TargetServerMemoryKB	uint64
    TotalServerMemoryKB	uint64
}

func c_mssql_buffer(svc_dst []Win32_Service) (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerBufferManager
	var q = wmi.CreateQuery(&dst, "")
    for _, w := range svc_dst {
        var label = ""
        if w.Name != `MSSQLSERVER` {
            q = strings.Replace(strings.Replace(q,`MSSQLSERVER_SQLServer`,w.Name + `_` + w.Name,1),`$`,"",2)
            label = strings.ToLower(w.Name[6:len(w.Name)]) + "."
        } 
        err := queryWmi(q, &dst)
        if err != nil {
            return nil, err
        }
        for _, v := range dst {
            Add(&md, "mssql." + label + "background_writer_pages_psec", v.BackgroundwriterpagesPersec, nil, metadata.Counter,  metadata.PerSecond, descMSSQLBackgroundwriterpagesPersec)
            Add(&md, "mssql." + label + "buffer_cache_hit_ratio", v.Buffercachehitratio, nil, metadata.Counter,  metadata.Count, descMSSQLBuffercachehitratio)
            //Add(&md, "mssql.buffercachehitratio_base", v.Buffercachehitratio_Base, nil, metadata.Counter,  metadata.None, descMSSQLBuffercachehitratio_Base)
            Add(&md, "mssql." + label + "checkpointpagespersec", v.CheckpointpagesPersec, nil, metadata.Counter,  metadata.PerSecond, descMSSQLCheckpointpagesPersec)
            Add(&md, "mssql." + label + "databasepages", v.Databasepages, nil, metadata.Gauge,  metadata.Count, descMSSQLDatabasepages)
            Add(&md, "mssql." + label + "extensionallocatedpages", v.Extensionallocatedpages, nil, metadata.Gauge,  metadata.Count, descMSSQLExtensionallocatedpages)
            Add(&md, "mssql." + label + "extensionfreepages", v.Extensionfreepages, nil, metadata.Gauge,  metadata.Count, descMSSQLExtensionfreepages)
            Add(&md, "mssql." + label + "extensioninuseaspercentage", v.Extensioninuseaspercentage, nil, metadata.Gauge,  metadata.Count, descMSSQLExtensioninuseaspercentage)
            Add(&md, "mssql." + label + "extensionoutstandingiocounter", v.ExtensionoutstandingIOcounter, nil, metadata.Counter,  metadata.Count, descMSSQLExtensionoutstandingIOcounter)
            Add(&md, "mssql." + label + "extensionpageevictionspersec", v.ExtensionpageevictionsPersec, nil, metadata.Counter,  metadata.PerSecond, descMSSQLExtensionpageevictionsPersec)
            Add(&md, "mssql." + label + "extensionpagereadspersec", v.ExtensionpagereadsPersec, nil, metadata.Counter,  metadata.PerSecond, descMSSQLExtensionpagereadsPersec)
            Add(&md, "mssql." + label + "extensionpageunreferencedtime", v.Extensionpageunreferencedtime, nil, metadata.Gauge,  metadata.Count, descMSSQLExtensionpageunreferencedtime)
            Add(&md, "mssql." + label + "extensionpagewritespersec", v.ExtensionpagewritesPersec, nil, metadata.Counter,  metadata.PerSecond, descMSSQLExtensionpagewritesPersec)
            Add(&md, "mssql." + label + "freeliststallspersec", v.FreeliststallsPersec, nil, metadata.Counter,  metadata.PerSecond, descMSSQLFreeliststallsPersec)
            Add(&md, "mssql." + label + "integralcontrollerslope", v.IntegralControllerSlope, nil, metadata.Gauge,  metadata.Count, descMSSQLIntegralControllerSlope)
            Add(&md, "mssql." + label + "lazywritespersec", v.LazywritesPersec, nil, metadata.Counter,  metadata.PerSecond, descMSSQLLazywritesPersec)
            Add(&md, "mssql." + label + "pagelifeexpectancy", v.Pagelifeexpectancy, nil, metadata.Gauge,  metadata.Count, descMSSQLPagelifeexpectancy)
            Add(&md, "mssql." + label + "pagelookupspersec", v.PagelookupsPersec, nil, metadata.Counter,  metadata.PerSecond, descMSSQLPagelookupsPersec)
            Add(&md, "mssql." + label + "pagereadspersec", v.PagereadsPersec, nil, metadata.Counter,  metadata.PerSecond, descMSSQLPagereadsPersec)
            Add(&md, "mssql." + label + "pagewritespersec", v.PagewritesPersec, nil, metadata.Counter,  metadata.PerSecond, descMSSQLPagewritesPersec)
            Add(&md, "mssql." + label + "readaheadpagespersec", v.ReadaheadpagesPersec, nil, metadata.Counter,  metadata.PerSecond, descMSSQLReadaheadpagesPersec)
            Add(&md, "mssql." + label + "readaheadtimepersec", v.ReadaheadtimePersec, nil, metadata.Counter,  metadata.PerSecond, descMSSQLReadaheadtimePersec)
            Add(&md, "mssql." + label + "targetpages", v.Targetpages, nil, metadata.Gauge,  metadata.Count, descMSSQLTargetpages)
        }
	}
	return md, nil
}

const (
    descMSSQLBackgroundwriterpagesPersec = "Background writer pages/sec"
    descMSSQLBuffercachehitratio = "Buffer cache hit ratio"
    descMSSQLBuffercachehitratio_Base = ""
    descMSSQLCheckpointpagesPersec = "Checkpoint pages/sec"
    descMSSQLDatabasepages = "Database pages"
    descMSSQLExtensionallocatedpages = "Extension allocated pages"
    descMSSQLExtensionfreepages = "Extension free pages"
    descMSSQLExtensioninuseaspercentage = "Extension in use as percentage"
    descMSSQLExtensionoutstandingIOcounter = "Extension outstanding IO counter"
    descMSSQLExtensionpageevictionsPersec = "Extension page evictions/sec"
    descMSSQLExtensionpagereadsPersec = "Extension page reads/sec"
    descMSSQLExtensionpageunreferencedtime = "Extension page unreferenced time"
    descMSSQLExtensionpagewritesPersec = "Extension page writes/sec"
    descMSSQLFreeliststallsPersec = "Free list stalls/sec"
    descMSSQLIntegralControllerSlope = "Integral Controller Slope"
    descMSSQLLazywritesPersec = "Lazy writes/sec"
    descMSSQLPagelifeexpectancy = "Page life expectancy"
    descMSSQLPagelookupsPersec = "Page lookups/sec"
    descMSSQLPagereadsPersec = "Page reads/sec"
    descMSSQLPagewritesPersec = "Page writes/sec"
    descMSSQLReadaheadpagesPersec = "Readahead pages/sec"
    descMSSQLReadaheadtimePersec = "Readahead time/sec"
    descMSSQLTargetpages = "Target pages"
)

type Win32_PerfRawData_MSSQLSERVER_SQLServerBufferManager struct {
    BackgroundwriterpagesPersec	uint64
    Buffercachehitratio	uint64
    Buffercachehitratio_Base	uint64
    CheckpointpagesPersec	uint64
    Databasepages	uint64
    Extensionallocatedpages	uint64
    Extensionfreepages	uint64
    Extensioninuseaspercentage	uint64
    ExtensionoutstandingIOcounter	uint64
    ExtensionpageevictionsPersec	uint64
    ExtensionpagereadsPersec	uint64
    Extensionpageunreferencedtime	uint64
    ExtensionpagewritesPersec	uint64
    FreeliststallsPersec	uint64
    IntegralControllerSlope	uint64
    LazywritesPersec	uint64
    Pagelifeexpectancy	uint64
    PagelookupsPersec	uint64
    PagereadsPersec	uint64
    PagewritesPersec	uint64
    ReadaheadpagesPersec	uint64
    ReadaheadtimePersec	uint64
    Targetpages	uint64
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

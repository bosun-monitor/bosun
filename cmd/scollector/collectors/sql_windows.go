package collectors

import (
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, c_mssql_general)
	collectors = append(collectors, c_mssql_statistics)
	collectors = append(collectors, c_mssql_locks)
}

const SQL_GENERAL = `
	SELECT 
		UserConnections, ConnectionResetPersec, LoginsPersec, LogoutsPersec,
		MarsDeadlocks, Processesblocked, TempTablesCreationRate, TempTablesForDestruction,
		Transactions
	FROM Win32_PerfRawData_MSSQLSERVER_SQLServerGeneralStatistics
	WHERE Name <> '_Total'
`

const SQL_STATISTICS = `
	SELECT 
		AutoParamAttemptsPersec, FailedAutoParamsPersec, ForcedParameterizationsPersec,
		SafeAutoParamsPersec, UnSafeAutoParamsPersec, BatchRequestsPersec, GuidedplanexecutionsPersec,
		MisguidedplanexecutionsPersec, SQLCompilationsPersec, SQLReCompilationsPersec
	FROM Win32_PerfRawData_MSSQLSERVER_SQLServerSQLStatistics
	WHERE Name <> '_Total'
`

const SQL_LOCKS = `
	SELECT 
		Name, AverageWaitTimems, LockRequestsPersec, LockTimeoutsPersec,
		LockWaitsPersec, LockWaitTimems, NumberofDeadlocksPersec 
	FROM Win32_PerfRawData_MSSQLSERVER_SQLServerLocks
	WHERE Name = 'Page' OR Name = 'Extent' OR Name = 'Object' or Name = 'Database'
`

func c_mssql_general() opentsdb.MultiDataPoint {
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerGeneralStatistics
	err := wmi.Query(SQL_GENERAL, &dst)
	if err != nil {
		l.Println("sql_general:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "mssql.user_connections", v.UserConnections, nil)
		Add(&md, "mssql.connection_resets", v.ConnectionResetPersec, nil)
		Add(&md, "mssql.logins", v.LoginsPersec, nil)
		Add(&md, "mssql.logouts", v.LogoutsPersec, nil)
		Add(&md, "mssql.mars_deadlocks", v.MarsDeadlocks, nil)
		Add(&md, "mssql.proc_blocked", v.Processesblocked, nil)
		Add(&md, "mssql.temptables_created", v.TempTablesCreationRate, nil)
		Add(&md, "mssql.temptables_to_destroy", v.TempTablesForDestruction, nil)
		Add(&md, "mssql.transactions", v.Transactions, nil)
	}
	return md
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

func c_mssql_statistics() opentsdb.MultiDataPoint {
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerSQLStatistics
	err := wmi.Query(`root\CIMV2`, SQL_STATISTICS, &dst)
	if err != nil {
		l.Println("sql_stats:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "mssql.autoparam_attempts", v.AutoParamAttemptsPersec, nil)
		Add(&md, "mssql.autoparam_failed", v.FailedAutoParamsPersec, nil)
		Add(&md, "mssql.autoparam_forced", v.ForcedParameterizationsPersec, nil)
		Add(&md, "mssql.autoparam_safe", v.SafeAutoParamsPersec, nil)
		Add(&md, "mssql.autoparam_unsafe", v.UnsafeAutoParamsPersec, nil)
		Add(&md, "mssql.batches", v.BatchRequestsPersec, nil)
		Add(&md, "mssql.guided_plans", v.GuidedplanexecutionsPersec, nil)
		Add(&md, "mssql.misguided_plans", v.MisguidedplanexecutionsPersec, nil)
		Add(&md, "mssql.compilations", v.SQLCompilationsPersec, nil)
		Add(&md, "mssql.recompilations", v.SQLReCompilationsPersec, nil)
	}
	return md
}

type Win32_PerfRawData_MSSQLSERVER_SQLServerSQLStatistics struct {
	AutoParamAttemptsPersec uint64
	BatchRequestsPersec uint64
	Caption string
	Description string
	FailedAutoParamsPersec uint64
	ForcedParameterizationsPersec uint64
	GuidedplanexecutionsPersec uint64
	MisguidedplanexecutionsPersec uint64
	SafeAutoParamsPersec uint64
	SQLCompilationsPersec uint64
	SQLReCompilationsPersec uint64
	UnsafeAutoParamsPersec uint64
}

func c_mssql_locks() opentsdb.MultiDataPoint {
	var dst []Win32_PerfRawData_MSSQLSERVER_SQLServerLocks
	err := wmi.Query(`root\CIMV2`, SQL_LOCKS, &dst)
	if err != nil {
		l.Println("sql_locks:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "mssql.lock_wait_time", v.AverageWaitTimems, opentsdb.TagSet{"type": v.Name})
		Add(&md, "mssql.lock_requests", v.LockRequestsPersec, opentsdb.TagSet{"type": v.Name})
		Add(&md, "mssql.lock_timeouts", v.LockTimeoutsPersec, opentsdb.TagSet{"type": v.Name})
		Add(&md, "mssql.lock_waits", v.LockWaitsPersec, opentsdb.TagSet{"type": v.Name})
		Add(&md, "mssql.deadlocks", v.NumberofDeadlocksPersec, opentsdb.TagSet{"type": v.Name})
	}
	return md
}

type Win32_PerfRawData_MSSQLSERVER_SQLServerLocks struct {
	AverageWaitTimems uint64
	Caption string
	Description string
	LockRequestsPersec uint64
	LockTimeoutsPersec uint64
	LockTimeoutstimeout0Persec uint64
	LockWaitsPersec uint64
	Name string
	NumberofDeadlocksPersec uint64
}

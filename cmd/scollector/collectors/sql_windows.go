package collectors

import (
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, c_mssql_general)
}

// KMB: Might be worth monitoring cache at
// Win32_PerfRawData_W3SVC_WebServiceCache, but the type isn't accessible via
// MSDN currently (getting Page Not Found).

const SQL_GENERAL = `
	SELECT 
		UserConnections, ConnectionResetPersec, LoginsPersec, LogoutsPersec,
		MarsDeadlocks, Processesblocked, TempTablesCreationRate, TempTablesForDestruction,
		Transactions
	FROM Win32_PerfRawData_MSSQLSERVER_SQLServerGeneralStatistics
	WHERE Name <> '_Total'
`

func c_mssql_general() opentsdb.MultiDataPoint {
	var dst []wmi.Win32_PerfRawData_MSSQLSERVER_SQLServerGeneralStatistics
	err := wmi.Query(SQL_GENERAL, &dst)
	if err != nil {
		l.Println("sql_general:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "mssql.user_connections", v.UserConnections, opentsdb.TagSet{})
		Add(&md, "mssql.connection_resets", v.ConnectionResetPersec, opentsdb.TagSet{})
		Add(&md, "mssql.logins", v.LoginsPersec, opentsdb.TagSet{})
		Add(&md, "mssql.logouts", v.LogoutsPersec, opentsdb.TagSet{})
		Add(&md, "mssql.mars_deadlocks", v.MarsDeadlocks, opentsdb.TagSet{})
		Add(&md, "mssql.proc_blocked", v.Processesblocked, opentsdb.TagSet{})
		Add(&md, "mssql.temptables_created", v.TempTablesCreationRate, opentsdb.TagSet{})
		Add(&md, "mssql.temptables_to_destroy", v.TempTablesForDestruction, opentsdb.TagSet{})
		Add(&md, "mssql.transactions", v.Transactions, opentsdb.TagSet{})
	}
	return md
}

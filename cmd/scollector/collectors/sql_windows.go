package collectors

import (
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, c_mssql_general)
}

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

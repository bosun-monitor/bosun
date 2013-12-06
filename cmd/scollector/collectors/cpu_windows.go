package collectors

import (
	"github.com/StackExchange/tcollector/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, c_cpu_windows)
}

const CPU_QUERY = `
	SELECT Name, PercentPrivilegedTime, PercentInterruptTime, PercentUserTime
	FROM Win32_PerfRawData_PerfOS_Processor
	WHERE Name <> '_Total'
`

func c_cpu_windows() opentsdb.MultiDataPoint {
	var dst []wmi.Win32_PerfRawData_PerfOS_Processor
	err := wmi.Query(CPU_QUERY, &dst)
	if err != nil {
		l.Println("cpu:", err)
		return nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "cpu.time", v.PercentPrivilegedTime, opentsdb.TagSet{"cpu": v.Name, "type": "privileged"})
		Add(&md, "cpu.time", v.PercentInterruptTime, opentsdb.TagSet{"cpu": v.Name, "type": "interrupt"})
		Add(&md, "cpu.time", v.PercentUserTime, opentsdb.TagSet{"cpu": v.Name, "type": "user"})
	}
	return md
}

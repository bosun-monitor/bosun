package collectors

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"bosun.org/slog"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
	"github.com/StackExchange/wmi"
)

var regexesDotNet = []*regexp.Regexp{}

func init() {
	AddProcessDotNetConfig = func(params conf.ProcessDotNet) error {
		if params.Name == "" {
			return fmt.Errorf("empty dotnet process name")
		}
		reg, err := regexp.Compile(params.Name)
		if err != nil {
			return err
		}
		regexesDotNet = append(regexesDotNet, reg)
		return nil
	}
	WatchProcessesDotNet = func() {
		if len(regexesDotNet) == 0 {
			// If no process_dotnet settings configured in config file, use this set instead.
			regexesDotNet = append(regexesDotNet, regexp.MustCompile("^w3wp"))
		}

		c := &IntervalCollector{
			F: c_dotnet_loading,
		}
		c.init = wmiInit(c, func() interface{} {
			return &[]Win32_PerfRawData_NETFramework_NETCLRLoading{}
		}, "", &dotnetLoadingQuery)
		collectors = append(collectors, c)

		c = &IntervalCollector{
			F: c_dotnet_memory,
		}
		c.init = wmiInit(c, func() interface{} {
			return &[]Win32_PerfRawData_NETFramework_NETCLRMemory{}
		}, `WHERE ProcessID <> 0`, &dotnetMemoryQuery)
		collectors = append(collectors, c)

		c = &IntervalCollector{
			F: c_dotnet_sql,
		}
		c.init = wmiInit(c, func() interface{} {
			return &[]Win32_PerfRawData_NETDataProviderforSqlServer_NETDataProviderforSqlServer{}
		}, "", &dotnetSQLQuery)
		collectors = append(collectors, c)
	}
}

var (
	dotnetLoadingQuery string
	dotnetMemoryQuery  string
	dotnetSQLQuery     string
)

func c_dotnet_loading() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_NETFramework_NETCLRLoading
	err := queryWmi(dotnetLoadingQuery, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		if !util.NameMatches(v.Name, regexesDotNet) {
			continue
		}
		id := "0"
		raw_name := strings.Split(v.Name, "#")
		name := raw_name[0]
		if len(raw_name) == 2 {
			id = raw_name[1]
		}
		// If you have a hash sign in your process name you don't deserve monitoring ;-)
		if len(raw_name) > 2 {
			continue
		}
		tags := opentsdb.TagSet{"name": name, "id": id}
		Add(&md, "dotnet.current.appdomains", v.Currentappdomains, tags, metadata.Gauge, metadata.Count, descWinDotNetLoadingCurrentappdomains)
		Add(&md, "dotnet.current.assemblies", v.CurrentAssemblies, tags, metadata.Gauge, metadata.Count, descWinDotNetLoadingCurrentAssemblies)
		Add(&md, "dotnet.current.classes", v.CurrentClassesLoaded, tags, metadata.Gauge, metadata.Count, descWinDotNetLoadingCurrentClassesLoaded)
		Add(&md, "dotnet.total.appdomains", v.TotalAppdomains, tags, metadata.Gauge, metadata.Count, descWinDotNetLoadingTotalAppdomains)
		Add(&md, "dotnet.total.appdomains_unloaded", v.Totalappdomainsunloaded, tags, metadata.Gauge, metadata.Count, descWinDotNetLoadingTotalappdomainsunloaded)
		Add(&md, "dotnet.total.assemblies", v.TotalAssemblies, tags, metadata.Gauge, metadata.Count, descWinDotNetLoadingTotalAssemblies)
		Add(&md, "dotnet.total.classes", v.TotalClassesLoaded, tags, metadata.Gauge, metadata.Count, descWinDotNetLoadingTotalClassesLoaded)
		Add(&md, "dotnet.total.load_failures", v.TotalNumberofLoadFailures, tags, metadata.Gauge, metadata.Count, descWinDotNetLoadingTotalNumberofLoadFailures)
	}
	return md, nil
}

const (
	descWinDotNetLoadingCurrentappdomains         = "This counter displays the current number of AppDomains loaded in this application. AppDomains (application domains) provide a secure and versatile unit of processing that the CLR can use to provide isolation between applications running in the same process."
	descWinDotNetLoadingCurrentAssemblies         = "This counter displays the current number of Assemblies loaded across all AppDomains in this application. If the Assembly is loaded as domain-neutral from multiple AppDomains then this counter is incremented once only. Assemblies can be loaded as domain-neutral when their code can be shared by all AppDomains or they can be loaded as domain-specific when their code is private to the AppDomain."
	descWinDotNetLoadingCurrentClassesLoaded      = "This counter displays the current number of classes loaded in all Assemblies."
	descWinDotNetLoadingTotalAppdomains           = "This counter displays the peak number of AppDomains loaded since the start of this application."
	descWinDotNetLoadingTotalappdomainsunloaded   = "This counter displays the total number of AppDomains unloaded since the start of the application. If an AppDomain is loaded and unloaded multiple times this counter would count each of those unloads as separate."
	descWinDotNetLoadingTotalAssemblies           = "This counter displays the total number of Assemblies loaded since the start of this application. If the Assembly is loaded as domain-neutral from multiple AppDomains then this counter is incremented once only."
	descWinDotNetLoadingTotalClassesLoaded        = "This counter displays the cumulative number of classes loaded in all Assemblies since the start of this application."
	descWinDotNetLoadingTotalNumberofLoadFailures = "This counter displays the peak number of classes that have failed to load since the start of the application. These load failures could be due to many reasons like inadequate security or illegal format."
)

type Win32_PerfRawData_NETFramework_NETCLRLoading struct {
	Currentappdomains         uint32
	CurrentAssemblies         uint32
	CurrentClassesLoaded      uint32
	Name                      string
	TotalAppdomains           uint32
	Totalappdomainsunloaded   uint32
	TotalAssemblies           uint32
	TotalClassesLoaded        uint32
	TotalNumberofLoadFailures uint32
}

func c_dotnet_memory() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_NETFramework_NETCLRMemory
	err := queryWmi(dotnetMemoryQuery, &dst)
	if err != nil {
		return nil, err
	}
	var svc_dst []Win32_Service
	var svc_q = wmi.CreateQuery(&svc_dst, `WHERE Started=true`)
	err = queryWmi(svc_q, &svc_dst)
	if err != nil {
		return nil, err
	}
	var iis_dst []WorkerProcess
	iis_q := wmi.CreateQuery(&iis_dst, "")
	err = queryWmiNamespace(iis_q, &iis_dst, "root\\WebAdministration")
	if err != nil {
		iis_dst = nil
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		var name string
		service_match := false
		iis_match := false
		process_match := util.NameMatches(v.Name, regexesDotNet)
		id := "0"
		if process_match {
			raw_name := strings.Split(v.Name, "#")
			name = raw_name[0]
			if len(raw_name) == 2 {
				id = raw_name[1]
			}
			// If you have a hash sign in your process name you don't deserve monitoring ;-)
			if len(raw_name) > 2 {
				continue
			}
		}
		// A Service match could "overwrite" a process match, but that is probably what we would want.
		for _, svc := range svc_dst {
			if util.NameMatches(svc.Name, regexesDotNet) {
				// It is possible the pid has gone and been reused, but I think this unlikely
				// and I'm not aware of an atomic join we could do anyways.
				if svc.ProcessId != 0 && svc.ProcessId == v.ProcessID {
					id = "0"
					service_match = true
					name = svc.Name
					break
				}
			}
		}
		for _, a_pool := range iis_dst {
			if a_pool.ProcessId == v.ProcessID {
				id = "0"
				iis_match = true
				name = strings.Join([]string{"iis", a_pool.AppPoolName}, "_")
				break
			}
		}
		if !(service_match || process_match || iis_match) {
			continue
		}
		tags := opentsdb.TagSet{"name": name, "id": id}
		Add(&md, "dotnet.memory.finalization_survivors", v.FinalizationSurvivors, tags, metadata.Gauge, metadata.Count, descWinDotNetMemoryFinalizationSurvivors)
		Add(&md, "dotnet.memory.gen0_promoted", v.Gen0PromotedBytesPerSec, tags, metadata.Counter, metadata.BytesPerSecond, descWinDotNetMemoryGen0PromotedBytesPerSec)
		Add(&md, "dotnet.memory.gen0_promoted_finalized", v.PromotedFinalizationMemoryfromGen0, tags, metadata.Gauge, metadata.PerSecond, descWinDotNetMemoryPromotedFinalizationMemoryfromGen0)
		Add(&md, "dotnet.memory.gen1_promoted", v.Gen1PromotedBytesPerSec, tags, metadata.Counter, metadata.BytesPerSecond, descWinDotNetMemoryGen1PromotedBytesPerSec)
		Add(&md, "dotnet.memory.heap_allocations", v.AllocatedBytesPersec, tags, metadata.Counter, metadata.BytesPerSecond, descWinDotNetMemoryAllocatedBytesPersec)
		Add(&md, "dotnet.memory.heap_size_gen0_max", v.Gen0heapsize, tags, metadata.Gauge, metadata.Bytes, descWinDotNetMemoryGen0heapsize)
		Add(&md, "dotnet.memory.heap_size", v.Gen1heapsize, opentsdb.TagSet{"type": "gen1"}.Merge(tags), metadata.Gauge, metadata.Bytes, descWinDotNetMemoryGen1heapsize)
		Add(&md, "dotnet.memory.heap_size", v.Gen2heapsize, opentsdb.TagSet{"type": "gen2"}.Merge(tags), metadata.Gauge, metadata.Bytes, descWinDotNetMemoryGen2heapsize)
		Add(&md, "dotnet.memory.heap_size", v.LargeObjectHeapsize, opentsdb.TagSet{"type": "large_object"}.Merge(tags), metadata.Gauge, metadata.Bytes, descWinDotNetMemoryLargeObjectHeapsize)
		Add(&md, "dotnet.memory.heap_size", v.NumberBytesinallHeaps, opentsdb.TagSet{"type": "total"}.Merge(tags), metadata.Gauge, metadata.Bytes, descWinDotNetMemoryNumberBytesinallHeaps)
		Add(&md, "dotnet.memory.gc_handles", v.NumberGCHandles, tags, metadata.Gauge, metadata.Count, descWinDotNetMemoryNumberGCHandles)
		Add(&md, "dotnet.memory.gc_collections", v.NumberGen0Collections, opentsdb.TagSet{"type": "gen0"}.Merge(tags), metadata.Counter, metadata.Count, descWinDotNetMemoryNumberGen0Collections)
		Add(&md, "dotnet.memory.gc_collections", v.NumberGen1Collections, opentsdb.TagSet{"type": "gen1"}.Merge(tags), metadata.Counter, metadata.Count, descWinDotNetMemoryNumberGen1Collections)
		Add(&md, "dotnet.memory.gc_collections", v.NumberGen2Collections, opentsdb.TagSet{"type": "gen2"}.Merge(tags), metadata.Counter, metadata.Count, descWinDotNetMemoryNumberGen2Collections)
		Add(&md, "dotnet.memory.gc_collections", v.NumberInducedGC, opentsdb.TagSet{"type": "induced"}.Merge(tags), metadata.Counter, metadata.Count, descWinDotNetMemoryNumberInducedGC)
		Add(&md, "dotnet.memory.pinned_objects", v.NumberofPinnedObjects, tags, metadata.Gauge, metadata.Count, descWinDotNetMemoryNumberofPinnedObjects)
		Add(&md, "dotnet.memory.sink_blocks", v.NumberofSinkBlocksinuse, tags, metadata.Gauge, metadata.Count, descWinDotNetMemoryNumberofSinkBlocksinuse)
		Add(&md, "dotnet.memory.virtual_committed", v.NumberTotalcommittedBytes, tags, metadata.Gauge, metadata.Bytes, descWinDotNetMemoryNumberTotalcommittedBytes)
		Add(&md, "dotnet.memory.virtual_reserved", v.NumberTotalreservedBytes, tags, metadata.Gauge, metadata.Bytes, descWinDotNetMemoryNumberTotalreservedBytes)
		if v.PercentTimeinGC_Base != 0 {
			Add(&md, "dotnet.memory.gc_time", float64(v.PercentTimeinGC)/float64(v.PercentTimeinGC_Base)*100, tags, metadata.Gauge, metadata.Pct, descWinDotNetMemoryPercentTimeinGC)
		}
	}
	return md, nil
}

const (
	descWinDotNetMemoryAllocatedBytesPersec               = "This counter displays the rate of bytes per second allocated on the GC Heap. This counter is updated at the end of every GC; not at each allocation."
	descWinDotNetMemoryFinalizationSurvivors              = "This counter displays the number of garbage collected objects that survive a collection because they are waiting to be finalized. If these objects hold references to other objects then those objects also survive but are not counted by this counter; This counter is not a cumulative counter; its updated at the end of every GC with count of the survivors during that particular GC only. This counter was designed to indicate the extra overhead that the application might incur because of finalization."
	descWinDotNetMemoryGen0heapsize                       = "This counter displays the maximum bytes that can be allocated in generation 0 (Gen 0); its does not indicate the current number of bytes allocated in Gen 0. A Gen 0 GC is triggered when the allocations since the last GC exceed this size. The Gen 0 size is tuned by the Garbage Collector and can change during the execution of the application. At the end of a Gen 0 collection the size of the Gen 0 heap is infact 0 bytes; this counter displays the size (in bytes) of allocations that would trigger the next Gen 0 GC. This counter is updated at the end of a GC; its not updated on every allocation."
	descWinDotNetMemoryGen0PromotedBytesPerSec            = "This counter displays the bytes per second that are promoted from generation 0 (youngest) to generation 1; objects that are promoted just because they are waiting to be finalized are not included in this counter. Memory is promoted when it survives a garbage collection. This counter was designed as an indicator of relatively long-lived objects being created per sec."
	descWinDotNetMemoryGen1heapsize                       = "This counter displays the current number of bytes in generation 1 (Gen 1); this counter does not display the maximum size of Gen 1. Objects are not directly allocated in this generation; they are promoted from previous Gen 0 GCs. This counter is updated at the end of a GC; its not updated on every allocation."
	descWinDotNetMemoryGen1PromotedBytesPerSec            = "This counter displays the bytes per second that are promoted from generation 1 to generation 2 (oldest); objects that are promoted just because they are waiting to be finalized are not included in this counter. Memory is promoted when it survives a garbage collection. Nothing is promoted from generation 2 since it is the oldest."
	descWinDotNetMemoryGen2heapsize                       = "This counter displays the current number of bytes in generation 2 (Gen 2)."
	descWinDotNetMemoryLargeObjectHeapsize                = "This counter displays the current size of the Large Object Heap in bytes. Objects greater than a threshold are treated as large objects by the Garbage Collector and are directly allocated in a special heap; they are not promoted through the generations. In CLR v1.1 and above this threshold is equal to 85000 bytes."
	descWinDotNetMemoryNumberBytesinallHeaps              = "This counter is the sum of four other counters; Gen 0 Heap Size; Gen 1 Heap Size; Gen 2 Heap Size and the Large Object Heap Size. This counter indicates the current memory allocated in bytes on the GC Heaps."
	descWinDotNetMemoryNumberGCHandles                    = "This counter displays the current number of GC Handles in use. GCHandles are handles to resources external to the CLR and the managed environment. Handles occupy small amounts of memory in the GCHeap but potentially expensive unmanaged resources."
	descWinDotNetMemoryNumberGen0Collections              = "This counter displays the number of times the generation 0 objects (youngest; most recently allocated) are garbage collected (Gen 0 GC) since the start of the application. Gen 0 GC occurs when the available memory in generation 0 is not sufficient to satisfy an allocation request. This counter is incremented at the end of a Gen 0 GC. Higher generation GCs include all lower generation GCs. This counter is explicitly incremented when a higher generation (Gen 1 or Gen 2) GC occurs. _Global_ counter value is not accurate and should be ignored."
	descWinDotNetMemoryNumberGen1Collections              = "This counter displays the number of times the generation 1 objects are garbage collected since the start of the application. The counter is incremented at the end of a Gen 1 GC. Higher generation GCs include all lower generation GCs. This counter is explicitly incremented when a higher generation (Gen 2) GC occurs. _Global_ counter value is not accurate and should be ignored."
	descWinDotNetMemoryNumberGen2Collections              = "This counter displays the number of times the generation 2 objects (older) are garbage collected since the start of the application. The counter is incremented at the end of a Gen 2 GC (also called full GC). _Global_ counter value is not accurate and should be ignored."
	descWinDotNetMemoryNumberInducedGC                    = "This counter displays the peak number of times a garbage collection was performed because of an explicit call to GC.Collect. Its a good practice to let the GC tune the frequency of its collections."
	descWinDotNetMemoryNumberofPinnedObjects              = "This counter displays the number of pinned objects encountered in the last GC. This counter tracks the pinned objects only in the heaps that were garbage collected e.g. a Gen 0 GC would cause enumeration of pinned objects in the generation 0 heap only. A pinned object is one that the Garbage Collector cannot move in memory."
	descWinDotNetMemoryNumberofSinkBlocksinuse            = "This counter displays the current number of sync blocks in use. Sync blocks are per-object data structures allocated for storing synchronization information. Sync blocks hold weak references to managed objects and need to be scanned by the Garbage Collector. Sync blocks are not limited to storing synchronization information and can also store COM interop metadata. This counter was designed to indicate performance problems with heavy use of synchronization primitives."
	descWinDotNetMemoryNumberTotalcommittedBytes          = "This counter displays the amount of virtual memory (in bytes) currently committed by the Garbage Collector. Committed memory is the physical memory for which space has been reserved on the disk paging file."
	descWinDotNetMemoryNumberTotalreservedBytes           = "This counter displays the amount of virtual memory (in bytes) currently reserved by the Garbage Collector. Reserved memory is the virtual memory space reserved for the application but no disk or main memory pages have been used."
	descWinDotNetMemoryPercentTimeinGC                    = "Percent Time in GC is the percentage of elapsed time that was spent in performing a garbage collection (GC) since the last GC cycle. This counter is usually an indicator of the work done by the Garbage Collector on behalf of the application to collect and compact memory. This counter is updated only at the end of every GC and the counter value reflects the last observed value; its not an average."
	descWinDotNetMemoryPromotedFinalizationMemoryfromGen0 = "This counter displays the bytes of memory that are promoted from generation 0 to generation 1 just because they are waiting to be finalized. This counter displays the value observed at the end of the last GC; its not a cumulative counter."
)

type Win32_PerfRawData_NETFramework_NETCLRMemory struct {
	AllocatedBytesPersec               uint32
	FinalizationSurvivors              uint32
	Gen0heapsize                       uint32
	Gen0PromotedBytesPerSec            uint32
	Gen1heapsize                       uint32
	Gen1PromotedBytesPerSec            uint32
	Gen2heapsize                       uint32
	LargeObjectHeapsize                uint32
	Name                               string
	NumberBytesinallHeaps              uint32
	NumberGCHandles                    uint32
	NumberGen0Collections              uint32
	NumberGen1Collections              uint32
	NumberGen2Collections              uint32
	NumberInducedGC                    uint32
	NumberofPinnedObjects              uint32
	NumberofSinkBlocksinuse            uint32
	NumberTotalcommittedBytes          uint32
	NumberTotalreservedBytes           uint32
	PercentTimeinGC                    uint32
	PercentTimeinGC_Base               uint32
	ProcessID                          uint32
	PromotedFinalizationMemoryfromGen0 uint32
}

// The PID must be extracted from the Name field, and we get the name from the Pid
var dotNetSQLPIDRegex = regexp.MustCompile(`.*\[(\d+)\]$`)

func c_dotnet_sql() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_NETDataProviderforSqlServer_NETDataProviderforSqlServer
	err := queryWmi(dotnetSQLQuery, &dst)
	if err != nil {
		return nil, err
	}
	var svc_dst []Win32_Service
	var svc_q = wmi.CreateQuery(&svc_dst, `WHERE Started=true`)
	err = queryWmi(svc_q, &svc_dst)
	if err != nil {
		return nil, err
	}
	var iis_dst []WorkerProcess
	iis_q := wmi.CreateQuery(&iis_dst, "")
	err = queryWmiNamespace(iis_q, &iis_dst, "root\\WebAdministration")
	if err != nil {
		iis_dst = nil
	}
	var md opentsdb.MultiDataPoint

	// We add the values of multiple pools that share a PID, this could cause some counter edge cases
	// pidToRows is a map of pid to all the row indexes that share that IP
	pidToRows := make(map[string][]int)
	for i, v := range dst {
		m := dotNetSQLPIDRegex.FindStringSubmatch(v.Name)
		if len(m) != 2 {
			slog.Errorf("unable to extract pid from dontnet SQL Provider for '%s'", v.Name)
			continue
		}
		pidToRows[m[1]] = append(pidToRows[m[1]], i)
	}
	// skipIndex is used to skip entries in the metric loop that had their values added to an earlier entry
	skipIndex := make(map[int]bool)
	for _, rows := range pidToRows {
		if len(rows) == 1 {
			continue // Only one entry for this PID
		}
		// All fields that share a PID are summed to the first entry, other entries are marked
		// for skipping in main metric loop
		firstRow := &dst[rows[0]]
		for _, idx := range rows[1:] {
			skipIndex[idx] = true
			nextRow := &dst[idx]
			firstRow.HardDisconnectsPerSecond += nextRow.HardDisconnectsPerSecond
			firstRow.NumberOfActiveConnectionPoolGroups += nextRow.NumberOfActiveConnectionPoolGroups
			firstRow.NumberOfActiveConnectionPools += nextRow.NumberOfActiveConnectionPools
			firstRow.NumberOfActiveConnections += nextRow.NumberOfActiveConnections
			firstRow.NumberOfFreeConnections += nextRow.NumberOfFreeConnections
			firstRow.NumberOfInactiveConnectionPoolGroups += nextRow.NumberOfInactiveConnectionPoolGroups
			firstRow.NumberOfInactiveConnectionPools += nextRow.NumberOfInactiveConnectionPools
			firstRow.NumberOfNonPooledConnections += nextRow.NumberOfNonPooledConnections
			firstRow.NumberOfPooledConnections += nextRow.NumberOfPooledConnections
			firstRow.NumberOfReclaimedConnections += nextRow.NumberOfReclaimedConnections
			firstRow.NumberOfStasisConnections += nextRow.NumberOfStasisConnections
			firstRow.SoftConnectsPerSecond += nextRow.SoftConnectsPerSecond
			firstRow.SoftDisconnectsPerSecond += nextRow.SoftDisconnectsPerSecond
		}
	}

	for i, v := range dst {
		if skipIndex[i] {
			// Skip entries that were summed into the first entry
			continue
		}
		var name string
		// Extract PID from the Name field, which is odd in this class
		m := dotNetSQLPIDRegex.FindStringSubmatch(v.Name)
		if len(m) != 2 {
			// Error captured above
			continue
		}
		pid, err := strconv.ParseUint(m[1], 10, 32)
		if err != nil {
			slog.Errorf("unable to parse extracted pid '%v' from '%v' into a unit32 in dotnet sql provider", m[1], v.Name)
			continue
		}
		// We only look for Service and IIS matches in this collector
		service_match := false
		iis_match := false
		// A Service match could "overwrite" a process match, but that is probably what we would want.
		for _, svc := range svc_dst {
			if util.NameMatches(svc.Name, regexesDotNet) {
				// It is possible the pid has gone and been reused, but I think this unlikely
				// and I'm not aware of an atomic join we could do anyways.
				if svc.ProcessId != 0 && svc.ProcessId == uint32(pid) {
					service_match = true
					name = svc.Name
					break
				}
			}
		}
		for _, a_pool := range iis_dst {
			if a_pool.ProcessId == uint32(pid) {
				iis_match = true
				name = strings.Join([]string{"iis", a_pool.AppPoolName}, "_")
				break
			}
		}
		if !(service_match || iis_match) {
			continue
		}
		tags := opentsdb.TagSet{"name": name, "id": "0"}
		// Not a 100% on counter / gauge here, may see some wrong after collecting more data. PerSecond being a counter is expected
		// however since this is a PerfRawData table
		Add(&md, "dotnet.sql.hard_connects", v.HardConnectsPerSecond, tags, metadata.Counter, metadata.Connection, descWinDotNetSQLHardConnectsPerSecond)
		Add(&md, "dotnet.sql.hard_disconnects", v.HardDisconnectsPerSecond, tags, metadata.Counter, metadata.Connection, descWinDotNetSQLHardDisconnectsPerSecond)

		Add(&md, "dotnet.sql.soft_connects", v.SoftConnectsPerSecond, tags, metadata.Counter, metadata.Connection, descWinDotNetSQLSoftConnectsPerSecond)
		Add(&md, "dotnet.sql.soft_disconnects", v.SoftDisconnectsPerSecond, tags, metadata.Counter, metadata.Connection, descWinDotNetSQLSoftDisconnectsPerSecond)

		Add(&md, "dotnet.sql.active_conn_pool_groups", v.NumberOfActiveConnectionPoolGroups, tags, metadata.Gauge, metadata.Group, descWinDotNetSQLNumberOfActiveConnectionPoolGroups)
		Add(&md, "dotnet.sql.inactive_conn_pool_groups", v.NumberOfInactiveConnectionPoolGroups, tags, metadata.Gauge, metadata.Group, descWinDotNetSQLNumberOfInactiveConnectionPoolGroups)

		Add(&md, "dotnet.sql.active_conn_pools", v.NumberOfActiveConnectionPools, tags, metadata.Gauge, metadata.Pool, descWinDotNetSQLNumberOfActiveConnectionPools)
		Add(&md, "dotnet.sql.inactive_conn_pools", v.NumberOfInactiveConnectionPools, tags, metadata.Gauge, metadata.Pool, descWinDotNetSQLNumberOfInactiveConnectionPools)

		Add(&md, "dotnet.sql.active_connections", v.NumberOfActiveConnections, tags, metadata.Gauge, metadata.Connection, descWinDotNetSQLNumberOfActiveConnections)
		Add(&md, "dotnet.sql.free_connections", v.NumberOfFreeConnections, tags, metadata.Gauge, metadata.Connection, descWinDotNetSQLNumberOfFreeConnections)
		Add(&md, "dotnet.sql.non_pooled_connections", v.NumberOfNonPooledConnections, tags, metadata.Gauge, metadata.Connection, descWinDotNetSQLNumberOfNonPooledConnections)
		Add(&md, "dotnet.sql.pooled_connections", v.NumberOfPooledConnections, tags, metadata.Gauge, metadata.Connection, descWinDotNetSQLNumberOfPooledConnections)
		Add(&md, "dotnet.sql.reclaimed_connections", v.NumberOfReclaimedConnections, tags, metadata.Gauge, metadata.Connection, descWinDotNetSQLNumberOfReclaimedConnections)
		Add(&md, "dotnet.sql.statis_connections", v.NumberOfStasisConnections, tags, metadata.Gauge, metadata.Connection, descWinDotNetSQLNumberOfStasisConnections)
	}
	return md, nil
}

const (
	descWinDotNetSQLHardConnectsPerSecond                = "The number of actual connections per second that are being made to servers."
	descWinDotNetSQLHardDisconnectsPerSecond             = "The number of actual disconnects per second that are being made to servers."
	descWinDotNetSQLNumberOfActiveConnectionPoolGroups   = "The number of unique connection pool groups."
	descWinDotNetSQLNumberOfActiveConnectionPools        = "The number of active connection pools."
	descWinDotNetSQLNumberOfActiveConnections            = "The number of connections currently in-use."
	descWinDotNetSQLNumberOfFreeConnections              = "The number of connections currently available for use."
	descWinDotNetSQLNumberOfInactiveConnectionPoolGroups = "The number of unique pool groups waiting for pruning."
	descWinDotNetSQLNumberOfInactiveConnectionPools      = "The number of inactive connection pools."
	descWinDotNetSQLNumberOfNonPooledConnections         = "The number of connections that are not using connection pooling."
	descWinDotNetSQLNumberOfPooledConnections            = "The number of connections that are managed by the connection pooler."
	descWinDotNetSQLNumberOfReclaimedConnections         = "The number of connections reclaimed from GCed external connections."
	descWinDotNetSQLNumberOfStasisConnections            = "The number of connections currently waiting to be made ready for use."
	descWinDotNetSQLSoftConnectsPerSecond                = "The number of connections opened from the pool per second."
	descWinDotNetSQLSoftDisconnectsPerSecond             = "The number of connections closed and returned to the pool per second."
)

// Win32_PerfRawData_NETDataProviderforSqlServer_NETDataProviderforSqlServer is actually a CIM_StatisticalInformation type
type Win32_PerfRawData_NETDataProviderforSqlServer_NETDataProviderforSqlServer struct {
	HardConnectsPerSecond                uint32
	HardDisconnectsPerSecond             uint32
	Name                                 string
	NumberOfActiveConnectionPoolGroups   uint32
	NumberOfActiveConnectionPools        uint32
	NumberOfActiveConnections            uint32
	NumberOfFreeConnections              uint32
	NumberOfInactiveConnectionPoolGroups uint32
	NumberOfInactiveConnectionPools      uint32
	NumberOfNonPooledConnections         uint32
	NumberOfPooledConnections            uint32
	NumberOfReclaimedConnections         uint32
	NumberOfStasisConnections            uint32
	SoftConnectsPerSecond                uint32
	SoftDisconnectsPerSecond             uint32
}

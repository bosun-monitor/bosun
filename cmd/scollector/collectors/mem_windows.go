package collectors

import (
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"github.com/StackExchange/wmi"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_simple_mem_windows})
	collectors = append(collectors, &IntervalCollector{F: c_windows_memory})
	collectors = append(collectors, &IntervalCollector{F: c_windows_pagefile})
}

func c_simple_mem_windows() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_OperatingSystem
	var q = wmi.CreateQuery(&dst, "")
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "win.mem.vm.total", v.TotalVirtualMemorySize*1024, nil, metadata.Gauge, metadata.Bytes, descWinMemVirtual_Total)
		Add(&md, "win.mem.vm.free", v.FreeVirtualMemory*1024, nil, metadata.Gauge, metadata.Bytes, descWinMemVirtual_Free)
		Add(&md, "win.mem.total", v.TotalVisibleMemorySize*1024, nil, metadata.Gauge, metadata.Bytes, descWinMemVisible_Total)
		Add(&md, "win.mem.free", v.FreePhysicalMemory*1024, nil, metadata.Gauge, metadata.Bytes, descWinMemVisible_Free)
		Add(&md, OSMemTotal, v.TotalVisibleMemorySize*1024, nil, metadata.Gauge, metadata.Bytes, OSMemTotalDesc)
		Add(&md, OSMemFree, v.FreePhysicalMemory*1024, nil, metadata.Gauge, metadata.Bytes, OSMemFreeDesc)
		Add(&md, OSMemUsed, v.TotalVisibleMemorySize*1024-v.FreePhysicalMemory*1024, nil, metadata.Gauge, metadata.Bytes, OSMemUsedDesc)
		Add(&md, OSMemPctFree, float64(v.FreePhysicalMemory)/float64(v.TotalVisibleMemorySize)*100, nil, metadata.Gauge, metadata.Pct, OSMemPctFreeDesc)
	}
	return md, nil
}

const (
	descWinMemVirtual_Total = "Number, in bytes, of virtual memory."
	descWinMemVirtual_Free  = "Number, in bytes, of virtual memory currently unused and available."
	descWinMemVisible_Total = "Total amount, in bytes, of physical memory available to the operating system."
	descWinMemVisible_Free  = "Number, in bytes, of physical memory currently unused and available."
)

func c_windows_memory() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PerfRawData_PerfOS_Memory
	var q = wmi.CreateQuery(&dst, "")
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		Add(&md, "win.mem.cache", v.CacheBytes, nil, metadata.Gauge, metadata.Bytes, descWinMemCacheBytes)
		Add(&md, "win.mem.cache_peak", v.CacheBytesPeak, nil, metadata.Gauge, metadata.Bytes, descWinMemCacheBytesPeak)
		Add(&md, "win.mem.committed", v.CommittedBytes, nil, metadata.Gauge, metadata.Bytes, descWinMemCommittedBytes)
		Add(&md, "win.mem.committed_limit", v.CommitLimit, nil, metadata.Gauge, metadata.Bytes, descWinMemCommitLimit)
		Add(&md, "win.mem.committed_percent", float64(v.PercentCommittedBytesInUse)/float64(v.PercentCommittedBytesInUse_Base)*100, nil, metadata.Gauge, metadata.Pct, descWinMemPercentCommittedBytesInUse)
		Add(&md, "win.mem.modified", v.ModifiedPageListBytes, nil, metadata.Gauge, metadata.Bytes, descWinMemModifiedPageListBytes)
		Add(&md, "win.mem.page_faults", v.PageFaultsPersec, nil, metadata.Counter, metadata.PerSecond, descWinMemPageFaultsPersec)
		Add(&md, "win.mem.faults", v.CacheFaultsPersec, opentsdb.TagSet{"type": "cache"}, metadata.Counter, metadata.PerSecond, descWinMemCacheFaultsPersec)
		Add(&md, "win.mem.faults", v.DemandZeroFaultsPersec, opentsdb.TagSet{"type": "demand_zero"}, metadata.Counter, metadata.PerSecond, descWinMemDemandZeroFaultsPersec)
		Add(&md, "win.mem.faults", v.TransitionFaultsPersec, opentsdb.TagSet{"type": "transition"}, metadata.Counter, metadata.PerSecond, descWinMemTransitionFaultsPersec)
		Add(&md, "win.mem.faults", v.WriteCopiesPersec, opentsdb.TagSet{"type": "write_copies"}, metadata.Counter, metadata.PerSecond, descWinMemWriteCopiesPersec)
		Add(&md, "win.mem.page_operations", v.PageReadsPersec, opentsdb.TagSet{"type": "read"}, metadata.Counter, metadata.PerSecond, descWinMemPageReadsPersec)
		Add(&md, "win.mem.page_operations", v.PageWritesPersec, opentsdb.TagSet{"type": "write"}, metadata.Counter, metadata.PerSecond, descWinMemPageWritesPersec)
		Add(&md, "win.mem.page_operations", v.PagesInputPersec, opentsdb.TagSet{"type": "input"}, metadata.Counter, metadata.PerSecond, descWinMemPagesInputPersec)
		Add(&md, "win.mem.page_operations", v.PagesOutputPersec, opentsdb.TagSet{"type": "output"}, metadata.Counter, metadata.PerSecond, descWinMemPagesOutputPersec)
		Add(&md, "win.mem.pool.bytes", v.PoolNonpagedBytes, opentsdb.TagSet{"type": "nonpaged"}, metadata.Gauge, metadata.Bytes, descWinMemPoolNonpagedBytes)
		Add(&md, "win.mem.pool.bytes", v.PoolPagedBytes, opentsdb.TagSet{"type": "paged"}, metadata.Gauge, metadata.Bytes, descWinMemPoolPagedBytes)
		Add(&md, "win.mem.pool.bytes", v.PoolPagedResidentBytes, opentsdb.TagSet{"type": "paged_resident"}, metadata.Gauge, metadata.Bytes, descWinMemPoolPagedResidentBytes)
		Add(&md, "win.mem.pool.allocations", v.PoolPagedAllocs, opentsdb.TagSet{"type": "paged"}, metadata.Gauge, metadata.Operation, descWinMemPoolPagedAllocs)
		Add(&md, "win.mem.pool.allocations", v.PoolNonpagedAllocs, opentsdb.TagSet{"type": "nonpaged"}, metadata.Gauge, metadata.Operation, descWinMemPoolNonpagedAllocs)
	}
	return md, nil
}

const (
	descWinMemCacheBytes                 = "Cache Bytes the size, in bytes, of the portion of the system file cache which is currently resident and active in physical memory."
	descWinMemCacheBytesPeak             = "Cache Bytes Peak is the maximum number of bytes used by the system file cache since the system was last restarted. This might be larger than the current size of the cache."
	descWinMemCacheFaultsPersec          = "Cache Faults/sec is the rate at which faults occur when a page sought in the file system cache is not found and must be retrieved from elsewhere in memory (a soft fault) or from disk (a hard fault). The file system cache is an area of physical memory that stores recently used pages of data for applications. Cache activity is a reliable indicator of most application I/O operations. This counter shows the number of faults, without regard for the number of pages faulted in each operation."
	descWinMemCommitLimit                = "Commit Limit is the amount of virtual memory that can be committed without having to extend the paging file(s).  It is measured in bytes. Committed memory is the physical memory which has space reserved on the disk paging files. If the paging file(s) are be expanded, this limit increases accordingly."
	descWinMemCommittedBytes             = "Committed Bytes is the amount of committed virtual memory, in bytes. Committed memory is the physical memory which has space reserved on the disk paging file(s)."
	descWinMemDemandZeroFaultsPersec     = "Demand Zero Faults/sec is the rate at which a zeroed page is required to satisfy the fault.  Zeroed pages, pages emptied of previously stored data and filled with zeros, are a security feature of Windows that prevent processes from seeing data stored by earlier processes that used the memory space. Windows maintains a list of zeroed pages to accelerate this process. This counter shows the number of faults, without regard to the number of pages retrieved to satisfy the fault."
	descWinMemModifiedPageListBytes      = "Modified Page List Bytes is the amount of physical memory, in bytes, that is assigned to the modified page list. This memory contains cached data and code that is not actively in use by processes, the system and the system cache. This memory needs to be written out before it will be available for allocation to a process or for system use."
	descWinMemPageFaultsPersec           = "Page Faults/sec is the average number of pages faulted per second. It is measured in number of pages faulted per second because only one page is faulted in each fault operation, hence this is also equal to the number of page fault operations. This counter includes both hard faults (those that require disk access) and soft faults (where the faulted page is found elsewhere in physical memory.) Most processors can handle large numbers of soft faults without significant consequence. However, hard faults, which require disk access, can cause significant delays."
	descWinMemPageReadsPersec            = "Page Reads/sec is the rate at which the disk was read to resolve hard page faults. It shows the number of reads operations, without regard to the number of pages retrieved in each operation. Hard page faults occur when a process references a page in virtual memory that is not in working set or elsewhere in physical memory, and must be retrieved from disk. This counter is a primary indicator of the kinds of faults that cause system-wide delays. It includes read operations to satisfy faults in the file system cache (usually requested by applications) and in non-cached mapped memory files. Compare the value of Memory\\Pages Reads/sec to the value of Memory\\Pages Input/sec to determine the average number of pages read during each operation."
	descWinMemPagesInputPersec           = "Pages Input/sec is the rate at which pages are read from disk to resolve hard page faults. Hard page faults occur when a process refers to a page in virtual memory that is not in its working set or elsewhere in physical memory, and must be retrieved from disk. When a page is faulted, the system tries to read multiple contiguous pages into memory to maximize the benefit of the read operation. Compare the value of Memory\\Pages Input/sec to the value of  Memory\\Page Reads/sec to determine the average number of pages read into memory during each read operation."
	descWinMemPagesOutputPersec          = "Pages Output/sec is the rate at which pages are written to disk to free up space in physical memory. Pages are written back to disk only if they are changed in physical memory, so they are likely to hold data, not code. A high rate of pages output might indicate a memory shortage. Windows writes more pages back to disk to free up space when physical memory is in short supply.  This counter shows the number of pages, and can be compared to other counts of pages, without conversion."
	descWinMemPageWritesPersec           = "Page Writes/sec is the rate at which pages are written to disk to free up space in physical memory. Pages are written to disk only if they are changed while in physical memory, so they are likely to hold data, not code. This counter shows write operations, without regard to the number of pages written in each operation."
	descWinMemPercentCommittedBytesInUse = "% Committed Bytes In Use is the ratio of Memory\\Committed Bytes to the Memory\\Commit Limit. Committed memory is the physical memory in use for which space has been reserved in the paging file should it need to be written to disk. The commit limit is determined by the size of the paging file.  If the paging file is enlarged, the commit limit increases, and the ratio is reduced)."
	descWinMemPoolNonpagedAllocs         = "Pool Nonpaged Allocs is the number of calls to allocate space in the nonpaged pool. The nonpaged pool is an area of system memory area for objects that cannot be written to disk, and must remain in physical memory as long as they are allocated.  It is measured in numbers of calls to allocate space, regardless of the amount of space allocated in each call."
	descWinMemPoolNonpagedBytes          = "Pool Nonpaged Bytes is the size, in bytes, of the nonpaged pool, an area of the system virtual memory that is used for objects that cannot be written to disk, but must remain in physical memory as long as they are allocated.  Memory\\Pool Nonpaged Bytes is calculated differently than Process\\Pool Nonpaged Bytes, so it might not equal Process(_Total)\\Pool Nonpaged Bytes."
	descWinMemPoolPagedAllocs            = "Pool Paged Allocs is the number of calls to allocate space in the paged pool. The paged pool is an area of the system virtual memory that is used for objects that can be written to disk when they are not being used. It is measured in numbers of calls to allocate space, regardless of the amount of space allocated in each call."
	descWinMemPoolPagedBytes             = "Pool Paged Bytes is the size, in bytes, of the paged pool, an area of the system virtual memory that is used for objects that can be written to disk when they are not being used."
	descWinMemPoolPagedResidentBytes     = "Pool Paged Resident Bytes is the size, in bytes, of the portion of the paged pool that is currently resident and active in physical memory."
	descWinMemTransitionFaultsPersec     = "Transition Faults/sec is the rate at which page faults are resolved by recovering pages that were being used by another process sharing the page, or were on the modified page list or the standby list, or were being written to disk at the time of the page fault. The pages were recovered without additional disk activity. Transition faults are counted in numbers of faults; because only one page is faulted in each operation, it is also equal to the number of pages faulted."
	descWinMemWriteCopiesPersec          = "Write Copies/sec is the rate at which page faults are caused by attempts to write that have been satisfied by coping of the page from elsewhere in physical memory. This is an economical way of sharing data since pages are only copied when they are written to; otherwise, the page is shared. This counter shows the number of copies, without regard for the number of pages copied in each operation."
)

type Win32_PerfRawData_PerfOS_Memory struct {
	CacheBytes                      uint64
	CacheBytesPeak                  uint64
	CacheFaultsPersec               uint32
	CommitLimit                     uint64
	CommittedBytes                  uint64
	DemandZeroFaultsPersec          uint32
	ModifiedPageListBytes           uint64
	PageFaultsPersec                uint32
	PageReadsPersec                 uint32
	PagesInputPersec                uint32
	PagesOutputPersec               uint32
	PageWritesPersec                uint32
	PercentCommittedBytesInUse      uint32
	PercentCommittedBytesInUse_Base uint32
	PoolNonpagedAllocs              uint32
	PoolNonpagedBytes               uint64
	PoolPagedAllocs                 uint32
	PoolPagedBytes                  uint64
	PoolPagedResidentBytes          uint64
	TransitionFaultsPersec          uint32
	WriteCopiesPersec               uint32
}

func c_windows_pagefile() (opentsdb.MultiDataPoint, error) {
	var dst []Win32_PageFileUsage
	var q = wmi.CreateQuery(&dst, "")
	err := queryWmi(q, &dst)
	if err != nil {
		return nil, err
	}
	var md opentsdb.MultiDataPoint
	for _, v := range dst {
		driveletter := "unknown"
		if len(v.Name) >= 1 {
			driveletter = v.Name[0:1]
		}
		tags := opentsdb.TagSet{"drive": driveletter}
		Add(&md, "win.mem.pagefile.size", int64(v.AllocatedBaseSize)*1024*1024, tags, metadata.Gauge, metadata.Bytes, descWinMemPagefileAllocatedBaseSize)
		Add(&md, "win.mem.pagefile.usage_current", int64(v.CurrentUsage)*1024*1024, tags, metadata.Gauge, metadata.Bytes, descWinMemPagefileCurrentUsage)
		Add(&md, "win.mem.pagefile.usage_peak", int64(v.PeakUsage)*1024*1024, tags, metadata.Gauge, metadata.Bytes, descWinMemPagefilePeakUsage)
		Add(&md, "win.mem.pagefile.usage_percent", float64(v.CurrentUsage)/float64(v.AllocatedBaseSize)*100, tags, metadata.Gauge, metadata.Pct, descWinMemPagefilePercent)
	}
	return md, nil
}

const (
	descWinMemPagefileAllocatedBaseSize = "The actual amount of disk space in bytes allocated for use with this page file. This value corresponds to the range established in Win32_PageFileSetting under the InitialSize and MaximumSize properties, set at system startup."
	descWinMemPagefileCurrentUsage      = "How many bytes of the total reserved page file are currently in use."
	descWinMemPagefilePeakUsage         = "The maximum number of bytes used in the page file since the system was restarted."
	descWinMemPagefilePercent           = "The current used page file size / total page file size."
)

type Win32_PageFileUsage struct {
	AllocatedBaseSize uint32
	CurrentUsage      uint32
	PeakUsage         uint32
	Name              string
}

package collectors // import "bosun.org/cmd/scollector/collectors"

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
)

var collectors []Collector

type Collector interface {
	Run(chan<- *opentsdb.DataPoint, <-chan struct{})
	Name() string
	Init()
	AddTagOverrides(map[string]string, opentsdb.TagSet) error
	ApplyTagOverrides(opentsdb.TagSet)
}

//These should be in alphabetical order to help prevent merge conflicts
const (
	osCPU              = "os.cpu"
	osCPUClock         = "os.cpu.clock"
	osDiskFree         = "os.disk.fs.space_free"
	osDiskPctFree      = "os.disk.fs.percent_free"
	osDiskTotal        = "os.disk.fs.space_total"
	osDiskUsed         = "os.disk.fs.space_used"
	osMemFree          = "os.mem.free"
	osMemPctFree       = "os.mem.percent_free"
	osMemTotal         = "os.mem.total"
	osMemUsed          = "os.mem.used"
	osNetAdminStatus   = "os.net.admin_status"
	osNetBondBroadcast = "os.net.bond.packets_broadcast"
	osNetBondBytes     = "os.net.bond.bytes"
	osNetBondDropped   = "os.net.bond.dropped"
	osNetBondErrors    = "os.net.bond.errs"
	osNetBondIfSpeed   = "os.net.bond.ifspeed"
	osNetBondMulticast = "os.net.bond.packets_multicast"
	osNetBondPackets   = "os.net.bond.packets"
	osNetBondUnicast   = "os.net.bond.packets_unicast"
	osNetBroadcast     = "os.net.packets_broadcast"
	osNetBytes         = "os.net.bytes"
	osNetDropped       = "os.net.dropped"
	osNetErrors        = "os.net.errs"
	osNetIfSpeed       = "os.net.ifspeed"
	osNetMTU           = "os.net.mtu"
	osNetMulticast     = "os.net.packets_multicast"
	osNetOperStatus    = "os.net.oper_status"
	osNetPackets       = "os.net.packets"
	osNetPauseFrames   = "os.net.pause_frames"
	osNetUnicast       = "os.net.packets_unicast"
	osProcCount        = "os.proc.count"
	osProcCPU          = "os.proc.cpu"
	osProcMemReal      = "os.proc.mem.real"
	osProcMemVirtual   = "os.proc.mem.virtual"
	osServiceRunning   = "os.service.running"
	osSystemUptime     = "os.system.uptime"
)

const (
	osCPUClockDesc       = "The current speed of the processor in MHz."
	osDiskFreeDesc       = "The space_free property indicates in bytes how much free space is available on the disk."
	osDiskPctFreeDesc    = "The percent_free property indicates what percentage of the disk is available."
	osDiskTotalDesc      = "The space_total property indicates in bytes how much total space is on the disk."
	osDiskUsedDesc       = "The space_used property indicates in bytes how much space is used on the disk."
	osMemFreeDesc        = "The number of bytes of physical memory currently unused and available. In Linux this metric considers buffers, cache, and slab to be free memory."
	osMemPctFreeDesc     = "The percent of free memory. In Linux this metric considers buffers, cache, and slab to be free memory."
	osMemTotalDesc       = "The total amount of physical memory available in bytes to the operating system."
	osMemUsedDesc        = "The amount of used memory. In Linux this metric excludes buffers, cache, from used memory."
	osNetAdminStatusDesc = "The desired state of the interface. The testing(3) state indicates that no operational packets can be passed. When a managed system initializes, all interfaces start with ifAdminStatus in the down(2) state. As a result of either explicit management action or per configuration information retained by the managed system, ifAdminStatus is then changed to either the up(1) or testing(3) states (or remains in the down(2) state)."
	osNetBroadcastDesc   = "The rate at which broadcast packets are sent or received on the network interface."
	osNetBytesDesc       = "The rate at which bytes are sent or received over the network interface."
	osNetDroppedDesc     = "The number of packets that were chosen to be discarded even though no errors had been detected to prevent transmission."
	osNetErrorsDesc      = "The number of packets that could not be transmitted because of errors."
	osNetIfSpeedDesc     = "The total link speed of the network interface in Megabits per second."
	osNetMTUDesc         = "The maximum transmission unit for the ethernet frame."
	osNetMulticastDesc   = "The rate at which multicast packets are sent or received on the network interface."
	osNetOperStatusDesc  = "The current operational state of the interface. The testing(3) state indicates that no operational packets can be passed. If ifAdminStatus is down(2) then ifOperStatus should be down(2). If ifAdminStatus is changed to up(1) then ifOperStatus should change to up(1) if the interface is ready to transmit and receive network traffic; it should change to dormant(5) if the interface is waiting for external actions (such as a serial line waiting for an incoming connection); it should remain in the down(2) state if and only if there is a fault that prevents it from going to the up(1) state; it should remain in the notPresent(6) state if the interface has missing (typically, hardware) components."
	osNetPacketsDesc     = "The rate at which packets are sent or received on the network interface."
	osNetPauseFrameDesc  = "The rate of pause frames sent or recieved on the network interface. An overwhelmed network element can send a pause frame, which halts the transmission of the sender for a specified period of time."
	osNetUnicastDesc     = "The rate at which unicast packets are sent or received on the network interface."
	osProcCountDesc      = "The number of processes running with this name."
	osProcCPUDesc        = "The summed percentage of CPU time used by processes with this name (0-100)."
	osProcMemRealDesc    = "The total amount of real memory used by the processes with this name. For Linux this is RSS and in Windows it is the private working set."
	osProcMemVirtualDesc = "The total amount of virtual memory used by the processes with this name."
	osProcPID            = "The PID of the process being tracked by a given ID tag. As this metric value represents the actual PID, it is not suitable for any form of aggregation."
	osServiceRunningDesc = "1: active, 0: inactive"
	osSystemUptimeDesc   = "Seconds since last reboot."
)

var (
	// DefaultFreq is the duration between collection intervals if none is
	// specified.
	DefaultFreq = time.Second * 15

	timestamp = time.Now().Unix()
	tlock     sync.Mutex
	AddTags   opentsdb.TagSet

	metricFilters = make([]*regexp.Regexp, 0)

	AddProcessDotNetConfig = func(params conf.ProcessDotNet) error {
		return fmt.Errorf("process_dotnet watching not implemented on this platform")
	}
	WatchProcessesDotNet = func() {}

	KeepalivedCommunity = ""

	//TotalScollectorMemory stores the total memory used by Scollector (including CGO and WMI)
	TotalScollectorMemoryMB uint64

	MetricPrefix = ""
)

func init() {
	go func() {
		for t := range time.Tick(time.Second) {
			tlock.Lock()
			timestamp = t.Unix()
			tlock.Unlock()
		}
	}()
}

func now() (t int64) {
	tlock.Lock()
	t = timestamp
	tlock.Unlock()
	return
}

func matchPattern(s string, patterns []string) bool {
	for _, p := range patterns {
		if !strings.HasPrefix(p, "-") {
			if strings.Contains(s, p) {
				return true
			}
		}
	}
	return false
}

func matchInvertPattern(s string, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasPrefix(p, "-") {
			var np = p[1:]
			if strings.Contains(s, np) {
				return true
			}
		}
	}
	return false
}

// Search returns all collectors matching the pattern s.
func Search(s []string) []Collector {
	if len(s) == 0 {
		return collectors
	}
	var r []Collector
	sort.Strings(s)
	i := sort.SearchStrings(s, "*")
	IncludeAll := i < len(s) && s[i] == "*"
	for _, c := range collectors {
		if matchInvertPattern(c.Name(), s) {
			continue
		} else if IncludeAll || matchPattern(c.Name(), s) {
			r = append(r, c)
		}
	}
	return r
}

// Adds configured tag overrides to all matching collectors
func AddTagOverrides(s []Collector, tagOverride []conf.TagOverride) error {
	for _, to := range tagOverride {
		re := regexp.MustCompile(to.CollectorExpr)
		for _, c := range s {
			if re.MatchString(c.Name()) {
				err := c.AddTagOverrides(to.MatchedTags, to.Tags)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// Run runs specified collectors. Use nil for all collectors.
func Run(cs []Collector) (chan *opentsdb.DataPoint, chan struct{}) {
	if cs == nil {
		cs = collectors
	}
	ch := make(chan *opentsdb.DataPoint)
	quit := make(chan struct{})
	for _, c := range cs {
		go c.Run(ch, quit)
	}
	return ch, quit
}

type initFunc func(*conf.Conf)

var inits = []initFunc{}

func registerInit(i initFunc) {
	inits = append(inits, i)
}

func Init(c *conf.Conf) {
	if c.MetricPrefix != "" {
		MetricPrefix = c.MetricPrefix
	}
	for _, f := range inits {
		f(c)
	}
}

type MetricMeta struct {
	Metric   string
	TagSet   opentsdb.TagSet
	RateType metadata.RateType
	Unit     metadata.Unit
	Desc     string
}

// AddTS is the same as Add but lets you specify the timestamp
func AddTS(md *opentsdb.MultiDataPoint, name string, ts int64, value interface{}, t opentsdb.TagSet, rate metadata.RateType, unit metadata.Unit, desc string) {
	// Check if we really want that metric
	if skipMetric(name) {
		return
	}
	// Add Prefix
	if MetricPrefix != "" {
		name = MetricPrefix + "." + name
	}

	tags := t.Copy()
	if host, present := tags["host"]; !present {
		tags["host"] = util.Hostname
	} else if host == "" {
		delete(tags, "host")
	}
	// if tags are not cleanable, log a message and skip it
	if err := tags.Clean(); err != nil {
		line := ""
		//attempt to log where Add was called from
		if _, filename, l, ok := runtime.Caller(1); ok {
			if filepath.Base(filename) == "collectors.go" {
				_, filename, l, ok = runtime.Caller(2)
			}
			if ok {
				line = fmt.Sprintf("%s:%d", filepath.Base(filename), l)
			}
		}
		slog.Errorf("Invalid tagset discovered: %s. Skipping datapoint. Added from: %s", tags.String(), line)
		return
	}
	if rate != metadata.Unknown {
		metadata.AddMeta(name, nil, "rate", rate, false)
	}
	if unit != metadata.None {
		metadata.AddMeta(name, nil, "unit", unit, false)
	}
	if desc != "" {
		metadata.AddMeta(name, tags, "desc", desc, false)
	}
	tags = AddTags.Copy().Merge(tags)
	if b, ok := value.(bool); ok {
		if b {
			value = 1
		} else {
			value = 0
		}
	}
	d := opentsdb.DataPoint{
		Metric:    name,
		Timestamp: ts,
		Value:     value,
		Tags:      tags,
	}
	*md = append(*md, &d)
}

// Add appends a new data point with given metric name, value, and tags. Tags
// may be nil. If tags is nil or does not contain a host key, it will be
// automatically added. If the value of the host key is the empty string, it
// will be removed (use this to prevent the normal auto-adding of the host tag).
func Add(md *opentsdb.MultiDataPoint, name string, value interface{}, t opentsdb.TagSet, rate metadata.RateType, unit metadata.Unit, desc string) {
	AddTS(md, name, now(), value, t, rate, unit, desc)
}

func readLine(fname string, line func(string) error) error {
	f, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if err := line(scanner.Text()); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// IsDigit returns true if s consists of decimal digits.
func IsDigit(s string) bool {
	r := strings.NewReader(s)
	for {
		ch, _, err := r.ReadRune()
		if ch == 0 || err != nil {
			break
		} else if ch == utf8.RuneError {
			return false
		} else if !unicode.IsDigit(ch) {
			return false
		}
	}
	return true
}

// IsAlNum returns true if s is alphanumeric.
func IsAlNum(s string) bool {
	r := strings.NewReader(s)
	for {
		ch, _, err := r.ReadRune()
		if ch == 0 || err != nil {
			break
		} else if ch == utf8.RuneError {
			return false
		} else if !unicode.IsDigit(ch) && !unicode.IsLetter(ch) {
			return false
		}
	}
	return true
}

func TSys100NStoEpoch(nsec uint64) int64 {
	nsec -= 116444736000000000
	seconds := nsec / 1e7
	return int64(seconds)
}

func metaIfaces(f func(iface net.Interface, tags opentsdb.TagSet)) {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if strings.HasPrefix(iface.Name, "lo") {
			continue
		}
		tags := opentsdb.TagSet{"iface": iface.Name}
		metadata.AddMeta("", tags, "name", iface.Name, true)
		if mac := strings.ToUpper(strings.Replace(iface.HardwareAddr.String(), ":", "", -1)); mac != "" {
			metadata.AddMeta("", tags, "mac", mac, true)
		}
		rawAds, _ := iface.Addrs()
		addrs := make([]string, len(rawAds))
		for i, rAd := range rawAds {
			addrs[i] = rAd.String()
		}
		sort.Strings(addrs)
		j, _ := json.Marshal(addrs)
		metadata.AddMeta("", tags, "addresses", string(j), true)
		if f != nil {
			f(iface, tags)
		}
	}
}

// AddMetricFilters adds metric filters provided by the conf
func AddMetricFilters(s string) error {
	re, err := regexp.Compile(s)
	if err != nil {
		return err
	}
	metricFilters = append(metricFilters, re)
	return nil
}

// skipMetric will return true if we need to skip this metric
func skipMetric(index string) bool {
	// If no filters provided, we skip nothing
	if len(metricFilters) == 0 {
		return false
	}
	for _, re := range metricFilters {
		if re.MatchString(index) {
			return false
		}
	}
	return true
}

type tsIntegrator func(int64, float64) float64

func getTsIntegrator() tsIntegrator {
	var total float64
	var lastTimestamp int64
	return func(timestamp int64, v float64) float64 {
		if lastTimestamp > 0 {
			total += v * float64(timestamp-lastTimestamp)
		}
		lastTimestamp = timestamp
		return total
	}
}

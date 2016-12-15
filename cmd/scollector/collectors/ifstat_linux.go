package collectors

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

func init() {
	registerInit(func(c *conf.Conf) {
		if c.IfaceExpr != "" {
			ifstatRE = regexp.MustCompile(fmt.Sprintf("(%s):(.*)", c.IfaceExpr))
		}

		collectors = append(collectors, &IntervalCollector{F: c_ifstat_linux})
		collectors = append(collectors, &IntervalCollector{F: c_ipcount_linux})
		collectors = append(collectors, &IntervalCollector{F: c_if_team_linux})
		collectors = append(collectors, &IntervalCollector{F: c_if_bond_linux})
	})
}

var netFields = []struct {
	key  string
	rate metadata.RateType
	unit metadata.Unit
}{
	{"bytes", metadata.Counter, metadata.Bytes},
	{"packets", metadata.Counter, metadata.Count},
	{"errs", metadata.Counter, metadata.Count},
	{"dropped", metadata.Counter, metadata.Count},
	{"fifo.errs", metadata.Counter, metadata.Count},
	{"frame.errs", metadata.Counter, metadata.Count},
	{"compressed", metadata.Counter, metadata.Count},
	{"multicast", metadata.Counter, metadata.Count},
	{"bytes", metadata.Counter, metadata.Bytes},
	{"packets", metadata.Counter, metadata.Count},
	{"errs", metadata.Counter, metadata.Count},
	{"dropped", metadata.Counter, metadata.Count},
	{"fifo.errs", metadata.Counter, metadata.Count},
	{"collisions", metadata.Counter, metadata.Count},
	{"carrier.errs", metadata.Counter, metadata.Count},
	{"compressed", metadata.Counter, metadata.Count},
}

var ifstatRE = regexp.MustCompile(`\s+(eth\d+|em\d+_\d+/\d+|em\d+_\d+|em\d+|` +
	`bond\d+|team\d+|` + `p\d+p\d+_\d+/\d+|p\d+p\d+_\d+|p\d+p\d+):(.*)`)

func c_ipcount_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	v4c := 0
	v6c := 0
	err := util.ReadCommand(func(line string) error {
		tl := strings.TrimSpace(line)
		if strings.HasPrefix(tl, "inet ") {
			v4c++
		}
		if strings.HasPrefix(tl, "inet6 ") {
			v6c++
		}
		return nil
	}, "ip", "addr", "list")
	if err != nil {
		return md, err
	}
	Add(&md, "linux.net.ip_count", v4c, opentsdb.TagSet{"version": "4"}, metadata.Gauge, "IP_Addresses", "")
	Add(&md, "linux.net.ip_count", v6c, opentsdb.TagSet{"version": "6"}, metadata.Gauge, "IP_Addresses", "")
	return md, nil
}

// getNicSpeed returns the speed of the interface
func getNicSpeed(r io.Reader) (speed string, err error) {
	scanner := bufio.NewScanner(r)
	scanner.Scan() // read only one line
	if err = scanner.Err(); err != nil {
		return
	}
	speed = scanner.Text()
	if _, err := strconv.Atoi(speed); err != nil {
		return "", err
	}
	return
}

func recordNicSpeed(md *opentsdb.MultiDataPoint, intf string, bond string) error {
	r, err := os.Open("/sys/class/net/" + intf + "/speed")
	if err != nil {
		return err
	}
	defer r.Close()
	speed, err := getNicSpeed(r)
	if err != nil {
		return err
	}

	tags := opentsdb.TagSet{"iface": intf}
	Add(md, "linux.net."+bond+"ifspeed", speed, tags, metadata.Gauge, metadata.Megabit, osNetIfSpeedDesc)
	Add(md, "os.net."+bond+"ifspeed", speed, tags, metadata.Gauge, metadata.Megabit, osNetIfSpeedDesc)
	return nil
}

func parseProcNetDev(line string) (intf string, stats []string, err error) {
	m := ifstatRE.FindStringSubmatch(line) // intname: val1 val2 val3...
	if m == nil {
		return "", nil, errors.New("Can't parse line") // ugly
	}

	intf = m[1]
	stats = strings.Fields(m[2])
	return
}

func c_ifstat_linux() (md opentsdb.MultiDataPoint, err error) {
	direction := func(i int) string {
		if i >= 8 {
			return "out"
		}
		return "in"
	}

	// see: http://stackoverflow.com/a/4943975/995368
	err = readLine("/proc/net/dev", func(s string) error {
		intf, stats, err := parseProcNetDev(s)
		if err != nil {
			return nil
		} // fixme: should return an error

		var bond string
		if strings.HasPrefix(intf, "bond") || strings.HasPrefix(intf, "team") {
			bond = "bond."
		}

		// Find speed of the interface in question
		if err := recordNicSpeed(&md, intf, bond); err != nil {
			return err
		}

		// more specific fields are located under linux.net. domain
		// generic OS agnostic data can be found under os.net. domain
		normalize := func(s string) string { return strings.Replace(s, ".", "_", -1) }
		for i, v := range stats {
			tags := opentsdb.TagSet{"iface": intf, "direction": direction(i)}
			nf := netFields[i]
			Add(&md, "linux.net."+bond+normalize(nf.key), v, tags, nf.rate, nf.unit, "")
			if i < 4 || (i >= 8 && i < 12) {
				Add(&md, "os.net."+bond+normalize(nf.key), v, tags, nf.rate, nf.unit, "")
			}
		}
		return nil
	})
	return md, err
}

const (
	linuxNetBondSlaveIsUpDesc = "The status of the bonded or teamed interface."
	linuxNetBondSlaveCount    = "The number of slaves on the bonded or teamed interface."
)

func c_if_bond_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	const bondingPath = "/proc/net/bonding"
	bondDevices, err := ioutil.ReadDir(bondingPath)
	if err != nil {
		return md, nil
	}
	for _, fi := range bondDevices {
		var iface string
		var slaveCount int
		if err := readLine(filepath.Join(bondingPath, fi.Name()), func(s string) error {
			f := strings.SplitN(s, ":", 2)
			if len(f) != 2 {
				return nil
			}
			f[0] = strings.TrimSpace(f[0])
			f[1] = strings.TrimSpace(f[1])
			if f[0] == "Slave Interface" {
				iface = f[1]
				slaveCount++
			}
			// TODO: This will probably need to be updated for other types of bonding beside LACP, but I have no examples available to work with at the moment
			if f[0] == "MII Status" && iface != "" {
				var status int
				if f[1] == "up" {
					status = 1
				}
				Add(&md, "linux.net.bond.slave.is_up", status, opentsdb.TagSet{"slave": iface, "bond": fi.Name()}, metadata.Gauge, metadata.Bool, linuxNetBondSlaveIsUpDesc)
			}
			return nil
		}); err != nil {
			return md, err
		}
		Add(&md, "linux.net.bond.slave.count", slaveCount, opentsdb.TagSet{"bond": fi.Name()}, metadata.Gauge, metadata.Count, linuxNetBondSlaveCount)
	}
	return md, nil
}

func c_if_team_linux() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	getState := func(iname string) (TeamState, error) {
		var ts TeamState
		reader, err := util.Command(time.Second*5, nil, "teamdctl", iname, "state", "dump")
		if err != nil {
			return ts, err
		}
		err = json.NewDecoder(reader).Decode(&ts)
		if err != nil {
			return ts, err
		}
		return ts, nil
	}
	teamdFiles, err := ioutil.ReadDir("/var/run/teamd")
	if err != nil {
		return md, nil
	}
	for _, f := range teamdFiles {
		name := f.Name()
		if strings.HasSuffix(name, ".pid") {
			name = strings.TrimSuffix(name, ".pid")
			ts, err := getState(name)
			if err != nil {
				return md, err
			}
			var slaveCount int
			var speed int64
			for portName, port := range ts.TeamPorts {
				slaveCount++
				speed += int64(port.Link.Speed)
				metadata.AddMeta("", opentsdb.TagSet{"iface": portName}, "master", name, true)
				Add(&md, "linux.net.bond.slave.is_up", port.Link.Up, opentsdb.TagSet{"slave": portName, "bond": name}, metadata.Gauge, metadata.Bool, linuxNetBondSlaveIsUpDesc)
			}
			Add(&md, "os.net.bond.ifspeed", speed, opentsdb.TagSet{"bond": name}, metadata.Gauge, metadata.Megabit, osNetIfSpeedDesc)
			Add(&md, "linux.net.bond.slave.count", slaveCount, opentsdb.TagSet{"bond": name}, metadata.Gauge, metadata.Count, linuxNetBondSlaveCount)
		}
	}
	return md, nil
}

type TeamState struct {
	TeamPorts map[string]TeamPort `json:"ports"`
	Runner    struct {
		Active       bool    `json:"active"`
		FastRate     bool    `json:"fast_rate"`
		SelectPolicy string  `json:"select_policy"`
		SysPrio      float64 `json:"sys_prio"`
	} `json:"runner"`
	Setup struct {
		Daemonized         bool    `json:"daemonized"`
		DbusEnabled        bool    `json:"dbus_enabled"`
		DebugLevel         float64 `json:"debug_level"`
		KernelTeamModeName string  `json:"kernel_team_mode_name"`
		Pid                float64 `json:"pid"`
		PidFile            string  `json:"pid_file"`
		RunnerName         string  `json:"runner_name"`
		ZmqEnabled         bool    `json:"zmq_enabled"`
	} `json:"setup"`
	TeamDevice struct {
		Ifinfo struct {
			DevAddr    string  `json:"dev_addr"`
			DevAddrLen float64 `json:"dev_addr_len"`
			Ifindex    float64 `json:"ifindex"`
			Ifname     string  `json:"ifname"`
		} `json:"ifinfo"`
	} `json:"team_device"`
}

type TeamPort struct {
	Ifinfo struct {
		DevAddr    string  `json:"dev_addr"`
		DevAddrLen float64 `json:"dev_addr_len"`
		Ifindex    float64 `json:"ifindex"`
		Ifname     string  `json:"ifname"`
	}
	Link struct {
		Duplex string  `json:"duplex"`
		Speed  float64 `json:"speed"`
		Up     bool    `json:"up"`
	} `json:"link"`
	LinkWatches struct {
		List struct {
			LinkWatch0 struct {
				DelayDown float64 `json:"delay_down"`
				DelayUp   float64 `json:"delay_up"`
				Name      string  `json:"name"`
				Up        bool    `json:"up"`
			} `json:"link_watch_0"`
		} `json:"list"`
		Up bool `json:"up"`
	} `json:"link_watches"`
	Runner struct {
		ActorLacpduInfo struct {
			Key            float64 `json:"key"`
			Port           float64 `json:"port"`
			PortPriority   float64 `json:"port_priority"`
			State          float64 `json:"state"`
			System         string  `json:"system"`
			SystemPriority float64 `json:"system_priority"`
		} `json:"actor_lacpdu_info"`
		Aggregator struct {
			ID       float64 `json:"id"`
			Selected bool    `json:"selected"`
		} `json:"aggregator"`
		Key               float64 `json:"key"`
		PartnerLacpduInfo struct {
			Key            float64 `json:"key"`
			Port           float64 `json:"port"`
			PortPriority   float64 `json:"port_priority"`
			State          float64 `json:"state"`
			System         string  `json:"system"`
			SystemPriority float64 `json:"system_priority"`
		} `json:"partner_lacpdu_info"`
		Prio     float64 `json:"prio"`
		Selected bool    `json:"selected"`
		State    string  `json:"state"`
	} `json:"runner"`
}

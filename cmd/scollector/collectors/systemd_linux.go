package collectors

import (
	"fmt"
	"regexp"
	"strings"

	"bosun.org/_third_party/github.com/coreos/go-systemd/dbus"
	"bosun.org/cmd/scollector/conf"
	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/util"
)

type systemdServiceConfig struct {
	regex *regexp.Regexp
	watch bool
}

var systemdServices []*systemdServiceConfig

func init() {
	registerInit(func(c *conf.Conf) {
		for _, s := range c.SystemdService {
			AddSystemdServiceConfig(s)
		}
		collectors = append(collectors, &IntervalCollector{
			F: func() (opentsdb.MultiDataPoint, error) {
				return c_systemd()
			},
			name: "c_systemd",
		})
	})
}

func AddSystemdServiceConfig(params conf.ServiceParams) error {
	if params.Name == "" {
		return fmt.Errorf("empty service Name")
	}
	reg, err := regexp.Compile(params.Name)
	if err != nil {
		return err
	}
	serviceConfig := systemdServiceConfig{regex: reg, watch: params.WatchProc}
	systemdServices = append(systemdServices, &serviceConfig)
	return nil
}

// c_systemd() iterates through all units provided by systemd's dbus info.
// If the unit is a service we care about, it sends service metrics.
// Also sends process data if WatchProc was set to true in the [[SystemdService]] config.
func c_systemd() (opentsdb.MultiDataPoint, error) {
	conn, err := dbus.New()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	units, err := conn.ListUnits()
	if err != nil {
		return nil, err
	}

	var md opentsdb.MultiDataPoint
	for _, unit := range units {
		if strings.HasSuffix(unit.Name, ".service") {
			shortName := strings.TrimSuffix(unit.Name, ".service")
			for _, config := range systemdServices {
				if config.regex.MatchString(shortName) {
					if config.watch {
						err = watchSystemdServiceProc(&md, conn, unit)
						if err != nil {
							return nil, err
						}
					}
					if _, ok := activeState[unit.ActiveState]; !ok {
						return nil, fmt.Errorf("error: unit %s has unknown ActiveState: %s\n", unit.Name, unit.ActiveState)
					}
					systemdTags := opentsdb.TagSet{"name": unit.Name}
					osTags := opentsdb.TagSet{"name": shortName}
					Add(&md, "linux.systemd.unit.activestate", activeState[unit.ActiveState], systemdTags, metadata.Gauge, metadata.StatusCode, descActiveState)
					Add(&md, osServiceRunning, util.Btoi(unit.ActiveState == "active"), osTags, metadata.Gauge, metadata.Bool, osServiceRunningDesc)
				}
			}
		}
	}

	return md, err
}

// watchSystemdService() attempts to determine the main PID of a service and
// builds a WatchedProc{} which is then sent to linuxProcMonitor()
func watchSystemdServiceProc(md *opentsdb.MultiDataPoint, conn *dbus.Conn, unit dbus.UnitStatus) error {
	// ExecMainPID can be non-running. MainPID is the pid of the running service.
	mainPIDProp, err := conn.GetUnitTypeProperty(unit.Name, "Service", "MainPID")
	if err != nil {
		return err
	}

	mainPID := mainPIDProp.Value.Value().(uint32)
	// MainPID is 0 if there is no running service.
	if mainPID == 0 {
		return nil
	}

	cmdline, err := getLinuxCmdline(fmt.Sprint(mainPID))
	if err != nil {
		return err
	}
	if cmdline == nil {
		return nil
	}

	wp := WatchedProc{
		Command:   regexp.MustCompile("^" + regexp.QuoteMeta(cmdline[0]) + "$"),
		Name:      strings.TrimSuffix(unit.Name, ".service"),
		Processes: make(map[string]int),
		ArgMatch:  regexp.MustCompile(""),
		idPool:    new(idPool)}

	// Since we only have one PID per service (at the moment), this is always set to 1
	wp.Processes[fmt.Sprint(mainPID)] = wp.get()

	if e := linuxProcMonitor(&wp, md); e != nil {
		return e
	}

	return err
}

var activeState = map[string]int{
	"active":       0,
	"reloading":    1,
	"inactive":     2,
	"failed":       3,
	"activating":   4,
	"deactivating": 5,
}

const (
	descActiveState = "0: active, 1: reloading, 2: inactive, 3: failed, 4: activating, 5: deactivating"
)

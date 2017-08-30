package collectors

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
	"bosun.org/slog"
	"bosun.org/util"
	"github.com/StackExchange/mof"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_dsc_mof, Interval: time.Minute * 5})
	collectors = append(collectors, &IntervalCollector{F: c_dsc_status, Interval: time.Minute * 5})
}

const (
	dscLCM    = "dsc.lcm."
	dscMof    = "dsc.mof."
	dscStatus = "dsc.status."

	//dscDefaultConfigurationName is used when the ConfigurationName value in the struct is NULL (non-pull based configuration usually)
	dscDefaultConfigurationName = "__no_name__"
)

var (
	dscpath     = os.ExpandEnv(`${SYSTEMROOT}\system32\Configuration\`)
	mapMofFiles = map[string]string{
		"MetaConfig.mof":       "Meta_Config",
		"Current.mof":          "Current_Config",
		"backup.mof":           "Backup_Config",
		"pending.mof":          "Pending_Config",
		"DSCStatusHistory.mof": "DSC_History",
		"DSCEngineCache.mof":   "DSC_Cache",
	}
)

// c_dsc_mof monitors the size and last modified time of each mof file.
// These out of band metrics can be used to verify the DSC WMI Status metrics.
func c_dsc_mof() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	c := 0
	if _, err := os.Stat(dscpath + "MetaConfig.mof"); os.IsNotExist(err) {
		c = 1
	}
	Add(&md, dscLCM+"configured", c, nil, metadata.Gauge, metadata.StatusCode, descDSCLCMConfigured)
	if c == 1 {
		return md, nil
	}
	for filename, filetype := range mapMofFiles {
		tags := opentsdb.TagSet{"type": filetype}
		s := int64(-1)
		l := int64(-1)
		if fi, fierr := os.Stat(dscpath + filename); fierr == nil {
			s = fi.Size()
			l = time.Now().Unix() - fi.ModTime().Unix()
		}
		Add(&md, dscMof+"size", s, tags, metadata.Gauge, metadata.Bytes, descDSCMofSize)
		Add(&md, dscMof+"last_modified", l, tags, metadata.Gauge, metadata.Second, descDSCMofModified)
	}
	return md, nil
}

const (
	descDSCLCMConfigured = "Indicates if DSC Local Configuration Manager is configured: 0=configured, 1=not configured. If the LCM is not configured then the rest of the dsc.* metrics will be skipped on that server."
	descDSCMofSize       = "Size of the mof file in bytes or -1 if file does not exist."
	descDSCMofModified   = "Number of seconds since the mof file was last modified or -1 if file does not exist."
)

func c_dsc_status() (opentsdb.MultiDataPoint, error) {
	var md opentsdb.MultiDataPoint
	if _, err := os.Stat(dscpath + "MetaConfig.mof"); os.IsNotExist(err) {
		return md, nil
	}
	var dst MSFT_DSCConfigurationStatus
	dscstatusmof, err := util.Command(time.Minute, nil, "wmic",
		`/namespace:\\ROOT\Microsoft\Windows\DesiredStateConfiguration`, "class",
		"MSFT_DSCLocalConfigurationManager", "call", "GetConfigurationStatus")
	if err != nil {
		//Skip if dsc is currently running a consistency check
		if err.Error() == "exit status 2147749889" {
			return md, nil
		} else {
			return nil, slog.Wrap(err)
		}
	}
	dscstatusbuffer := new(bytes.Buffer)
	_, err = dscstatusbuffer.ReadFrom(dscstatusmof)
	if err != nil {
		return nil, slog.Wrap(err)
	}
	err = mof.Unmarshal(dscstatusbuffer.Bytes(), &dst)
	if err != nil {
		return nil, slog.Wrap(err)
	}
	if dst.ReturnValue != 0 {
		return nil, fmt.Errorf("GetConfigurationStatus ReturnValue %v", dst.ReturnValue)
	}
	for _, v := range dst.MSFT_DSCConfigurationStatus {
		Add(&md, dscLCM+"auto_reboot", v.MetaConfiguration.RebootNodeIfNeeded, nil, metadata.Gauge, metadata.Bool, descWinDSCAutoReboot)
		Add(&md, dscLCM+"frequency_config", v.MetaConfiguration.ConfigurationModeFrequencyMins, nil, metadata.Gauge, metadata.Count, descWinDSCFreqConfig)
		Add(&md, dscLCM+"frequency_refresh", v.MetaConfiguration.RefreshFrequencyMins, nil, metadata.Gauge, metadata.Count, descWinDSCFreqRefresh)
		Add(&md, dscLCM+"refresh_mode", dscModeToStatusCode(v.Mode), nil, metadata.Gauge, metadata.StatusCode, descWinDSCMode)
		Add(&md, dscLCM+"state", dscStateToStatusCode(v.MetaConfiguration.LCMState), nil, metadata.Gauge, metadata.StatusCode, descWinDSCState)
		Add(&md, dscStatus+"reboot_requested", v.RebootRequested, nil, metadata.Gauge, metadata.Bool, descWinDSCRebootRequested)
		Add(&md, dscStatus+"resources_total", v.NumberOfResources, nil, metadata.Gauge, metadata.Count, descWinDSCNumberOfResources)
		Add(&md, dscStatus+"run_age", dscStartDateToAge(v.StartDate), nil, metadata.Gauge, metadata.Second, descWinDSCAge)
		Add(&md, dscStatus+"run_duration", v.DurationInSeconds, nil, metadata.Gauge, metadata.Second, descWinDSCDurationInSeconds)
		Add(&md, dscStatus+"run_success", v.Status == "Success", nil, metadata.Gauge, metadata.Bool, descWinDSCStatus)
		Add(&md, dscStatus+"run_type", dscTypeToStatusCode(v.Type), nil, metadata.Gauge, metadata.Count, descWinDSCType)
		configurations := make(map[string]dscResourceCount)
		for _, r := range v.ResourcesInDesiredState {
			name := dscGetConfigurationName(r.ConfigurationName)
			c := configurations[name]
			c.Success++
			c.Duration += r.DurationInSeconds
			configurations[name] = c
		}
		for _, r := range v.ResourcesNotInDesiredState {
			name := dscGetConfigurationName(r.ConfigurationName)
			c := configurations[name]
			c.Failed++
			c.Duration += r.DurationInSeconds
			configurations[name] = c
		}
		for key, value := range configurations {
			Add(&md, dscStatus+"resources", value.Success, opentsdb.TagSet{"state": "Success", "configuration": key}, metadata.Gauge, metadata.Count, descWinDSCResourceState)
			Add(&md, dscStatus+"resources", value.Failed, opentsdb.TagSet{"state": "Failed", "configuration": key}, metadata.Gauge, metadata.Count, descWinDSCResourceState)
			Add(&md, dscStatus+"config_duration", value.Duration, opentsdb.TagSet{"configuration": key}, metadata.Gauge, metadata.Second, descWinDSCConfigSeconds)
		}
	}
	return md, nil
}

const (
	descWinDSCAge               = "Seconds since the last DSC run."
	descWinDSCAutoReboot        = "Bool for if the LCM can automatically reboot the system when needed."
	descWinDSCDurationInSeconds = "Seconds taken to process entire configuration."
	descWinDSCConfigSeconds     = "Seconds taken to process each configuration."
	descWinDSCFreqConfig        = "Specifies how often (in minutes) LCM ensures that the configuration is in the desired state."
	descWinDSCFreqRefresh       = "Specifies how often (in minutes) LCM attempts to obtain the configuration from the pull server. If configuration on the pull server differs from the current one on the target node, it is copied to the pending store and applied."
	descWinDSCMode              = "Refresh mode of configuration. 0=Pull, 1=Push, 2=Disabled, -1=Unknown."
	descWinDSCNumberOfResources = "Total number of resources in configuration."
	descWinDSCRebootRequested   = "Reboot was requested during configuration run."
	descWinDSCResourceState     = "Count of resources from each named configuration that were successful or failed."
	descWinDSCStartDate         = "Date and time when the configuration was started."
	descWinDSCState             = "Current LCM Statue. 0=Idle, 1=PendingConfiguration, 2=Ready, 3=Busy, 4=PendingReboot, -1=Unknown."
	descWinDSCStatus            = "Status of configuration."
	descWinDSCType              = "Type of configuration run. 0=Reboot, 1=Initial, 2=Consistency, 3=ReadOnly, -1=Unknown."
)

type MSFT_DSCConfigurationStatus struct {
	MSFT_DSCConfigurationStatus []struct {
		DurationInSeconds int
		MetaConfiguration struct {
			ConfigurationModeFrequencyMins int
			LCMState                       string
			RebootNodeIfNeeded             bool
			RefreshFrequencyMins           int
		}
		Mode                    string
		NumberOfResources       int
		RebootRequested         bool
		ResourcesInDesiredState []struct {
			ConfigurationName string
			DurationInSeconds float64
		}
		ResourcesNotInDesiredState []struct {
			ConfigurationName string
			DurationInSeconds float64
		}
		StartDate string
		Status    string
		Type      string
	}
	ReturnValue int
}

type dscResourceCount struct {
	Success  int
	Failed   int
	Duration float64
}

func dscModeToStatusCode(mode string) int64 {
	switch strings.ToUpper(mode) {
	case "PULL":
		return 0
	case "PUSH":
		return 1
	case "DISABLED":
		return 2
	default:
		return -1
	}
}

func dscTypeToStatusCode(t string) int64 {
	switch strings.ToUpper(t) {
	case "REBOOT":
		return 0
	case "INITIAL":
		return 1
	case "CONSISTENCY":
		return 2
	case "READONLY":
		return 3
	default:
		return -1
	}
}

func dscStateToStatusCode(t string) int64 {
	switch strings.ToUpper(t) {
	case "IDLE":
		return 0
	case "PENDINGCONFIGURATION":
		return 1
	case "READY":
		return 2
	case "BUSY":
		return 3
	case "PENDINGREBOOT":
		return 4
	default:
		return -1
	}
}

func dscStartDateToAge(startdate string) float64 {
	var t = time.Time{}
	var err error
	// See https://msdn.microsoft.com/en-us/library/aa387237(v=vs.85).aspx for different WMI date time formats
	if len(startdate) == 25 && strings.IndexAny(startdate, "+-") == 21 {
		//Parse yyyymmddHHMMSS.mmmmmmsUUU where sUUU is timezone in +/- minutes from UTC
		tzmin, err := strconv.Atoi(startdate[21:])
		if err != nil {
			return -1
		}
		t, err = time.ParseInLocation("20060102150405.999999", startdate[0:21], time.FixedZone("WMI", tzmin*60))
	} else {
		//Parse yyyy-mm-dd HH:MM:SS:mmm and assume UTC
		t, err = time.Parse("2006/01/02 15:04:05", startdate)
	}
	if err != nil {
		return -1
	}
	return time.Now().UTC().Sub(t).Seconds()
}

func dscGetConfigurationName(Name string) string {
	if Name != "" {
		return Name
	}
	return dscDefaultConfigurationName
}

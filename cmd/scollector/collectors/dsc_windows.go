package collectors

import (
	"os"
	"time"

	"bosun.org/metadata"
	"bosun.org/opentsdb"
)

func init() {
	collectors = append(collectors, &IntervalCollector{F: c_dsc_mof, Interval: time.Minute * 5})
}

const (
	dscLCM = "dsc.lcm."
	dscMof = "dsc.mof."
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

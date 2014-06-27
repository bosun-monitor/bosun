package metadata

import (
	"strings"

	"github.com/StackExchange/bosun/_third_party/github.com/StackExchange/scollector/opentsdb"
)

func init() {
	metafuncs = append(metafuncs, metaLinuxVersion, metaLinuxIfaces)
}

func metaLinuxVersion() {
	readCommand(func(line string) {
		AddMeta("", nil, "uname", line)
	}, "uname", "-a")
	readCommand(func(line string) {
		f := strings.Fields(line)
		if len(f) < 1 || f[0] != "CentOS" {
			return
		}
		AddMeta("", nil, "version", line)
	}, "cat", "/etc/issue")
}

func metaLinuxIfaces() {
	var iface string
	readCommand(func(line string) {
		sp := strings.Fields(line)
		if len(sp) == 0 {
			iface = ""
			return
		}
		if iface == "" {
			iface = sp[0]
		}
		if iface == "lo" {
			return
		}
		if len(sp) > 1 && sp[0] == "inet" {
			asp := strings.Split(sp[1], ":")
			if len(asp) == 2 && asp[0] == "addr" {
				AddMeta("", opentsdb.TagSet{"iface": iface}, "addr", asp[1])
			}
		}
	}, "ifconfig")
}

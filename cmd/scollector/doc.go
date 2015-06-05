/*

Scollector is a metric collection agent for OpenTSDB 2.0 and Bosun.

tcollector (https://github.com/OpenTSDB/tcollector) is OpenTSDB's data
collection framework built for OpenTSDB 1.0. scollector aims to be tcollector
for OpenTSDB 2.0 and is one method of sending data to Bosun (http://bosun.org/)
for monitoring.

Unlike tcollector, scollector is a single binary where all collectors are
compiled into scollector itself. scollector supports external collectors, but
your goal should be to use those temporarily until the go version is written or
the target system send data directly to OpenTSDB or Bosun. scollector has
native collectors for Linux, Darwin, and Windows and can pull data from other
systems such as AWS, SNMP, and vSphere.

Usage:
	scollector [flag]

The flags are:

	-h=""
		OpenTSDB or Bosun host. Overrides Host in conf file.
	-f=""
		Filters collectors matching these terms, separated by
		comma. Overrides Filter in conf file.
	-b=0
		OpenTSDB batch size. Used for debugging bad data.
	-conf=""
		Location of configuration file. Defaults to scollector.toml in directory of
		the scollector executable.
	-l
		List available collectors (after Filter is applied).
	-m
		Disable sending of metadata.
	-version
		Prints the version and exits.

Additional flags on Windows:
	-winsvc=""
		Windows Service management; can be: install, remove, start, stop

Debug flags:
	-d
		enables debug output
	-p
		print to screen instead of sending to a host
	-fake=0
		generates X fake data points per second on the test.fake metric

The only required paremeter is the host, which may be specified in the conf
file or with -h.

Warning

scollector has not been tested outside of the Stack Exchange environment, and
thus may act incorrectly elsewhere.

scollector requires the new HTTP API of OpenTSDB 2.1 with gzip support. Ensure
that is in use if not using the OpenTSDB docker image.

Logs

If started with -p or -d, scollector logs to Stdout. Otherwise, on Unixes,
scollector logs to syslog. On Windows when started as a service, the Event Log
is used.

External Collectors

External collectors are executables that scollector invokes, collects output
from, and uses that like other collector data. The -c option specfies the
external collectors directory. It should contain numbered directories like
OpenTSDB tcollector expects. Any executable file in those directories is run
every N seconds, where N is the name of the directory. Use 0 for a program that
should be run continuously and simply pass data through to OpenTSDB (the program
will be restarted if it exits). Data output format is:

	metric timestamp value tag1=val1 tag2=val2 ...

Timestamp is in Unix format (seconds since epoch). Tags are optional. A host tag
is automatically added, but overridden if specified. Stderr output is passed to
scollector's log.

Configuration File

If scollector.conf exists in the same directory as the scollector
executable or is specified via the -conf="" flag, it's content
will be used to set configuration flags. The format is toml
(https://github.com/toml-lang/toml/blob/master/versions/en/toml-v0.2.0.md).
Available keys are:

Host (string): the OpenTSDB or Bosun host to send data.

FullHost (string): enables full hostnames: doesn't truncate to first ".".

ColDir (string): is the external collectors directory.

Tags (table of strings): are added to every datapoint. If a collector specifies
the same tag key, this one will be overwritten. The host tag is not supported.

Hostname (string): overrides the system hostname.

DisableSelf (boolean): disables sending of scollector self metrics.

Freq (integer): is the default frequency in seconds for most collectors.

Filter (array of string): filters collectors matching these terms.

Collector configuration keys

Following are configurations for collectors that do not autodetect.

KeepalivedCommunity (string): if not empty, enables the Keepalived collector
with the specified community.

	KeepalivedCommunity = "keepalivedcom"

HAProxy (array of table, keys are User, Password, Instances): HAProxy instances
to poll. The Instances key is an array of table with keys Tier and URL.

	[[HAProxy]]
	  User = "hauser"
	  Password = "hapass"
	  [[HAProxy.Instances]]
	    Tier = "1"
	    URL = "http://ny-host01:17/haproxy\;csv"
	  [[HAProxy.Instances]]
	    Tier = "2"
	    URL = "http://ny-host01:26/haproxy\;csv"
	  [[HAProxy.Instances]]
	    Tier = "3"
	    URL = "http://ny-host01:40/haproxy\;csv"

SNMP (array of table, keys are Community and Host): SNMP hosts to connect
to at a 5 minute poll interval.

	[[SNMP]]
	  Community = com
	  Host = host
	[[SNMP]]
	  Community = com2
	  Host = host2

ICMP (array of table, keys are Host): ICMP hosts to ping.

	[[ICMP]]
	  Host = "internal-router"
	[[ICMP]]
	  Host = "backup-router"

Vsphere (array of table, keys are Host, User, Password): vSphere hosts to poll.

	[[Vsphere]]
	  Host = "vsphere01"
	  User = "vuser"
	  Password = "pass"

AWS (array of table, keys are AccessKey, SecretKey, Region): AWS hosts to poll.

	[[AWS]]
	  AccessKey = "aoesnuth"
	  SecretKey = "snch0d"
	  Region = "somewhere"

Process (array of table, keys are Command, Name, Args for Linux, and Name
for Windows): processes to monitor. Name is optional, and defaults to Command.

	# Linux
	[[Process]]
	  Command = "redis-server *:6379"
	  Name = "redis-main"
	[[Process]]
	  Command = "redis-server *:6380"
	  Name = "redis-slave"

	# Windows
	[[Process]]
	  Command = "^java"
	[[Process]]
	  Command = "^powershell"


ProcessDotNet (array of table, keys are Name): .NET processes to monitor
on Windows.

	[[ProcessDotNet]]
	  Name = "^w3wp"
	[[ProcessDotNet]]
	  Name = "^Scheduler"

HTTPUnit (array of table, key is File): httpunit toml files to read and
monitor. See https://github.com/StackExchange/httpunit for documentation
about the toml file.

	[[HTTPUnit]]
	  File = "/path/to/httpunit.toml"

Windows

scollector has full Windows support. It can be run standalone, or installed as a
service (see -winsvc). The Event Log is used when installed as a service.

*/
package main

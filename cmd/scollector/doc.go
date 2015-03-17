/*

Scollector is a metric collection agent for OpenTSDB 2.0 and http://Bosun.org

tcollector (https://github.com/OpenTSDB/tcollector) is OpenTSDB's data
collection framework built for OpenTSDB 1.0. scollector aims to be tcollector
for OpenTSDB 2.0 and is one method of sending data to Bosun for monitoring.

Unlike tcollector, scollector is a single binary where all collectors are
compiled into scollector itself. scollector supports external collectors, but
your goal should be to use those temporarily until the go version is written or
the target system send data directly to OpenTSDB or Bosun. scollector has
native collectors for Linux, Darwin, and Windows and can pull data from other
systems such as AWS, SNMP, and vSphere.

Usage:
	scollector [flag]

The flags are:
	-aws=""
	    AWS keys and region, format: "access_key:secret_key@region".
	-b=0
	    OpenTSDB batch size. Used for debugging bad data.
	-c=""
	    External collectors directory.
	-conf=""
	    Location of configuration file. Defaults to scollector.conf in directory of
	    the scollector executable.
	-f=""
	    Filters collectors matching this term, multiple terms separated by comma.
	    Works with all other arguments.
	-freq="15"
	    Set the default frequency in seconds for most collectors.
	-h=""
	    Bosun or OpenTSDB host; can optionally specify a port and scheme
	    ("http://bosun.example.com:8070"), but will default to
	    http://bosun:80/
	-hostname=""
	    If set, use as value of host tag instead of system hostname.
	-i=""
	    ICMP host to ping of the format: "host[,host...]".
	-l
	    List available collectors.
	-m
	    Disable sending of metadata.
	-n
	    Disable sending of scollector self metrics.
	-s=""
	    SNMP host to poll of the format: "community@host[,community@host...]".
	-t=""
	    Tags to add to every datapoint in the format dc=ny,rack=3. If a collector
	    specifies the same tag key, this one will be overwritten. The host tag is
	    not supported.
	-u
	    Enables full hostnames: doesn't truncate to first ".".
	-v=""
	    vSphere host to poll of the format:
	    "user:password@host[,user:password@host...]".
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

scollector will, by default, report to the host `bosun`, which you should
configure on your local nameserver to point to your TSDB server. This makes it
possible to run scollector correctly with zero configuration or command line
flags.

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

If scollector.conf exists in the same directory as the scollector executable or
is specified via the -conf="" flag, it's content will be used to set
configuration flags. Configuration file values overwrite command line flags.
The configuration file is of the form key = value, one per line. Supported keys
are: host (-h), hostname (-hostname), filter (-f), coldir (-c),
snmp (-s), icmp (-i), vsphere (-v). Example:

	host = other-tsdb:1234
	filter = snmp
	snmp = com@theswitch

There also are additional values that are used to configure specific collectors.

	// Linux processes to monitor use the form "command,name,command line regex"
	process = ruby,puppet-agent,puppet
	process = java,opentsdb,opentsdb
	process = java,elastic,elasticsearch
	process = java,logstash,logstash
	process = /opt/bosun/bosun,bosun,
	process = /opt/scollector/scollector,scollector,

	// Windows processes and service monitors use the form "name regex"
	process = ^chrome
	process = ^powershell
	process = ^scollector
	process = ^WinRM
	process = (?i)^MSSQLServer //Matches are case sensitive unless specified

	// Dotnet processes to monitor use the form "process/service name regex"
	process_dotnet=^w3wp //Optional, as IIS processes are always monitored
	process_dotnet=^MyCustomService
	process_dotnet=^powershell

Windows

scollector has full Windows support. It can be run standalone, or installed as a
service (see -winsvc). The Event Log is used when installed as a service.

SNMP

By default, scollector will collect data from the current host and report it to
the TSDB server. scollector has an SNMP mode where it also polls a given host:

	scollector -s community@host[,community@host...]

Poll frequency currently defaults to 5 minutes. Some common OIDs regarding
interfaces are collected. Others can be added easily.

*/
package main

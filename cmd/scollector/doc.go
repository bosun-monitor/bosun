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
		OpenTSDB batch size. Default is 500.
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

See http://bosun.org/scollector/external-collectors for details about using
external scripts or programs to collect metrics.

Configuration File

If scollector.toml exists in the same directory as the scollector
executable or is specified via the -conf="" flag, it's content
will be used to set configuration flags. The format is toml
(https://github.com/toml-lang/toml/blob/master/versions/en/toml-v0.2.0.md).
Available keys are:

Host (string): the OpenTSDB or Bosun host to send data, supports TLS and
HTTP Basic Auth.

	Host = "https://user:password@example.com/"

FullHost (boolean): enables full hostnames: doesn't truncate to first ".".

ColDir (string): is the external collectors directory.

Tags (table of strings): are added to every datapoint. If a collector specifies
the same tag key, this one will be overwritten. The host tag is not supported.

Hostname (string): overrides the system hostname.

DisableSelf (boolean): disables sending of scollector self metrics.

Freq (integer): is the default frequency in seconds for most collectors.

BatchSize (integer): is the number of metrics that will be sent in each batch.
Default is 500.

Filter (array of string): filters collectors matching these terms.

MetricFilters (array of string): filters metrics matching these regular
expressions.

IfaceExpr (string): Replaces the default regular expression for interface name
matching on Linux.

PProf (string): optional IP:Port binding to be used for debugging with pprof.
Examples: localhost:6060 for loopback or :6060 for all IP addresses.

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
	  Community = "com"
	  Host = "host"
	  MIBs = ["cisco"]
	[[SNMP]]
	  Community = "com2"
	  Host = "host2"
	  # List of mibs to run for this host. Default is built-in set of ["ifaces","cisco"]
	  MIBs = ["custom", "ifaces"]

MIBs (map of string to table): Allows user-specified, custom SNMP configurations.

    [MIBs]
      [MIBs.cisco] #can name anything you want
        BaseOid = "1.3.6.1.4.1.9.9" # common base for all metrics in this mib

        # simple, single key metrics
        [[MIBs.cisco.Metrics]]
          Metric = "cisco.cpu"
          Oid = ".109.1.1.1.1.6"
          Unit = "percent"
          RateType = "gauge"
          Description = "cpu percent used by this device"

        # can also iterate over snmp tables
        [[MIBs.cisco.Trees]]
          BaseOid = ".48.1.1.1" #common base oid for this tree

          # tags to apply to metrics in this tree. Can come from another oid, or specify "idx" to use
          # the numeric index as the tag value. Can specify multiple tags, but must supply one.
          # all tags and metrics should have the same number of rows per query.
          [[MIBs.cisco.Trees.Tags]]
            Key = "name"
            Oid = ".2"
          [[MIBs.cisco.Trees.Metrics]]
            Metric = "cisco.mem.used"
            Oid = ".5"
          [[MIBs.cisco.Trees.Metrics]]
            Metric = "cisco.mem.free"
            Oid = ".6"

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


Process: processes to monitor.

ProcessDotNet: .NET processes to monitor on Windows.

See http://bosun.org/scollector/process-monitoring for details about Process and
ProcessDotNet.

HTTPUnit (array of table, keys are TOML, Hiera): httpunit TOML and Hiera
files to read and monitor. See https://github.com/StackExchange/httpunit
for documentation about the toml file. TOML and Hiera may both be specified,
or just one.

	[[HTTPUnit]]
	  TOML = "/path/to/httpunit.toml"
	  Hiera = "/path/to/listeners.json"
	[[HTTPUnit]]
	  TOML = "/some/other.toml"

Riak (array of table, keys are URL): Riak hosts to poll.

	[[Riak]]
	  URL = "http://localhost:8098/stats"

RabbitMQ (array of table, keys are URL): RabbitMQ hosts to poll.
Regardless of config the collector will automatically poll
management plugin on http://guest:guest@127.0.0.1:15672/ .

	[[RabbitMQ]]
	  URL = "https://user:password@hostname:15671"

Cadvisor: Cadvisor endpoints to poll.
Cadvisor collects system statistics about running containers.
See https://github.com/google/cadvisor/ for documentation about configuring
cadvisor.

	[[Cadvisor]]
		URL = "http://localhost:8080"

RedisCounters: Reads a hash of metric/counters from a redis database.

    [[RedisCounters]]
        Server = "localhost:6379"
        Database = 2

Expects data populated via bosun's udp listener in the "scollectorCounters" hash.

ExtraHop (array of table): ExtraHop hosts to poll. The two filter options specify how
scollector should filter out traffic from being submitted. The valid options are:

	- namedprotocols (Only protocols that have an explicit name are submitted. The rest of the
					  traffic will be pushed into proto=unnamed. So any protocol that begins with
					  "tcp", "udp" or "SSL" will not be submitted (with the exception of SSL443).
	- toppercent 	 (The top n% of traffic by volume will be submitted. The rest of the traffic
				  	  will be pushed into proto=otherproto)
	- none 			 (All protocols of any size will be submitted)

FilterPercent applies when the FilterBy option is set to "toppercent". Only protocols that account
for this much traffic will be logged. For example, if this is set to 90, then if the protocol
accounts for less than 10% of the traffic, it will be dropped. This is OK if your traffic is
heavilly dominated by asmall set of protocols, but if you have a fairly even spread of protocols
then this filtering loses its usefulness.

	[[ExtraHop]]
	  Host = "extrahop01"
	  APIkey = "abcdef1234567890"
	  FilterBy = "toppercent"
	  FilterPercent = 75

LocalListener (string): local_listener will listen for HTTP request and forward
the request to the configured OpenTSDB host while adding defined tags to
metrics.

	LocalListener = "localhost:4242"

Windows

scollector has full Windows support. It can be run standalone, or installed as a
service (see -winsvc). The Event Log is used when installed as a service.


Database collector

For configuring the database collector see:

http://bosun.org/scollector/database-collector

*/
package main

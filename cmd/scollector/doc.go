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
		Only include collectors matching these comma separated terms. Prefix
		with - to invert match and exclude collectors matching those terms. Use
		*,-term,-anotherterm to include all collectors except excluded terms.
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

MaxQueueLen (integer): is the number of metrics keept internally.
Default is 200000.

UserAgentMessage (string): is an optional message that will be appended to the
User Agent when making HTTP requests. This can be used to add contact details
so external services are aware of who is making the requests.
Example: Scollector/0.6.0 (UserAgentMessage added here)

Filter (array of string): Only include collectors matching these terms. Prefix
with - to invert match and exclude collectors matching those terms. Use
*,-term,-anotherterm to include all collectors except excluded terms.

MetricFilters (array of string): only send metrics matching these regular
expressions. Example ['^(win\.cpu|win\.system\..*)$', 'free']

IfaceExpr (string): Replaces the default regular expression for interface name
matching on Linux.

PProf (string): optional IP:Port binding to be used for debugging with pprof.
Examples: localhost:6060 for loopback or :6060 for all IP addresses.

MetricPrefix (string): optional Prefix prepended to all metrics path.

Collector configuration keys

Following are configurations for collectors that do not autodetect.

KeepalivedCommunity (string): if not empty, enables the Keepalived collector
with the specified community.

	KeepalivedCommunity = "keepalivedcom"

HAProxy (array of table, keys are User, Password, Instances): HAProxy instances
to poll. The Instances key is an array of table with keys User, Password, Tier,
and URL. If User is specified for an instance, User and Password override the
common ones.

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
	  [[HAProxy.Instances]]
	    User = "hauser2"
	    Pass = "hapass2"
	    Tier = "1"
	    URL = "http://ny-host01:80/haproxy\;csv"

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

AWS (array of table, keys are AccessKey, SecretKey, Region, BillingProductCodesRegex,
BillingBucketName, BillingBucketPath, BillingPurgeDays): AWS hosts to poll, and associated
billing information.

To report AWS billing information to OpenTSDB or Bosun, you need to configure AWS to
generate billing reports, which will be put into an S3 bucket. See for more detail:
http://docs.aws.amazon.com/awsaccountbilling/latest/aboutv2/detailed-billing-reports.html

Once the reports are going into the S3, bucket, the Bucket Name and the Prefix Path that
you entered during the report setup need to be entered below. Do not enter a blank bucket
path as this is not supported.

Reports that are over a certain number of days old are purged by scollector. Set the key
BillingPurgeDays to 0 to disable purging of old reports (not that this may increase your S3
usage costs as all reports are processed each time the collector runs).

Do not populate the Billing keys if you do not wish to load billing data into OpenTSDB or
Bosun.

Only products whose name matches the BillingProductCodesRegex key will have their billing
data sent to OpenTSDB or Bosun.

	[[AWS]]
	  AccessKey = "aoesnuth"
	  SecretKey = "snch0d"
	  Region = "somewhere"
	  BillingProductCodesRegex = "^Amazon(S3|Glacier|Route53)$"
	  BillingBucketName = "mybucket.billing"
	  BillingBucketPath = "reports"
	  BillingPurgeDays = 2


AzureEA (array of table, keys are EANumber, APIKey and LogBillingDetails): Azure Enterprise
Agreements to poll for billing information.

EANumber is your Enterprise Agreement number. You can find this in your Enterprise Agreement portal.

APIKey is the API key as provided by the Azure EA Portal. To generate your API key for this collector,
you will need to log into your Azure Enterprise Agreement portal (ea.azure.com), click the
"Download Usage" link, then choose "API Key" on the download page. You can then generate your API
key there. Keys are valid 6 months, so you will require some maintenance of this collector twice a year.

LogBillingDetails tells scollector to add the following tags to your metrics:
   - costcenter
	 - accountname
	 - subscription
If you are a heavy Azure EA user, then these additional tags may be useful for breaking down costs.

	[[AzureEA]]
	  EANumber = "123456"
	  APIKey = "joiIiwiaXNzIjoiZWEubWljcm9zb2Z0YXp1cmUuY29tIiwiYXVkIjoiY2xpZW50LmVhLm1"
	  LogBillingDetails = false

Process: processes to monitor.

ProcessDotNet: .NET processes to monitor on Windows.

See http://bosun.org/scollector/process-monitoring for details about Process and
ProcessDotNet.

HTTPUnit (array of table, keys are TOML, Hiera): httpunit TOML and Hiera
files to read and monitor. See https://github.com/StackExchange/httpunit
for documentation about the toml file. TOML and Hiera may both be specified,
or just one. Freq is collector frequency as a duration string (default 5m).

	[[HTTPUnit]]
	  TOML = "/path/to/httpunit.toml"
	  Hiera = "/path/to/listeners.json"
	[[HTTPUnit]]
	  TOML = "/some/other.toml"
	  Freq = "30s"

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
cadvisor. You can enable per cpu usage metric reporting optionally, and
optionally use IsRemote to disable block device lookups.

	[[Cadvisor]]
		URL = "http://localhost:8080"
		PerCpuUsage = true
		IsRemote = false

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

AdditionalMetrics is formatted as such: [object_type].[object_id].[metric_category].[metric_spec_name]

    - object_type:  is one of: "network", "device", "application", "vlan", "device_group", "activity_group"
    - object_id:    can be found by querying the ExtraHop API (through the API Explorer) under the endpoint
                    for the object type. For example, for "application", you would query the "/applications/"
                    endpoint and locate the ID of the application you want to query.
    - metric_category:  can be found in the Metric Catalogue for the metric you are wanting to query. e.g. for
                        custom metrics, this is always "custom_detail"
    - metric_spec_name: can be found in the Metric Catalogue for the metric you are wanting to query. e.g. for
                        custom metrics, this is name you have specified in metricAddDetailCount() function in
                        a trigger.

For these additional metrics, it is expected that the key for the metric is in a keyvalue, comma seperated pair.
This key will be converted into an OpenTSDB tagset. For example, if you have a key of
"client=192.168.0.1,server=192.168.0.9,port=21441", this will be converted into an OpenTSDB tagset of the same
values.

CAUTION: Do not include unbounded values in your key if you can help it. Putting in something like client IP, or
source/destination port, which are out of your control and specified by people external to your network, could
end up putting millions of different keys into your Bosun instance - something you probably don't want.

CertificateSubjectMatch and CertificateActivityGroup are used for collecting SSL information from ExtraHop. The
key CertificateSubjectMatch is used to match against the certificate subject. If there is no match, we discard
the certificate record. This is important as certificate subjects are essentially unbound, as EH return all
certificates it sees, regardless of where they originated.

The key CertificateActivityGroup is the Activity Group you want to pass through to ExtraHop to pull the certificates
from. There is a group called "SSL Servers" which is most likely the group you want to use. You will need to discover
the group number for this group and put it in here.

	[[ExtraHop]]
	  Host = "extrahop01"
	  APIkey = "abcdef1234567890"
	  FilterBy = "toppercent"
	  FilterPercent = 75
    AdditionalMetrics = [ "application.12.custom_detail.my trigger metric" ]
		CertificateSubjectMatch = "example.(com|org|net)"
		CertificateActivityGroup = 46

LocalListener (string): local_listener will listen for HTTP request and forward
the request to the configured OpenTSDB host while adding defined tags to
metrics.

	LocalListener = "localhost:4242"

TagOverride (array of tables, key are CollectorExpr, MatchedTags and Tags): if a collector
name matches CollectorExpr MatchedTags and Tags will be merged to all outgoing message
produced by the collector, in that order. MatchedTags will apply a regexp to the tag
defined by the key name and add tags based on the named match groups defined in the
regexp. After tags defined in Tags will be merged, defining a tag as empty string
will deletes it.

	[[TagOverride]]
	  CollectorExpr = 'cadvisor'
	  [TagOverride.MatchedTags]
	    docker_name = 'k8s_(?P<container_name>[^\.]+)\.[0-9a-z]+_(?P<pod_name>[^-]+)'
	    docker_id = '^(?P<docker_id>.{12})'
	  [TagOverride.Tags]
	    docker_name = ''
	    source = 'kubelet'

Oracles (array of table, keys are ClusterName, Instances): Oracle database
instances to poll. The Instances key is an array of table with keys
ConnectionString and Role, which are the same as using sqlplus.

	[[Oracles]]
	  ClusterName = "oracle rac name"
	  [[Oracles.instances]]
	    ConnectionString = "/"
	    Role = "sysdba"
	  [[Oracles.instances]]
	    ConnectionString = "username/password@oraclehost/sid"
	  [[Oracles.instances]]
	    ConnectionString = "/@localnodevip/sid"
	    Role = "sysdba"


Windows

scollector has full Windows support. It can be run standalone, or installed as a
service (see -winsvc). The Event Log is used when installed as a service.


*/
package main

/*

tsdbrelay relays OpenTSDB traffic to TSD and Bosun servers.

Requests to /api/put relays the request to OpenTSDB. Any response (success or
failure) is returned to the source. In case of success, the relay attempts to
write to Bosun but with a path of /api/index. If Bosun is down or otherwise
fails, the data is dropped, but no negative status is returned to the source.

Requests to /api/metadata/put will relay only to Bosun, not OpenTSDB. Other
URLs will relay only to OpenTSDB, not Bosun.

Additional relays may be specified, and tsdbrelay will send all datapoints there as well. This enables basic replication to seperate tsdb clusters.

tsdbrelay also can receive "external counters" for infrequent or sporadic metrics. It can increment counters in a redis instance to track counts of things that would otherwise be difficult to keep track of.
To enable this, supply a redis server with the `-redis` flag, and send counter data to `/api/count` in the same format as expected by `/api/put`. There is an scollector feature to periodically pull these counters into bosun/opentsdb (see RedisCounters section of https://godoc.org/bosun.org/cmd/scollector).

tsdbrelay can "denormalize"" metrics in order to decrease metric cardinality for better query performance on metrics with a lot of tags. For example `-denormalize=os.cpu__host` will create an additional data point for `os.cpu{host=web01}` into `__web01.os.cpu{host=web01}` as well.

Usage:
	tsdbrelay [-l listen-address] [-b bosun-server] -t tsdb-server

The flags are:
	-b="bosun"
		Target Bosun server. Can specify as host, host:port, or https://host:port.
	-t=""
		Target OpenTSDB server. Can specify as host, host:port or https://host:port.
	-l=":4242"
		Listen address.
	-v=false
	    Enable verbose logging
	-r=""
		Additional relays to send data to, comma seperated. Intended for secondary data center replication. Only response from primary tsdb server wil be relayed to clients.
		Examples: hostA:port,https://hostB:port,hostC#data-only,https://hostD:8080#metadata-only
	-redis=""
		Redis host to store external counter data in
	-db=0
		Redis database number to use
	-denormalize=""
		List of metrics to denormalize. Comma seperated list of `metric__tagname__tagname` rules. Will be translated to `__tagvalue.tagvalue.metric`

*/
package main

/*

tsdbrelay relays OpenTSDB traffic to TSD and Bosun servers.

Requests to /api/put relays the request to OpenTSDB. Any response (success or
failure) is returned to the source. In case of success, the relay attempts to
write to Bosun but with a path of /api/index. If Bosun is down or otherwise
fails, the data is dropped, but no negative status is returned to the source.

Requests to /api/metadata/put will relay only to Bosun, not OpenTSDB. Other
URLs will relay only to OpenTSDB, not Bosun.

Usage:
	tsdbrelay [-l listen-address] [-b bosun-server] -t tsdb-server

The flags are:
	-b="bosun"
		Target Bosun server. Can specify port with host:port.
	-t=""
		Target OpenTSDB server. Can specify port with host:port.
	-l=":4242"
		Listen address.

*/
package main

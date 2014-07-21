/*

Scollector is a metric collection agent for OpenTSDB 2.0.

tcollector (https://github.com/OpenTSDB/tcollector) is OpenTSDB's data collection framework built for OpenTSDB 1.0. scollector aims to be tcollector for OpenTSDB 2.0.

Unlike tcollector, scollector is a single binary with all collectors are compiled into scollector itself. scollector supports external collectors, but your goal should be to use those temporarily until the go version is written. It has native collectors for Linux, Darwin, and Windows.

Usage:
	scollector [flag]

The flags are:
	-h="bosun"
		OpenTSDB host; can optionally specify a port
		("tsdb.example.com:4000"), but will default to 4242
	-c=""
		passthrough collector directory; it should contain numbered
		directories like OpenTSDB tcollector expects; any executable
		file in those directories is run every N seconds, where N is the
		name of the directory; use 0 for a program that should be run
		continuously and simply pass data through to OpenTSDB (the
		program will be restarted if it exits; data output format is:
		"metric timestamp value tag1=val1 tag2=val2 ..."; timestamp is
		in Unix format (seconds since epoch); tags are optional; a host
		tag is automatically added if not present
	-s=""
		SNMP host to poll of the format:
		"community@host[,community@host...]"
	-i=""
		ICMP host to ping of the format:
		"host[,host...]"
	-f=""
		filter collectors matching this term (regex)
	-t
		test mode: run collectors once, print, and exit
	-l
		list enabled collectors
	-p
		print to screen instead of sending to a host
	-fake=0
		generates X fake data points per second on the test.fake metric
	-b=0
		OpenTSDB batch size; used for debugging bad data

Additional flags on Windows:
	-winsvc=""
		Windows Service management; can be: install, remove, start, stop

scollector will, by default, report to the host `bosun`, which you should configure on your local nameserver to point to your TSDB server. This makes it possible to run scollector correctly with zero configuration or command line flags.

Configuration File

If scollector.conf exists in the same directory as the scollector executable, it will set configuration flags. Configuration file values overwrite command line flags. The configuration file is of the form key = value, one per line. Supported keys are: host (-h), filter (-f), coldir (-c), snmp (-s), icmp (-i), vsphere (-v). Example:

	host = other-tsdb:1234
	filter = snmp
	snmp = com@theswitch

Warning

scollector is in testing, and you should not depend on it for production use yet. We are using it internally at Stack Exchange, but we are still fixing bugs. It is designed to be used with an upcoming and unreleased project we are working on (called TSAF), and so may not work in your environment. Some collector details may be specific to our environment, and so may act weirdly in yours. These will eventually be moved out into config options so the defaults works well for everyone.

An OpenTSDB 2.0 server is required, since it uses the HTTP API. You may have to enable chunked requests on your server (not sure about this). scollector sends gzipped data to OpenTSDB. This feature will be available by default with OpenTSDB 2.1, but is not yet in the next branch. A patch is available to enable this manually (https://groups.google.com/d/msg/opentsdb/JQ7azVR5x_g/37yLYukU5R4J).

Windows

scollector has full Windows support. It can be run standalone, or installed as a service (see -winsvc). The Event Log is used when installed as a service.

SNMP

By default, scollector will collect data from the current host and report it to the TSDB server. scollector has an SNMP mode where it instead polls a given host:

	scollector -s community@host[,community@host...]

Poll frequency currently defaults to 5 minutes. Some common OIDs regarding interfaces are collected. Others can be added easily.

*/
package main

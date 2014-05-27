# scollector

OpenTSDB's tcollector for the v2 HTTP API

[tcollector](https://github.com/OpenTSDB/tcollector) is OpenTSDB's data collection framework built for OpenTSDB 1.0. scollector aims to be tcollector for OpenTSDB 2.0.

# usage

```
$ go get github.com/StackExchange/scollector
$ scollector [-h your-tsdb-server.com]
```

See `scollector -help` for other options.

scollector is implemented in go and thus compiles into a single binary. No python. No installation of anything other than the single scollector binary. All collectors are compiled into scollector itself. (scollector does support external collectors, but your goal should be to use those temporarily until the go version is written.) scollector has native collectors for Linux, Darwin, and Windows. You can cross compile go from one architecture to another, so one build server is enough to build for any machine.

scollector will, by default, report to the host `tsaf`, which you should configure on your local nameserver to point to your TSDB server. This makes it possible to run scollector correctly with zero configuration or command line flags.

# warning

1. scollector is in testing, and you should not depend on it for production use yet. We are using it internally at Stack Exchange, but we are still fixing bugs.
1. Some collector details may be specific to our environment, and so may act weirdly in yours. These will eventually be moved out into config options so the defaults works well for everyone.
1. An OpenTSDB 2.0 server is required, since it uses the HTTP API. You may have to enable chunked requests on your server (not sure about this).
1. It is designed to be used with an upcoming and unreleased project we are working on (called TSAF), and so may not work in your environment.
1. scollector (as of mid April 2014) sends gzipped data to OpenTSDB. This feature will be available by default with OpenTSDB 2.1, but is not yet in the next branch. [A patch is available to enable this manually.](https://groups.google.com/d/msg/opentsdb/JQ7azVR5x_g/37yLYukU5R4J)

# Windows

scollector has full Windows support. It can be run standalone, or installed as a service (see --help for options). The Event Log is used when installed as a service.

# snmp

By default, scollector will collect data from the current host and report it to the TSDB server. scollector has an SNMP mode where it instead polls a given host: `scollector -s community@host`. Poll frequency currently defaults to 5 minutes. Some common OIDs regarding interfaces are collected. Others can be added easily.

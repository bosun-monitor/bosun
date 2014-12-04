# scollector

scollector is a replacement for OpenTSDB's tcollector.

Benefits of scollector over tcollector:

- uses the OpenTSDB v2 API, not the older v1 API
- more resource efficient
- integrates with [Bosun](http://bosun.org)
- comes with many collectors for Windows, Linux, and Mac

OpenTSDB 1.0 uses [tcollector](https://github.com/OpenTSDB/tcollector) to
collect data. This project aims to make scollector the preferred collector for
OpenTSDB 2.0.

## documentation

[http://godoc.org/bosun.org/cmd/scollector](http://godoc.org/bosun.org/cmd/scollector)

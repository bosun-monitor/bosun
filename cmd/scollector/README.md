# scollector

scollector is a replacement for OpenTSDB's tcollector.

Benefits of scollector over tcollector:

- scollector uses the v2 API, not the older v1 API
- scollector is more resource efficient
- scollector integrates with Bosun

OpenTSDB 1.0 uses the
[tcollector](https://github.com/OpenTSDB/tcollector)
data collection framework.
This project aims to make scollector the
preferred collector for OpenTSDB 2.0.

## installation

```
$ go get github.com/StackExchange/scollector
```

## documentation

[http://godoc.org/github.com/StackExchange/scollector](http://godoc.org/github.com/StackExchange/scollector)

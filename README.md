# Bosun

Bosun is a time series alerting framework developed by Stack Exchange. Scollector is a metric collection agent. Learn more at [bosun.org](http://bosun.org).

[![Build Status](https://travis-ci.org/bosun-monitor/bosun.svg?branch=master)](https://travis-ci.org/bosun-monitor/bosun/branches)

## Building

bosun and scollector are found under the `cmd` directory. Run `go build` in the corresponding directories to build each project.
There's also a [Makefile] available for most tasks. 

## Developing

Install:

* Run `make deps` and `make testdeps` to set up all dependencies. 
* Run `make generate` when new static assets (like JS and CSS files) are added or changed.

The `w.sh` script will automatically build and run bosun in a loop.
It will update itself when go/js/ts files change, and it runs in read-only mode, not sending any alerts.

```
$ cd cmd/bosun
$ ./w.sh
```

Go Version:
  * See the version number in `.travis.yml` in the root of this repo for the version of Go to use. 
  Generally speaking, you should be able to use newer versions of Go if you are able to build Bosun without error.
  
Miniprofiler:
 * Bosun includes [miniprofiler] in the web UI which can help with debugging. 
 The key combination `ALT-P` will show miniprofiler. This allows you to see timings, as well as the raw queries sent to TSDBs.

[Makefile]: https://github.com/bosun-monitor/bosun/blob/master/Makefile
[miniprofiler]: https://github.com/MiniProfiler/go

# Bosun

Bosun is a time series alerting framework developed by Stack Exchange. Scollector is a metric collection agent. Learn more at [bosun.org](http://bosun.org).

[![Build Status](https://travis-ci.org/bosun-monitor/bosun.svg?branch=master)](https://travis-ci.org/bosun-monitor/bosun/branches)

## Building

bosun and scollector are found under the `cmd` directory. Run `go build` in the corresponding directories to build each project.
There's also a [Makefile] available for most tasks. 

## Running 

For a full stack with all dependencies, run `docker-compose up` from the `docker` directory. Don't forget to rebuild 
images and containers if you change the code:

    $ cd docker
    $ docker-compose down
    $ docker-compose up --build
    
If you only need the dependencies (Redis, OpenTSDB, HBase) and would like to run Bosun on your machine directly (e.g. to attach
a debugger), you can bring up the dependencies with these three commands from the repository's root:

    $ docker run -p 6379:6379 --name redis redis:6
    $ docker build -f docker/opentsdb.Dockerfile -t opentsdb .
    $ docker run -p 4242:4242 --name opentsdb opentsdb

The OpenTSDB container will be reachable at http://localhost:4242. Redis listens on its default port `6379`.
Bosun, if brought up in a Docker container, is available at http://localhost:8070.

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

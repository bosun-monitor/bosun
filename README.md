# bosun

Bosun is a time series alerting framework developed by Stack Exchange. Scollector is a metric collection agent. Learn more at [bosun.org](http://bosun.org).

[![Build Status](https://travis-ci.org/bosun-monitor/bosun.svg?branch=master)](https://travis-ci.org/bosun-monitor/bosun/branches)

## building

To build bosun and scollector, clone to `$GOPATH/src/bosun.org`:

```
$ go get bosun.org/cmd/bosun
```

bosun and scollector are found under the `cmd` directory. Run `go build` in the corresponding directories to build each project.

## developing

Install:

* `npm install typescript@<version> -g` to be able to compile the ts files to js files. The current version of typescript to install will be in the `.tavis.yml` file in the root of this repo.
* `go get github.com/mjibson/esc` to embed the static files. Run `go generate` in `cmd/bosun` when new static assets (like JS and CSS files) are added or changed.

The `w.sh` script will automatically build and run bosun in a loop.
It will update itself when go/js/ts files change, and it runs in read-only mode, not sending any alerts.

```
$ cd cmd/bosun
$ ./w.sh
```

Go Version:
  * See the version number in `.travis.yml` in the root of this repo for the version of Go to use. Generally speaking, you should be able to use newer versions of Go if you are able to build Bosun without error.
  
Miniprofiler:
 * Bosun includes [miniprofiler](https://github.com/MiniProfiler/go) in the web UI which can help with debugging. The key combination `ALT-P` will show miniprofiler. This allows you to see timings, as well as the raw queries sent to TSDBs.

## Auth Permissions

To avoid requiring people to find the auth settings in the code, we'll add them here:

### Permissions

* `View Dashboard`: Can view dashboard and alert state data, metrics, and graphs
* `View Config`: Can view bosun configuration page
* `Put Data`: Can put and index opentsdb data and metadata
* `Actions`: Can acknowlege and close alerts
* `Run Tests`: Can execute expressions, graphs, and rule tests
* `Save Config`: Can alter and save bosun rule config
* `View Annotations`: Can view annotations on graph page
* `Create Annotations`: Can add and manage annotations via api
* `Silence`: Can add and manage silences
* `Manage Tokens`: Can manage authorization tokens
* `Set Username`: Allows external services to set username in api requests

### Roles

* `Reader`: Read access to dashboard and alert data
* `Admin`: Full access to all functionality
* `Writer`: Write access to alert data and actions


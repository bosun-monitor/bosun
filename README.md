# bosun

Bosun is a time series alerting framework developed by Stack Exchange. Scollector is a metric collection agent.

## building

To build bosun and scollector, clone it to `$GOPATH/src/bosun.org`:

```
  $ go get bosun.org/cmd/bosun
```

bosun and scollector are found under the cmd/ directory. Run `go build` in the corresponding directories to build each project.


## developing

Install:

* typescript to be able to compile the ts files to js files.

The `w.sh` script will automatically build and run bosun in a loop.
It will update itself when go/js/ts files change, and it runs in read-only mode, not sending any alerts.


```
$ cd cmd/bosun
$ ./w.sh
```

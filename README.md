bosun
====

Time Series Alerting Framework

# usage

`bosun [-c=dev.conf] [-t]`

`-c` specifies the config file to use, defaults to `dev.conf`. `-t` parses the config file, validates it, and exits.

# installation

1. `export GOPATH=$HOME/go`
1. `mkdir -p $GOPATH/src/github.com/StackExchange/bosun`
1. `cd $GOPATH/src/github.com/StackExchange/bosun`
1. `git clone git@github.com:StackExchange/bosun.git .`
1. `go build .`

Now you have a `bosun` executable in that directory.

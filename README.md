tsaf
====

Time Series Alerting Framework

# usage

`tsaf [-c=dev.conf] [-t] [-w]`

`-c` specifies the config file to use, defaults to `dev.conf`. `-t` parses the config file, validates it, and exits.

# installation

1. `export GOPATH=$HOME/go`
1. `mkdir -p $GOPATH/src/github.com/StackExchange/tsaf`
1. `cd $GOPATH/src/github.com/StackExchange/tsaf`
1. `git clone git@github.com:StackExchange/tsaf.git .`
1. `go build .`

Now you have a `tsaf` executable in that directory.

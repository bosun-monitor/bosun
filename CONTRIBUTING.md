# Contributing to bosun and scollector

Bosun and scollector are open source projects. We appreciate your help.

## Contributing code

Use GitHub pull requests to submit code. General submission guidelines:

1. Use `gofmt`.
1. If using new third party packages, install party (`go get github.com/mjibson/party`) and run `party` in the root directory (`$GOPATH/src/bosun.org`) to vendor them and rewrite import paths.
1. Squash all commits into one. This may be done as the final step before merging.

Unless otherwise noted, the source files are distributed under the MIT license found in the LICENSE file.

### Style Guidelines
We use the golang [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) document as the basis for our style. Take particular note of Error Strings, Mixed Caps, and Indent error flow secionts. Also we don't have blank lines within functions.

### bosun submission guidelines

1. If changing HTML, JS, or other static content, install esc (`go get github.com/mjibson/esc`), then run `go generate` in `cmd/bosun`.
1. [typescript](https://www.npmjs.com/package/typescript) is required if changing JS files. Invoke bosun with `-w` to watch for `.ts` changes and automatically run typescript.

### scollector submission guidelines

1. New scollector collectors must have units, types, and descriptions for all new metrics. Descriptions should not be in the Add line, but in another data structure or constant. See [keepalive collectors](https://github.com/bosun-monitor/bosun/blob/master/cmd/scollector/collectors/keepalived_linux.go) for the constants, and the [memcached](https://github.com/bosun-monitor/bosun/blob/master/cmd/scollector/collectors/memcached_unix.go) for good patterns.

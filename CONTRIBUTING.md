# Contributing to bosun and scollector

Bosun and scollector are open source projects. We appreciate your help.

## Contributing code

Use GitHub pull requests to submit code. General submission guidelines:

1. Use `gofmt -s -w` and `go vet bosun.org/...`. See `build/validate.sh` for the full list of validation checks that will be run by Travis CI on each commit.
1. If using new third party packages, install party (`go get github.com/mjibson/party`) and run `party` in the root directory (`$GOPATH/src/bosun.org`) to vendor them and rewrite import paths.
1. Squash all non-`_third_party` commits into one. `_third_party` changes should be squashed down separately and precede any code changes which require them. This may be done as the final step before merging.
1. The commit message should indicate what folder is being changed (example: `cmd/scollector: new xyz collector` or `docs: fix typo in window function`)
1. Documentation changes should be made in the same branch as code changes using the `docs` folder. After the PR is approved we will use the `build/docs/publish.go` script to publish the changes to the `gh-pages` branch. Please don't submit changes directly to the `gh-pages` branch, always use the docs folder.

Unless otherwise noted, the source files are distributed under the MIT license found in the LICENSE file.

### Style Guidelines

We use the golang [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments) document as the basis for our style. Take particular note of Error Strings, Mixed Caps, and Indent Error Flow sections. Also we don't have blank lines within functions.

### bosun submission guidelines

1. If changing HTML, JS, or other static content, install esc (`go get github.com/mjibson/esc`), then run `go generate` in `cmd/bosun`.
1. [typescript](https://www.npmjs.com/package/typescript) is required if changing JS files. Invoke bosun with `-w` to watch for `.ts` changes and automatically run typescript.

#### Note for vim users

As vim will usually save file in a temporary buffer, then will rename the file. 
So for the *-w* option to work, one could eventually at this in
`.vimrc`, assuming GOPATH is the homedir:

```
let &backupskip .= ',' . escape(expand('$HOME'), '\') . '/src/bosun.org/cmd/bosun/*.go'
let &backupskip .= ',' . escape(expand('$HOME'), '\') . '/src/bosun.org/cmd/bosun/web/static/templates/*.html'
let &backupskip .= ',' . escape(expand('$HOME'), '\') . '/src/bosun.org/cmd/bosun/web/static/js/*.ts'
```

Or one could prefer the shorter version:

```
set backupskip+=*/cmd/bosun/{*.go\\,web/static/{js/*.ts\\,templates/*.html}}
```

### scollector submission guidelines

1. New scollector collectors must have units, types, and descriptions for all new metrics. Descriptions should not be in the `Add()` line, but in another data structure or constant. See [keepalive collectors](https://github.com/bosun-monitor/bosun/blob/master/cmd/scollector/collectors/keepalived_linux.go) for the constants, and the [memcached](https://github.com/bosun-monitor/bosun/blob/master/cmd/scollector/collectors/memcached_unix.go) for good patterns.

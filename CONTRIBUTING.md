# Contributing to bosun and scollector
 
We're glad you want to make a contribution!
 
Fork this repository and send in a pull request when you're finished with your changes. Link any relevant issues in too.
 
Take note of the build status of your pull request; only builds that pass will be accepted. 
Please also keep to our conventions and style so we can keep this repository as clean as possible.
 
## Licence
 
By contributing your code, you agree to licence your contribution under the terms of the [MIT licence].
 
All files are released with the MIT licence.

## Before contributing

If you're about to raise an issue because you think you've found a bug, or you'd like to request a feature, or for any other reason, please read this first.

We're tracking issues in the issue tracker on GitHub. This includes bugs and feature requests. 
It's not a support forum where we can answer questions about concrete Bosun instances or their configuration; unless, of course, they expose a bug in Bosun itself.

Please use [StackOverflow] for support questions. There exists a Slack instance where general help questions can be asked or you can reach out to the developers. You can [request an invite here][SlackInvite].

Please also see also our [Code of Conduct].

### Bug reports

A bug is a _demonstrable problem_ that is caused by the code in the repository. Good bug reports are extremely helpful - thank you!

Guidelines for bug reports:

1. **Use the GitHub issue search** &mdash; check if the issue has already been
   reported.

1. **Check if the issue has been fixed** &mdash; look for [closed issues] 
   or try to reproduce it using the latest code on the `master` branch.

A good bug report shouldn't leave others needing to chase you up for more information. 
Be sure to include the details of your environment and relevant tests that demonstrate the failure.

### Feature requests

Feature requests are very welcome. Please take a moment to make sure you consider the following:

1. **Use the GitHub search** to see if a request for that feature already exists. 
   If so, please contribute to the issue instead of raising a new one.
1. Take a moment to think about whether your idea fits with the scope and aims of the project.
1. Remember, it's up to *you* to make a strong case to convince the maintainers of the merits of this feature. 
   Please provide as much detail and context as possible, this means explaining the use case and why it is likely to be common.

## Contributing code

Thank you for contributing to Bosun :raised_hands:

Before opening a pull request, please take a moment to raise an issue describing the change or addition you'd like to 
contribute. We'll do our best to support and review contributions, but there will be some things that don't fit the 
plans we have for the project, which we therefore cannot accept. Reaching out to us on an issue helps all of us to be on 
the same page and can save valuable time.

We're using Github's labels to tag certain issues that we'd like help with. If you're a first time contributor, look out
for issues tagged with [good first issue] for some of the easier issues to get you up to speed with the code base.

Use GitHub pull requests to submit code. General submission guidelines:

1. Add/update unit tests for your new feature or fix 
1. Use `gofmt -s -w` and `go vet bosun.org/...`. See `build/validate.sh` for the full list of validation checks that will be run by Travis CI on each commit.
1. If using new third party packages, install party (`go get github.com/mjibson/party`) and run `party` in the root directory (`$GOPATH/src/bosun.org`) to vendor them and rewrite import paths.
1. Squash all non-`_third_party` commits into one. `_third_party` changes should be squashed down separately and precede any code changes which require them. This may be done as the final step before merging.
1. The commit message should indicate what folder is being changed (example: `cmd/scollector: new xyz collector` or `docs: fix typo in window function`)
1. Documentation changes should be made in the same branch as code changes using the `docs` folder. After the PR is approved is merged into the master branch it will show up on https://bosun.org (and is rendered using Github Pages).  

Unless otherwise noted, the source files are distributed under the MIT license found in the LICENSE file.

### Style guidelines

We use the golang [Go code review comments] document as the basis for our style. 
Take particular note of Error Strings, Mixed Caps, and Indent Error Flow sections. 
Also we don't have blank lines within functions.

### Bosun submission guidelines

1. If changing HTML, JS, or other static content, install esc (`go get github.com/mjibson/esc`), then run `go generate` in `cmd/bosun`.
1. [typescript][typescript] is required if changing JS files. 
   Invoke bosun with `-w` to watch for `.ts` changes and automatically run typescript. 
   We currently use typescript 2.3.1: `npm i -g typescript@2.3.1`

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

1. New scollector collectors must have units, types, and descriptions for all new metrics. 
   Descriptions should not be in the `Add()` line, but in another data structure or constant. 
   See [keepalive collectors] for the constants, and the [memcached] for good patterns.

[Code of Conduct]: CODE_OF_CONDUCT.md "Code of Conduct"
[MIT licence]: LICENSE "MIT licence"
[closed issues]: https://github.com/bosun-monitor/bosun/issues?q=is%3Aissue+is%3Aclosed "closed issues"
[StackOverflow]: https://stackoverflow.com/questions/tagged/bosun "StackOverflow"
[SlackInvite]: https://bosun.org/slackInvite
[Go code review comments]: https://github.com/golang/go/wiki/CodeReviewComments "Go code review comments"
[typescript]: https://www.npmjs.com/package/typescript
[keepalive collectors]: https://github.com/bosun-monitor/bosun/blob/master/cmd/scollector/collectors/keepalived_linux.go "keepalive collectors"
[memcached]: https://github.com/bosun-monitor/bosun/blob/master/cmd/scollector/collectors/memcached_unix.go "memcached"
[good first issue]: https://github.com/bosun-monitor/bosun/labels/good%20first%20issue "good first issue"

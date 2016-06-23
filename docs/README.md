These are the official docs for bosun, and the source code for https://bosun.org. They are in the main bosun repo so that documentation can be included with code changes in the same pr.

To publish to github pages, `go run build/docs/publish.go`. You will need a personal access token for your github account. Put it in an environment variable called `GITHUB_ACCESS_TOKEN`

For local doc development run the following in this directory:

```
gem install jekyll
gem install jekyll-redirect-from
jekyll server
```

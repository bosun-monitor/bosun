These are the official docs for bosun, and the source code for https://bosun.org. They are in the main bosun repo so that documentation can be included with code changes in the same pr.

To publish to github pages, `go run build/docs/publish.go`. You will need a personal access token for your github account. Put it in an environment variable called `GITHUB_ACCESS_TOKEN`

The best way to develop locally is to use docker. This can be run using the `docker.sh` script inside this directory. This is because it includes some github pages specific libraries that are needed for everything to render correctly.

Alternatively you can use jekyll locally (however more gems are needed to render correctly):

```
gem install jekyll
gem install jekyll-redirect-from
jekyll server
```
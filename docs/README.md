These are the official docs for bosun, and the source code for https://bosun.org. They are in the main bosun repo so that documentation can be included with code changes in the same pr.

Once changes have been committed to master they will show up on the https://bosun.org website as soon as github pages picks them up.

The best way to develop locally is to use docker. This can be run using the `docker-compose.yml` inside the `docker` 
directory. This is because it includes some github pages specific libraries that are needed for everything to render 
correctly.

Alternatively you can use jekyll locally (however more gems are needed to render correctly):

```
gem install jekyll
gem install jekyll-redirect-from
jekyll server
```

#!/bin/sh

docker run --rm -v "$PWD:/src"   -p 4000:4000   -p 35729:35729   markkimsal/jekyll-plus
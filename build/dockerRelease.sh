#!/bin/sh

rm -rf buildoutput;
mkdir buildoutput
set -e
docker run --rm \
   -v "$PWD":/go/src/bosun.org \
   -v "$PWD"/buildoutput:/output \
   -w /go/src/bosun.org \
   -e OUTPUTDIR=/output/ \
   -e GITHUB_ACCESS_TOKEN=$GITHUB_ACCESS_TOKEN \
   -e GITHUB_VERSION_SHA=$BUILD_VCS_NUMBER \
   golang:1.4.2-cross /go/src/bosun.org/build/release.sh
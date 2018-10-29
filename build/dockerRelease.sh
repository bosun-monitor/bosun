#!/bin/sh
# Must be run from $GOPATH/src/bosun.org, not the build directory.
set -e

rm -rf buildoutput
mkdir buildoutput

docker run --rm \
   -v "$PWD":/go/src/bosun.org \
   -v "$PWD"/buildoutput:/output \
   -w /go/src/bosun.org \
   -e OUTPUTDIR=/output/ \
   -e GITHUB_ACCESS_TOKEN=$GITHUB_ACCESS_TOKEN \
   golang:1.11 /go/src/bosun.org/build/release.sh

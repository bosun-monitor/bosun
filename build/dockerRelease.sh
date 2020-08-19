#!/bin/sh
# Must be run from $GOPATH/src/bosun.org, not the build directory.
set -e

rm -rf buildoutput
mkdir buildoutput

docker run --rm \
   -v "$PWD":/src/bosun.org \
   -v "$PWD"/buildoutput:/output \
   -w /src/bosun.org \
   -e OUTPUTDIR=/output/ \
   -e GITHUB_ACCESS_TOKEN=$GITHUB_ACCESS_TOKEN \
   golang:1.13 /src/bosun.org/build/release.sh /src/bosun.org

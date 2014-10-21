#!/bin/sh

set -e

$(boot2docker shellinit)
docker run --rm -v "$(pwd)":/usr/src/myapp -w /usr/src/myapp golang:cross sh build.sh

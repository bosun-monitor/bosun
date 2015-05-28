#!/bin/sh

set -e

if  type boot2docker >/dev/null 2>&1; then
	boot2docker ssh sudo ntpclient -s -h pool.ntp.org
fi

DIR=/go/src/github.com/bosun-monitor/bosun-monitor.github.io
docker run --rm -v "$(pwd)":$DIR -w $DIR golang:cross sh build.sh

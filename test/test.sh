#!/bin/sh

set -e
set -x

#docker build -t tsdb tsdb
docker build -t tsdb cdh

docker rm -f tsdb || true
docker run -d -p 127.0.0.1:4242:4242 --name tsdb tsdb

#docker build -t influx influx
#docker run -d -p 127.0.0.1:8083:8083 -p 127.0.0.1:8086:8086 --name influx influx
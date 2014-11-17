#!/bin/sh

while true; do
	statik -src=web/static -dest=web -pkg=github.com/bosun-monitor/bosun/_third_party/github.com/rakyll/statik/fs
	go run main.go -w -dev -r -q
	if [ $? != 0 ] ; then
		exit
	fi
	echo restarting
done

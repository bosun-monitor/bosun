#!/bin/sh

while true; do
	statik -src=web/static -dest=web
	go run main.go -w -dev -r -q
	if [ $? != 0 ] ; then
		exit
	fi
	echo restarting
done

#!/bin/sh

while true; do
	go run main.go -w -dev -r -q
	if [ $? != 0 ] ; then
		exit
	fi
	echo restarting
done

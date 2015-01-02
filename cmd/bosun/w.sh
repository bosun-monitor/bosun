#!/bin/sh

while true; do
	go generate
	go run main.go -w -dev -r -q
	if [ $? != 0 ] ; then
		go generate
		exit
	fi
	echo restarting
done

#!/bin/sh

while true; do
	go run main.go -w -r
	if [ $? != 0 ] ; then
		exit
	fi
	echo restarting
done

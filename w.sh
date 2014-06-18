#!/bin/sh

while true; do
	go run main.go -w
	if [ $? != 0 ] ; then
		exit
	fi
	echo restarting
done

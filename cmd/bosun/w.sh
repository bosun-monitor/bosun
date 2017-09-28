#!/bin/sh

while echo "(RE)STARTING BOSUN"; do
	go run main.go -w -de -q || exit
done

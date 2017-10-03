#!/bin/sh

while echo "(RE)STARTING BOSUN"; do
	go run main.go -w -q -dev || exit
done

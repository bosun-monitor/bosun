#!/bin/bash
echo "127.0.0.1 localhost $(hostname)" > /etc/hosts
trap "echo stopping hbase;$HBASE/bin/hbase master stop>>/var/log/hbase-stop.log 2>&1; exit" HUP INT TERM EXIT
echo "starting hbase"
$HBASE/bin/hbase master start >> /var/log/hbase-start.log 2>&1 &
while true
do
  sleep 1
done

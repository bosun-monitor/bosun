tsaf
====

Time Series Alerting Framework

# usage

`tsaf [-c=dev.conf] [-t]`

`-c` specifies the config file to use, defaults to `dev.conf`. `-t` parses the config file, validates it, and exits.

# installation

1. `export GOPATH=$HOME/go`
1. `mkdir -p $GOPATH/src/github.com/StackExchange/tsaf`
1. `cd $GOPATH/src/github.com/StackExchange/tsaf`
1. `git clone git@github.com:StackExchange/tsaf.git .`
1. `go build .`

Now you have a `tsaf` executable in that directory.

Getting HBase and OpenTSDB Up with Cloudera on CentOS
====

1. Get the java jdk from http://www.oracle.com/technetwork/java/javase/downloads/jdk7-downloads-1880260.html and install it with `rpm -i jdk-7u60-linux-x64.rpm` or whatever minor version you have
2. Add the Cloudera Repo from http://archive.cloudera.com/cm4/redhat/6/x86_64/cm/cloudera-manager.repo
3. yum install the following `yum install cloudera-manager-agent cloudera-manager-server cloudera-manager-server-db cloudera-manager-daemons lzo lzop`. (Note: From here on, we will be using cloudera's parcel system to install things like Mapreduce, Hbase, Hadopp etc)
4. start the cloudera integrated db `service cloudera-scm-server-db start`, then start the cloudera server, `cloudera-scm-server start`. The web service takes a little while to start, you can follow its progress with `tail -f /var/log/cloudera-scm-server/cloudera-scm-server.log`
5. Go to your http://localhost:7180 and login as admin / admin. Follow the cluster setup (standard edition is fine) and install hbase, zookeeper, hadoop, and mapreduce. Ensure that all host validation checks pass. In particular make sure your DNS is working with both forward and reverse lookups
6. Once you have it up, going to Hosts :: Partials inside Cloudera Manager. Go to edit settings and add the following parcel repo `http://archive.cloudera.com/gplextras/parcels/latest/`. Then go back to the Parcels page, get the HADOOP_LZO parcel, download, distribute, and activate it.
7. Poke at stuff with Cloudera Manager Home until it seems like you have a reasonably healthy cluster. Once we do, we can move on to setting up OpenTSDB. 

The OpenTSDB Setup:

1. Make a folder `mkdir /root/opentsdb`
2. `cd /root/opentsdb`
3. `git clone https://github.com/OpenTSDB/opentsdb next-$(date +%s)`
4. Get the gzip patch `wget 'https://groups.google.com/group/opentsdb/attach/1ee514e9628bbcdf/opentsdb-2.0-gzip-http-post.patch?part=4&authuser=1' -O gzip.patch` 
5. `cd next-<whatever_timestamp>`
6. Checkout the next branch `git checkout next`
6. Apply the gzip patch `patch -p1 < ../gzip.patch
7. `./build.sh`
8. `cd src`
9. `make install`
10. Create the tables (This will hang after the first table if you skipped the step in cloudera with LZO) `env COMPRESSION=LZO HBASE_HOME=/opt/cloudera/parcels/CDH-4.6.0-1.cdh4.6.0.p0.26 /root/opentsdb/next-1401309842/src/create_table.sh`. Your exact next-blah timestamp and CDH-4... version may be a little different so adjust the command accordinly.
11. Put an opentsdb.conf file in /root/opentsdb that is something like the file below
12. Put the run-opentsdb.sh in /root/opentsdb and run it. Tail the nohup log, heopfully things are working. You can visit opentsdb at your http://yourhost:4242

Example Conf:
`tsd.core.auto_create_metrics=true
tsd.core.meta.enable_realtime_ts=false
tsd.core.meta.enable_realtime_uid=false
tsd.core.meta.enable_tsuid_incrementing=false
tsd.core.meta.enable_tracking=false
tsd.core.plugin_path=
tsd.core.tree.enable_processing=false
tsd.http.cachedir=/tmp/tsd
tsd.http.request.cors_domains=
tsd.http.request.enable_chunked=true
tsd.http.request.max_chunk=33554432
tsd.http.show_stack_trace=true
tsd.http.staticroot=build/staticroot
tsd.network.async_io=true
tsd.network.bind=0.0.0.0
tsd.network.keep_alive=true
tsd.network.port=4242
tsd.network.reuse_address=true
tsd.network.tcp_no_delay=true
tsd.network.worker_threads=
tsd.rtpublisher.enable=false
tsd.rtpublisher.plugin=
tsd.search.enable=false
tsd.search.plugin=
tsd.stats.canonical=false
tsd.storage.enable_compaction=true
tsd.storage.flush_interval=1000
tsd.storage.hbase.data_table=tsdb
tsd.storage.hbase.meta_table=tsdb-meta
tsd.storage.hbase.tree_table=tsdb-tree
tsd.storage.hbase.uid_table=tsdb-uid
tsd.storage.hbase.zk_basedir=/hbase
tsd.storage.hbase.zk_quorum=or-devtsdb01.ds.stackexchange.com
tsd.storage.fix_duplicates=true`

The run-opentsdb.sh scipt:
`#!/bin/bash

CONFIG_FILE=/root/opentsdb/opentsdb.conf

tsdtmp=${TMPDIR-'/tmp'}/tsd
mkdir -p "$tsdtmp"
nohup 2>&1 tsdb tsd --port=4242 --staticroot=/usr/local/share/opentsdb/static --cachedir="$tsdtmp" --config $CONFIG_FILE &
echo To see log: tail -f $(pwd)/nohup.out`




FROM debian:wheezy

RUN apt-get update && apt-get install -y \
	gnuplot \
	make \
	openjdk-7-jre-headless \
	supervisor \
	--no-install-recommends \
	&& rm -rf /var/lib/apt/lists/*

RUN mkdir -p /tsdb /hbase /bosun /scollector /data

ENV TSDB /tsdb
ENV HBASE /hbase/hbase
ENV HBASE_HOME $HBASE
ENV JAVA_HOME /usr/lib/jvm/java-7-openjdk-amd64

COPY bosun.conf /data/
COPY hbase-site.xml $HBASE/conf/
COPY start_hbase.sh /hbase/
COPY opentsdb.conf /tsdb/
COPY start_opentsdb.sh /tsdb/
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY create_tsdb_tables.sh /tsdb/

COPY tmp /

EXPOSE 8070 4242 9565
CMD ["/usr/bin/supervisord"]

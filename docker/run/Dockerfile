FROM debian:wheezy

RUN apt-get update && apt-get install -y \
	gnuplot \
	make \
	openjdk-6-jre-headless \
	supervisor \
	--no-install-recommends \
	&& rm -rf /var/lib/apt/lists/*

RUN mkdir -p /tsdb /hbase /bosun /scollector

ENV TSDB /tsdb
ENV HBASEVER 0.98.7-hadoop2
ENV HBASE /hbase/hbase-$HBASEVER
ENV HBASE_HOME $HBASE
ENV JAVA_HOME /usr/lib/jvm/java-6-openjdk-amd64

COPY tmp /

COPY bosun.conf /bosun/
COPY hbase-site.xml $HBASE/conf/
COPY start_hbase.sh /hbase/
COPY start_opentsdb.sh /tsdb/
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY create_tsdb_tables.sh /tsdb/

EXPOSE 8070 4242
CMD ["/usr/bin/supervisord"]

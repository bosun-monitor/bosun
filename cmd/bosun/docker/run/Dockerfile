FROM debian:wheezy

RUN apt-get update && apt-get install -y \
	gnuplot \
	make \
	openjdk-7-jre-headless \
	supervisor \
	nano \
	vim \
	less \
	wget \
	--no-install-recommends \
	&& rm -rf /var/lib/apt/lists/*

RUN mkdir -p /tsdb /hbase /bosun /scollector /data

ENV TSDB /tsdb
ENV HBASE /hbase/hbase
ENV HBASE_HOME $HBASE
ENV JAVA_HOME /usr/lib/jvm/java-7-openjdk-amd64
ENV TSDBRELAY_OPTS -b localhost:8070 -t localhost:4242 -l 0.0.0.0:5252 -redis localhost:9565
ENV TERM dumb

COPY bosun.toml /data/
COPY rules.conf /data/
COPY scollector.toml /data/
COPY hbase-site.xml $HBASE/conf/
COPY start_hbase.sh /hbase/
COPY opentsdb.conf /tsdb/
COPY start_opentsdb.sh /tsdb/
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY create_tsdb_tables.sh /tsdb/

COPY tmp /

EXPOSE 8070 4242 5252 9565 16010
VOLUME ["/data", "/var/log", "/tmp", "/tsdb"]
CMD ["/usr/bin/supervisord"]

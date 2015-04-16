FROM debian:wheezy

RUN apt-get update && apt-get install -y \
	automake \
	curl \
	git \
	gnuplot \
	make \
	openjdk-7-jdk \
	python \
	supervisor \
	--no-install-recommends \
	&& rm -rf /var/lib/apt/lists/*

ENV TSDB /tsdb
RUN git clone -b next --single-branch --depth 1 git://github.com/OpenTSDB/opentsdb.git $TSDB && \
	cd $TSDB && bash ./build.sh

ENV HBASEVER 1.0.0
ENV HBASE /hbase/hbase
ENV HBASE_HOME $HBASE
ENV JAVA_HOME /usr/lib/jvm/java-7-openjdk-amd64

RUN mkdir -p /hbase \
	&& curl -SL http://apache.org/dist/hbase/stable/hbase-$HBASEVER-bin.tar.gz \
	| tar -xzC /hbase \
	&& mv /hbase/hbase-$HBASEVER $HBASE

RUN mkdir -p /data

COPY hbase-site.xml $HBASE/conf/
COPY start_hbase.sh /hbase/
COPY opentsdb.conf /tsdb/
COPY start_opentsdb.sh /tsdb/
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY create_tsdb_tables.sh /tsdb/

EXPOSE 4242
CMD ["/usr/bin/supervisord"]

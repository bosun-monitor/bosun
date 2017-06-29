FROM debian:wheezy AS build

RUN apt-get update && apt-get install -y \
    automake \
    curl \
    git \
    make \
    openjdk-7-jdk \
    python \
    --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

ENV TSDB /tsdb
RUN git clone --single-branch --branch v2.2.1 --depth 1 git://github.com/OpenTSDB/opentsdb.git $TSDB && \
    cd $TSDB && bash ./build.sh

ENV GOPATH /go
ENV HBASEVER 1.2.4
ENV HBASE /hbase/hbase-$HBASEVER
ENV JAVA_HOME /usr/lib/jvm/java-7-openjdk-amd64

RUN mkdir -p /hbase \
    && curl -SL http://archive.apache.org/dist/hbase/$HBASEVER/hbase-$HBASEVER-bin.tar.gz \
    | tar -xzC /hbase \
    && mv /hbase/hbase-$HBASEVER /hbase/hbase

RUN curl -SL https://storage.googleapis.com/golang/go1.7.linux-amd64.tar.gz \
    | tar -xzC /usr/local

COPY . $GOPATH/src/bosun.org/
WORKDIR $GOPATH/src/bosun.org

ENV PATH $PATH:/usr/local/go/bin:$GOPATH/bin

RUN go run build/build.go \
    && bosun -version \
    && scollector -version \
    && tsdbrelay -version


## Final container image
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
ENV TERM xterm

COPY cmd/bosun/docker/bosun.conf /data/
COPY cmd/bosun/docker/bosunrules.conf /data/
COPY cmd/bosun/docker/bosun.toml /data/
COPY cmd/bosun/docker/scollector.toml /data/
COPY cmd/bosun/docker/hbase-site.xml $HBASE/conf/
COPY cmd/bosun/docker/start_hbase.sh /hbase/
COPY cmd/bosun/docker/opentsdb.conf /tsdb/
COPY cmd/bosun/docker/start_opentsdb.sh /tsdb/
COPY cmd/bosun/docker/supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY cmd/bosun/docker/create_tsdb_tables.sh /tsdb/

COPY --from=build /go/bin/bosun /bosun/
COPY --from=build /go/bin/scollector /scollector/
COPY --from=build /go/bin/tsdbrelay /tsdbrelay/
COPY --from=build /hbase /
COPY --from=build /tsdb /

EXPOSE 8070 4242 5252 9565 16010
VOLUME ["/data", "/var/log", "/tmp", "/tsdb"]
CMD ["/usr/bin/supervisord"]
FROM golang:1.13 AS bosun_builder

WORKDIR /bosun
COPY . /bosun

RUN make bosun scollector tsdbrelay

FROM alpine:latest

ENV HBASE_VERSION 2.2.4
ENV GNUPLOT_VERSION 5.2.8

ENV BOSUN_ROOT "cmd/bosun/docker"

ENV PACKAGES "ca-certificates rsyslog bash openjdk8 curl libgd libpng libjpeg libwebp libjpeg-turbo cairo pango jruby lua supervisor asciidoctor"
ENV BUILD_PACKAGES "build-base autoconf make automake git python cairo-dev pango-dev gd-dev lua-dev readline-dev libpng-dev libjpeg-turbo-dev libwebp-dev"

ENV DATA_DIR /data
ENV TSDB_DIR /tsdb
ENV HBASE_DIR /hbase
ENV HBASE_HOME ${HBASE_DIR}

ENV TERM xterm
ENV TSDBRELAY_OPTS -b localhost:8070 -t localhost:4242 -l 0.0.0.0:5252 -redis localhost:9565
ENV JAVA_HOME=/usr/lib/jvm/java-1.8-openjdk
ENV PATH="$JAVA_HOME/bin:${PATH}"


# Install dependencies
RUN apk --update add apk-tools \
    && apk add ${PACKAGES} ${BUILD_PACKAGES}
WORKDIR /tmp/gnuplot
RUN cd /tmp \
    && curl -L -o - https://downloads.sourceforge.net/project/gnuplot/gnuplot/${GNUPLOT_VERSION}/gnuplot-${GNUPLOT_VERSION}.tar.gz | tar -xzf - --strip-components 1 \
    && ./configure \
    && make install \
    && rm -rf /tmp/gnuplot


# Install HBase
WORKDIR /hbase
RUN curl -L -o - http://archive.apache.org/dist/hbase/${HBASE_VERSION}/hbase-${HBASE_VERSION}-bin.tar.gz | tar -xzf - --strip-components 1

COPY ${BOSUN_ROOT}/hbase-site.xml /hbase/conf/
COPY ${BOSUN_ROOT}/start_hbase.sh /hbase/


# Install OpenTSDB
RUN cd /tmp \
    && curl -OL https://github.com/OpenTSDB/opentsdb/archive/v2.4.0.zip \
    && unzip v2.4.0.zip \
    && mv opentsdb-2.4.0 /tsdb \
    && rm /tmp/v2.4.0.zip \
    && cd /tsdb \
    && find . -name '*.mk' | xargs sed -i s#http://central.maven.org#https://repo1.maven.org#g \
    && find . -name '*.mk' | xargs sed -i s#http://repo1.maven.org#https://repo1.maven.org#g \
    && ./build.sh

COPY ${BOSUN_ROOT}/tsdb/opentsdb.conf ${TSDB_DIR}
COPY ${BOSUN_ROOT}/tsdb/start_opentsdb.sh ${TSDB_DIR}
COPY ${BOSUN_ROOT}/tsdb/create_tsdb_tables.sh ${TSDB_DIR}


# Copy Bosun from the build image
WORKDIR /bosun
RUN mkdir /scollector /tsdbrelay
COPY --from=bosun_builder /bosun/bosun /bosun
COPY --from=bosun_builder /bosun/scollector /scollector
COPY --from=bosun_builder /bosun/tsdbrelay /tsdbrelay

# Copy Bosun config
COPY ${BOSUN_ROOT}/data/bosun.conf ${DATA_DIR}/
COPY ${BOSUN_ROOT}/data/bosunrules.conf ${DATA_DIR}/
COPY ${BOSUN_ROOT}/data/bosun.toml ${DATA_DIR}/
COPY ${BOSUN_ROOT}/data/scollector.toml ${DATA_DIR}/

# Copy supervisor config
COPY ${BOSUN_ROOT}/data/supervisord.conf ${DATA_DIR}/supervisord.conf

EXPOSE 4242 16010 16070 8070 5252 9565
VOLUME ["${DATA_DIR}", "/var/log", "${TSDB_DIR}"]
CMD ["sh", "-c", "/usr/bin/supervisord -c ${DATA_DIR}/supervisord.conf"]

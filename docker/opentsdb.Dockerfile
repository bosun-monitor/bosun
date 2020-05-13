FROM alpine:latest

ENV HBASE_VERSION 2.2.4
ENV GNUPLOT_VERSION 5.2.8

ENV PACKAGES "ca-certificates rsyslog bash openjdk8 curl libgd libpng libjpeg libwebp libjpeg-turbo cairo pango jruby lua supervisor asciidoctor"
ENV BUILD_PACKAGES "build-base autoconf make automake git python cairo-dev pango-dev gd-dev lua-dev readline-dev libpng-dev libjpeg-turbo-dev libwebp-dev"

ENV DATA_DIR /data
ENV TSDB_DIR /tsdb
ENV HBASE_DIR /hbase
ENV HBASE_HOME ${HBASE_DIR}
ENV DOCKER_ROOT "docker"

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
WORKDIR ${HBASE_DIR}
RUN curl -L -o - http://archive.apache.org/dist/hbase/${HBASE_VERSION}/hbase-${HBASE_VERSION}-bin.tar.gz | tar -xzf - --strip-components 1

COPY ${DOCKER_ROOT}/hbase-site.xml ${HBASE_DIR}/conf/
COPY ${DOCKER_ROOT}/start_hbase.sh ${HBASE_DIR}/


# Install OpenTSDB
RUN cd /tmp \
    && curl -OL https://github.com/OpenTSDB/opentsdb/archive/v2.4.0.zip \
    && unzip v2.4.0.zip \
    && mv opentsdb-2.4.0 ${TSDB_DIR} \
    && rm /tmp/v2.4.0.zip \
    && cd ${TSDB_DIR} \
    && find . -name '*.mk' | xargs sed -i s#http://central.maven.org#https://repo1.maven.org#g \
    && find . -name '*.mk' | xargs sed -i s#http://repo1.maven.org#https://repo1.maven.org#g \
    && ./build.sh

COPY ${DOCKER_ROOT}/tsdb/opentsdb.conf ${TSDB_DIR}
COPY ${DOCKER_ROOT}/tsdb/start_opentsdb.sh ${TSDB_DIR}
COPY ${DOCKER_ROOT}/tsdb/create_tsdb_tables.sh ${TSDB_DIR}

# Copy supervisor config
COPY ${DOCKER_ROOT}/data/supervisord-opentsdb.conf ${DATA_DIR}/

EXPOSE 4242
VOLUME ["${DATA_DIR}", "/var/log", "${TSDB_DIR}"]
CMD ["sh", "-c", "/usr/bin/supervisord -c ${DATA_DIR}/supervisord-opentsdb.conf"]
